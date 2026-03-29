package server

import (
	"christjesus/internal"
	"christjesus/pkg/types"
	"net/http"
	"strings"
)

func (s *Service) handleGetOnboardingAboutYou(w http.ResponseWriter, r *http.Request) {
	state, ok := s.authUserStateFromRequest(r)
	if ok && strings.TrimSpace(state.GivenName) != "" {
		http.Redirect(w, r, s.route(RouteOnboarding), http.StatusSeeOther)
		return
	}

	if err := s.renderTemplate(w, r, "page.onboarding.about_you", &types.CompleteProfilePageData{
		BasePageData: types.BasePageData{Title: "Complete Your Profile"},
	}); err != nil {
		s.logger.WithError(err).Error("failed to render onboarding about you page")
		s.internalServerError(w)
		return
	}

}

func (s *Service) handlePostOnboardingAboutYou(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form submission", http.StatusBadRequest)
		return
	}

	givenName := strings.TrimSpace(r.FormValue("given_name"))
	familyName := strings.TrimSpace(r.FormValue("family_name"))

	renderErr := func(msg string) {
		err := s.renderTemplate(w, r, "page.onboarding.about_you", &types.CompleteProfilePageData{
			BasePageData: types.BasePageData{Title: "Complete Your Profile"},
			Error:        msg,
			GivenName:    givenName,
			FamilyName:   familyName,
		})
		if err != nil {
			s.logger.WithError(err).Error("failed to render onboarding about you error")
		}
	}

	if givenName == "" {
		renderErr("First name is required.")
		return
	}
	if familyName == "" {
		renderErr("Last name is required.")
		return
	}
	if len([]rune(givenName)) > 100 || len([]rune(familyName)) > 100 {
		renderErr("Name must be 100 characters or fewer.")
		return
	}

	state, ok := s.authUserStateFromRequest(r)
	if !ok {
		http.Redirect(w, r, s.route(RouteLogin), http.StatusSeeOther)
		return
	}

	authSubject := strings.TrimSpace(state.AuthSubject)
	if authSubject == "" {
		renderErr("Session error: missing identity.")
		return
	}

	if err := s.auth0PatchUser(ctx, authSubject, map[string]any{
		"given_name":  givenName,
		"family_name": familyName,
		"user_metadata": map[string]string{
			"given_name":  givenName,
			"family_name": familyName,
		},
	}); err != nil {
		s.logger.WithError(err).WithField("auth_subject", authSubject).Error("failed to complete user profile on Auth0")
		renderErr("Unable to save your profile. Please try again.")
		return
	}

	state.GivenName = givenName
	state.FamilyName = familyName
	s.setAuthUserStateCookie(w, state, s.config.SessionMaxAgeSec)

	// Consume the pending redirect cookie if present, otherwise go home.
	redirectCookie, err := r.Cookie(internal.COOKIE_REDIRECT_NAME)
	if err != nil {
		http.Redirect(w, r, s.route(RouteOnboarding), http.StatusSeeOther)
		return
	}

	var path string
	if err := s.cookie.Decode(internal.COOKIE_REDIRECT_NAME, redirectCookie.Value, &path); err != nil ||
		!strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		s.clearRedirectCookie(w)
		http.Redirect(w, r, s.route(RouteOnboarding), http.StatusSeeOther)
		return
	}

	s.clearRedirectCookie(w)
	http.Redirect(w, r, path, http.StatusSeeOther)
}
