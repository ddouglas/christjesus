package server

import (
	"christjesus/internal"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type auth0TokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

func (s *Service) handleGetLogin(w http.ResponseWriter, r *http.Request) {
	_, err := r.Cookie(internal.COOKIE_ACCESS_TOKEN_NAME)
	if err == nil {
		if _, ok := s.authClaimsFromRequest(r); ok {
			s.logger.Info("user is already logged in, redirecting to Browse Needs")
			http.Redirect(w, r, s.route(RouteBrowse, nil), http.StatusSeeOther)
			return
		}

		s.clearAccessTokenCookie(w)
		s.clearAuthUserStateCookie(w)
	}

	s.startAuth0Authorization(w, r, "")
}

func (s *Service) handlePostLogin(w http.ResponseWriter, r *http.Request) {
	s.startAuth0Authorization(w, r, "")
}

func (s *Service) handleGetAuthCallback(w http.ResponseWriter, r *http.Request) {
	var ctx = r.Context()
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if state == "" || code == "" {
		http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
		return
	}

	stateCookie, err := r.Cookie(internal.COOKIE_AUTH_STATE)
	if err != nil {
		s.clearAuthFlowCookies(w)
		http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
		return
	}
	var stateFromCookie string
	if err := s.cookie.Decode(internal.COOKIE_AUTH_STATE, stateCookie.Value, &stateFromCookie); err != nil || !subtleCompare(stateFromCookie, state) {
		s.clearAuthFlowCookies(w)
		http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
		return
	}
	nonceCookie, err := r.Cookie(internal.COOKIE_AUTH_NONCE)
	if err != nil {
		s.clearAuthFlowCookies(w)
		http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
		return
	}
	var nonceFromCookie string
	if err := s.cookie.Decode(internal.COOKIE_AUTH_NONCE, nonceCookie.Value, &nonceFromCookie); err != nil || strings.TrimSpace(nonceFromCookie) == "" {
		s.clearAuthFlowCookies(w)
		http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
		return
	}

	payload := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     s.config.Auth0ClientID,
		"client_secret": s.config.Auth0ClientSecret,
		"code":          code,
		"redirect_uri":  strings.TrimSpace(s.config.Auth0CallbackURL),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		s.logger.WithError(err).Error("failed to marshal auth0 token exchange payload")
		s.internalServerError(w)
		return
	}

	tokenURL := strings.TrimRight(s.auth0DomainURL(), "/") + "/oauth/token"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, tokenURL, strings.NewReader(string(body)))
	if err != nil {
		s.logger.WithError(err).Error("failed to create auth0 token exchange request")
		s.internalServerError(w)
		return
	}
	req.Header.Set("content-type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.WithError(err).Error("failed to exchange authorization code with auth0")
		http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
		return
	}
	defer resp.Body.Close()

	var tokenResp auth0TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		s.logger.WithError(err).WithField("status", resp.StatusCode).Error("failed to decode auth0 token response")
		http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
		return
	}

	if resp.StatusCode >= 400 || strings.TrimSpace(tokenResp.IDToken) == "" {
		s.logger.WithField("status", resp.StatusCode).Warn("auth0 token exchange returned unsuccessful response")
		http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
		return
	}

	claims, err := s.authClaimsFromToken(r.Context(), tokenResp.IDToken)
	if err != nil {
		s.clearAuthFlowCookies(w)
		http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
		return
	}

	if claims.Nonce == "" || !subtleCompare(nonceFromCookie, claims.Nonce) {
		s.logger.WithField("auth_subject", claims.Subject).Warn("auth callback nonce mismatch")
		s.clearAuthFlowCookies(w)
		http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
		return
	}

	userID, err := s.upsertIdentity(r.Context(), claims.Subject, claims.Email, claims.GivenName, claims.FamilyName)
	if err != nil {
		s.logger.WithError(err).WithField("auth_subject", claims.Subject).Error("failed to sync user identity in auth callback")
		s.internalServerError(w)
		return
	}

	var userType string
	user, err := s.authIdentityRepo.User(ctx, userID)
	if err == nil && user != nil && user.UserType != nil {
		userType = strings.TrimSpace(*user.UserType)
	}

	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = s.config.SessionMaxAgeSec
	}

	encryptedToken, err := s.cookie.Encode(internal.COOKIE_ACCESS_TOKEN_NAME, tokenResp.IDToken)
	if err != nil {
		s.logger.WithError(err).Error("failed to encrypt id token")
		s.internalServerError(w)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     internal.COOKIE_ACCESS_TOKEN_NAME,
		Value:    encryptedToken,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   expiresIn,
		Path:     "/",
	})

	s.setAuthUserStateCookie(w, authUserState{
		UserID:      userID,
		AuthSubject: claims.Subject,
		Email:       strings.TrimSpace(claims.Email),
		GivenName:   strings.TrimSpace(claims.GivenName),
		FamilyName:  strings.TrimSpace(claims.FamilyName),
		UserType:    userType,
		IsAdmin:     claims.IsAdmin,
	}, expiresIn)

	s.clearAuthFlowCookies(w)

	redirectCookie, err := r.Cookie(internal.COOKIE_REDIRECT_NAME)
	if err == nil {
		path := redirectCookie.Value
		s.clearRedirectCookie(w)
		http.Redirect(w, r, path, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, s.route(RouteHome, nil), http.StatusSeeOther)
}

