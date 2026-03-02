package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"christjesus/pkg/types"
)

func (s *Service) handleHome(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	_ = sampleNeeds()

	data := &types.HomePageData{
		BasePageData: types.BasePageData{Title: ""},
		Notice:       r.URL.Query().Get("notice"),
		Error:        r.URL.Query().Get("error"),
		// FeaturedNeed: needs[0], // First need is featured
		// Needs:        needs[1:], // Rest are in the grid
		Categories: sampleCategories(),
		Stats:      getStats(),
		Steps:      getSteps(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "page.home", data); err != nil {
		s.logger.WithError(err).Error("failed to render home page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Service) redirectWithNotice(w http.ResponseWriter, r *http.Request, notice string) {
	v := url.Values{}
	v.Set("notice", notice)
	http.Redirect(w, r, "/?"+v.Encode(), http.StatusSeeOther)
}

func (s *Service) redirectWithError(w http.ResponseWriter, r *http.Request, msg string) {
	v := url.Values{}
	v.Set("error", msg)
	http.Redirect(w, r, "/?"+v.Encode(), http.StatusSeeOther)
}

func required(v string) bool {
	return strings.TrimSpace(v) != ""
}

func (s *Service) internalServerError(w http.ResponseWriter) {
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func (s *Service) handleBrowse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	needs, err := s.needsRepo.BrowseNeeds(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch browse needs")
		s.internalServerError(w)
		return
	}

	userNameCache := make(map[string]string)
	userAddressCache := make(map[string]*types.UserAddress)
	categoryNameCache := make(map[string]string)
	cards := make([]*types.BrowseNeedCard, 0, len(needs))
	for _, need := range needs {
		ownerName := "Anonymous"
		if cached, ok := userNameCache[need.UserID]; ok {
			ownerName = cached
		} else {
			user, err := s.userRepo.User(ctx, need.UserID)
			if err == nil {
				ownerName = userDisplayName(user)
			} else if !errors.Is(err, types.ErrUserNotFound) {
				s.logger.WithError(err).WithField("user_id", need.UserID).Warn("failed to fetch user for browse need")
			}
			userNameCache[need.UserID] = ownerName
		}

		address := userAddressCache[need.UserID]
		if address == nil {
			if need.UserAddressID != nil && strings.TrimSpace(*need.UserAddressID) != "" {
				selectedAddress, err := s.userAddressRepo.ByIDAndUserID(ctx, strings.TrimSpace(*need.UserAddressID), need.UserID)
				if err != nil {
					s.logger.WithError(err).WithField("need_id", need.ID).Warn("failed to fetch selected address for browse need")
				} else {
					address = selectedAddress
				}
			}

			if address == nil {
				primaryAddress, err := s.userAddressRepo.PrimaryByUserID(ctx, need.UserID)
				if err != nil {
					s.logger.WithError(err).WithField("user_id", need.UserID).Warn("failed to fetch primary address for browse need")
				} else {
					address = primaryAddress
				}
			}

			userAddressCache[need.UserID] = address
		}

		cityState := browseCityState(address)

		primaryCategory := "General Need"
		assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID(ctx, need.ID)
		if err != nil {
			s.logger.WithError(err).WithField("need_id", need.ID).Warn("failed to fetch need category assignments for browse card")
		} else {
			for _, assignment := range assignments {
				if !assignment.IsPrimary {
					continue
				}

				if cachedName, ok := categoryNameCache[assignment.CategoryID]; ok {
					primaryCategory = cachedName
					break
				}

				category, err := s.categoryRepo.CategoryByID(ctx, assignment.CategoryID)
				if err != nil {
					s.logger.WithError(err).WithField("category_id", assignment.CategoryID).Warn("failed to fetch primary category for browse card")
					break
				}
				if category != nil {
					primaryCategory = category.Name
					categoryNameCache[assignment.CategoryID] = category.Name
				}
				break
			}
		}

		urgencyLabel, urgencyDotClass := browseUrgency(need.Status, need.AmountNeededCents, need.AmountRaisedCents)

		verificationLabel := "Story Shared"
		if need.VerifiedAt != nil {
			verificationLabel = "Personally Verified"
		}

		cards = append(cards, &types.BrowseNeedCard{
			ID:                need.ID,
			OwnerName:         ownerName,
			CityState:         cityState,
			UrgencyLabel:      urgencyLabel,
			UrgencyDotClass:   urgencyDotClass,
			PrimaryCategory:   primaryCategory,
			VerificationLabel: verificationLabel,
			ShortDescription:  need.ShortDescription,
			Status:            need.Status,
			AmountNeededCents: need.AmountNeededCents,
			AmountRaisedCents: need.AmountRaisedCents,
		})
	}

	data := &types.BrowsePageData{
		BasePageData: types.BasePageData{Title: "Browse Needs"},
		Needs:        cards,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "page.browse", data); err != nil {
		s.logger.WithError(err).Error("failed to render browse page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleNeedDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("id")
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		if errors.Is(err, types.ErrNeedNotFound) {
			http.NotFound(w, r)
			return
		}
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need detail")
		s.internalServerError(w)
		return
	}

	data := &types.NeedDetailPageData{
		BasePageData: types.BasePageData{},
		// Title: need.Name + " - Need Details",
		Need: need,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "page.need-detail", data); err != nil {
		s.logger.WithError(err).Error("failed to render need detail page")
		s.internalServerError(w)
		return
	}
}

func sampleNeeds() []*types.Need {
	return []*types.Need{}
}

func sampleCategories() []types.CategoryData {
	return []types.CategoryData{
		{Name: "Unhoused", Slug: "unhoused", Count: 18, Icon: "home"},
		{Name: "Unbanked", Slug: "unbanked", Count: 9, Icon: "wallet"},
		{Name: "Malnourished", Slug: "malnourished", Count: 12, Icon: "utensils"},
		{Name: "Health Condition", Slug: "health-condition", Count: 15, Icon: "heart-pulse"},
		{Name: "Unemployment", Slug: "unemployment", Count: 22, Icon: "briefcase"},
		{Name: "Utility & Basic Needs", Slug: "utility-basic-needs", Count: 14, Icon: "lightbulb"},
		{Name: "Family & Children", Slug: "family-children", Count: 11, Icon: "users"},
		{Name: "Legal Documentation", Slug: "legal-documentation", Count: 7, Icon: "file-text"},
	}
}

func getStats() types.StatsData {
	return types.StatsData{
		TotalRaised:  7824000, // $78,240
		NeedsFunded:  214,
		LivesChanged: 389,
	}
}

func getSteps() []types.StepData {
	return []types.StepData{
		{
			Number:      1,
			Title:       "Share your verified need",
			Description: "Complete a simple form and connect with our verification team to share your story.",
		},
		{
			Number:      2,
			Title:       "Connect with sponsors & organizations",
			Description: "We match your need with caring individuals and local organizations ready to help.",
		},
		{
			Number:      3,
			Title:       "Receive support & transform",
			Description: "Get the assistance you need and join our community of hope and transformation.",
		},
	}
}

func userDisplayName(user *types.User) string {
	if user == nil {
		return "Anonymous"
	}

	given := strings.TrimSpace(strings.TrimSpace(func() string {
		if user.GivenName == nil {
			return ""
		}
		return *user.GivenName
	}()))
	family := strings.TrimSpace(strings.TrimSpace(func() string {
		if user.FamilyName == nil {
			return ""
		}
		return *user.FamilyName
	}()))

	full := strings.TrimSpace(strings.Join([]string{given, family}, " "))
	if full != "" {
		return full
	}

	if user.Email != nil {
		email := strings.TrimSpace(*user.Email)
		if email != "" {
			if at := strings.Index(email, "@"); at > 0 {
				return email[:at]
			}
			return email
		}
	}

	return "Anonymous"
}

func browseCityState(address *types.UserAddress) string {
	city := "N/A"
	state := "N/A"

	if address != nil {
		if address.City != nil {
			trimmed := strings.TrimSpace(*address.City)
			if trimmed != "" {
				city = trimmed
			}
		}
		if address.State != nil {
			trimmed := strings.TrimSpace(*address.State)
			if trimmed != "" {
				state = strings.ToUpper(trimmed)
			}
		}
	}

	if city == "N/A" || state == "N/A" {
		return "N/A"
	}

	return city + ", " + state
}

func browseUrgency(status types.NeedStatus, amountNeededCents, amountRaisedCents int) (string, string) {
	if status == types.NeedStatusSubmitted || status == types.NeedStatusUnderReview {
		return "URGENT", "bg-[color:var(--cj-error)]"
	}

	if amountNeededCents <= 0 {
		return "LOW", "bg-[color:var(--cj-success)]"
	}

	percent := (amountRaisedCents * 100) / amountNeededCents
	switch {
	case percent < 35:
		return "HIGH", "bg-[color:var(--cj-error)]"
	case percent < 70:
		return "MEDIUM", "bg-[color:var(--cj-warning)]"
	default:
		return "LOW", "bg-[color:var(--cj-success)]"
	}
}
