package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"christjesus/pkg/types"
)

type needReviewCoreData struct {
	Need                *types.Need
	Story               *types.NeedStory
	PrimaryCategory     *types.NeedCategory
	SecondaryCategories []*types.NeedCategory
	SelectedAddress     *types.UserAddress
	Documents           []types.ReviewDocument
}

type needReviewSharedData struct {
	Need                *types.Need
	Story               *types.NeedStory
	PrimaryCategory     *types.NeedCategory
	SecondaryCategories []*types.NeedCategory
	Documents           []types.NeedDocument
}

func (s *Service) handleGetProfileNeedEditReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	if _, err := s.profileEditableNeed(ctx, needID); err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	core, err := s.loadNeedReviewCoreData(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to load need review core data")
		s.internalServerError(w)
		return
	}

	data := &types.NeedReviewPageData{
		BasePageData:        types.BasePageData{Title: "Edit Need Review"},
		ID:                  needID,
		Need:                core.Need,
		SelectedAddress:     core.SelectedAddress,
		Story:               core.Story,
		PrimaryCategory:     core.PrimaryCategory,
		SecondaryCategories: core.SecondaryCategories,
		Documents:           core.Documents,
		EditLocationHref:    s.route(RouteProfileNeedEditLocation, Param("needID", needID)),
		EditCategoriesHref:  s.route(RouteProfileNeedEditCategories, Param("needID", needID)),
		EditStoryHref:       s.route(RouteProfileNeedEditStory, Param("needID", needID)),
		EditDocumentsHref:   s.route(RouteProfileNeedEditDocs, Param("needID", needID)),
		SubmitAction:        s.route(RouteProfileNeedEditReview, Param("needID", needID)),
		BackHref:            s.route(RouteProfileNeedEditDocs, Param("needID", needID)),
		SubmitLabel:         "Submit Updated Need",
		Notice:              strings.TrimSpace(r.URL.Query().Get("notice")),
		Error:               strings.TrimSpace(r.URL.Query().Get("error")),
	}

	if err := s.renderTemplate(w, r, "page.onboarding.need.review", data); err != nil {
		s.logger.WithError(err).Error("failed to render profile need review page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostProfileNeedEditReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	need, err := s.profileEditableNeed(ctx, needID)
	if err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to parse review form")
		s.internalServerError(w)
		return
	}

	if r.FormValue("agreeTerms") != "on" || r.FormValue("agreeVerification") != "on" {
		q := url.Values{}
		q.Set("error", "Please agree to the terms and verification statements before submitting.")
		http.Redirect(w, r, s.routeWithQuery(RouteProfileNeedEditReview, q, Param("needID", needID)), http.StatusSeeOther)
		return
	}

	now := time.Now()
	need.CurrentStep = types.NeedStepReview
	need.Status = types.NeedStatusReadyForReview
	need.SubmittedAt = &now

	if err := s.needsRepo.UpdateNeed(ctx, need.ID, need); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to update need status on profile edit submit")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepReview)
	s.redirectProfileNeedReviewWithNotice(w, r, needID, "Updated need submitted for review.")
}

func (s *Service) profileEditableNeed(ctx context.Context, needID string) (*types.Need, error) {
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		return nil, err
	}

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if need.UserID != userID {
		return nil, types.ErrNeedNotFound
	}

	if need.Status != types.NeedStatusSubmitted && need.Status != types.NeedStatusChangesRequested {
		return nil, fmt.Errorf("need is not editable in its current state")
	}

	return need, nil
}

