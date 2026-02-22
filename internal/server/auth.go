package server

import (
	"christjesus/internal"
	"net/http"
	"time"
)

func (s *Service) handleGetLogin(w http.ResponseWriter, r *http.Request) {

	_, err := r.Cookie(internal.COOKIE_ACCESS_TOKEN_NAME)
	if err == nil {
		s.logger.Info("user is already logged in, redirecting to Browse Needs")
		http.Redirect(w, r, "/browse", http.StatusSeeOther)
		return
	}

	err = s.templates.ExecuteTemplate(w, "page.login", nil)
	if err != nil {
		s.logger.WithError(err).Error("failed to render login page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostLogin(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	email := r.FormValue("email")
	password := r.FormValue("password")

	// Sign in with Supabase
	resp, err := s.supauth.SignInWithEmailPassword(email, password)

	if err != nil {
		s.logger.WithError(err).Error("failed to login user")
		http.Error(w, "Login failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Success! resp contains User and Session with AccessToken
	s.logger.WithField("user_id", resp.User.ID).Info("user logged in")

	encryptedToken, err := s.cookie.Encode(internal.COOKIE_ACCESS_TOKEN_NAME, resp.AccessToken)
	if err != nil {
		s.logger.WithError(err).Error("failed to encrypt access token")
		http.Error(w, "Login failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Set httpOnly, secure cookie with access token
	http.SetCookie(w, &http.Cookie{
		Name:     internal.COOKIE_ACCESS_TOKEN_NAME,
		Value:    encryptedToken,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   resp.ExpiresIn,
		Path:     "/",
	})

	redirectCookie, err := r.Cookie(internal.COOKIE_REDIRECT_NAME)
	if err == nil {
		path := redirectCookie.Value
		http.SetCookie(w, &http.Cookie{
			Name:     internal.COOKIE_REDIRECT_NAME,
			Value:    "",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
			MaxAge:   -1,
		})
		http.Redirect(w, r, path, http.StatusSeeOther)
	}

	http.Redirect(w, r, "/browse", http.StatusSeeOther)
}

func (s *Service) setRedirectCookie(w http.ResponseWriter, path string, age time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     internal.COOKIE_REDIRECT_NAME,
		Value:    path,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   int(age.Seconds()),
	})
}
