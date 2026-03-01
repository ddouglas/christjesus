package server

import (
	"christjesus/internal"
	"christjesus/pkg/types"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	ctypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

func (s *Service) handleGetLogin(w http.ResponseWriter, r *http.Request) {

	_, err := r.Cookie(internal.COOKIE_ACCESS_TOKEN_NAME)
	if err == nil {
		s.logger.Info("user is already logged in, redirecting to Browse Needs")
		http.Redirect(w, r, "/browse", http.StatusSeeOther)
		return
	}

	data := &types.LoginPageData{
		BasePageData: types.BasePageData{Title: "Login"},
	}

	// Check if user was redirected here after confirming their email
	if r.URL.Query().Get("confirmed") == "true" || r.URL.Query().Get("confirmed") == "1" {
		data.Message = "Your account has been confirmed! You can now log in."
	}

	err = s.renderTemplate(w, r, "page.login", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render login page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostLogin(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	data := &types.LoginPageData{
		BasePageData: types.BasePageData{Title: "Login"},
		Email:        email,
	}

	if email == "" || password == "" {
		data.Error = "Email and password are required."
		err := s.renderTemplate(w, r, "page.login", data)
		if err != nil {
			s.logger.WithError(err).Error("failed to render login page with validation errors")
			s.internalServerError(w)
		}
		return
	}

	input := &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: ctypes.AuthFlowTypeUserPasswordAuth,
		ClientId: aws.String(s.config.CognitoClientID),
		AuthParameters: map[string]string{
			"USERNAME": email,
			"PASSWORD": password,
		},
	}

	resp, err := s.cognitoClient.InitiateAuth(r.Context(), input)
	if err != nil {
		var notAuthorized *ctypes.NotAuthorizedException
		var userNotFound *ctypes.UserNotFoundException
		var userNotConfirmed *ctypes.UserNotConfirmedException

		switch {
		case errors.As(err, &userNotConfirmed):
			data.Error = "Your account is not confirmed yet. Enter the verification code to continue."
			data.Message = "Check your email for a verification code."
			if renderErr := s.renderTemplate(w, r, "page.login", data); renderErr != nil {
				s.logger.WithError(renderErr).Error("failed to render login page with unconfirmed-user error")
				s.internalServerError(w)
			}
			return
		case errors.As(err, &notAuthorized), errors.As(err, &userNotFound):
			data.Error = "Invalid email or password."
		default:
			s.logger.WithError(err).Error("failed to login user")
			data.Error = "Unable to log in right now. Please try again."
		}

		if renderErr := s.renderTemplate(w, r, "page.login", data); renderErr != nil {
			s.logger.WithError(renderErr).Error("failed to render login page with auth errors")
			s.internalServerError(w)
		}
		return
	}

	if resp.AuthenticationResult == nil || resp.AuthenticationResult.IdToken == nil {
		data.Error = "Login failed. Please try again."
		err := s.renderTemplate(w, r, "page.login", data)
		if err != nil {
			s.logger.WithError(err).Error("failed to render login page with missing authentication result")
			s.internalServerError(w)
		}
		return
	}

	idToken := aws.ToString(resp.AuthenticationResult.IdToken)
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

	encryptedToken, err := s.cookie.Encode(internal.COOKIE_ACCESS_TOKEN_NAME, idToken)
	if err != nil {
		s.logger.WithError(err).Error("failed to encrypt id token")
		data.Error = "Login failed. Please try again."
		err = s.renderTemplate(w, r, "page.login", data)
		if err != nil {
			s.logger.WithError(err).Error("failed to render login page after token encryption failure")
			s.internalServerError(w)
		}
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
	s.clearRedirectCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
