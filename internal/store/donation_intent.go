package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"christjesus/internal/utils"
	"christjesus/pkg/types"

	sq "github.com/Masterminds/squirrel"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5/pgxpool"
)

const donationIntentTableName = "christjesus.donation_intents"

var donationIntentColumns = utils.StructTagValues(types.DonationIntent{})

type DonationIntentRepository struct {
	pool *pgxpool.Pool
}

func NewDonationIntentRepository(pool *pgxpool.Pool) *DonationIntentRepository {
	return &DonationIntentRepository{pool: pool}
}

func (r *DonationIntentRepository) Create(ctx context.Context, intent *types.DonationIntent) error {
	now := time.Now()
	intent.CreatedAt = now
	intent.UpdatedAt = now

	query, args, err := psql().
		Insert(donationIntentTableName).
		SetMap(utils.StructToMap(intent)).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate donation intent insert query: %w", err)
	}

	if _, err = r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("failed to create donation intent: %w", err)
	}

	return nil
}

func (r *DonationIntentRepository) ByID(ctx context.Context, intentID string) (*types.DonationIntent, error) {
	query, args, err := psql().
		Select(donationIntentColumns...).
		From(donationIntentTableName).
		Where(sq.Eq{"id": intentID}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate donation intent by id query: %w", err)
	}

	var intent types.DonationIntent
	err = pgxscan.Get(ctx, r.pool, &intent, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch donation intent: %w", err)
	}

	return &intent, nil
}

func (r *DonationIntentRepository) SetCheckoutSessionID(ctx context.Context, intentID, checkoutSessionID string) error {
	now := time.Now()

	query, args, err := psql().
		Update(donationIntentTableName).
		Set("checkout_session_id", checkoutSessionID).
		Set("updated_at", now).
		Where(sq.Eq{"id": intentID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate donation intent checkout session update query: %w", err)
	}

	if _, err = r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("failed to update donation intent checkout session id: %w", err)
	}

	return nil
}

func (r *DonationIntentRepository) RecordWebhookEventIfNew(ctx context.Context, stripeEventID, eventType string, payload []byte) (bool, error) {
	now := time.Now()

	query, args, err := psql().
		Insert("christjesus.stripe_webhook_events").
		Columns("id", "stripe_event_id", "event_type", "payload", "created_at").
		Values(utils.NanoID(), stripeEventID, eventType, json.RawMessage(payload), now).
		Suffix("ON CONFLICT (stripe_event_id) DO NOTHING").
		ToSql()
	if err != nil {
		return false, fmt.Errorf("failed to generate webhook event insert query: %w", err)
	}

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("failed to insert webhook event: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

func (r *DonationIntentRepository) FinalizeIntentByID(ctx context.Context, intentID string, checkoutSessionID, paymentIntentID *string) (bool, error) {
	now := time.Now()

	qb := psql().
		Update(donationIntentTableName).
		Set("payment_status", types.DonationPaymentStatusFinalized).
		Set("updated_at", now).
		Where(sq.Eq{"id": intentID}).
		Where(sq.NotEq{"payment_status": types.DonationPaymentStatusFinalized})

	if checkoutSessionID != nil && *checkoutSessionID != "" {
		qb = qb.Set("checkout_session_id", *checkoutSessionID)
	}
	if paymentIntentID != nil && *paymentIntentID != "" {
		qb = qb.Set("payment_intent_id", *paymentIntentID)
	}

	query, args, err := qb.ToSql()
	if err != nil {
		return false, fmt.Errorf("failed to generate finalize donation intent query: %w", err)
	}

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("failed to finalize donation intent: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

func (r *DonationIntentRepository) MarkIntentFailedByID(ctx context.Context, intentID string, checkoutSessionID, paymentIntentID *string) (bool, error) {
	now := time.Now()

	qb := psql().
		Update(donationIntentTableName).
		Set("payment_status", types.DonationPaymentStatusFailed).
		Set("updated_at", now).
		Where(sq.Eq{"id": intentID}).
		Where(sq.NotEq{"payment_status": types.DonationPaymentStatusFinalized})

	if checkoutSessionID != nil && *checkoutSessionID != "" {
		qb = qb.Set("checkout_session_id", *checkoutSessionID)
	}
	if paymentIntentID != nil && *paymentIntentID != "" {
		qb = qb.Set("payment_intent_id", *paymentIntentID)
	}

	query, args, err := qb.ToSql()
	if err != nil {
		return false, fmt.Errorf("failed to generate fail donation intent query: %w", err)
	}

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("failed to fail donation intent: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}
