package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"christjesus/internal"

	"github.com/k0kubun/pp"
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

// RequireAuth middleware checks for valid access token and adds user to context
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Get the cookie
		cookie, err := r.Cookie(internal.COOKIE_ACCESS_TOKEN_NAME)
		if err != nil {
			s.logger.WithError(err).Debug("no access token cookie found")

			s.setRedirectCookie(w, r.URL.Path, time.Minute*5)

			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		// 2. Decrypt the token
		var accessToken string
		err = s.cookie.Decode(internal.COOKIE_ACCESS_TOKEN_NAME, cookie.Value, &accessToken)
		if err != nil {
			s.logger.WithError(err).Error("failed to decrypt access token")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// 3. Fetch JWK and verify JWT
		set, err := s.jwksCache.Lookup(r.Context(), s.jwksURL)
		if err != nil {
			s.logger.WithError(err).Error("failed to fetch JWKS")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		token, err := jwt.Parse(
			[]byte(accessToken),
			jwt.WithKeySet(set),
			jwt.WithValidate(true),
		)
		if err != nil {
			s.logger.WithError(err).Error("failed to parse JWT")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// 4. Extract user info from claims
		// Use Subject() for the standard "sub" claim
		userID, ok := token.Subject()
		if !ok || userID == "" {
			s.logger.Error("no user ID in JWT subject claim")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Use Get() for private/custom claims like "email"
		var email string
		if err := token.Get("email", &email); err != nil {
			s.logger.WithError(err).Warn("no email claim in JWT")
			// email is optional, so we don't redirect
		}

		// 5. Add user info to context
		ctx := r.Context()
		ctx = context.WithValue(ctx, contextKeyUserID, userID)
		if email != "" {
			ctx = context.WithValue(ctx, contextKeyEmail, email)
		}

		pp.Println(userID, email)

		s.logger.WithFields(logrus.Fields{
			"user_id": userID,
			"email":   email,
		}).Debug("authenticated user")

		// Continue to the next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
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