func (s *Service) loadNeedReviewCoreData(ctx context.Context, needID string) (*needReviewCoreData, error) {
	shared, err := s.loadNeedReviewSharedData(ctx, needID)
	if err != nil {
		return nil, err
	}

	var selectedAddress *types.UserAddress
	if shared.Need.UserAddressID != nil && *shared.Need.UserAddressID != "" {
		selectedAddress, err = s.userAddressRepo.ByIDAndUserID(ctx, *shared.Need.UserAddressID, shared.Need.UserID)
		if err != nil {
			return nil, err
		}
	}
	if selectedAddress == nil {
		selectedAddress, err = s.userAddressRepo.PrimaryByUserID(ctx, shared.Need.UserID)
		if err != nil {
			return nil, err
		}
	}

	reviewDocs := make([]types.ReviewDocument, 0, len(shared.Documents))
	for _, doc := range shared.Documents {
		reviewDocs = append(reviewDocs, types.ReviewDocument{ID: doc.ID, FileName: doc.FileName, TypeLabel: documentTypeLabel(doc.DocumentType), SizeBytes: doc.FileSizeBytes, UploadedAt: doc.UploadedAt})
	}

	return &needReviewCoreData{
		Need:                shared.Need,
		Story:               shared.Story,
		PrimaryCategory:     shared.PrimaryCategory,
		SecondaryCategories: shared.SecondaryCategories,
		SelectedAddress:     selectedAddress,
		Documents:           reviewDocs,
	}, nil
}

func (s *Service) loadNeedReviewSharedData(ctx context.Context, needID string) (*needReviewSharedData, error) {
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		return nil, err
	}

	story, err := s.storyRepo.GetStoryByNeedID(ctx, needID)
	if err != nil {
		return nil, err
	}

	assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID(ctx, needID)
	if err != nil {
		return nil, err
	}

	categoryIDs := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		categoryID := strings.TrimSpace(assignment.CategoryID)
		if categoryID == "" {
			continue
		}
		categoryIDs = append(categoryIDs, categoryID)
	}

	categoryByID := make(map[string]*types.NeedCategory)
	if len(categoryIDs) > 0 {
		categories, err := s.categoryRepo.CategoriesByIDs(ctx, categoryIDs)
		if err != nil {
			return nil, err
		}
		for _, category := range categories {
			if category == nil {
				continue
			}
			categoryByID[category.ID] = category
		}
	}

	var primaryCategory *types.NeedCategory
	secondaryCategories := make([]*types.NeedCategory, 0)
	for _, assignment := range assignments {
		categoryID := strings.TrimSpace(assignment.CategoryID)
		if categoryID == "" {
			continue
		}

		category := categoryByID[categoryID]
		if category == nil {
			continue
		}
		if assignment.IsPrimary {
			primaryCategory = category
			continue
		}
		secondaryCategories = append(secondaryCategories, category)
	}

	documents, err := s.documentRepo.DocumentsByNeedID(ctx, needID)
	if err != nil {
		return nil, err
	}

	return &needReviewSharedData{
		Need:                need,
		Story:               story,
		PrimaryCategory:     primaryCategory,
		SecondaryCategories: secondaryCategories,
		Documents:           documents,
	}, nil
}

func (s *Service) needDocumentDeleteActions(routeName RouteName, needID string, documentIDs []string) map[string]string {
	actions := make(map[string]string, len(documentIDs))
	for _, documentID := range documentIDs {
		actions[documentID] = s.route(routeName, Param("needID", needID), Param("documentID", documentID))
	}
	return actions
}

func needDocumentIDs(documents []types.NeedDocument) []string {
	ids := make([]string, 0, len(documents))
	for _, document := range documents {
		ids = append(ids, document.ID)
	}
	return ids
}

func (s *Service) handleProfileEditableNeedError(w http.ResponseWriter, r *http.Request, needID string, err error) {
	if errors.Is(err, types.ErrNeedNotFound) {
		http.NotFound(w, r)
		return
	}

	if strings.Contains(strings.ToLower(err.Error()), "current state") {
		s.redirectProfileNeedReviewWithError(w, r, needID, "This need cannot be edited in its current status.")
		return
	}

	s.logger.WithError(err).WithField("need_id", needID).Error("failed to load editable need")
	s.internalServerError(w)
}
