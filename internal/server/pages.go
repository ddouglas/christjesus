package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"christjesus/internal/store"
	"christjesus/pkg/types"
)

const browseDefaultPageSize = 18

func (s *Service) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	latestNeeds, err := s.needsRepo.LatestNeeds(ctx, 5)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch latest needs for home page")
		s.internalServerError(w)
		return
	}

	homeNeedCards := s.buildHomeNeedCards(ctx, latestNeeds)

	var featuredNeed *types.BrowseNeedCard
	featuredNeeds := make([]*types.BrowseNeedCard, 0, 4)
	if len(homeNeedCards) > 0 {
		featuredNeed = homeNeedCards[0]
	}
	if len(homeNeedCards) > 1 {
		featuredNeeds = append(featuredNeeds, homeNeedCards[1:]...)
		if len(featuredNeeds) > 4 {
			featuredNeeds = featuredNeeds[:4]
		}
	}

	data := &types.HomePageData{
		BasePageData: types.BasePageData{Title: ""},
		Notice:       r.URL.Query().Get("notice"),
		Error:        r.URL.Query().Get("error"),
		FeaturedNeed: featuredNeed,
		Needs:        featuredNeeds,
		Categories:   s.buildHomeCategories(ctx),
		Stats:        s.buildHomeStats(ctx),
		Steps:        getSteps(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "page.home", data); err != nil {
		s.logger.WithError(err).Error("failed to render home page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) buildHomeCategories(ctx context.Context) []types.CategoryData {
	categories, err := s.categoryRepo.Categories(ctx)
	if err != nil {
		s.logger.WithError(err).Warn("failed to fetch categories for home page")
		return []types.CategoryData{}
	}

	categoryIDs := make([]string, 0, len(categories))
	for _, category := range categories {
		if category == nil || strings.TrimSpace(category.ID) == "" {
			continue
		}
		categoryIDs = append(categoryIDs, category.ID)
	}

	countsByCategoryID, err := s.needCategoryAssignmentsRepo.PrimaryNeedCountsByCategoryIDs(ctx, categoryIDs)
	if err != nil {
		s.logger.WithError(err).Warn("failed to fetch category counts for home page")
		countsByCategoryID = map[string]int{}
	}

	homeCategories := make([]types.CategoryData, 0, len(categories))
	for _, category := range categories {
		if category == nil {
			continue
		}

		slug := strings.TrimSpace(category.Slug)
		if slug == "" {
			slug = slugifyCategoryName(category.Name)
		}

		icon := "home"
		if category.Icon != nil && strings.TrimSpace(*category.Icon) != "" {
			icon = strings.TrimSpace(*category.Icon)
		}

		homeCategories = append(homeCategories, types.CategoryData{
			Name:  category.Name,
			Slug:  slug,
			Count: countsByCategoryID[category.ID],
			Icon:  icon,
		})
	}

	if len(homeCategories) == 0 {
		return []types.CategoryData{}
	}

	sort.SliceStable(homeCategories, func(i, j int) bool {
		if homeCategories[i].Count != homeCategories[j].Count {
			return homeCategories[i].Count > homeCategories[j].Count
		}
		return strings.ToLower(homeCategories[i].Name) < strings.ToLower(homeCategories[j].Name)
	})

	if len(homeCategories) > 4 {
		homeCategories = homeCategories[:4]
	}

	return homeCategories
}

func (s *Service) buildHomeStats(ctx context.Context) types.StatsData {
	stats, err := s.donationIntentRepo.HomeImpactStats(ctx)
	if err != nil {
		s.logger.WithError(err).Warn("failed to fetch home impact stats")
		return types.StatsData{}
	}

	return stats
}

func (s *Service) buildHomeNeedCards(ctx context.Context, needs []*types.Need) []*types.BrowseNeedCard {
	return s.buildNeedCards(ctx, needs, "home featured needs")
}

type needCardBuildContext struct {
	userNamesByID             map[string]string
	selectedAddressesByID     map[string]*types.UserAddress
	primaryAddressesByUserID  map[string]*types.UserAddress
	assignmentsByNeedID       map[string][]*types.NeedCategoryAssignment
	categoryNamesByCategoryID map[string]string
}

func (s *Service) buildNeedCards(ctx context.Context, needs []*types.Need, logContext string) []*types.BrowseNeedCard {
	if len(needs) == 0 {
		return []*types.BrowseNeedCard{}
	}

	ctxData := s.loadNeedCardBuildContext(ctx, needs, logContext)
	cards := make([]*types.BrowseNeedCard, 0, len(needs))

	for _, need := range needs {
		if need == nil {
			continue
		}

		ownerName := "Anonymous"
		if cached, ok := ctxData.userNamesByID[need.UserID]; ok && strings.TrimSpace(cached) != "" {
			ownerName = cached
		}

		var address *types.UserAddress
		if need.UserAddressID != nil {
			selectedAddressID := strings.TrimSpace(*need.UserAddressID)
			if selectedAddressID != "" {
				if selectedAddress, ok := ctxData.selectedAddressesByID[selectedAddressID]; ok && selectedAddress != nil && selectedAddress.UserID == need.UserID {
					address = selectedAddress
				}
			}
		}
		if address == nil {
			address = ctxData.primaryAddressesByUserID[need.UserID]
		}

		city, state, cityState := browseCityStateParts(address)

		primaryCategory := "General Need"
		primaryCategoryID := ""
		for _, assignment := range ctxData.assignmentsByNeedID[need.ID] {
			if !assignment.IsPrimary {
				continue
			}
			primaryCategoryID = assignment.CategoryID
			if cachedName, ok := ctxData.categoryNamesByCategoryID[assignment.CategoryID]; ok && strings.TrimSpace(cachedName) != "" {
				primaryCategory = cachedName
			}
			break
		}
		if primaryCategoryID == "" {
			primaryCategoryID = strings.ToLower(strings.ReplaceAll(primaryCategory, " ", "-"))
		}

		urgencyLabel, urgencyDotClass := browseUrgency(need.Status, need.AmountNeededCents, need.AmountRaisedCents)

		cards = append(cards, &types.BrowseNeedCard{
			ID:                need.ID,
			OwnerName:         ownerName,
			City:              city,
			State:             state,
			CityState:         cityState,
			UrgencyLabel:      urgencyLabel,
			UrgencyDotClass:   urgencyDotClass,
			PrimaryCategoryID: primaryCategoryID,
			PrimaryCategory:   primaryCategory,
			ShortDescription:  need.ShortDescription,
			Status:            need.Status,
			AmountNeededCents: need.AmountNeededCents,
			AmountRaisedCents: need.AmountRaisedCents,
			FundingPercent:    fundingPercentFromCents(need.AmountRaisedCents, need.AmountNeededCents),
			CreatedAt:         need.CreatedAt,
		})
	}

	return cards
}

func (s *Service) loadNeedCardBuildContext(ctx context.Context, needs []*types.Need, logContext string) needCardBuildContext {
	userIDs := make([]string, 0, len(needs))
	needIDs := make([]string, 0, len(needs))
	selectedAddressIDs := make([]string, 0, len(needs))
	seenUserIDs := make(map[string]bool)
	seenNeedIDs := make(map[string]bool)
	seenAddressIDs := make(map[string]bool)

	for _, need := range needs {
		if need == nil {
			continue
		}

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
			if selectedAddressID != "" && !seenAddressIDs[selectedAddressID] {
				seenAddressIDs[selectedAddressID] = true
				selectedAddressIDs = append(selectedAddressIDs, selectedAddressID)
			}
		}
	}

	ctxData := needCardBuildContext{
		userNamesByID:             make(map[string]string),
		selectedAddressesByID:     make(map[string]*types.UserAddress),
		primaryAddressesByUserID:  make(map[string]*types.UserAddress),
		assignmentsByNeedID:       make(map[string][]*types.NeedCategoryAssignment),
		categoryNamesByCategoryID: make(map[string]string),
	}

	users, err := s.userRepo.UsersByIDs(ctx, userIDs)
	if err != nil {
		s.logger.WithError(err).Warnf("failed to batch fetch users for %s", logContext)
	} else {
		for _, user := range users {
			if user == nil || strings.TrimSpace(user.ID) == "" {
				continue
			}
			ctxData.userNamesByID[user.ID] = userDisplayName(user)
		}
	}

	selectedAddresses, err := s.userAddressRepo.ByIDs(ctx, selectedAddressIDs)
	if err != nil {
		s.logger.WithError(err).Warnf("failed to batch fetch selected addresses for %s", logContext)
	} else {
		for _, address := range selectedAddresses {
			if address == nil || strings.TrimSpace(address.ID) == "" {
				continue
			}
			ctxData.selectedAddressesByID[address.ID] = address
		}
	}

	primaryAddresses, err := s.userAddressRepo.PrimaryByUserIDs(ctx, userIDs)
	if err != nil {
		s.logger.WithError(err).Warnf("failed to batch fetch primary addresses for %s", logContext)
	} else {
		for _, address := range primaryAddresses {
			if address == nil || strings.TrimSpace(address.UserID) == "" {
				continue
			}
			if _, exists := ctxData.primaryAddressesByUserID[address.UserID]; !exists {
				ctxData.primaryAddressesByUserID[address.UserID] = address
			}
		}
	}

	primaryCategoryIDs := make([]string, 0)
	seenPrimaryCategoryIDs := make(map[string]bool)
	assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedIDs(ctx, needIDs)
	if err != nil {
		s.logger.WithError(err).Warnf("failed to batch fetch category assignments for %s", logContext)
	} else {
		for _, assignment := range assignments {
			if assignment == nil || strings.TrimSpace(assignment.NeedID) == "" {
				continue
			}
			ctxData.assignmentsByNeedID[assignment.NeedID] = append(ctxData.assignmentsByNeedID[assignment.NeedID], assignment)
			if assignment.IsPrimary {
				categoryID := strings.TrimSpace(assignment.CategoryID)
				if categoryID != "" && !seenPrimaryCategoryIDs[categoryID] {
					seenPrimaryCategoryIDs[categoryID] = true
					primaryCategoryIDs = append(primaryCategoryIDs, categoryID)
				}
			}
		}
	}

	primaryCategories, err := s.categoryRepo.CategoriesByIDs(ctx, primaryCategoryIDs)
	if err != nil {
		s.logger.WithError(err).Warnf("failed to batch fetch categories for %s", logContext)
	} else {
		for _, category := range primaryCategories {
			if category == nil || strings.TrimSpace(category.ID) == "" {
				continue
			}
			ctxData.categoryNamesByCategoryID[category.ID] = category.Name
		}
	}

	return ctxData
}

func (s *Service) internalServerError(w http.ResponseWriter) {
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func (s *Service) handleBrowse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filters := parseBrowseFilters(r.URL.Query())

	prefsApplied := false
	hasDonorPrefs := false

	// Only apply preference defaults on full page loads, not HTMX filter submissions.
	// HTMX requests represent explicit user filter interactions — trust them as-is.
	if !isHXRequest(r) {
		if session, ok := sessionFromContext(ctx); ok && session.UserID != "" {
			pref, err := s.donorPreferenceRepo.ByUserID(ctx, session.UserID)
			if err != nil {
				s.logger.WithError(err).Warn("failed to fetch donor preferences for browse pre-population")
			}
			if pref != nil {
				hasDonorPrefs = true
				if filters.UsePrefs != "0" {
					if filters.ZipCode == "" && pref.ZipCode != nil && *pref.ZipCode != "" {
						filters.ZipCode = *pref.ZipCode
						prefsApplied = true
					}
					if filters.Radius == "" && pref.Radius != nil && *pref.Radius != "" {
						if normalized := normalizeBrowseRadius(*pref.Radius); normalized != "" {
							filters.Radius = normalized
							prefsApplied = true
						}
					}
				}
			}

			if filters.UsePrefs != "0" && len(filters.CategoryIDs) == 0 {
				assignments, err := s.donorPreferenceAssignRepo.AssignmentsByUserID(ctx, session.UserID)
				if err != nil {
					s.logger.WithError(err).Warn("failed to fetch donor preference categories for browse pre-population")
				} else if len(assignments) > 0 {
					hasDonorPrefs = true
					filters.CategoryIDs = make(map[string]bool, len(assignments))
					for _, a := range assignments {
						filters.CategoryIDs[a.CategoryID] = true
					}
					prefsApplied = true
				}
			}
		}
	}

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

	data := &types.BrowsePageData{
		BasePageData:         types.BasePageData{Title: "Browse Needs"},
		Categories:           categories,
		Filters:              filters,
		LoadResultsOnRender:  true,
		ShowResultsSkeletons: true,
		PrefsApplied:         prefsApplied,
		HasDonorPrefs:        hasDonorPrefs,
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

	usePrefs := ""
	if strings.TrimSpace(query.Get("use_prefs")) == "0" {
		usePrefs = "0"
	}

	return types.BrowseFilters{
		Search:      strings.TrimSpace(query.Get("search")),
		ZipCode:     normalizeBrowseZipCode(query.Get("zip")),
		Radius:      normalizeBrowseRadius(query.Get("radius")),
		CategoryIDs: selectedCategories,
		Urgency:     strings.TrimSpace(query.Get("urgency")),
		FundingMax:  fundingMax,
		ViewMode:    normalizeBrowseViewMode(query.Get("view")),
		SortBy:      normalizeBrowseSortBy(query.Get("sort")),
		Page:        parsePositiveInt(query.Get("page"), 1),
		PageSize:    browseDefaultPageSize,
		UsePrefs:    usePrefs,
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

func normalizeBrowseZipCode(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) != 5 {
		return ""
	}
	for _, c := range trimmed {
		if c < '0' || c > '9' {
			return ""
		}
	}
	return trimmed
}

func normalizeBrowseRadius(raw string) string {
	// Accept both "15" and "15-miles" (donor_preferences format)
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimSuffix(trimmed, "-miles")
	switch trimmed {
	case "5", "15", "25", "50":
		return trimmed
	case "anywhere":
		return "anywhere"
	default:
		return ""
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

func (s *Service) browseStoreFilter(filters types.BrowseFilters) store.BrowseNeedsFilter {
	categoryIDs := make([]string, 0, len(filters.CategoryIDs))
	for id := range filters.CategoryIDs {
		categoryIDs = append(categoryIDs, id)
	}

	sf := store.BrowseNeedsFilter{
		CategoryIDs: categoryIDs,
		Urgency:     filters.Urgency,
		FundingMax:  filters.FundingMax,
		Search:      filters.Search,
		SortBy:      filters.SortBy,
		Page:        filters.Page,
		PageSize:    filters.PageSize,
	}

	if filters.ZipCode != "" {
		sf.ZipCode = filters.ZipCode
		if filters.Radius != "" && filters.Radius != "anywhere" {
			if radiusMiles, err := strconv.ParseFloat(filters.Radius, 64); err == nil {
				sf.RadiusMiles = &radiusMiles
			}
		}
	}

	return sf
}

func (s *Service) buildBrowseResultsPageData(ctx context.Context, filters types.BrowseFilters) (*types.BrowsePageData, error) {
	sf := s.browseStoreFilter(filters)

	totalNeeds, err := s.needsRepo.BrowseNeedsCount(ctx, sf)
	if err != nil {
		return nil, err
	}

	totalPages := totalNeeds / filters.PageSize
	if totalNeeds%filters.PageSize != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}
	if filters.Page > totalPages {
		filters.Page = totalPages
		sf.Page = totalPages
	}

	rows, err := s.needsRepo.BrowseNeedsPage(ctx, sf)
	if err != nil {
		return nil, err
	}

	needs := make([]*types.Need, 0, len(rows))
	distanceByNeedID := make(map[string]*float64, len(rows))
	for _, row := range rows {
		needs = append(needs, &row.Need)
		distanceByNeedID[row.Need.ID] = row.DistanceMiles
	}

	cards := s.buildNeedCards(ctx, needs, "browse needs")
	for _, card := range cards {
		card.DistanceMiles = distanceByNeedID[card.ID]
	}

	categories, err := s.categoryRepo.Categories(ctx)
	if err != nil {
		return nil, err
	}

	sort.Slice(categories, func(i, j int) bool {
		return strings.ToLower(categories[i].Name) < strings.ToLower(categories[j].Name)
	})

	prevHref := ""
	nextHref := ""
	if filters.Page > 1 {
		prevHref = s.buildBrowsePageHref(filters, filters.Page-1)
	}
	if filters.Page < totalPages {
		nextHref = s.buildBrowsePageHref(filters, filters.Page+1)
	}

	return &types.BrowsePageData{
		Needs:      cards,
		Categories: categories,
		Filters:    filters,
		Page:       filters.Page,
		TotalNeeds: totalNeeds,
		TotalPages: totalPages,
		PrevHref:   prevHref,
		NextHref:   nextHref,
	}, nil
}

func (s *Service) buildBrowsePageHref(filters types.BrowseFilters, page int) string {
	v := url.Values{}
	v.Set("page", strconv.Itoa(page))
	if filters.Search != "" {
		v.Set("search", filters.Search)
	}
	if filters.ZipCode != "" {
		v.Set("zip", filters.ZipCode)
	}
	if filters.Radius != "" {
		v.Set("radius", filters.Radius)
	}
	for id := range filters.CategoryIDs {
		v.Add("category", id)
	}
	if filters.Urgency != "" {
		v.Set("urgency", filters.Urgency)
	}
	if filters.FundingMax < 100 {
		v.Set("funding_max", strconv.Itoa(filters.FundingMax))
	}
	if filters.ViewMode != "grid" {
		v.Set("view", filters.ViewMode)
	}
	if filters.SortBy != "urgency" {
		v.Set("sort", filters.SortBy)
	}
	if filters.UsePrefs == "0" {
		v.Set("use_prefs", "0")
	}
	return s.routeWithQuery(RouteBrowse, nil, v)
}

func (s *Service) handleNeedDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

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
	if need.DeletedAt != nil {
		http.NotFound(w, r)
		return
	}

	ownerName := "Anonymous"
	user, err := s.userRepo.User(ctx, need.UserID)
	if err == nil {
		ownerName = userDisplayName(user)
	} else if !errors.Is(err, types.ErrUserNotFound) {
		s.logger.WithError(err).WithField("user_id", need.UserID).Warn("failed to fetch need owner for detail page")
	}

	story, err := s.storyRepo.GetStoryByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need story")
		s.internalServerError(w)
		return
	}

	assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need category assignments")
		s.internalServerError(w)
		return
	}

	categoryIDs := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		categoryID := strings.TrimSpace(assignment.CategoryID)
		if categoryID != "" {
			categoryIDs = append(categoryIDs, categoryID)
		}
	}

	categoryByID := make(map[string]*types.NeedCategory)
	if len(categoryIDs) > 0 {
		categories, err := s.categoryRepo.CategoriesByIDs(ctx, categoryIDs)
		if err != nil {
			s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need categories")
			s.internalServerError(w)
			return
		}

		for _, category := range categories {
			if category == nil || strings.TrimSpace(category.ID) == "" {
				continue
			}
			categoryByID[category.ID] = category
		}
	}

	var primaryCategory *types.NeedCategory
	secondaryCategories := make([]*types.NeedCategory, 0)
	for _, assignment := range assignments {
		category := categoryByID[assignment.CategoryID]
		if category == nil {
			continue
		}

		if assignment.IsPrimary {
			primaryCategory = category
			continue
		}

		secondaryCategories = append(secondaryCategories, category)
	}

	var selectedAddress *types.UserAddress
	if need.UserAddressID != nil {
		selectedAddressID := strings.TrimSpace(*need.UserAddressID)
		if selectedAddressID != "" {
			selectedAddress, err = s.userAddressRepo.ByIDAndUserID(ctx, selectedAddressID, need.UserID)
			if err != nil {
				s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch selected address for need detail")
				s.internalServerError(w)
				return
			}
		}
	}

	if selectedAddress == nil {
		selectedAddress, err = s.userAddressRepo.PrimaryByUserID(ctx, need.UserID)
		if err != nil {
			s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch primary address for need detail")
			s.internalServerError(w)
			return
		}
	}

	_, _, cityState := browseCityStateParts(selectedAddress)

	urgencyLabel, urgencyDotClass := browseUrgency(need.Status, need.AmountNeededCents, need.AmountRaisedCents)

	fundingPercent := fundingPercentFromCents(need.AmountRaisedCents, need.AmountNeededCents)

	docs, err := s.documentRepo.DocumentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need documents")
		s.internalServerError(w)
		return
	}

	reviewDocs := make([]types.ReviewDocument, 0, len(docs))
	for _, doc := range docs {
		reviewDocs = append(reviewDocs, types.ReviewDocument{
			ID:         doc.ID,
			FileName:   doc.FileName,
			TypeLabel:  documentTypeLabel(doc.DocumentType),
			SizeBytes:  doc.FileSizeBytes,
			UploadedAt: doc.UploadedAt,
		})
	}

	relatedNeeds := make([]*types.BrowseNeedCard, 0, 3)
	if primaryCategory != nil && strings.TrimSpace(primaryCategory.ID) != "" {
		relatedFilters := types.BrowseFilters{
			CategoryIDs: map[string]bool{primaryCategory.ID: true},
			FundingMax:  100,
			ViewMode:    "grid",
			SortBy:      "urgency",
			Page:        1,
			PageSize:    4,
		}

		relatedData, err := s.buildBrowseResultsPageData(ctx, relatedFilters)
		if err != nil {
			s.logger.WithError(err).WithField("need_id", needID).Warn("failed to load related needs")
		} else {
			for _, related := range relatedData.Needs {
				if related == nil || related.ID == needID {
					continue
				}
				relatedNeeds = append(relatedNeeds, related)
				if len(relatedNeeds) == 3 {
					break
				}
			}
		}
	}

	data := &types.NeedDetailPageData{
		BasePageData:        types.BasePageData{Title: "Need Details"},
		ID:                  needID,
		Need:                need,
		OwnerName:           ownerName,
		SelectedAddress:     selectedAddress,
		CityState:           cityState,
		UrgencyLabel:        urgencyLabel,
		UrgencyDotClass:     urgencyDotClass,
		FundingPercent:      fundingPercent,
		Story:               story,
		PrimaryCategory:     primaryCategory,
		SecondaryCategories: secondaryCategories,
		Documents:           reviewDocs,
		RelatedNeeds:        relatedNeeds,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "page.need-detail", data); err != nil {
		s.logger.WithError(err).Error("failed to render need detail page")
		s.internalServerError(w)
		return
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
	if status == types.NeedStatusSubmitted || status == types.NeedStatusReadyForReview || status == types.NeedStatusUnderReview {
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
