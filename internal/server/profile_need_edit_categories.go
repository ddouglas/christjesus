package server

import (
	"net/http"
	"net/url"
	"strings"

	"christjesus/pkg/types"
)

func (s *Service) handleGetProfileNeedEditCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	need, err := s.profileEditableNeed(ctx, needID)
	if err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	categories, err := s.categoryRepo.Categories(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to load categories from database")
		s.internalServerError(w)
		return
	}

	if len(categories) == 0 {
		s.logger.Warn("no categories found in database - run 'just seed' to populate categories")
	}

	selectedPrimaryCategoryID, selectedSecondaryCategoryIDs, err := s.selectedNeedCategories(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to load selected categories for profile edit")
		s.internalServerError(w)
		return
	}

	data := &types.NeedCategoriesPageData{
		BasePageData:                 types.BasePageData{Title: "Edit Need Categories"},
		Need:                         need,
		Categories:                   categories,
		SelectedPrimaryCategoryID:    selectedPrimaryCategoryID,
		SelectedSecondaryCategoryIDs: selectedSecondaryCategoryIDs,
		FormAction:                   s.route(RouteProfileNeedEditCategories, Param("needID", needID)),
		BackHref:                     s.route(RouteProfileNeedEditLocation, Param("needID", needID)),
		Error:                        strings.TrimSpace(r.URL.Query().Get("error")),
	}

	if err := s.renderTemplate(w, r, "page.onboarding.need.categories", data); err != nil {
		s.logger.WithError(err).Error("failed to render profile need categories edit page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostProfileNeedEditCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	need, err := s.profileEditableNeed(ctx, needID)
	if err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	err = r.ParseForm()
	if err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		s.internalServerError(w)
		return
	}

	if len(r.Form["primary"]) != 1 {
		s.redirectProfileNeedEditCategoriesWithError(w, r, needID, "Please select exactly one primary category.")
		return
	}

	primaryID := strings.TrimSpace(r.Form.Get("primary"))
	if primaryID == "" {
		s.redirectProfileNeedEditCategoriesWithError(w, r, needID, "Please select exactly one primary category.")
		return
	}

	secondaryIDs := make([]string, 0, len(r.Form["secondary"]))
	seenSecondaryIDs := make(map[string]bool, len(r.Form["secondary"]))
	for _, rawID := range r.Form["secondary"] {
		categoryID := strings.TrimSpace(rawID)
		if categoryID == "" || categoryID == primaryID || seenSecondaryIDs[categoryID] {
			continue
		}
		seenSecondaryIDs[categoryID] = true
		secondaryIDs = append(secondaryIDs, categoryID)
	}

	ids := make([]string, 0, len(secondaryIDs)+1)
	ids = append(ids, primaryID)
	ids = append(ids, secondaryIDs...)

	categories, err := s.categoryRepo.CategoriesByIDs(ctx, ids)
	if err != nil {
		s.logger.WithError(err).Error("failed to load categories from database")
		s.internalServerError(w)
		return
	}

	categoryMap := make(map[string]*types.NeedCategory)
	for _, c := range categories {
		categoryMap[c.ID] = c
	}

	needCategories := make([]*types.NeedCategory, 0, len(ids))
	for _, id := range ids {
		cat, ok := categoryMap[id]
		if !ok {
			s.logger.WithField("category_id", id).Error("submitted category ID not found in database")
			s.internalServerError(w)
			return
		}
		needCategories = append(needCategories, cat)
	}

	primaryCategory := categoryMap[primaryID]
	secondaryCategories := make([]*types.NeedCategory, 0, len(secondaryIDs))
	for _, id := range secondaryIDs {
		secondaryCategories = append(secondaryCategories, categoryMap[id])
	}

	err = s.needCategoryAssignmentsRepo.DeleteAllAssignmentsByNeedID(ctx, need.ID)
	if err != nil {
		s.logger.WithError(err).Error("failed to delete existing need category assignments")
		s.internalServerError(w)
		return
	}

	assignments := make([]*types.NeedCategoryAssignment, 0, len(needCategories))
	assignments = append(assignments, &types.NeedCategoryAssignment{NeedID: need.ID, CategoryID: primaryCategory.ID, IsPrimary: true})
	for _, cat := range secondaryCategories {
		assignments = append(assignments, &types.NeedCategoryAssignment{NeedID: need.ID, CategoryID: cat.ID, IsPrimary: false})
	}

	err = s.needCategoryAssignmentsRepo.CreateAssignments(ctx, assignments)
	if err != nil {
		s.logger.WithError(err).Error("failed to create need category assignments")
		s.internalServerError(w)
		return
	}

	need.CurrentStep = types.NeedStepCategories
	err = s.needsRepo.UpdateNeed(ctx, need.ID, need)
	if err != nil {
		s.logger.WithError(err).Error("failed to update need with selected categories")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepCategories)
	http.Redirect(w, r, s.route(RouteProfileNeedEditStory, Param("needID", need.ID)), http.StatusSeeOther)
}

func (s *Service) redirectProfileNeedEditCategoriesWithError(w http.ResponseWriter, r *http.Request, needID, msg string) {
	q := url.Values{}
	q.Set("error", msg)
	http.Redirect(w, r, s.routeWithQuery(RouteProfileNeedEditCategories, q, Param("needID", needID)), http.StatusSeeOther)
}
