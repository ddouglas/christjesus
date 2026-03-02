package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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
	filters := parseBrowseFilters(r.URL.Query())

	if isHXRequest(r) {
		data, err := s.buildBrowseResultsPageData(ctx, filters)
		if err != nil {
			s.logger.WithError(err).Error("failed to build browse results")
			s.internalServerError(w)
			return
		}

		data.BasePageData = types.BasePageData{Title: "Browse Needs"}
		data.LoadResultsOnRender = false
		data.ShowResultsSkeletons = false

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.renderTemplate(w, r, "component.browse-results", data); err != nil {
			s.logger.WithError(err).Error("failed to render browse results partial")
			s.internalServerError(w)
			return
		}
		return
	}

	categories, err := s.categoryRepo.Categories(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch category options for browse filters")
		s.internalServerError(w)
		return
	}

	cityOptions := []string{}
	if filters.City != "" {
		cityOptions = append(cityOptions, filters.City)
	}

	data := &types.BrowsePageData{
		BasePageData:         types.BasePageData{Title: "Browse Needs"},
		Categories:           categories,
		Cities:               cityOptions,
		Filters:              filters,
		LoadResultsOnRender:  true,
		ShowResultsSkeletons: true,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "page.browse", data); err != nil {
		s.logger.WithError(err).Error("failed to render browse page")
		s.internalServerError(w)
		return
	}
}

func parseBrowseFilters(query url.Values) types.BrowseFilters {
	fundingMax := 100
	if raw := strings.TrimSpace(query.Get("funding_max")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			if parsed < 0 {
				parsed = 0
			}
			if parsed > 100 {
				parsed = 100
			}
			fundingMax = parsed
		}
	}

	selectedCategories := make(map[string]bool)
	for _, categoryID := range query["category"] {
		trimmed := strings.TrimSpace(categoryID)
		if trimmed != "" {
			selectedCategories[trimmed] = true
		}
	}

	selectedVerification := make(map[string]bool)
	for _, verificationID := range query["verification"] {
		trimmed := strings.TrimSpace(verificationID)
		if trimmed != "" {
			selectedVerification[trimmed] = true
		}
	}

	return types.BrowseFilters{
		Search:          strings.TrimSpace(query.Get("search")),
		City:            strings.TrimSpace(query.Get("city")),
		CategoryIDs:     selectedCategories,
		VerificationIDs: selectedVerification,
		Urgency:         strings.TrimSpace(query.Get("urgency")),
		FundingMax:      fundingMax,
		ViewMode:        normalizeBrowseViewMode(query.Get("view")),
		SortBy:          normalizeBrowseSortBy(query.Get("sort")),
	}
}

func normalizeBrowseViewMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "list":
		return "list"
	default:
		return "grid"
	}
}

func normalizeBrowseSortBy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "newest", "closest", "nearest", "urgency":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "urgency"
	}
}

func isHXRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}

