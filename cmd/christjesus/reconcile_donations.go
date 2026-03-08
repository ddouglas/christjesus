package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"christjesus/internal/db"
	"christjesus/internal/store"
	"christjesus/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v84"
	"github.com/urfave/cli/v2"
)

var reconcileDonationsCommand = &cli.Command{
	Name:  "reconcile-donations",
	Usage: "Backfill and reconcile stale pending Stripe donation records",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:  "stale-minutes",
			Value: 30,
			Usage: "Treat pending donations older than this many minutes as stale and eligible for reconciliation",
		},
		&cli.IntFlag{
			Name:  "limit",
			Value: 200,
			Usage: "Maximum number of stale pending donations to reconcile in one run",
		},
		&cli.BoolFlag{
			Name:  "dry-run",
			Usage: "Log intended updates without persisting status changes",
		},
	},
	Action: reconcileDonations,
}

func reconcileDonations(cCtx *cli.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if strings.TrimSpace(cfg.StripeSecretKey) == "" {
		return fmt.Errorf("set STRIPE_SECRET_KEY before running reconcile-donations")
	}

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	ctx := cCtx.Context

	pool, err := db.Connect(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	donationIntentRepo := store.NewDonationIntentRepository(pool)
	stripeClient := stripe.NewClient(cfg.StripeSecretKey)

	staleMinutes := cCtx.Int("stale-minutes")
	if staleMinutes <= 0 {
		staleMinutes = 30
	}

	limit := cCtx.Int("limit")
	if limit <= 0 {
		limit = 200
	}

	dryRun := cCtx.Bool("dry-run")
	cutoff := time.Now().Add(-time.Duration(staleMinutes) * time.Minute)

	intents, err := donationIntentRepo.PendingIntentsOlderThan(ctx, cutoff, limit)
	if err != nil {
		return fmt.Errorf("failed to query stale pending donations: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"stale_minutes": staleMinutes,
		"limit":         limit,
		"matched":       len(intents),
		"dry_run":       dryRun,
	}).Info("loaded stale pending donations for reconciliation")

	var finalizedCount int
	var failedCount int
	var canceledCount int
	var skippedCount int

	for _, intent := range intents {
		action, checkoutSessionID, paymentIntentID, reason, resolveErr := resolveDonationStatusFromStripe(ctx, stripeClient, intent)
		if resolveErr != nil {
			logger.WithError(resolveErr).WithField("intent_id", intent.ID).Warn("failed to resolve donation status from Stripe")
			skippedCount++
			continue
		}

		if action == "skip" {
			logger.WithFields(logrus.Fields{
				"intent_id": intent.ID,
				"reason":    reason,
			}).Info("skipping stale pending donation with no terminal Stripe signal")
			skippedCount++
			continue
		}

		if dryRun {
			logger.WithFields(logrus.Fields{
				"intent_id":             intent.ID,
				"action":                action,
				"checkout_session_id":   derefString(checkoutSessionID),
				"payment_intent_id":     derefString(paymentIntentID),
				"stripe_resolution":     reason,
				"current_payment_state": intent.PaymentStatus,
			}).Info("dry-run reconciliation decision")
			continue
		}

		switch action {
		case "finalize":
			_, err = donationIntentRepo.FinalizeIntentByID(ctx, intent.ID, checkoutSessionID, paymentIntentID)
			if err != nil {
				logger.WithError(err).WithField("intent_id", intent.ID).Warn("failed to finalize stale pending donation")
				skippedCount++
				continue
			}
			finalizedCount++
		case "fail":
			_, err = donationIntentRepo.MarkIntentFailedByID(ctx, intent.ID, checkoutSessionID, paymentIntentID)
			if err != nil {
				logger.WithError(err).WithField("intent_id", intent.ID).Warn("failed to fail stale pending donation")
				skippedCount++
				continue
			}
			failedCount++
		case "cancel":
			_, err = donationIntentRepo.MarkIntentCanceledByID(ctx, intent.ID, checkoutSessionID, paymentIntentID)
			if err != nil {
				logger.WithError(err).WithField("intent_id", intent.ID).Warn("failed to cancel stale pending donation")
				skippedCount++
				continue
			}
			canceledCount++
		default:
			skippedCount++
		}
	}

	logger.WithFields(logrus.Fields{
		"processed":   len(intents),
		"finalized":   finalizedCount,
		"failed":      failedCount,
		"canceled":    canceledCount,
		"skipped":     skippedCount,
		"dry_run":     dryRun,
		"stale_until": cutoff.Format(time.RFC3339),
	}).Info("donation reconciliation run complete")

	return nil
}

func resolveDonationStatusFromStripe(ctx context.Context, stripeClient *stripe.Client, intent *types.DonationIntent) (action string, checkoutSessionID *string, paymentIntentID *string, reason string, err error) {
	if intent == nil {
		return "skip", nil, nil, "intent was nil", nil
	}

	if intent.PaymentIntentID != nil && strings.TrimSpace(*intent.PaymentIntentID) != "" {
		paymentIntentIDValue := strings.TrimSpace(*intent.PaymentIntentID)
		paymentIntent, retrieveErr := stripeClient.V1PaymentIntents.Retrieve(ctx, paymentIntentIDValue, nil)
		if retrieveErr != nil {
			return "skip", nil, nil, "", fmt.Errorf("retrieve stripe payment intent %s: %w", paymentIntentIDValue, retrieveErr)
		}

		action, reason = mapPaymentIntentStatusToAction(string(paymentIntent.Status))
		return action, intent.CheckoutSessionID, &paymentIntentIDValue, reason, nil
	}

	if intent.CheckoutSessionID != nil && strings.TrimSpace(*intent.CheckoutSessionID) != "" {
		checkoutSessionIDValue := strings.TrimSpace(*intent.CheckoutSessionID)
		session, retrieveErr := stripeClient.V1CheckoutSessions.Retrieve(ctx, checkoutSessionIDValue, nil)
		if retrieveErr != nil {
			return "skip", nil, nil, "", fmt.Errorf("retrieve stripe checkout session %s: %w", checkoutSessionIDValue, retrieveErr)
		}

		return resolveFromCheckoutSession(ctx, stripeClient, session, checkoutSessionIDValue)
	}

	if strings.TrimSpace(intent.ID) != "" {
		params := &stripe.CheckoutSessionListParams{}
		params.Limit = stripe.Int64(100)
		params.CreatedRange = &stripe.RangeQueryParams{GreaterThanOrEqual: intent.CreatedAt.Unix()}

		for session, listErr := range stripeClient.V1CheckoutSessions.List(ctx, params) {
			if listErr != nil {
				return "skip", nil, nil, "", fmt.Errorf("list stripe checkout sessions for intent %s: %w", intent.ID, listErr)
			}
			if session == nil {
				continue
			}
			if strings.TrimSpace(session.ClientReferenceID) != strings.TrimSpace(intent.ID) {
				continue
			}

			checkoutSessionIDValue := strings.TrimSpace(session.ID)
			action, checkoutSessionID, paymentIntentID, reason, err = resolveFromCheckoutSession(ctx, stripeClient, session, checkoutSessionIDValue)
			if err != nil {
				return "skip", nil, nil, "", err
			}
			if reason != "" {
				reason = "matched checkout session by client_reference_id; " + reason
			}
			return action, checkoutSessionID, paymentIntentID, reason, nil
		}
	}

	return "skip", nil, nil, "no stripe IDs available for reconciliation or lookup", nil
}

func resolveFromCheckoutSession(ctx context.Context, stripeClient *stripe.Client, session *stripe.CheckoutSession, sessionID string) (action string, checkoutSessionID *string, paymentIntentID *string, reason string, err error) {
	if session == nil {
		return "skip", nil, nil, "checkout session not found", nil
	}

	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		trimmedSessionID = strings.TrimSpace(session.ID)
	}
	if trimmedSessionID != "" {
		checkoutSessionID = &trimmedSessionID
	}

	if session.PaymentIntent != nil && strings.TrimSpace(session.PaymentIntent.ID) != "" {
		paymentIntentIDValue := strings.TrimSpace(session.PaymentIntent.ID)
		paymentIntentID = &paymentIntentIDValue

		paymentIntent, retrieveErr := stripeClient.V1PaymentIntents.Retrieve(ctx, paymentIntentIDValue, nil)
		if retrieveErr != nil {
			return "skip", checkoutSessionID, paymentIntentID, "", fmt.Errorf("retrieve stripe payment intent %s from checkout session: %w", paymentIntentIDValue, retrieveErr)
		}

		action, reason = mapPaymentIntentStatusToAction(string(paymentIntent.Status))
		if reason != "" {
			reason = "payment intent lookup from checkout session; " + reason
		}
		return action, checkoutSessionID, paymentIntentID, reason, nil
	}

	action, reason = mapCheckoutSessionToAction(session)
	return action, checkoutSessionID, paymentIntentID, reason, nil
}

func mapPaymentIntentStatusToAction(status string) (action string, reason string) {
	switch strings.TrimSpace(status) {
	case string(stripe.PaymentIntentStatusSucceeded):
		return "finalize", "payment_intent.succeeded"
	case string(stripe.PaymentIntentStatusCanceled):
		return "cancel", "payment_intent.canceled"
	case string(stripe.PaymentIntentStatusRequiresPaymentMethod), string(stripe.PaymentIntentStatusRequiresAction):
		return "skip", fmt.Sprintf("payment intent requires customer action; non-terminal (%s)", status)
	default:
		return "skip", fmt.Sprintf("payment intent still non-terminal (%s)", status)
	}
}

func mapCheckoutSessionToAction(session *stripe.CheckoutSession) (action string, reason string) {
	if session == nil {
		return "skip", "checkout session not found"
	}

	if session.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid {
		return "finalize", "checkout session paid"
	}

	sessionStatus := strings.TrimSpace(string(session.Status))
	if sessionStatus == string(stripe.CheckoutSessionStatusExpired) {
		return "cancel", "checkout session expired"
	}

	return "skip", fmt.Sprintf("checkout session still non-terminal (status=%s payment_status=%s)", sessionStatus, session.PaymentStatus)
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
