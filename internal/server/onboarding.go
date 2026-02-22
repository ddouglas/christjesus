package server

import (
	"christjesus/pkg/types"
	"context"
	"fmt"
	"net/http"

	"github.com/k0kubun/pp"
)

func (s *Service) handleGetOnboarding(w http.ResponseWriter, r *http.Request) {

	var _ = r.Context()

	err := s.templates.ExecuteTemplate(w, "page.onboarding", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need welcome page")
		s.internalServerError(w)
		return
	}
}

type onboardingDirector struct {
	Path string `form:"path"`
}

func (s *Service) handlePostOnboarding(w http.ResponseWriter, r *http.Request) {

	var ctx = r.Context()

	err := r.ParseForm()
	if err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		return
	}

	var onboarding = new(onboardingDirector)
	err = decoder.Decode(onboarding, r.Form)
	if err != nil {
		s.logger.WithError(err).Error("failed to decode form")
		s.internalServerError(w)
		return
	}

	switch onboarding.Path {
	case "need":
		s.handleCreateNeed(ctx, w, r)
		return
	}

	err = s.templates.ExecuteTemplate(w, "page.onboarding", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need welcome page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleCreateNeed(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("ctx doesn't contain user")
		s.internalServerError(w)
		return
	}

	need := &types.Need{
		UserID:      userID,
		Status:      types.NeedStatusDraft,
		CurrentStep: types.NeedStepWelcome,
	}

	err = s.needsRepo.CreateNeed(ctx, need)
	if err != nil {
		s.logger.WithError(err).Error("failed to create need in datastore")
		s.internalServerError(w)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/onboarding/need/%s/welcome", need.ID), http.StatusSeeOther)
}

func (s *Service) handleGetOnboardingNeedWelcome(w http.ResponseWriter, r *http.Request) {
	var ctx = r.Context()

	needID := r.PathValue("needID")

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}

	err = s.templates.ExecuteTemplate(w, "page.onboarding.need.welcome", need)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need welcome page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostOnboardingNeedWelcome(w http.ResponseWriter, r *http.Request) {
	var ctx = r.Context()

	needID := r.PathValue("needID")
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepWelcome)
	http.Redirect(w, r, fmt.Sprintf("/onboarding/need/%s/location", need.ID), http.StatusSeeOther)

}

func (s *Service) handleGetOnboardingNeedLocation(w http.ResponseWriter, r *http.Request) {
	var ctx = r.Context()

	needID := r.PathValue("needID")
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}

	pp.Print(need)

	err = s.templates.ExecuteTemplate(w, "page.onboarding.need.location", need)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need location page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostOnboardingNeedLocation(w http.ResponseWriter, r *http.Request) {

	var ctx = r.Context()

	needID := r.PathValue("needID")
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}

	err = r.ParseForm()
	if err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		return
	}

	var location = new(types.NeedLocation)
	err = decoder.Decode(location, r.Form)
	if err != nil {
		s.logger.WithError(err).Error("failed to decode form onto location form")
		s.internalServerError(w)
		return
	}

	need.CurrentStep = types.NeedStepLocation
	need.NeedLocation = location

	pp.Print(need)

	err = s.needsRepo.UpdateNeed(ctx, needID, need)
	if err != nil {
		s.logger.WithError(err).Error("failed to update need with location data")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepLocation)

	http.Redirect(w, r, fmt.Sprintf("/onboarding/need/%s/categories", need.ID), http.StatusSeeOther)
}

func (s *Service) handleGetOnboardingNeedCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	categories, err := s.categoryRepo.AllCategories(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to load categories from database")
		s.internalServerError(w)
		return
	}

	if len(categories) == 0 {
		s.logger.Warn("no categories found in database - run 'just seed' to populate categories")
	}

	data := map[string]interface{}{
		"Title":      "Select Categories",
		"ID":         needID,
		"Categories": categories,
	}

	err = s.templates.ExecuteTemplate(w, "page.onboarding.need.categories", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need categories page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetOnboardingNeedDetails(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	err := s.templates.ExecuteTemplate(w, "page.onboarding.need.details", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need details page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetOnboardingNeedStory(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	err := s.templates.ExecuteTemplate(w, "page.onboarding.need.story", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need story page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetOnboardingNeedDocuments(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	err := s.templates.ExecuteTemplate(w, "page.onboarding.need.documents", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need documents page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetOnboardingNeedReview(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	err := s.templates.ExecuteTemplate(w, "page.onboarding.need.review", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need review page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetOnboardingNeedConfirmation(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	err := s.templates.ExecuteTemplate(w, "page.onboarding.need.confirmation", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need confirmation page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostOnboardingNeedCategories(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/onboarding/need/details", http.StatusSeeOther)
}

func (s *Service) handlePostOnboardingNeedDetails(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/onboarding/need/story", http.StatusSeeOther)
}

func (s *Service) handlePostOnboardingNeedStory(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/onboarding/need/documents", http.StatusSeeOther)
}

func (s *Service) handlePostOnboardingNeedDocuments(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/onboarding/need/review", http.StatusSeeOther)
}

func (s *Service) handlePostOnboardingNeedReview(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/onboarding/need/confirmation", http.StatusSeeOther)
}

// Donor onboarding handlers
func (s *Service) handleGetOnboardingDonorWelcome(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	err := s.templates.ExecuteTemplate(w, "page.onboarding.donor.welcome", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render donor welcome page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetOnboardingDonorPreferences(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	data := struct {
		Title      string
		Categories []any
	}{
		Title: "Donor Preferences",
		Categories: func() []any {
			cats := sampleCategories()
			result := make([]any, len(cats))
			for i, c := range cats {
				result[i] = c
			}
			return result
		}(),
	}

	err := s.templates.ExecuteTemplate(w, "page.onboarding.donor.preferences", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render donor preferences page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostOnboardingDonorPreferences(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/browse?onboarding=true", http.StatusSeeOther)
}

// Sponsor onboarding handlers (placeholders)
// func (s *Service) handleGetOnboardingSponsorIndividualWelcome(w http.ResponseWriter, r *http.Request) {
// 	var _ = r.Context()

// 	err := s.templates.ExecuteTemplate(w, "page.onboarding.sponsor.individual.welcome", nil)
// 	if err != nil {
// 		s.logger.WithError(err).Error("failed to render sponsor individual welcome page")
// 		s.internalServerError(w)
// 		return
// 	}
// }

// func (s *Service) handleGetOnboardingSponsorOrganizationWelcome(w http.ResponseWriter, r *http.Request) {
// 	var _ = r.Context()

// 	err := s.templates.ExecuteTemplate(w, "page.onboarding.sponsor.organization.welcome", nil)
// 	if err != nil {
// 		s.logger.WithError(err).Error("failed to render sponsor organization welcome page")
// 		s.internalServerError(w)
// 		return
// 	}
// }

func (s *Service) recordNeedProgress(ctx context.Context, needID string, step types.NeedStep) {
	err := s.progressRepo.RecordStepCompletion(ctx, needID, step)
	if err != nil {
		s.logger.WithError(err).
			WithField("need_id", needID).
			WithField("step", step).
			Warn("failed to record progress event")
	}
}
