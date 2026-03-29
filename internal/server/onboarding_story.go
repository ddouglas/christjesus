package server

import (
	"christjesus/pkg/types"
	"fmt"
	"net/http"
)

func (s *Service) handleGetOnboardingNeedStory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	// Get the need
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need")
		s.internalServerError(w)
		return
	}

	if s.redirectIfNeedSubmitted(w, r, need) {
		return
	}

	// Get all assignments and find primary
	var primaryCategory *types.NeedCategory
	assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch category assignments")
		s.internalServerError(w)
		return
	}

	for _, assignment := range assignments {
		if assignment.IsPrimary {
			primaryCategory, err = s.categoryRepo.CategoryByID(ctx, assignment.CategoryID)
			if err != nil {
				s.logger.WithError(err).Error("failed to fetch primary category")
				s.internalServerError(w)
				return
			}
			break
		}
	}

	// Get story (may be nil if not yet created)
	story, err := s.storyRepo.GetStoryByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch story")
		s.internalServerError(w)
		return
	}

	data := &types.NeedStoryPageData{
		BasePageData:      types.BasePageData{Title: "Share Your Story"},
		ID:                needID,
		AmountNeededCents: need.AmountNeededCents,
		PrimaryCategory:   primaryCategory,
		Story:             story,
		FormAction:        s.route(RouteOnboardingNeedStory, Param("needID", needID)),
		BackHref:          s.route(RouteOnboardingNeedCategories, Param("needID", needID)),
	}

	err = s.renderTemplate(w, r, "page.onboarding.need.story", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need story page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostOnboardingNeedStory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	// Parse form
	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		s.internalServerError(w)
		return
	}

	// Get the need
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need")
		s.internalServerError(w)
		return
	}

	if s.redirectIfNeedSubmitted(w, r, need) {
		return
	}

	// Decode story data
	story := &types.NeedStory{
		NeedID: needID,
	}
	if err := decoder.Decode(story, r.Form); err != nil {
		s.logger.WithError(err).Error("failed to decode story form")
		s.internalServerError(w)
		return
	}

	// Parse and convert amount from whole dollars to cents
	amountStr := r.FormValue("amount")
	if amountStr != "" {
		var amountDollars int
		if _, err := fmt.Sscanf(amountStr, "%d", &amountDollars); err == nil {
			need.AmountNeededCents = amountDollars * 100
		}
	}

	// Update need with amount
	err = s.needsRepo.UpdateNeed(ctx, need.ID, need)
	if err != nil {
		s.logger.WithError(err).Error("failed to update need with amount")
		s.internalServerError(w)
		return
	}

	// Upsert story
	err = s.storyRepo.UpsertStory(ctx, story)
	if err != nil {
		s.logger.WithError(err).Error("failed to upsert story")
		s.internalServerError(w)
		return
	}

	// Update current step
	need.CurrentStep = types.NeedStepStory
	err = s.needsRepo.UpdateNeed(ctx, need.ID, need)
	if err != nil {
		s.logger.WithError(err).Error("failed to update need step")
		s.internalServerError(w)
		return
	}

	// Record progress
	s.recordNeedProgress(ctx, need.ID, types.NeedStepStory)

	http.Redirect(w, r, s.route(RouteOnboardingNeedDocuments, Param("needID", need.ID)), http.StatusSeeOther)
}
