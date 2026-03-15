package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"christjesus/internal"
	"christjesus/pkg/types"

	"github.com/gorilla/securecookie"
	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/sirupsen/logrus"
)

type mockAuthIdentityRepo struct {
	upsertIdentityFn func(ctx context.Context, authSubject, email, givenName, familyName string) (string, error)
	userFn           func(ctx context.Context, userID string) (*types.User, error)
}

func (m *mockAuthIdentityRepo) UpsertIdentity(ctx context.Context, authSubject, email, givenName, familyName string) (string, error) {
	if m.upsertIdentityFn == nil {
		return "", errors.New("upsertIdentityFn not configured")
	}
	return m.upsertIdentityFn(ctx, authSubject, email, givenName, familyName)
}

func (m *mockAuthIdentityRepo) User(ctx context.Context, userID string) (*types.User, error) {
	if m.userFn == nil {
		return nil, errors.New("userFn not configured")
	}
	return m.userFn(ctx, userID)
}

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

// Ensures the login entrypoint does not trust cookie presence alone.
// Benefit: users with corrupted/expired cookies are recovered by clearing the cookie and restarting auth.
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

// Exercises the full happy path for Auth0 callback handling.
// Benefit: verifies state/nonce flow, token exchange, token validation, identity sync, and cookie issuance all work together.
func TestHandleGetAuthCallback_Success(t *testing.T) {
	t.Parallel()

	state := "state-123"
	nonce := "nonce-123"
	issuer := "https://issuer.example/"
	clientID := "client_123"

	idToken, jwksURL := buildSignedIDTokenAndJWKS(t, issuer, clientID, nonce)

	auth0 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id_token":   idToken,
			"expires_in": 3600,
		})
	}))
	t.Cleanup(auth0.Close)

	s := newAuthCallbackTestService(t, auth0.URL, issuer, clientID, jwksURL)
	mockRepo := &mockAuthIdentityRepo{}
	mockRepo.upsertIdentityFn = func(ctx context.Context, authSubject, email, givenName, familyName string) (string, error) {
		if authSubject == "" {
			t.Fatal("expected non-empty auth subject")
		}
		return "user_123", nil
	}
	mockRepo.userFn = func(ctx context.Context, userID string) (*types.User, error) {
		if userID == "" {
			t.Fatal("expected non-empty user id")
		}
		return &types.User{ID: userID}, nil
	}
	s.authIdentityRepo = mockRepo

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&code=abc123", nil)
	addEncodedCookie(t, s, req, internal.COOKIE_AUTH_STATE, state)
	addEncodedCookie(t, s, req, internal.COOKIE_AUTH_NONCE, nonce)
	rr := httptest.NewRecorder()

	s.handleGetAuthCallback(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if got := rr.Header().Get("Location"); got != "/" {
		t.Fatalf("Location = %q, want %q", got, "/")
	}

	cookies := rr.Result().Cookies()
	assertCookiePresent(t, cookies, internal.COOKIE_ACCESS_TOKEN_NAME)
	assertCookieCleared(t, cookies, internal.COOKIE_AUTH_STATE)
	assertCookieCleared(t, cookies, internal.COOKIE_AUTH_NONCE)
}

// Verifies callback rejects when query state and cookie state do not match.
// Benefit: protects against CSRF/state-fixation attacks in the auth callback.
func TestHandleGetAuthCallback_StateMismatch(t *testing.T) {
	t.Parallel()

	jwksURL := newJWKSStubURL(t)
	s := newAuthCallbackTestService(t, "http://127.0.0.1:1", "https://issuer.example/", "client_123", jwksURL)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=from-query&code=abc123", nil)
	addEncodedCookie(t, s, req, internal.COOKIE_AUTH_STATE, "different-state")
	addEncodedCookie(t, s, req, internal.COOKIE_AUTH_NONCE, "nonce-123")
	rr := httptest.NewRecorder()

	s.handleGetAuthCallback(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if got := rr.Header().Get("Location"); got != "/login" {
		t.Fatalf("Location = %q, want %q", got, "/login")
	}
}

// Verifies callback rejects when token nonce and nonce cookie do not match.
// Benefit: ensures replayed/mismatched ID tokens cannot complete login.
func TestHandleGetAuthCallback_NonceMismatch(t *testing.T) {
	t.Parallel()

	state := "state-123"
	issuer := "https://issuer.example/"
	clientID := "client_123"

	idToken, jwksURL := buildSignedIDTokenAndJWKS(t, issuer, clientID, "nonce-from-token")

	auth0 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id_token":   idToken,
			"expires_in": 3600,
		})
	}))
	t.Cleanup(auth0.Close)

	s := newAuthCallbackTestService(t, auth0.URL, issuer, clientID, jwksURL)
	mockRepo := &mockAuthIdentityRepo{}
	mockRepo.upsertIdentityFn = func(ctx context.Context, authSubject, email, givenName, familyName string) (string, error) {
		t.Fatal("upsertIdentity should not be called on nonce mismatch")
		return "", nil
	}
	mockRepo.userFn = func(ctx context.Context, userID string) (*types.User, error) {
		return &types.User{ID: userID}, nil
	}
	s.authIdentityRepo = mockRepo

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&code=abc123", nil)
	addEncodedCookie(t, s, req, internal.COOKIE_AUTH_STATE, state)
	addEncodedCookie(t, s, req, internal.COOKIE_AUTH_NONCE, "different-nonce")
	rr := httptest.NewRecorder()

	s.handleGetAuthCallback(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if got := rr.Header().Get("Location"); got != "/login" {
		t.Fatalf("Location = %q, want %q", got, "/login")
	}
}

