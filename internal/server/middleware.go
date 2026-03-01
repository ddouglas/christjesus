package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"christjesus/internal"

	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/sirupsen/logrus"
)

// Context key types to avoid collisions
type contextKey string

const (
	contextKeyUserID contextKey = "user_id"
	contextKeyEmail  contextKey = "email"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
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
		userID, email, ok := s.authClaimsFromRequest(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, contextKeyUserID, userID)
		if email != "" {
			ctx = context.WithValue(ctx, contextKeyEmail, email)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) authClaimsFromRequest(r *http.Request) (string, string, bool) {
	// 1. Get cookie
	cookie, err := r.Cookie(internal.COOKIE_ACCESS_TOKEN_NAME)
	if err != nil {
		return "", "", false
	}

	// 2. Decrypt token
	var accessToken string
	err = s.cookie.Decode(internal.COOKIE_ACCESS_TOKEN_NAME, cookie.Value, &accessToken)
	if err != nil {
		s.logger.WithError(err).Debug("failed to decrypt access token")
		return "", "", false
	}

	// 3. Validate token
	set, err := s.jwksCache.Lookup(r.Context(), s.jwksURL)
	if err != nil {
		s.logger.WithError(err).Warn("failed to fetch JWKS")
		return "", "", false
	}

	token, err := jwt.Parse(
		[]byte(accessToken),
		jwt.WithKeySet(set),
		jwt.WithValidate(true),
		jwt.WithIssuer(s.config.CognitoIssuerURL),
		jwt.WithAudience(s.config.CognitoClientID),
	)
	if err != nil {
		s.logger.WithError(err).Debug("failed to parse JWT")
		return "", "", false
	}

	userID, ok := token.Subject()
	if !ok || userID == "" {
		return "", "", false
	}

	var tokenUse string
	if err := token.Get("token_use", &tokenUse); err != nil || tokenUse != "id" {
		return "", "", false
	}

	var email string
	if err := token.Get("email", &email); err != nil {
		email = ""
	}

	return userID, email, true
}

// RequireAuth middleware checks for valid access token and adds user to context
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(contextKeyUserID).(string)
		if !ok || userID == "" {
			s.setRedirectCookie(w, r.URL.Path, time.Minute*5)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
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
