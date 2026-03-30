package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/webhook"
)

const stripeWebhookPayloadMaxBytes int64 = 1 << 20

func (s *Service) handlePostStripeWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if strings.TrimSpace(s.config.StripeWebhookSecret) == "" {
		s.logger.Warn("stripe webhook called but STRIPE_WEBHOOK_SECRET is not configured")
		http.Error(w, "webhook not configured", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, stripeWebhookPayloadMaxBytes))
	if err != nil {
		s.logger.WithError(err).Warn("failed to read stripe webhook request body")
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEvent(body, signature, s.config.StripeWebhookSecret)
	if err != nil {
		s.logger.WithError(err).Warn("failed to verify stripe webhook signature")
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	isNew, err := s.donationIntentRepo.RecordWebhookEventIfNew(ctx, event.ID, string(event.Type), body)
	if err != nil {
		s.logger.WithError(err).WithField("stripe_event_id", event.ID).Error("failed to record stripe webhook event")
		s.internalServerError(w)
		return
	}
	if !isNew {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := s.processStripeWebhookEvent(ctx, event); err != nil {
		s.logger.WithError(err).WithFields(map[string]any{
			"stripe_event_id": event.ID,
			"event_type":      event.Type,
		}).Error("failed to process stripe webhook event")
		s.internalServerError(w)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Service) processStripeWebhookEvent(ctx context.Context, event stripe.Event) error {
	switch string(event.Type) {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded", "checkout.session.async_payment_failed":
		return s.processCheckoutSessionWebhookEvent(ctx, event)
	case "payment_intent.succeeded", "payment_intent.payment_failed", "payment_intent.canceled":
		return s.processPaymentIntentWebhookEvent(ctx, event)
	default:
		return nil
	}
}

func (s *Service) processCheckoutSessionWebhookEvent(ctx context.Context, event stripe.Event) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return fmt.Errorf("unmarshal checkout session event data: %w", err)
	}

	intentID := strings.TrimSpace(session.Metadata["donation_intent_id"])
	if intentID == "" {
		intentID = strings.TrimSpace(session.ClientReferenceID)
	}
	if intentID == "" {
		s.logger.WithFields(map[string]any{
			"stripe_event_id":     event.ID,
			"checkout_session_id": session.ID,
		}).Warn("stripe checkout session webhook missing donation intent correlation")
		return nil
	}

	var checkoutSessionID *string
	if id := strings.TrimSpace(session.ID); id != "" {
		checkoutSessionID = &id
	}

	var paymentIntentID *string
	if session.PaymentIntent != nil {
		if id := strings.TrimSpace(session.PaymentIntent.ID); id != "" {
			paymentIntentID = &id
		}
	}

	switch string(event.Type) {
	case "checkout.session.completed":
		if session.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
			s.logger.WithFields(map[string]any{
				"stripe_event_id": event.ID,
				"intent_id":       intentID,
				"payment_status":  session.PaymentStatus,
			}).Info("ignoring checkout.session.completed without paid status")
			return nil
		}

		finalized, err := s.donationIntentRepo.FinalizeIntentByID(ctx, intentID, checkoutSessionID, paymentIntentID)
		if err != nil {
			return fmt.Errorf("finalize donation intent from checkout.session.completed: %w", err)
		}
		if finalized {
			if err := s.maybeSendDonationReceiptEmail(ctx, intentID); err != nil {
				s.logger.WithError(err).WithField("donation_intent_id", intentID).Error("failed to send donation receipt email")
			}
		}
		return nil

	case "checkout.session.async_payment_succeeded":
		finalized, err := s.donationIntentRepo.FinalizeIntentByID(ctx, intentID, checkoutSessionID, paymentIntentID)
		if err != nil {
			return fmt.Errorf("finalize donation intent from async success: %w", err)
		}
		if finalized {
			if err := s.maybeSendDonationReceiptEmail(ctx, intentID); err != nil {
				s.logger.WithError(err).WithField("donation_intent_id", intentID).Error("failed to send donation receipt email")
			}
		}
		return nil

	case "checkout.session.async_payment_failed":
		_, err := s.donationIntentRepo.MarkIntentFailedByID(ctx, intentID, checkoutSessionID, paymentIntentID)
		if err != nil {
			return fmt.Errorf("mark donation intent failed from async failure: %w", err)
		}
		return nil
	default:
		return nil
	}
}

func (s *Service) processPaymentIntentWebhookEvent(ctx context.Context, event stripe.Event) error {
	var paymentIntent stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
		return fmt.Errorf("unmarshal payment intent event data: %w", err)
	}

	stripePaymentIntentID := strings.TrimSpace(paymentIntent.ID)
	if stripePaymentIntentID == "" {
		s.logger.WithField("stripe_event_id", event.ID).Warn("payment intent webhook missing payment intent id")
		return nil
	}

	intentID := strings.TrimSpace(paymentIntent.Metadata["donation_intent_id"])
	if intentID == "" {
		intent, err := s.donationIntentRepo.ByPaymentIntentID(ctx, stripePaymentIntentID)
		if err != nil {
			return fmt.Errorf("find donation intent by payment intent id: %w", err)
		}
		if intent != nil {
			intentID = intent.ID
		}
	}

	if intentID == "" {
		s.logger.WithFields(map[string]any{
			"stripe_event_id":       event.ID,
			"payment_intent_id":     stripePaymentIntentID,
			"payment_intent_status": paymentIntent.Status,
		}).Warn("payment intent webhook missing donation intent correlation")
		return nil
	}

	paymentIntentID := stripePaymentIntentID
	paymentIntentIDRef := &paymentIntentID

	switch string(event.Type) {
	case "payment_intent.succeeded":
		finalized, err := s.donationIntentRepo.FinalizeIntentByID(ctx, intentID, nil, paymentIntentIDRef)
		if err != nil {
			return fmt.Errorf("finalize donation intent from payment_intent.succeeded: %w", err)
		}
		if finalized {
			if err := s.maybeSendDonationReceiptEmail(ctx, intentID); err != nil {
				s.logger.WithError(err).WithField("donation_intent_id", intentID).Error("failed to send donation receipt email")
			}
		}
		return nil
	case "payment_intent.payment_failed":
		_, err := s.donationIntentRepo.MarkIntentFailedByID(ctx, intentID, nil, paymentIntentIDRef)
		if err != nil {
			return fmt.Errorf("mark donation intent failed from payment_intent.payment_failed: %w", err)
		}
		return nil
	case "payment_intent.canceled":
		_, err := s.donationIntentRepo.MarkIntentCanceledByID(ctx, intentID, nil, paymentIntentIDRef)
		if err != nil {
			return fmt.Errorf("mark donation intent canceled from payment_intent.canceled: %w", err)
		}
		return nil
	default:
		return nil
	}
}
