package server

import (
	"christjesus/internal"
	"net/http"
	"strings"
	"time"
)

func (s *Service) handleGetRegister(w http.ResponseWriter, r *http.Request) {
	s.startAuth0Authorization(w, r, "signup")
}

func (s *Service) handlePostRegister(w http.ResponseWriter, r *http.Request) {
	s.startAuth0Authorization(w, r, "signup")
}

func (s *Service) handleGetRegisterConfirm(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
}

func (s *Service) handlePostRegisterConfirm(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
}

func (s *Service) handlePostRegisterConfirmResend(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
}

func (s *Service) setRegisterConfirmCookie(w http.ResponseWriter, email string, age time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     internal.COOKIE_REGISTER_CONFIRM,
		Value:    email,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   int(age.Seconds()),
	})
}

func (s *Service) clearRegisterConfirmCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     internal.COOKIE_REGISTER_CONFIRM,
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	})
}

func (s *Service) getRegisterConfirmEmail(r *http.Request) string {
	cookie, err := r.Cookie(internal.COOKIE_REGISTER_CONFIRM)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(cookie.Value)
}