// Verifies callback behavior when Auth0 token exchange fails.
// Benefit: ensures external auth failures fail closed and safely redirect back to login.
func TestHandleGetAuthCallback_TokenExchangeFailure(t *testing.T) {
	t.Parallel()

	state := "state-123"
	nonce := "nonce-123"
	auth0 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant"})
	}))
	t.Cleanup(auth0.Close)

	jwksURL := newJWKSStubURL(t)
	s := newAuthCallbackTestService(t, auth0.URL, "https://issuer.example/", "client_123", jwksURL)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&code=abc123", nil)
	addEncodedCookie(t, s, req, internal.COOKIE_AUTH_STATE, state)
	addEncodedCookie(t, s, req, internal.COOKIE_AUTH_NONCE, nonce)
	rr := httptest.NewRecorder()

	s.handleGetAuthCallback(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if got := rr.Header().Get("Location"); got != "/login" {
		t.Fatalf("Location = %q, want %q", got, "/login")
	}
}

func newAuthCallbackTestService(t *testing.T, auth0Domain, issuer, clientID, jwksURL string) *Service {
	t.Helper()

	hashKey := []byte("0123456789abcdef0123456789abcdef")
	blockKey := []byte("abcdef0123456789abcdef0123456789")

	var cache *jwk.Cache
	if strings.TrimSpace(jwksURL) != "" {
		var err error
		cache, err = jwk.NewCache(context.Background(), httprc.NewClient())
		if err != nil {
			t.Fatalf("failed to create jwk cache: %v", err)
		}
		if err := cache.Register(context.Background(), jwksURL); err != nil {
			t.Fatalf("failed to register jwks url: %v", err)
		}
	}

	return &Service{
		config: &types.Config{
			Auth0ClientID:     clientID,
			Auth0ClientSecret: "secret_123",
			Auth0CallbackURL:  "http://localhost:8080/auth/callback",
			Auth0Domain:       auth0Domain,
			AuthIssuerURL:     issuer,
			AuthClientID:      clientID,
			SessionMaxAgeSec:  3600,
		},
		cookie:     securecookie.New(hashKey, blockKey),
		logger:     logrus.New(),
		jwksCache:  cache,
		jwksURL:    jwksURL,
		httpClient: &http.Client{Timeout: 250 * time.Millisecond},
		authIdentityRepo: &mockAuthIdentityRepo{
			upsertIdentityFn: func(ctx context.Context, authSubject, email, givenName, familyName string) (string, error) {
				return "user_123", nil
			},
			userFn: func(ctx context.Context, userID string) (*types.User, error) {
				return &types.User{ID: userID}, nil
			},
		},
	}
}

func newJWKSStubURL(t *testing.T) string {
	t.Helper()

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jwks" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	t.Cleanup(stub.Close)

	return stub.URL + "/jwks"
}

func buildSignedIDTokenAndJWKS(t *testing.T, issuer, audience, nonce string) (string, string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed generating rsa key: %v", err)
	}

	kid := "test-key-1"

	pubKey, err := jwk.Import(priv.PublicKey)
	if err != nil {
		t.Fatalf("failed importing public key to jwk: %v", err)
	}
	if err := pubKey.Set(jwk.KeyIDKey, kid); err != nil {
		t.Fatalf("failed setting kid on public key: %v", err)
	}
	if err := pubKey.Set(jwk.AlgorithmKey, jwa.RS256()); err != nil {
		t.Fatalf("failed setting alg on public key: %v", err)
	}
	if err := pubKey.Set(jwk.KeyUsageKey, "sig"); err != nil {
		t.Fatalf("failed setting use on public key: %v", err)
	}

	set := jwk.NewSet()
	if err := set.AddKey(pubKey); err != nil {
		t.Fatalf("failed adding key to jwk set: %v", err)
	}

	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jwks" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(jwks.Close)

	privKey, err := jwk.Import(priv)
	if err != nil {
		t.Fatalf("failed importing private key to jwk: %v", err)
	}
	if err := privKey.Set(jwk.KeyIDKey, kid); err != nil {
		t.Fatalf("failed setting kid on private key: %v", err)
	}
	if err := privKey.Set(jwk.AlgorithmKey, jwa.RS256()); err != nil {
		t.Fatalf("failed setting alg on private key: %v", err)
	}

	tok, err := jwt.NewBuilder().
		Subject("auth0|user-123").
		Issuer(issuer).
		Audience([]string{audience}).
		IssuedAt(time.Now()).
		Expiration(time.Now().Add(time.Hour)).
		Claim("nonce", nonce).
		Claim("email", "user@example.com").
		Claim("given_name", "Test").
		Claim("family_name", "User").
		Build()
	if err != nil {
		t.Fatalf("failed building jwt: %v", err)
	}

	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), privKey))
	if err != nil {
		t.Fatalf("failed signing jwt: %v", err)
	}

	return string(signed), jwks.URL + "/jwks"
}

func addEncodedCookie(t *testing.T, s *Service, req *http.Request, name, value string) {
	t.Helper()
	encoded, err := s.cookie.Encode(name, value)
	if err != nil {
		t.Fatalf("failed to encode cookie %s: %v", name, err)
	}
	req.AddCookie(&http.Cookie{Name: name, Value: encoded})
}

func assertCookiePresent(t *testing.T, cookies []*http.Cookie, name string) {
	t.Helper()
	for _, c := range cookies {
		if c.Name == name && c.Value != "" {
			return
		}
	}
	t.Fatalf("expected cookie %s to be present", name)
}

func assertCookieCleared(t *testing.T, cookies []*http.Cookie, name string) {
	t.Helper()
	for _, c := range cookies {
		if c.Name == name && c.MaxAge < 0 {
			return
		}
	}
	t.Fatalf("expected cookie %s to be cleared", name)
}