func (s *Service) buildBrowseResultsPageData(ctx context.Context, filters types.BrowseFilters) (*types.BrowsePageData, error) {
	type categoryFilterOption struct {
		ID   string
		Name string
	}

	searchLower := strings.ToLower(filters.Search)

	needs, err := s.needsRepo.BrowseNeeds(ctx)
	if err != nil {
		return nil, err
	}

	categories, err := s.categoryRepo.Categories(ctx)
	if err != nil {
		return nil, err
	}
	s.logger.WithField("db_category_count", len(categories)).Info("browse: loaded category options from DB")

	userNameCache := make(map[string]string)
	userAddressCache := make(map[string]*types.UserAddress)
	categoryNameCache := make(map[string]string)
	categoryOptionMap := make(map[string]categoryFilterOption)
	cityOptionsSet := make(map[string]bool)
	userIDs := make([]string, 0, len(needs))
	needIDs := make([]string, 0, len(needs))
	selectedAddressIDs := make([]string, 0)
	seenUserIDs := make(map[string]bool)
	seenNeedIDs := make(map[string]bool)
	seenSelectedAddressIDs := make(map[string]bool)
	for _, need := range needs {
		if !seenUserIDs[need.UserID] {
			seenUserIDs[need.UserID] = true
			userIDs = append(userIDs, need.UserID)
		}
		if !seenNeedIDs[need.ID] {
			seenNeedIDs[need.ID] = true
			needIDs = append(needIDs, need.ID)
		}
		if need.UserAddressID != nil {
			selectedAddressID := strings.TrimSpace(*need.UserAddressID)
			if selectedAddressID != "" && !seenSelectedAddressIDs[selectedAddressID] {
				seenSelectedAddressIDs[selectedAddressID] = true
				selectedAddressIDs = append(selectedAddressIDs, selectedAddressID)
			}
		}
	}

	users, err := s.userRepo.UsersByIDs(ctx, userIDs)
	if err != nil {
		s.logger.WithError(err).Warn("failed to batch fetch users for browse needs")
		users = []*types.User{}
	}
	for _, user := range users {
		if user == nil || strings.TrimSpace(user.ID) == "" {
			continue
		}
		userNameCache[user.ID] = userDisplayName(user)
	}

	selectedAddressesByID := make(map[string]*types.UserAddress)
	selectedAddresses, err := s.userAddressRepo.ByIDs(ctx, selectedAddressIDs)
	if err != nil {
		s.logger.WithError(err).Warn("failed to batch fetch selected addresses for browse needs")
		selectedAddresses = []*types.UserAddress{}
	}
	for _, address := range selectedAddresses {
		if address == nil || strings.TrimSpace(address.ID) == "" {
			continue
		}
		selectedAddressesByID[address.ID] = address
	}

	primaryAddressesByUserID := make(map[string]*types.UserAddress)
	primaryAddresses, err := s.userAddressRepo.PrimaryByUserIDs(ctx, userIDs)
	if err != nil {
		s.logger.WithError(err).Warn("failed to batch fetch primary addresses for browse needs")
		primaryAddresses = []*types.UserAddress{}
	}
	for _, address := range primaryAddresses {
		if address == nil || strings.TrimSpace(address.UserID) == "" {
			continue
		}
		if _, exists := primaryAddressesByUserID[address.UserID]; !exists {
			primaryAddressesByUserID[address.UserID] = address
		}
	}

	assignmentsByNeedID := make(map[string][]*types.NeedCategoryAssignment)
	assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedIDs(ctx, needIDs)
	if err != nil {
		s.logger.WithError(err).Warn("failed to batch fetch category assignments for browse needs")
		assignments = []*types.NeedCategoryAssignment{}
	}
	primaryCategoryIDs := make([]string, 0)
	seenPrimaryCategoryIDs := make(map[string]bool)
	for _, assignment := range assignments {
		if assignment == nil || strings.TrimSpace(assignment.NeedID) == "" {
			continue
		}
		assignmentsByNeedID[assignment.NeedID] = append(assignmentsByNeedID[assignment.NeedID], assignment)
		if assignment.IsPrimary {
			categoryID := strings.TrimSpace(assignment.CategoryID)
			if categoryID != "" && !seenPrimaryCategoryIDs[categoryID] {
				seenPrimaryCategoryIDs[categoryID] = true
				primaryCategoryIDs = append(primaryCategoryIDs, categoryID)
			}
		}
	}

	primaryCategories, err := s.categoryRepo.CategoriesByIDs(ctx, primaryCategoryIDs)
	if err != nil {
		s.logger.WithError(err).Warn("failed to batch fetch primary categories for browse needs")
		primaryCategories = []*types.NeedCategory{}
	}
	for _, category := range primaryCategories {
		if category == nil || strings.TrimSpace(category.ID) == "" {
			continue
		}
		categoryNameCache[category.ID] = category.Name
	}

	cards := make([]*types.BrowseNeedCard, 0, len(needs))
	for _, need := range needs {
		ownerName := "Anonymous"
		if cached, ok := userNameCache[need.UserID]; ok && strings.TrimSpace(cached) != "" {
			ownerName = cached
		}

		address := userAddressCache[need.UserID]
		if address == nil {
			if need.UserAddressID != nil && strings.TrimSpace(*need.UserAddressID) != "" {
				selectedAddressID := strings.TrimSpace(*need.UserAddressID)
				if selectedAddress, ok := selectedAddressesByID[selectedAddressID]; ok && selectedAddress != nil {
					if selectedAddress.UserID == need.UserID {
						address = selectedAddress
					}
				}
			}

			if address == nil {
				if primaryAddress, ok := primaryAddressesByUserID[need.UserID]; ok {
					address = primaryAddress
				}
			}

			userAddressCache[need.UserID] = address
		}

		city, state, cityState := browseCityStateParts(address)
		if city != "N/A" {
			cityOptionsSet[city] = true
		}

		primaryCategory := "General Need"
		primaryCategoryID := ""
		for _, assignment := range assignmentsByNeedID[need.ID] {
			if !assignment.IsPrimary {
				continue
			}
			primaryCategoryID = assignment.CategoryID

			if cachedName, ok := categoryNameCache[assignment.CategoryID]; ok {
				primaryCategory = cachedName
			}
			break
		}
		if primaryCategoryID == "" {
			primaryCategoryID = strings.ToLower(strings.ReplaceAll(primaryCategory, " ", "-"))
		}
		if primaryCategoryID != "" {
			categoryOptionMap[primaryCategoryID] = categoryFilterOption{ID: primaryCategoryID, Name: primaryCategory}
		}

		urgencyLabel, urgencyDotClass := browseUrgency(need.Status, need.AmountNeededCents, need.AmountRaisedCents)
		urgencyID := strings.ToLower(urgencyLabel)

		verificationID := "story-shared"
		verificationLabel := "Story Shared"
		if need.VerifiedAt != nil {
			verificationID = "personally-verified"
			verificationLabel = "Personally Verified"
		}

		fundingPercent := 0
		if need.AmountNeededCents > 0 {
			fundingPercent = (need.AmountRaisedCents * 100) / need.AmountNeededCents
			if fundingPercent < 0 {
				fundingPercent = 0
			}
			if fundingPercent > 100 {
				fundingPercent = 100
			}
		}

		card := &types.BrowseNeedCard{
			ID:                need.ID,
			OwnerName:         ownerName,
			City:              city,
			State:             state,
			CityState:         cityState,
			UrgencyLabel:      urgencyLabel,
			UrgencyDotClass:   urgencyDotClass,
			PrimaryCategoryID: primaryCategoryID,
			PrimaryCategory:   primaryCategory,
			VerificationID:    verificationID,
			VerificationLabel: verificationLabel,
			ShortDescription:  need.ShortDescription,
			Status:            need.Status,
			AmountNeededCents: need.AmountNeededCents,
			AmountRaisedCents: need.AmountRaisedCents,
			FundingPercent:    fundingPercent,
			CreatedAt:         need.CreatedAt,
		}

		if filters.City != "" && !strings.EqualFold(card.City, filters.City) {
			continue
		}

		if len(filters.CategoryIDs) > 0 && !filters.CategoryIDs[card.PrimaryCategoryID] {
			continue
		}

		if len(filters.VerificationIDs) > 0 && !filters.VerificationIDs[card.VerificationID] {
			continue
		}

		if filters.Urgency != "" && filters.Urgency != urgencyID {
			continue
		}

		if card.FundingPercent > filters.FundingMax {
			continue
		}

		if searchLower != "" {
			haystack := strings.ToLower(strings.Join([]string{
				card.OwnerName,
				card.PrimaryCategory,
				card.CityState,
				derefString(card.ShortDescription),
			}, " "))
			if !strings.Contains(haystack, searchLower) {
				continue
			}
		}

		cards = append(cards, card)
	}

	sortBrowseCards(cards, filters.SortBy)

	cityOptions := make([]string, 0, len(cityOptionsSet))
	for city := range cityOptionsSet {
		cityOptions = append(cityOptions, city)
	}
	sort.Strings(cityOptions)

	categoryOptionsByID := make(map[string]*types.NeedCategory)
	for _, category := range categories {
		if category == nil || strings.TrimSpace(category.ID) == "" {
			continue
		}
		categoryOptionsByID[category.ID] = category
	}

	for _, option := range categoryOptionMap {
		if strings.TrimSpace(option.ID) == "" {
			continue
		}
		if _, exists := categoryOptionsByID[option.ID]; exists {
			continue
		}
		categoryOptionsByID[option.ID] = &types.NeedCategory{ID: option.ID, Name: option.Name}
	}

	categoryOptionIDs := make([]string, 0, len(categoryOptionsByID))
	for id := range categoryOptionsByID {
		categoryOptionIDs = append(categoryOptionIDs, id)
	}
	sort.Slice(categoryOptionIDs, func(i, j int) bool {
		left := strings.ToLower(categoryOptionsByID[categoryOptionIDs[i]].Name)
		right := strings.ToLower(categoryOptionsByID[categoryOptionIDs[j]].Name)
		return left < right
	})

	categoryOptions := make([]*types.NeedCategory, 0, len(categoryOptionIDs))
	for _, id := range categoryOptionIDs {
		categoryOptions = append(categoryOptions, categoryOptionsByID[id])
	}

	s.logger.WithFields(map[string]any{
		"db_category_count":      len(categories),
		"derived_category_count": len(categoryOptionMap),
		"final_category_count":   len(categoryOptions),
	}).Info("browse: category options resolved")

	return &types.BrowsePageData{
		Needs:      cards,
		Categories: categoryOptions,
		Cities:     cityOptions,
		Filters:    filters,
	}, nil
}

