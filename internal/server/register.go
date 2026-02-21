package server

import (
	"christjesus/internal"
	"net/http"

	"github.com/supabase-community/auth-go/types"
)

func (s *Service) handleGetRegister(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	_, err := r.Cookie(internal.COOKIE_ACCESS_TOKEN_NAME)
	if err == nil {
		s.logger.Info("user is already logged in, redirecting to Browse Needs")
		http.Redirect(w, r, "/browse", http.StatusSeeOther)
		return
	}

	err = s.templates.ExecuteTemplate(w, "page.register", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render register page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostRegister(w http.ResponseWriter, r *http.Request) {

	var _ = r.Context()

	email := r.FormValue("email")
	password := r.FormValue("password")
	// name := r.FormValue("name")

	resp, err := s.supauth.Signup(types.SignupRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to signup user")
		s.internalServerError(w)
		return
	}

	// Success! resp contains User and Session
	s.logger.WithField("user_id", resp.User.ID).Info("user registered")

	// Set httpOnly, secure cookie with access token
	if resp.Session.AccessToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "access_token",
			Value:    resp.Session.AccessToken,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   resp.Session.ExpiresIn,
			Path:     "/",
		})
	}

	// Redirect to onboarding
	http.Redirect(w, r, "/onboarding/need/welcome", http.StatusSeeOther)

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
