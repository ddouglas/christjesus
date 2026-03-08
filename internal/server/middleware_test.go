package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAdmin_UnauthenticatedRedirectsToLogin(t *testing.T) {
	t.Parallel()

	s := &Service{}

	h := s.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}

	if got := rr.Header().Get("Location"); got != "/login" {
		t.Fatalf("Location = %q, want %q", got, "/login")
	}
}

func TestRequireAdmin_NonAdminForbidden(t *testing.T) {
	t.Parallel()

	s := &Service{}

	h := s.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserID, "user_123"))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestRequireAdmin_AdminPasses(t *testing.T) {
	t.Parallel()

	s := &Service{}

	called := false
	h := s.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := context.WithValue(req.Context(), contextKeyUserID, "admin_123")
	ctx = context.WithValue(ctx, contextKeyIsAdmin, true)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}
