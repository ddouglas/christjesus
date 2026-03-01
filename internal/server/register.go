package server

import (
	"christjesus/internal"
	"christjesus/pkg/types"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	ctypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	// "github.com/supabase-community/auth-go/types"
)

func (s *Service) handleGetRegister(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	_, err := r.Cookie(internal.COOKIE_ACCESS_TOKEN_NAME)
	if err == nil {
		s.logger.Info("user is already logged in, redirecting to home")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := &types.RegisterPageData{
		BasePageData: types.BasePageData{Title: "Register"},
	}

	err = s.renderTemplate(w, r, "page.register", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render register page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostRegister(w http.ResponseWriter, r *http.Request) {

	var ctx = r.Context()

	givenName := strings.TrimSpace(r.FormValue("given_name"))
	familyName := strings.TrimSpace(r.FormValue("family_name"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	data := &types.RegisterPageData{
		BasePageData: types.BasePageData{Title: "Create Account"},
		GivenName:    givenName,
		FamilyName:   familyName,
		Email:        email,
	}

	data.FieldErrors = validateRegisterInput(givenName, familyName, email, password, confirmPassword)
	if len(data.FieldErrors) > 0 {
		s.logger.WithField("field_errors", data.FieldErrors).Info("validation errors during registration")

		data.Error = "Please fix the highlighted fields."
		err := s.renderTemplate(w, r, "page.register", data)
		if err != nil {
			s.logger.WithError(err).Error("failed to render register page with validation errors")
			s.internalServerError(w)
		}

		return
	}

	input := &cognitoidentityprovider.SignUpInput{
		ClientId: aws.String(s.config.CognitoClientID),
		Username: aws.String(email), // use email as username
		Password: aws.String(password),
		UserAttributes: []ctypes.AttributeType{
			{Name: aws.String("email"), Value: aws.String(email)},
			{Name: aws.String("given_name"), Value: aws.String(givenName)},
			{Name: aws.String("family_name"), Value: aws.String(familyName)},
		},
	}

	_, err := s.cognitoClient.SignUp(ctx, input)
	if err != nil {
		s.logger.WithError(err).Error("failed to signup user")

		data.Error, data.FieldErrors = s.mapCognitoSignUpError(err)
		renderErr := s.renderTemplate(w, r, "page.register", data)
		if renderErr != nil {
			s.logger.WithError(renderErr).Error("failed to render register page with cognito errors")
			s.internalServerError(w)
		}
		return
	}

	v := url.Values{}
	v.Set("email", email)

	// Redirect to onboarding
	http.Redirect(w, r, fmt.Sprintf("/register/confirm?%s", v.Encode()), http.StatusSeeOther)

}

func (s *Service) handleGetRegisterConfirm(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	email := strings.TrimSpace(r.URL.Query().Get("email"))

	data := &types.ConfirmRegisterPageData{
		BasePageData: types.BasePageData{Title: "Confirm Your Account"},
		Email:        email,
	}

	err := s.renderTemplate(w, r, "page.register.confirm", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render register page")
		s.internalServerError(w)
		return
	}

}

func (s *Service) handlePostRegisterConfirm(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	email := strings.TrimSpace(r.FormValue("email"))
	code := strings.TrimSpace(r.FormValue("code"))

	data := &types.ConfirmRegisterPageData{
		BasePageData: types.BasePageData{Title: "Confirm Your Account"},
		Email:        email,
	}

	input := &cognitoidentityprovider.ConfirmSignUpInput{
		ClientId:         aws.String(s.config.CognitoClientID),
		Username:         aws.String(email),
		ConfirmationCode: aws.String(code),
	}

	_, err := s.cognitoClient.ConfirmSignUp(r.Context(), input)
	if err != nil {
		s.logger.WithError(err).Error("failed to confirm user signup")

		var codeMismatch *ctypes.CodeMismatchException
		if errors.As(err, &codeMismatch) {
			data.Error = "Invalid confirmation code. Please check the code and try again."
		} else {
			data.Error = "Unable to confirm account. Please try again."
		}

		err := s.renderTemplate(w, r, "page.register.confirm", data)
		if err != nil {
			s.logger.WithError(err).Error("failed to render register confirm page with error")
			s.internalServerError(w)
		}
		return
	}

	// Redirect to login after successful confirmation
	http.Redirect(w, r, "/login?confirmed=true", http.StatusSeeOther)
}

var (
	hasUpperReg  = regexp.MustCompile(`[A-Z]`)
	hasLowerReg  = regexp.MustCompile(`[a-z]`)
	hasDigitReg  = regexp.MustCompile(`[0-9]`)
	hasSymbolReg = regexp.MustCompile(`[^A-Za-z0-9]`)
)

func validateRegisterInput(givenName, familyName, email, password, confirmPassword string) map[string]string {
	errs := map[string]string{}

	givenName = strings.TrimSpace(givenName)
	familyName = strings.TrimSpace(familyName)
	email = strings.TrimSpace(email)

	if givenName == "" {
		errs["given_name"] = "First name is required."
	}

	if familyName == "" {
		errs["family_name"] = "Last name is required."
	}

	if email == "" {
		errs["email"] = "Email is required."
	} else if _, err := mail.ParseAddress(email); err != nil {
		errs["email"] = "Enter a valid email address."
	}

	if password != confirmPassword {
		errs["confirm_password"] = "Passwords do not match."
	}

	hasUpper := hasUpperReg.MatchString(password)
	hasLower := hasLowerReg.MatchString(password)
	hasDigit := hasDigitReg.MatchString(password)
	hasSymbol := hasSymbolReg.MatchString(password)

	if len(password) < 12 || !hasUpper || !hasLower || !hasDigit || !hasSymbol {
		errs["password"] = "Password must be at least 12 characters and include uppercase, lowercase, number, and symbol."
	}

	return errs
}

func (s *Service) mapCognitoSignUpError(err error) (string, map[string]string) {
	fieldErrs := map[string]string{}

	var invalidPw *ctypes.InvalidPasswordException
	if errors.As(err, &invalidPw) {
		fieldErrs["password"] = "Password must include uppercase, lowercase, number, and symbol (min 12)."
		return "Please fix the highlighted fields.", fieldErrs
	}

	var userExists *ctypes.UsernameExistsException
	if errors.As(err, &userExists) {
		fieldErrs["email"] = "An account with this email already exists."
		return "Try logging in instead.", fieldErrs
	}

	var invalidParam *ctypes.InvalidParameterException
	if errors.As(err, &invalidParam) {
		return "Some details are invalid. Please review and try again.", fieldErrs
	}

	s.logger.WithError(err).Error("unhandled cognito signup error")

	return "Unable to create account right now. Please try again.", fieldErrs
}

// func (s *Service) handleRegisterSponsor(w http.ResponseWriter, r *http.Request) {
// 	var _ = r.Context()

// 	err := s.templates.ExecuteTemplate(w, "page.register.sponsor", nil)
// 	if err != nil {
// 		s.logger.WithError(err).Error("failed to render register sponsor page")
// 		s.internalServerError(w)
// 		return
// 	}
// }
