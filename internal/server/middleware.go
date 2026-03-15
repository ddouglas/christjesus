package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"christjesus/internal"
	"christjesus/pkg/types"

	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/sirupsen/logrus"
)

// Context key types to avoid collisions
type contextKey string

const (
	contextKeyUserID   contextKey = "user_id"
	contextKeyEmail    contextKey = "email"
	contextKeyUserName contextKey = "user_name"
	contextKeyUserType contextKey = "user_type"
	contextKeyIsAdmin  contextKey = "is_admin"
	contextKeyUser     contextKey = "user"
)

type authUserState struct {
	UserID      string
	AuthSubject string
	Email       string
	GivenName   string
	FamilyName  string
	UserType    string
	IsAdmin     bool
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

type AuthClaims struct {
	Subject    string
	Email      string
	GivenName  string
	FamilyName string
	Nonce      string
	IsAdmin    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// SecurityHeaders sets baseline defensive HTTP headers on every response.
func (s *Service) SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevents browsers from MIME-sniffing the Content-Type, which can
		// lead to XSS if a user-uploaded file is interpreted as HTML/JS.
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Blocks this site from being embedded in an iframe on another
		// origin, preventing clickjacking attacks.
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")

		// Controls how much referrer information is sent with navigations.
		// "strict-origin-when-cross-origin" sends the full path to same-origin
		// destinations but only the origin (no path) to cross-origin ones.
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// CSP frame-ancestors directive is the modern replacement for
		// X-Frame-Options. Both are set for backward compatibility with
		// older browsers that don't support CSP.
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'")

		next.ServeHTTP(w, r)
	})
}

func (s *Service) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		s.logger.WithFields(logrus.Fields{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      rw.statusCode,
			"duration_ms": time.Since(started).Milliseconds(),
		}).Info("http request")
	})
}

func (s *Service) AttachAuthContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := s.authClaimsFromRequest(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		state, ok := s.authUserStateFromRequest(r)
		if !ok || strings.TrimSpace(state.AuthSubject) != strings.TrimSpace(claims.Subject) || strings.TrimSpace(state.UserID) == "" {
			s.clearAccessTokenCookie(w)
			s.clearAuthUserStateCookie(w)
			next.ServeHTTP(w, r)
			return
		}

		userID := strings.TrimSpace(state.UserID)
		email := strings.TrimSpace(state.Email)
		if email == "" {
			email = strings.TrimSpace(claims.Email)
		}
		givenName := strings.TrimSpace(state.GivenName)
		if givenName == "" {
			givenName = strings.TrimSpace(claims.GivenName)
		}
		familyName := strings.TrimSpace(state.FamilyName)
		if familyName == "" {
			familyName = strings.TrimSpace(claims.FamilyName)
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, contextKeyUserID, userID)
		if email != "" {
			ctx = context.WithValue(ctx, contextKeyEmail, email)
		}
		if givenName != "" {
			ctx = context.WithValue(ctx, contextKeyUserName, givenName)
		}
		if strings.TrimSpace(state.UserType) != "" {
			ctx = context.WithValue(ctx, contextKeyUserType, strings.TrimSpace(state.UserType))
		}
		ctx = context.WithValue(ctx, contextKeyIsAdmin, claims.IsAdmin)

		var userTypePtr *string
		if strings.TrimSpace(state.UserType) != "" {
			userType := strings.TrimSpace(state.UserType)
			userTypePtr = &userType
		}
		var emailPtr *string
		if email != "" {
			em := email
			emailPtr = &em
		}
		var givenNamePtr *string
		if givenName != "" {
			gn := givenName
			givenNamePtr = &gn
		}
		var familyNamePtr *string
		if familyName != "" {
			fn := familyName
			familyNamePtr = &fn
		}
		ctx = context.WithValue(ctx, contextKeyUser, types.User{
			ID:         userID,
			UserType:   userTypePtr,
			Email:      emailPtr,
			GivenName:  givenNamePtr,
			FamilyName: familyNamePtr,
		})

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) authUserStateFromRequest(r *http.Request) (authUserState, bool) {
	var state authUserState

	cookie, err := r.Cookie(internal.COOKIE_AUTH_USER_STATE)
	if err != nil {
		return state, false
	}

	if err := s.cookie.Decode(internal.COOKIE_AUTH_USER_STATE, cookie.Value, &state); err != nil {
		return authUserState{}, false
	}

	return state, true
}

