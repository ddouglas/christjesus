package server

import (
	"christjesus/pkg/types"
	"net/http"
	"net/url"
	"time"
)

func (s *Service) handleGetOnboardingNeedReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need")
		s.internalServerError(w)
		return
	}

	if s.redirectIfNeedSubmitted(w, r, need) {
		return
	}

	core, err := s.loadNeedReviewCoreData(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to load need review core data")
		s.internalServerError(w)
		return
	}

	data := &types.NeedReviewPageData{
		BasePageData:        types.BasePageData{Title: "Review Need"},
		ID:                  needID,
		Need:                core.Need,
		SelectedAddress:     core.SelectedAddress,
		Story:               core.Story,
		PrimaryCategory:     core.PrimaryCategory,
		SecondaryCategories: core.SecondaryCategories,
		Documents:           core.Documents,
		EditLocationHref:    s.route(RouteOnboardingNeedLocation, Param("needID", needID)),
		EditCategoriesHref:  s.route(RouteOnboardingNeedCategories, Param("needID", needID)),
		EditStoryHref:       s.route(RouteOnboardingNeedStory, Param("needID", needID)),
		EditDocumentsHref:   s.route(RouteOnboardingNeedDocuments, Param("needID", needID)),
		SubmitAction:        s.route(RouteOnboardingNeedReview, Param("needID", needID)),
		BackHref:            s.route(RouteOnboardingNeedDocuments, Param("needID", needID)),
		SubmitLabel:         "Submit Profile",
		Notice:              r.URL.Query().Get("notice"),
		Error:               r.URL.Query().Get("error"),
	}

	err = s.renderTemplate(w, r, "page.onboarding.need.review", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need review page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetOnboardingNeedConfirmation(w http.ResponseWriter, r *http.Request) {
	needID := r.PathValue("needID")

	data := &types.NeedSubmittedPageData{
		BasePageData: types.BasePageData{Title: "Need Submitted"},
		ID:           needID,
	}

	err := s.renderTemplate(w, r, "page.onboarding.need.confirmation", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need confirmation page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostOnboardingNeedReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to parse review form")
		s.internalServerError(w)
		return
	}

	if r.FormValue("agreeTerms") != "on" || r.FormValue("agreeVerification") != "on" {
		q := url.Values{}
		q.Set("error", "Please agree to the terms and verification statements before submitting.")
		http.Redirect(w, r, s.routeWithQuery(RouteOnboardingNeedReview, q, Param("needID", needID)), http.StatusSeeOther)
		return
	}

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need for review submit")
		s.internalServerError(w)
		return
	}

	if s.redirectIfNeedSubmitted(w, r, need) {
		return
	}

	now := time.Now()
	need.CurrentStep = types.NeedStepReview
	need.Status = types.NeedStatusSubmitted
	need.SubmittedAt = &now

	err = s.needsRepo.UpdateNeed(ctx, need.ID, need)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to update need status on submit")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepReview)
	http.Redirect(w, r, s.route(RouteOnboardingNeedConfirmation, Param("needID", needID)), http.StatusSeeOther)
}
