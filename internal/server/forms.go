package server

import (
	"context"
	"net/http"
	"strings"
	"time"
)

func (s *Service) handlePrayerRequestSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectWithError(w, r, "invalid form payload")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	requestBody := strings.TrimSpace(r.FormValue("request"))

	if !required(name) || !required(requestBody) {
		s.redirectWithError(w, r, "name and request are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.forms.CreatePrayerRequest(ctx, name, email, requestBody); err != nil {
		s.logger.WithError(err).Error("failed to submit prayer request")
		s.redirectWithError(w, r, "unable to submit prayer request")
		return
	}

	s.redirectWithNotice(w, r, "Prayer request submitted")
}

func (s *Service) handleEmailSignupSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectWithError(w, r, "invalid form payload")
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	city := strings.TrimSpace(r.FormValue("city"))

	if !required(email) {
		s.redirectWithError(w, r, "email is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.forms.UpsertEmailSignup(ctx, email, city); err != nil {
		s.logger.WithError(err).Error("failed to submit email signup")
		s.redirectWithError(w, r, "unable to submit email signup")
		return
	}

	s.redirectWithNotice(w, r, "Signup received")
}
