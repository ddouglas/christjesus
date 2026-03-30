package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	svix "github.com/svix/svix-webhooks/go"

	"christjesus/internal/server"
	"christjesus/pkg/types"
)

// testWebhookSecret is a valid whsec_ prefixed base64 key for tests.
// Generated with: openssl rand -base64 32
const testWebhookSecret = "whsec_dGVzdHNlY3JldGtleWZvcnVuaXR0ZXN0aW5n"

// signPayload generates valid Svix headers for the given payload, secret, and message ID.
func signPayload(t *testing.T, secret, msgID string, payload []byte) http.Header {
	t.Helper()
	wh, err := svix.NewWebhook(secret)
	if err != nil {
		t.Fatalf("create test webhook signer: %v", err)
	}
	ts := time.Now()
	sig, err := wh.Sign(msgID, ts, payload)
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	h := http.Header{}
	h.Set("svix-id", msgID)
	h.Set("svix-timestamp", fmt.Sprintf("%d", ts.Unix()))
	h.Set("svix-signature", sig)
	h.Set("Content-Type", "application/json")
	return h
}

// --- fake email repository ---

type fakeEmailRepo struct {
	insertedEvents      []*types.EmailEvent
	insertedSuppressions []*types.EmailSuppression
	statusUpdates       []statusUpdate
	// controls InsertEmailEvent idempotency
	existingEventIDs map[string]bool
	// controls EmailMessageByProviderMessageID
	messagesByProviderID map[string]*types.EmailMessage
}

type statusUpdate struct {
	id     string
	status string
}

func (f *fakeEmailRepo) InsertEmailEvent(ctx context.Context, event *types.EmailEvent) (bool, error) {
	if f.existingEventIDs[event.ProviderEventID] {
		return false, nil
	}
	f.insertedEvents = append(f.insertedEvents, event)
	return true, nil
}

func (f *fakeEmailRepo) EmailMessageByProviderMessageID(ctx context.Context, providerMessageID string) (*types.EmailMessage, error) {
	if f.messagesByProviderID != nil {
		if msg, ok := f.messagesByProviderID[providerMessageID]; ok {
			return msg, nil
		}
	}
	return nil, nil
}

func (f *fakeEmailRepo) UpdateEmailMessageStatus(ctx context.Context, id, status string, providerMessageID *string) error {
	f.statusUpdates = append(f.statusUpdates, statusUpdate{id: id, status: status})
	return nil
}

func (f *fakeEmailRepo) UpsertEmailSuppression(ctx context.Context, suppression *types.EmailSuppression) error {
	f.insertedSuppressions = append(f.insertedSuppressions, suppression)
	return nil
}

// --- helper to build a signed webhook request ---

func newWebhookRequest(t *testing.T, secret, msgID string, payload any) *http.Request {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal webhook payload: %v", err)
	}
	if msgID == "" {
		msgID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
	}
	headers := signPayload(t, secret, msgID, body)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/resend", bytes.NewReader(body))
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Set(k, v)
		}
	}
	return req
}

// --- tests ---

func TestHandlePostResendWebhook_MissingSecret(t *testing.T) {
	repo := &fakeEmailRepo{}
	handler := server.NewResendWebhookHandler("", repo)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/resend", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandlePostResendWebhook_InvalidSignature(t *testing.T) {
	repo := &fakeEmailRepo{}
	handler := server.NewResendWebhookHandler(testWebhookSecret, repo)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/resend", strings.NewReader(`{"type":"email.delivered"}`))
	req.Header.Set("svix-id", "msg_bad")
	req.Header.Set("svix-timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	req.Header.Set("svix-signature", "v1,badsignature==")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandlePostResendWebhook_DuplicateEvent(t *testing.T) {
	repo := &fakeEmailRepo{
		existingEventIDs: map[string]bool{"msg_existing": true},
	}
	handler := server.NewResendWebhookHandler(testWebhookSecret, repo)

	payload := map[string]any{"type": "email.delivered", "data": map[string]any{"email_id": "msg_existing"}}
	req := newWebhookRequest(t, testWebhookSecret, "msg_existing", payload)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for duplicate event, got %d", w.Code)
	}
	if len(repo.statusUpdates) != 0 {
		t.Error("duplicate event should not trigger status update")
	}
}