func sortBrowseCards(cards []*types.BrowseNeedCard, sortBy string) {
	switch sortBy {
	case "newest":
		sort.SliceStable(cards, func(i, j int) bool {
			return cards[i].CreatedAt.After(cards[j].CreatedAt)
		})
	case "closest":
		sort.SliceStable(cards, func(i, j int) bool {
			if cards[i].FundingPercent != cards[j].FundingPercent {
				return cards[i].FundingPercent > cards[j].FundingPercent
			}
			return cards[i].CreatedAt.After(cards[j].CreatedAt)
		})
	case "nearest":
		return
	case "urgency":
		fallthrough
	default:
		sort.SliceStable(cards, func(i, j int) bool {
			left := urgencySortWeight(cards[i].UrgencyLabel)
			right := urgencySortWeight(cards[j].UrgencyLabel)
			if left != right {
				return left > right
			}
			return cards[i].CreatedAt.After(cards[j].CreatedAt)
		})
	}
}

func urgencySortWeight(label string) int {
	switch strings.ToUpper(strings.TrimSpace(label)) {
	case "URGENT":
		return 4
	case "HIGH":
		return 3
	case "MEDIUM":
		return 2
	case "LOW":
		return 1
	default:
		return 0
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
	_, _, cityState := browseCityStateParts(address)
	return cityState
}

func browseCityStateParts(address *types.UserAddress) (string, string, string) {
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
		return city, state, "N/A"
	}

	return city, state, city + ", " + state
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

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