func (s *Service) startAuth0Authorization(w http.ResponseWriter, r *http.Request, screenHint string) {
	state, err := generateAuthFlowToken()
	if err != nil {
		s.logger.WithError(err).Error("failed to generate auth state")
		s.internalServerError(w)
		return
	}
	nonce, err := generateAuthFlowToken()
	if err != nil {
		s.logger.WithError(err).Error("failed to generate auth nonce")
		s.internalServerError(w)
		return
	}

	age := int((5 * time.Minute).Seconds())
	encryptedState, err := s.cookie.Encode(internal.COOKIE_AUTH_STATE, state)
	if err != nil {
		s.logger.WithError(err).Error("failed to encrypt auth state")
		s.internalServerError(w)
		return
	}
	encryptedNonce, err := s.cookie.Encode(internal.COOKIE_AUTH_NONCE, nonce)
	if err != nil {
		s.logger.WithError(err).Error("failed to encrypt auth nonce")
		s.internalServerError(w)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     internal.COOKIE_AUTH_STATE,
		Value:    encryptedState,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   age,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     internal.COOKIE_AUTH_NONCE,
		Value:    encryptedNonce,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   age,
	})

	v := url.Values{}
	v.Set("response_type", "code")
	v.Set("client_id", s.config.Auth0ClientID)
	v.Set("redirect_uri", strings.TrimSpace(s.config.Auth0CallbackURL))
	v.Set("scope", "openid profile email")
	v.Set("state", state)
	v.Set("nonce", nonce)
	if audience := strings.TrimSpace(s.config.Auth0Audience); audience != "" {
		v.Set("audience", audience)
	}
	if hint := strings.TrimSpace(screenHint); hint != "" {
		v.Set("screen_hint", hint)
	}

	authorizeURL := strings.TrimRight(s.auth0DomainURL(), "/") + "/authorize?" + v.Encode()
	http.Redirect(w, r, authorizeURL, http.StatusSeeOther)
}

func (s *Service) clearAuthFlowCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     internal.COOKIE_AUTH_STATE,
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     internal.COOKIE_AUTH_NONCE,
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	})
}

func generateAuthFlowToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// subtleCompare is for secret callback values (state/nonce).
// A regular == check can leak timing information about how many leading bytes
// matched before the first mismatch; this comparison avoids that signal.
func subtleCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
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

func (s *Service) clearRedirectCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     internal.COOKIE_REDIRECT_NAME,
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	})
}

func (s *Service) clearAccessTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     internal.COOKIE_ACCESS_TOKEN_NAME,
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	})
}

func (s *Service) handlePostLogout(w http.ResponseWriter, r *http.Request) {
	s.clearAccessTokenCookie(w)
	s.clearAuthUserStateCookie(w)
	s.clearRedirectCookie(w)
	s.clearRegisterConfirmCookie(w)
	s.clearAuthFlowCookies(w)
	logoutURL := strings.TrimRight(s.auth0DomainURL(), "/") + "/v2/logout"
	v := url.Values{}
	v.Set("client_id", s.config.Auth0ClientID)
	v.Set("returnTo", strings.TrimSpace(s.config.Auth0LogoutURL))
	http.Redirect(w, r, logoutURL+"?"+v.Encode(), http.StatusSeeOther)
}

func (s *Service) auth0DomainURL() string {
	domain := strings.TrimSpace(s.config.Auth0Domain)
	if strings.HasPrefix(domain, "https://") || strings.HasPrefix(domain, "http://") {
		return strings.TrimRight(domain, "/")
	}
	return fmt.Sprintf("https://%s", strings.TrimRight(domain, "/"))
}

func (s *Service) setAuthUserStateCookie(w http.ResponseWriter, state authUserState, maxAge int) {
	encoded, err := s.cookie.Encode(internal.COOKIE_AUTH_USER_STATE, state)
	if err != nil {
		s.logger.WithError(err).Warn("failed to encode auth user state cookie")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     internal.COOKIE_AUTH_USER_STATE,
		Value:    encoded,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Path:     "/",
	})
}

func (s *Service) clearAuthUserStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     internal.COOKIE_AUTH_USER_STATE,
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	})
}

func (s *Service) updateAuthUserTypeCookie(w http.ResponseWriter, r *http.Request, userType string) {
	state, ok := s.authUserStateFromRequest(r)
	if !ok {
		return
	}

	state.UserType = strings.TrimSpace(userType)
	s.setAuthUserStateCookie(w, state, s.config.SessionMaxAgeSec)
}
