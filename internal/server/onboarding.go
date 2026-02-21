package server

import "net/http"

func (s *Service) handleGetOnboardingNeedWelcome(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	err := s.templates.ExecuteTemplate(w, "page.onboarding.need.welcome", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need welcome page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetOnboardingNeedLocation(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	err := s.templates.ExecuteTemplate(w, "page.onboarding.need.location", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need location page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetOnboardingNeedCategories(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	data := struct {
		Title      string
		Categories []any
	}{
		Title: "Foobar",
		Categories: func() []any {
			cats := sampleCategories()
			result := make([]any, len(cats))
			for i, c := range cats {
				result[i] = c
			}
			return result
		}(),
	}

	err := s.templates.ExecuteTemplate(w, "page.onboarding.need.categories", data)
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

// POST handlers - temporary pass-through redirects
func (s *Service) handlePostOnboardingNeedLocation(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/onboarding/need/categories", http.StatusSeeOther)
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
func (s *Service) handleGetOnboardingSponsorIndividualWelcome(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	err := s.templates.ExecuteTemplate(w, "page.onboarding.sponsor.individual.welcome", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render sponsor individual welcome page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetOnboardingSponsorOrganizationWelcome(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	err := s.templates.ExecuteTemplate(w, "page.onboarding.sponsor.organization.welcome", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render sponsor organization welcome page")
		s.internalServerError(w)
		return
	}
}