func (s *Service) authClaimsFromRequest(r *http.Request) (AuthClaims, bool) {
	empty := AuthClaims{}

	// 1. Get cookie
	cookie, err := r.Cookie(internal.COOKIE_ACCESS_TOKEN_NAME)
	if err != nil {
		return empty, false
	}

	// 2. Decrypt token
	var accessToken string
	err = s.cookie.Decode(internal.COOKIE_ACCESS_TOKEN_NAME, cookie.Value, &accessToken)
	if err != nil {
		s.logger.WithError(err).Debug("failed to decrypt access token")
		return empty, false
	}

	claims, err := s.authClaimsFromToken(r.Context(), accessToken)
	if err != nil {
		return empty, false
	}

	return claims, true
}

func (s *Service) authClaimsFromToken(ctx context.Context, tokenString string) (AuthClaims, error) {
	empty := AuthClaims{}

	set, err := s.jwksCache.Lookup(ctx, s.jwksURL)
	if err != nil {
		s.logger.WithError(err).Warn("failed to fetch JWKS")
		return empty, err
	}

	token, err := jwt.Parse(
		[]byte(tokenString),
		jwt.WithKeySet(set),
		jwt.WithValidate(true),
		jwt.WithIssuer(strings.TrimSpace(s.config.AuthIssuerURL)),
		jwt.WithAudience(strings.TrimSpace(s.config.AuthClientID)),
	)
	if err != nil {
		s.logger.WithError(err).WithField("auth_issuer", strings.TrimSpace(s.config.AuthIssuerURL)).Warn("failed to parse JWT")
		return empty, err
	}

	subject, ok := token.Subject()
	if !ok || strings.TrimSpace(subject) == "" {
		return empty, errors.New("missing subject claim")
	}

	var email string
	if err := token.Get("email", &email); err != nil {
		email = ""
	}

	var givenName string
	if err := token.Get("given_name", &givenName); err != nil {
		givenName = ""
	}
	givenName = strings.TrimSpace(givenName)

	var familyName string
	if err := token.Get("family_name", &familyName); err != nil {
		familyName = ""
	}
	familyName = strings.TrimSpace(familyName)

	var nonce string
	if err := token.Get("nonce", &nonce); err != nil {
		nonce = ""
	}
	nonce = strings.TrimSpace(nonce)

	isAdmin := false
	adminClaim := strings.TrimSpace(s.config.AuthAdminClaim)
	adminValue := strings.TrimSpace(s.config.AuthAdminValue)
	if adminClaim != "" {
		if groups, ok := tokenStringSliceClaim(token, adminClaim); ok {
			for _, group := range groups {
				if adminValue != "" && strings.EqualFold(strings.TrimSpace(group), adminValue) {
					isAdmin = true
					break
				}
			}
		}
	}

	return AuthClaims{
		Subject:    subject,
		Email:      email,
		GivenName:  givenName,
		FamilyName: familyName,
		Nonce:      nonce,
		IsAdmin:    isAdmin,
	}, nil
}

func tokenStringSliceClaim(token jwt.Token, claim string) ([]string, bool) {
	var raw any
	if err := token.Get(claim, &raw); err != nil {
		return nil, false
	}

	switch v := raw.(type) {
	case []string:
		if len(v) == 0 {
			return nil, false
		}
		return v, true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				trimmed := strings.TrimSpace(s)
				if trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, false
		}
		return []string{trimmed}, true
	default:
		return nil, false
	}
}

// RequireAuth middleware checks for valid access token and adds user to context
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(contextKeyUserID).(string)
		if !ok || userID == "" {
			s.setRedirectCookie(w, r.URL.Path, time.Minute*5)
			http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
			return
		}

		if !strings.HasPrefix(r.URL.Path, RoutePattern(RouteOnboarding)) {
			userType, _ := r.Context().Value(contextKeyUserType).(string)
			if strings.TrimSpace(userType) == "" {
				http.Redirect(w, r, s.route(RouteOnboarding, nil), http.StatusSeeOther)
				return
			}
		}

		email, _ := r.Context().Value(contextKeyEmail).(string)

		// 5. Add user info to context
		s.logger.WithFields(logrus.Fields{
			"user_id": userID,
			"email":   email,
		}).Debug("authenticated user")

		// Continue to the next handler
		next.ServeHTTP(w, r)

	})
}

// RequireAdmin middleware enforces authenticated admin access.
func (s *Service) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(contextKeyUserID).(string)
		if !ok || userID == "" {
			s.setRedirectCookie(w, r.URL.Path, time.Minute*5)
			http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
			return
		}

		isAdmin, ok := r.Context().Value(contextKeyIsAdmin).(bool)
		if !ok || !isAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Service) StripTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Only strip if path is not root and has trailing slash
		if path != "/" && strings.HasSuffix(path, "/") {
			// Build redirect URL
			newPath := strings.TrimSuffix(path, "/")
			newURL := *r.URL
			newURL.Path = newPath

			// Preserve query string
			http.Redirect(w, r, newURL.String(), http.StatusMovedPermanently)
			return
		}

		next.ServeHTTP(w, r)
	})
}
