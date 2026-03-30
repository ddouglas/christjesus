package email_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"christjesus/internal/email"
)

func TestResendSender_Send(t *testing.T) {
	t.Run("successful send returns message ID", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Bearer test-api-key" {
				t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("unexpected Content-Type header: %s", r.Header.Get("Content-Type"))
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "msg_abc123"})
		}))
		defer srv.Close()

		sender := email.NewResendSender("test-api-key", email.WithBaseURL(srv.URL))

		msg := email.Message{
			To:       "recipient@example.com",
			From:     "noreply@example.com",
			Subject:  "Test Subject",
			HTMLBody: "<p>Hello</p>",
		}

		result, err := sender.Send(context.Background(), msg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ProviderMessageID != "msg_abc123" {
			t.Errorf("expected message ID msg_abc123, got %s", result.ProviderMessageID)
		}
	})

	t.Run("API error response returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"name":    "validation_error",
				"message": "invalid email address",
			})
		}))
		defer srv.Close()

		sender := email.NewResendSender("test-api-key", email.WithBaseURL(srv.URL))

		msg := email.Message{
			To:      "bad-address",
			From:    "noreply@example.com",
			Subject: "Test",
		}

		_, err := sender.Send(context.Background(), msg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty API key returns error without making HTTP call", func(t *testing.T) {
		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		sender := email.NewResendSender("", email.WithBaseURL(srv.URL))

		_, err := sender.Send(context.Background(), email.Message{
			To:      "recipient@example.com",
			From:    "noreply@example.com",
			Subject: "Test",
		})
		if err == nil {
			t.Fatal("expected error for empty API key, got nil")
		}
		if called {
			t.Error("HTTP server should not have been called with empty API key")
		}
	})
}
