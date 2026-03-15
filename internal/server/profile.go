package server

import (
	"christjesus/pkg/types"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (s *Service) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	userEmail, _ := ctx.Value(contextKeyEmail).(string)
	userName, _ := ctx.Value(contextKeyUserName).(string)
	if strings.TrimSpace(userName) == "" {
		userName = displayNameFromEmail(userEmail)
	}

	userType := ""
	user, err := s.userRepo.User(ctx, userID)
	if err != nil {
		if !errors.Is(err, types.ErrUserNotFound) {
			s.logger.WithError(err).WithField("user_id", userID).Error("failed to fetch user for profile")
			s.internalServerError(w)
			return
		}
	} else if user.UserType != nil {
		userType = strings.TrimSpace(*user.UserType)
	}

	myNeeds := make([]*types.Need, 0)
	needSummaries := make([]types.ProfileNeedSummary, 0)
	donationSummaries := make([]types.ProfileDonationSummary, 0)
	if userType == string(types.UserTypeNeed) {
		needs, err := s.needsRepo.NeedsByUser(ctx, userID)
		if err != nil {
			if !errors.Is(err, types.ErrNeedNotFound) {
				s.logger.WithError(err).WithField("user_id", userID).Error("failed to fetch needs for profile")
				s.internalServerError(w)
				return
			}
		} else {
			myNeeds = needs

			needIDs := make([]string, 0, len(needs))
			for _, need := range needs {
				needIDs = append(needIDs, need.ID)
			}

			allAssignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedIDs(ctx, needIDs)
			if err != nil {
				s.logger.WithError(err).WithField("user_id", userID).Error("failed to batch fetch need category assignments for profile")
				s.internalServerError(w)
				return
			}

			primaryCategoryIDByNeed := make(map[string]string, len(needs))
			uniqueCategoryIDs := make(map[string]bool)
			for _, a := range allAssignments {
				if !a.IsPrimary {
					continue
				}
				if _, exists := primaryCategoryIDByNeed[a.NeedID]; exists {
					continue
				}
				primaryCategoryIDByNeed[a.NeedID] = a.CategoryID
				uniqueCategoryIDs[a.CategoryID] = true
			}

			categoryIDs := make([]string, 0, len(uniqueCategoryIDs))
			for id := range uniqueCategoryIDs {
				categoryIDs = append(categoryIDs, id)
			}

			categoryNameByID := make(map[string]string, len(categoryIDs))
			if len(categoryIDs) > 0 {
				categories, err := s.categoryRepo.CategoriesByIDs(ctx, categoryIDs)
				if err != nil {
					s.logger.WithError(err).Error("failed to batch fetch categories for profile")
					s.internalServerError(w)
					return
				}
				for _, cat := range categories {
					if cat != nil {
						categoryNameByID[cat.ID] = cat.Name
					}
				}
			}

			for _, need := range needs {
				reviewPortalHref := ""
				if need.Status != types.NeedStatusDraft {
					reviewPortalHref = s.route(RouteProfileNeedReview, map[string]string{"needID": need.ID})
				}

				primaryCategoryName := "Uncategorized"
				if catID, ok := primaryCategoryIDByNeed[need.ID]; ok {
					if name, ok := categoryNameByID[catID]; ok {
						primaryCategoryName = name
					}
				}

				needSummaries = append(needSummaries, types.ProfileNeedSummary{
					NeedID:              need.ID,
					PrimaryCategoryName: primaryCategoryName,
					RequestedAmount:     formatUSDFromCents(need.AmountNeededCents),
					CurrentStep:         formatNeedStepLabel(need.CurrentStep),
					Status:              need.Status,
					CanDelete:           need.Status == types.NeedStatusDraft,
					NeedsAttention:      need.Status == types.NeedStatusChangesRequested || need.Status == types.NeedStatusRejected,
					ReviewPortalHref:    reviewPortalHref,
				})
			}
		}
	}

	if userType == string(types.UserTypeDonor) {
		intents, err := s.donationIntentRepo.DonationIntentsByDonorUserID(ctx, userID)
		if err != nil {
			s.logger.WithError(err).WithField("user_id", userID).Error("failed to fetch donation intents for profile")
			s.internalServerError(w)
			return
		}

		distinctNeedIDs := make([]string, 0, len(intents))
		seenNeedIDs := make(map[string]bool)
		for _, intent := range intents {
			if intent == nil {
				continue
			}
			needID := strings.TrimSpace(intent.NeedID)
			if needID == "" || seenNeedIDs[needID] {
				continue
			}
			seenNeedIDs[needID] = true
			distinctNeedIDs = append(distinctNeedIDs, needID)
		}

		needLabelByID := make(map[string]string, len(distinctNeedIDs))
		needsByID := make(map[string]*types.Need)
		needs, err := s.needsRepo.NeedsByIDs(ctx, distinctNeedIDs)
		if err != nil {
			s.logger.WithError(err).WithField("user_id", userID).Error("failed to batch fetch needs for donor profile")
			s.internalServerError(w)
			return
		}
		for _, need := range needs {
			if need == nil {
				continue
			}
			needsByID[need.ID] = need
		}
		for _, needID := range distinctNeedIDs {
			needLabel := "Need request"
			if need, ok := needsByID[needID]; ok {
				shortDescription := strings.TrimSpace(derefString(need.ShortDescription))
				if shortDescription != "" {
					needLabel = shortDescription
				}
			}
			needLabelByID[needID] = needLabel
		}

		for _, intent := range intents {
			if intent == nil {
				continue
			}

			needID := strings.TrimSpace(intent.NeedID)
			needLabel := needLabelByID[needID]
			if strings.TrimSpace(needLabel) == "" {
				needLabel = "Need request"
			}

			isFinalized := strings.TrimSpace(strings.ToLower(intent.PaymentStatus)) == types.DonationPaymentStatusFinalized

			donationSummaries = append(donationSummaries, types.ProfileDonationSummary{
				IntentID:    intent.ID,
				NeedID:      needID,
				NeedLabel:   needLabel,
				Amount:      formatUSDFromCents(intent.AmountCents),
				Status:      formatDonationStatus(intent.PaymentStatus),
				IsFinalized: isFinalized,
				IsAnonymous: intent.IsAnonymous,
				CreatedAt:   intent.CreatedAt.Format("Jan 2, 2006"),
			})
		}
	}

	data := &types.ProfilePageData{
		BasePageData:      types.BasePageData{Title: "My Profile"},
		UserID:            userID,
		UserEmail:         userEmail,
		WelcomeName:       userName,
		UserType:          userType,
		Notice:            strings.TrimSpace(r.URL.Query().Get("notice")),
		Error:             strings.TrimSpace(r.URL.Query().Get("error")),
		SidebarItems:      buildProfileSidebar(userType),
		Needs:             myNeeds,
		NeedSummaries:     needSummaries,
		DonationSummaries: donationSummaries,
		HasNeeds:          len(myNeeds) > 0,
		HasDonations:      len(donationSummaries) > 0,
	}

	err = s.renderTemplate(w, r, "page.profile", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render profile page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostProfileNeedDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	needID := strings.TrimSpace(r.PathValue("needID"))
	if needID == "" {
		s.redirectProfileWithError(w, r, "Need not found.")
		return
	}

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		if errors.Is(err, types.ErrNeedNotFound) {
			s.redirectProfileWithError(w, r, "Need not found.")
			return
		}
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to load need for profile delete")
		s.internalServerError(w)
		return
	}

	if need.UserID != userID {
		s.redirectProfileWithError(w, r, "You do not have permission to delete that need.")
		return
	}

	if need.Status != types.NeedStatusDraft {
		s.redirectProfileWithError(w, r, "Only draft needs can be deleted.")
		return
	}

	docs, err := s.documentRepo.DocumentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to load need documents before profile delete")
		s.internalServerError(w)
		return
	}

	for _, doc := range docs {
		storageKey := strings.TrimSpace(doc.StorageKey)
		if storageKey == "" {
			continue
		}

		_, err = s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.config.S3BucketName),
			Key:    aws.String(storageKey),
		})
		if err != nil {
			s.logger.WithError(err).
				WithField("need_id", needID).
				WithField("document_id", doc.ID).
				WithField("storage_key", storageKey).
				Error("failed to delete need document from S3 during profile delete")
			s.redirectProfileWithError(w, r, "Could not delete uploaded files from storage. Please try again.")
			return
		}
	}

	err = s.needsRepo.DeleteNeed(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to delete draft need from profile")
		s.internalServerError(w)
		return
	}

	s.redirectProfileWithNotice(w, r, "Draft need deleted.")
}

