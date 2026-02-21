package server

import "net/http"

func (s *Service) handleRegister(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	err := s.templates.ExecuteTemplate(w, "page.register", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render register page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleRegisterSponsor(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	err := s.templates.ExecuteTemplate(w, "page.register.sponsor", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render register sponsor page")
		s.internalServerError(w)
		return
	}
}
