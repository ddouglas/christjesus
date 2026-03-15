package server

import (
	"fmt"
	"net/http"
	"strings"

	"christjesus/pkg/types"
)

func (s *Service) handleGetProfileNeedEditStory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	need, err := s.profileEditableNeed(ctx, needID)
	if err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	var primaryCategory *types.NeedCategory
	assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch category assignments")
		s.internalServerError(w)
		return
	}

	for _, assignment := range assignments {
		if !assignment.IsPrimary {
			continue
		}

		primaryCategory, err = s.categoryRepo.CategoryByID(ctx, assignment.CategoryID)
		if err != nil {
			s.logger.WithError(err).Error("failed to fetch primary category")
			s.internalServerError(w)
			return
		}
		break
	}

	story, err := s.storyRepo.GetStoryByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch story")
		s.internalServerError(w)
		return
	}

	data := &types.NeedStoryPageData{
		BasePageData:      types.BasePageData{Title: "Edit Need Story"},
		ID:                needID,
		AmountNeededCents: need.AmountNeededCents,
		PrimaryCategory:   primaryCategory,
		Story:             story,
		FormAction:        s.route(RouteProfileNeedEditStory, map[string]string{"needID": needID}),
		BackHref:          s.route(RouteProfileNeedEditCategories, map[string]string{"needID": needID}),
	}

	if err := s.renderTemplate(w, r, "page.onboarding.need.story", data); err != nil {
		s.logger.WithError(err).Error("failed to render profile need story edit page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostProfileNeedEditStory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	need, err := s.profileEditableNeed(ctx, needID)
	if err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		s.internalServerError(w)
		return
	}

	story := &types.NeedStory{NeedID: needID}
	if err := decoder.Decode(story, r.Form); err != nil {
		s.logger.WithError(err).Error("failed to decode story form")
		s.internalServerError(w)
		return
	}

	amountStr := r.FormValue("amount")
	if amountStr != "" {
		var amountDollars int
		if _, err := fmt.Sscanf(amountStr, "%d", &amountDollars); err == nil {
			need.AmountNeededCents = amountDollars * 100
		}
	}

	if err := s.needsRepo.UpdateNeed(ctx, need.ID, need); err != nil {
		s.logger.WithError(err).Error("failed to update need with amount")
		s.internalServerError(w)
		return
	}

	if err := s.storyRepo.UpsertStory(ctx, story); err != nil {
		s.logger.WithError(err).Error("failed to upsert story")
		s.internalServerError(w)
		return
	}

	need.CurrentStep = types.NeedStepStory
	if err := s.needsRepo.UpdateNeed(ctx, need.ID, need); err != nil {
		s.logger.WithError(err).Error("failed to update need step")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepStory)
	http.Redirect(w, r, s.route(RouteProfileNeedEditDocs, map[string]string{"needID": need.ID}), http.StatusSeeOther)
}
