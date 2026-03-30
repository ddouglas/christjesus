package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	svix "github.com/svix/svix-webhooks/go"
	"github.com/sirupsen/logrus"

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

// resendEventHandler processes a specific Resend event type given the resolved message.
type resendEventHandler func(ctx context.Context, event *types.EmailEvent, msg *types.EmailMessage) error

// ResendWebhookHandler handles inbound Resend webhook events.
type ResendWebhookHandler struct {
	secret   string
	repo     resendEmailRepo
	logger   *logrus.Logger
	handlers map[string]resendEventHandler
}

// NewResendWebhookHandler returns a new ResendWebhookHandler.
func NewResendWebhookHandler(secret string, repo resendEmailRepo, logger *logrus.Logger) *ResendWebhookHandler {
	h := &ResendWebhookHandler{
		secret: secret,
		repo:   repo,
		logger: logger,
	}
	h.handlers = map[string]resendEventHandler{
		"email.delivered":  h.handleDelivered,
		"email.bounced":    h.handleBounced,
		"email.complained": h.handleComplained,
	}
	return h
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
		h.logger.WithError(err).WithFields(logrus.Fields{
			"event_type":        payload.Type,
			"provider_event_id": providerEventID,
		}).Error("failed to process resend webhook event")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *ResendWebhookHandler) processEvent(ctx context.Context, event *types.EmailEvent, payload resendWebhookPayload) error {
	var data resendEmailEventData
	if err := json.Unmarshal(payload.Data, &data); err != nil || data.EmailID == "" {
		h.logger.WithFields(logrus.Fields{
			"event_type":        payload.Type,
			"provider_event_id": event.ProviderEventID,
		}).Warn("resend webhook event has unknown or missing email_id — skipping processing")
		return nil
	}

	handler, ok := h.handlers[payload.Type]
	if !ok {
		// Unrecognised event type — recorded in email_events but no further action needed.
		return nil
	}

	msg, err := h.repo.EmailMessageByProviderMessageID(ctx, data.EmailID)
	if err != nil {
		return err
	}
	if msg == nil {
		h.logger.WithFields(logrus.Fields{
			"event_type":          payload.Type,
			"provider_message_id": data.EmailID,
		}).Warn("resend webhook event references unknown provider message ID — skipping processing")
		return nil
	}

	return handler(ctx, event, msg)
}

func (h *ResendWebhookHandler) handleDelivered(ctx context.Context, _ *types.EmailEvent, msg *types.EmailMessage) error {
	return h.repo.UpdateEmailMessageStatus(ctx, msg.ID, types.EmailStatusDelivered, nil)
}

func (h *ResendWebhookHandler) handleBounced(ctx context.Context, event *types.EmailEvent, msg *types.EmailMessage) error {
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

func (h *ResendWebhookHandler) handleComplained(ctx context.Context, event *types.EmailEvent, msg *types.EmailMessage) error {
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
