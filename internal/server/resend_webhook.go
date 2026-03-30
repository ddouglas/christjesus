package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	svix "github.com/svix/svix-webhooks/go"

	"christjesus/internal/utils"
	"christjesus/pkg/types"
)

const resendWebhookPayloadMaxBytes int64 = 1 << 20

// resendEmailRepo is the subset of EmailRepository methods the webhook handler needs.
type resendEmailRepo interface {
	InsertEmailEvent(ctx context.Context, event *types.EmailEvent) (bool, error)
	EmailMessageByProviderMessageID(ctx context.Context, providerMessageID string) (*types.EmailMessage, error)
	UpdateEmailMessageStatus(ctx context.Context, id, status string, providerMessageID *string) error
	UpsertEmailSuppression(ctx context.Context, suppression *types.EmailSuppression) error
}

// ResendWebhookHandler handles inbound Resend webhook events.
type ResendWebhookHandler struct {
	secret string
	repo   resendEmailRepo
}

// NewResendWebhookHandler returns a new ResendWebhookHandler.
func NewResendWebhookHandler(secret string, repo resendEmailRepo) *ResendWebhookHandler {
	return &ResendWebhookHandler{secret: secret, repo: repo}
}

// resendWebhookPayload is the common envelope for all Resend webhook events.
type resendWebhookPayload struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// resendEmailEventData contains the fields we care about from event data.
type resendEmailEventData struct {
	EmailID string `json:"email_id"`
}

func (h *ResendWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if strings.TrimSpace(h.secret) == "" {
		http.Error(w, "webhook not configured", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, resendWebhookPayloadMaxBytes))
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	wh, err := svix.NewWebhook(h.secret)
	if err != nil {
		http.Error(w, "webhook misconfigured", http.StatusInternalServerError)
		return
	}
	if err := wh.Verify(body, r.Header); err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	var payload resendWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Use the svix-id as the provider event ID for idempotency.
	providerEventID := r.Header.Get("svix-id")

	event := &types.EmailEvent{
		ID:              utils.NanoID(),
		ProviderEventID: providerEventID,
		EventType:       payload.Type,
		Payload:         body,
	}

	isNew, err := h.repo.InsertEmailEvent(ctx, event)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !isNew {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.processEvent(ctx, event, payload); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *ResendWebhookHandler) processEvent(ctx context.Context, event *types.EmailEvent, payload resendWebhookPayload) error {
	var data resendEmailEventData
	if err := json.Unmarshal(payload.Data, &data); err != nil || data.EmailID == "" {
		// Unknown structure — log and move on, don't fail the webhook.
		return nil
	}

	msg, err := h.repo.EmailMessageByProviderMessageID(ctx, data.EmailID)
	if err != nil {
		return err
	}

	switch payload.Type {
	case "email.delivered":
		if msg != nil {
			return h.repo.UpdateEmailMessageStatus(ctx, msg.ID, types.EmailStatusDelivered, nil)
		}

	case "email.bounced":
		if msg != nil {
			if err := h.repo.UpdateEmailMessageStatus(ctx, msg.ID, types.EmailStatusBounced, nil); err != nil {
				return err
			}
			return h.repo.UpsertEmailSuppression(ctx, &types.EmailSuppression{
				ID:            utils.NanoID(),
				EmailAddress:  msg.Recipient,
				Reason:        types.EmailSuppressionReasonHardBounce,
				SourceEventID: &event.ID,
			})
		}

	case "email.complained":
		if msg != nil {
			if err := h.repo.UpdateEmailMessageStatus(ctx, msg.ID, types.EmailStatusComplained, nil); err != nil {
				return err
			}
			return h.repo.UpsertEmailSuppression(ctx, &types.EmailSuppression{
				ID:            utils.NanoID(),
				EmailAddress:  msg.Recipient,
				Reason:        types.EmailSuppressionReasonComplaint,
				SourceEventID: &event.ID,
			})
		}
	}

	return nil
}
