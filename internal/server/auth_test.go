package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"christjesus/internal"
	"christjesus/pkg/types"

	"github.com/gorilla/securecookie"
	"github.com/sirupsen/logrus"
)

func TestSubtleCompare_CallbackSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		a      string
		b      string
		match  bool
		reason string
	}{
		{
			name:   "state exact match",
			a:      "oauthed-state-value",
			b:      "oauthed-state-value",
			match:  true,
			reason: "callback state should pass only on exact equality",
		},
		{
			name:   "state mismatch",
			a:      "oauthed-state-value",
			b:      "oauthed-state-value-x",
			match:  false,
			reason: "callback state mismatch must reject login",
		},
		{
			name:   "nonce exact match",
			a:      "nonce-123456",
			b:      "nonce-123456",
			match:  true,
			reason: "id token nonce should match cookie nonce",
		},
		{
			name:   "nonce same prefix but different tail",
			a:      "nonce-123456",
			b:      "nonce-123457",
			match:  false,
			reason: "near matches are still invalid",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := subtleCompare(tt.a, tt.b)
			if got != tt.match {
				t.Fatalf("subtleCompare(%q, %q) = %v, want %v (%s)", tt.a, tt.b, got, tt.match, tt.reason)
			}
		})
	}
}

func TestHandleGetLogin_InvalidAccessTokenCookieClearsAndRestartsAuth(t *testing.T) {
	t.Parallel()

	s := &Service{
		config: &types.Config{
			Auth0ClientID:    "client_123",
			Auth0CallbackURL: "http://localhost:8080/auth/callback",
			Auth0Domain:      "example.us.auth0.com",
		},
		cookie: securecookie.New([]byte("0123456789abcdef0123456789abcdef"), []byte("abcdef0123456789abcdef0123456789")),
		logger: logrus.New(),
	}

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.AddCookie(&http.Cookie{Name: internal.COOKIE_ACCESS_TOKEN_NAME, Value: "not-a-valid-secure-cookie"})
	rr := httptest.NewRecorder()

	s.handleGetLogin(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}

	location := rr.Header().Get("Location")
	if !strings.HasPrefix(location, "https://example.us.auth0.com/authorize?") {
		t.Fatalf("Location = %q, want Auth0 authorize redirect", location)
	}

	cookies := rr.Result().Cookies()
	var cleared bool
	for _, c := range cookies {
		if c.Name == internal.COOKIE_ACCESS_TOKEN_NAME && c.MaxAge < 0 {
			cleared = true
			break
		}
	}
	if !cleared {
		t.Fatal("expected invalid access token cookie to be cleared")
	}
}
