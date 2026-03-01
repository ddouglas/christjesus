package server

import (
	"christjesus/internal"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
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

	input := &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: types.AuthFlowTypeUserPasswordAuth,
		ClientId: aws.String(s.config.CognitoClientID),
		AuthParameters: map[string]string{
			"USERNAME": email,
			"PASSWORD": password,
		},
	}

	resp, err := s.cognitoClient.InitiateAuth(r.Context(), input)
	if err != nil {
		// NotAuthorizedException, UserNotConfirmedException, etc.
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if resp.AuthenticationResult == nil || resp.AuthenticationResult.AccessToken == nil {
		http.Error(w, "Login failed", http.StatusUnauthorized)
		return
	}

	accessToken := aws.ToString(resp.AuthenticationResult.AccessToken)
	expiresIn := int(resp.AuthenticationResult.ExpiresIn)

	// // Sign in with Supabase
	// resp, err := s.supauth.SignInWithEmailPassword(email, password)

	// if err != nil {
	// 	s.logger.WithError(err).Error("failed to login user")
	// 	http.Error(w, "Login failed: "+err.Error(), http.StatusUnauthorized)
	// 	return
	// }

	// // Success! resp contains User and Session with AccessToken
	// s.logger.WithField("user_id", resp.User.ID).Info("user logged in")

	encryptedToken, err := s.cookie.Encode(internal.COOKIE_ACCESS_TOKEN_NAME, accessToken)
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
		MaxAge:   expiresIn,
		Path:     "/",
	})

	// Check to see if this login attempt was the result of an unauthed redirect
	redirectCookie, err := r.Cookie(internal.COOKIE_REDIRECT_NAME)
	if err == nil {
		// Cookie found, grab the value, clear the cookie
		path := redirectCookie.Value
		s.clearRedirectCookie(w)
		http.Redirect(w, r, path, http.StatusSeeOther)
		return
	}

	// Redirect back to the homepage
	http.Redirect(w, r, "/", http.StatusSeeOther)
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