func (s *Service) redirectProfileWithNotice(w http.ResponseWriter, r *http.Request, notice string) {
	v := url.Values{}
	v.Set("notice", notice)
	http.Redirect(w, r, s.routeWithQuery(RouteProfile, nil, v), http.StatusSeeOther)
}

func (s *Service) redirectProfileWithError(w http.ResponseWriter, r *http.Request, msg string) {
	v := url.Values{}
	v.Set("error", msg)
	http.Redirect(w, r, s.routeWithQuery(RouteProfile, nil, v), http.StatusSeeOther)
}

func formatUSDFromCents(cents int) string {
	dollars := float64(cents) / 100.0
	return fmt.Sprintf("$%.2f", dollars)
}

func formatNeedStepLabel(step types.NeedStep) string {
	switch step {
	case types.NeedStepWelcome:
		return "Welcome"
	case types.NeedStepLocation:
		return "Location"
	case types.NeedStepCategories:
		return "Categories"
	case types.NeedStepStory:
		return "Need Story"
	case types.NeedStepDocuments:
		return "Documents"
	case types.NeedStepReview:
		return "Review"
	case types.NeedStepComplete:
		return "Complete"
	default:
		return "Unknown"
	}
}

func formatDonationStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case types.DonationPaymentStatusFinalized:
		return "Finalized"
	case types.DonationPaymentStatusPending:
		return "Pending"
	case types.DonationPaymentStatusFailed:
		return "Failed"
	case types.DonationPaymentStatusCanceled:
		return "Canceled"
	default:
		return "Unknown"
	}
}

func buildProfileSidebar(userType string) []types.ProfileNavItem {
	items := []types.ProfileNavItem{
		{Label: "Profile Overview", Href: "#overview", Active: true, Section: "overview", ShowItem: true},
		{Label: "My Needs", Href: "#my-needs", Active: false, Section: "my-needs", ShowItem: userType == string(types.UserTypeNeed)},
		{Label: "Need Status", Href: "#need-status", Active: false, Section: "need-status", ShowItem: userType == string(types.UserTypeNeed)},
		{Label: "Donation History", Href: "#donations", Active: false, Section: "donations", ShowItem: userType == string(types.UserTypeDonor)},
	}

	filtered := make([]types.ProfileNavItem, 0, len(items))
	for _, item := range items {
		if item.ShowItem {
			filtered = append(filtered, item)
		}
	}

	return filtered
}