func TestHandlePostResendWebhook_Delivered(t *testing.T) {
	msgID := "msg_test_delivered"
	repo := &fakeEmailRepo{
		messagesByProviderID: map[string]*types.EmailMessage{
			msgID: {ID: "local_msg_1", Status: types.EmailStatusSent},
		},
	}
	handler := server.NewResendWebhookHandler(testWebhookSecret, repo)

	payload := map[string]any{
		"type": "email.delivered",
		"data": map[string]any{"email_id": msgID},
	}
	req := newWebhookRequest(t, testWebhookSecret, "", payload)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(repo.statusUpdates) != 1 {
		t.Fatalf("expected 1 status update, got %d", len(repo.statusUpdates))
	}
	if repo.statusUpdates[0].status != types.EmailStatusDelivered {
		t.Errorf("expected status %q, got %q", types.EmailStatusDelivered, repo.statusUpdates[0].status)
	}
	if len(repo.insertedSuppressions) != 0 {
		t.Error("delivered event should not create suppression")
	}
}

func TestHandlePostResendWebhook_Bounced(t *testing.T) {
	msgID := "msg_test_bounced"
	repo := &fakeEmailRepo{
		messagesByProviderID: map[string]*types.EmailMessage{
			msgID: {ID: "local_msg_2", Status: types.EmailStatusSent, Recipient: "bad@example.com"},
		},
	}
	handler := server.NewResendWebhookHandler(testWebhookSecret, repo)

	payload := map[string]any{
		"type": "email.bounced",
		"data": map[string]any{"email_id": msgID},
	}
	req := newWebhookRequest(t, testWebhookSecret, "", payload)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(repo.statusUpdates) != 1 || repo.statusUpdates[0].status != types.EmailStatusBounced {
		t.Errorf("expected bounced status update, got %+v", repo.statusUpdates)
	}
	if len(repo.insertedSuppressions) != 1 {
		t.Fatalf("expected 1 suppression, got %d", len(repo.insertedSuppressions))
	}
	if repo.insertedSuppressions[0].Reason != types.EmailSuppressionReasonHardBounce {
		t.Errorf("expected hard_bounce reason, got %q", repo.insertedSuppressions[0].Reason)
	}
	if repo.insertedSuppressions[0].EmailAddress != "bad@example.com" {
		t.Errorf("unexpected suppression address: %q", repo.insertedSuppressions[0].EmailAddress)
	}
}

func TestHandlePostResendWebhook_Complained(t *testing.T) {
	msgID := "msg_test_complained"
	repo := &fakeEmailRepo{
		messagesByProviderID: map[string]*types.EmailMessage{
			msgID: {ID: "local_msg_3", Status: types.EmailStatusSent, Recipient: "angry@example.com"},
		},
	}
	handler := server.NewResendWebhookHandler(testWebhookSecret, repo)

	payload := map[string]any{
		"type": "email.complained",
		"data": map[string]any{"email_id": msgID},
	}
	req := newWebhookRequest(t, testWebhookSecret, "", payload)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(repo.statusUpdates) != 1 || repo.statusUpdates[0].status != types.EmailStatusComplained {
		t.Errorf("expected complained status update, got %+v", repo.statusUpdates)
	}
	if len(repo.insertedSuppressions) != 1 {
		t.Fatalf("expected 1 suppression, got %d", len(repo.insertedSuppressions))
	}
	if repo.insertedSuppressions[0].Reason != types.EmailSuppressionReasonComplaint {
		t.Errorf("expected complaint reason, got %q", repo.insertedSuppressions[0].Reason)
	}
}
