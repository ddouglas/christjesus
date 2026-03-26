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
	"github.com/jackc/pgx/v5"
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

func (r *DonationIntentRepository) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	return r.pool.BeginTx(ctx, txOptions)
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

func (r *DonationIntentRepository) ByPaymentIntentID(ctx context.Context, paymentIntentID string) (*types.DonationIntent, error) {
	query, args, err := psql().
		Select(donationIntentColumns...).
		From(donationIntentTableName).
		Where(sq.Eq{"payment_intent_id": paymentIntentID}).
		OrderBy("updated_at desc").
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate donation intent by payment intent id query: %w", err)
	}

	var intent types.DonationIntent
	err = pgxscan.Get(ctx, r.pool, &intent, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch donation intent by payment intent id: %w", err)
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
		Where(sq.NotEq{"payment_status": types.DonationPaymentStatusFinalized}).
		Suffix("RETURNING need_id")

	if checkoutSessionID != nil && *checkoutSessionID != "" {
		qb = qb.Set("checkout_session_id", *checkoutSessionID)
	}
	if paymentIntentID != nil && *paymentIntentID != "" {
		qb = qb.Set("payment_intent_id", *paymentIntentID)
	}

	finalizeQuery, finalizeArgs, err := qb.ToSql()
	if err != nil {
		return false, fmt.Errorf("failed to generate finalize donation intent query: %w", err)
	}

	var finalized bool
	err = WithTx(ctx, r, func(tx pgx.Tx) error {
		var needID string
		err := tx.QueryRow(ctx, finalizeQuery, finalizeArgs...).Scan(&needID)
		if err != nil {
			if err.Error() == "no rows in result set" {
				return nil
			}
			return fmt.Errorf("failed to finalize donation intent: %w", err)
		}

		finalized = true

		syncQuery, syncArgs, err := psql().
			Update(needTableName).
			Set("amount_raised_cents", sq.Expr(
				"(SELECT COALESCE(SUM(amount_cents), 0) FROM "+donationIntentTableName+" WHERE need_id = ? AND LOWER(payment_status) = ?)",
				needID, types.DonationPaymentStatusFinalized,
			)).
			Set("updated_at", now).
			Where(sq.Eq{"id": needID}).
			ToSql()
		if err != nil {
			return fmt.Errorf("failed to generate sync need raised amount query: %w", err)
		}

		if _, err := tx.Exec(ctx, syncQuery, syncArgs...); err != nil {
			return fmt.Errorf("failed to sync need raised amount for need %s: %w", needID, err)
		}

		return nil
	})

	return finalized, err
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

func (r *DonationIntentRepository) MarkIntentCanceledByID(ctx context.Context, intentID string, checkoutSessionID, paymentIntentID *string) (bool, error) {
	now := time.Now()

	qb := psql().
		Update(donationIntentTableName).
		Set("payment_status", types.DonationPaymentStatusCanceled).
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
		return false, fmt.Errorf("failed to generate cancel donation intent query: %w", err)
	}

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("failed to cancel donation intent: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

func (r *DonationIntentRepository) PendingIntentsOlderThan(ctx context.Context, cutoff time.Time, limit int) ([]*types.DonationIntent, error) {
	if limit <= 0 {
		limit = 200
	}

	query, args, err := psql().
		Select(donationIntentColumns...).
		From(donationIntentTableName).
		Where(sq.Eq{"payment_status": types.DonationPaymentStatusPending}).
		Where(sq.Lt{"created_at": cutoff}).
		OrderBy("created_at asc").
		Limit(uint64(limit)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate pending stale donation intents query: %w", err)
	}

	intents := make([]*types.DonationIntent, 0)
	err = pgxscan.Select(ctx, r.pool, &intents, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return intents, nil
		}
		return nil, fmt.Errorf("failed to fetch pending stale donation intents: %w", err)
	}

	return intents, nil
}

func (r *DonationIntentRepository) FinalizedAmountByNeedID(ctx context.Context, needID string) (int, error) {
	query, args, err := psql().
		Select("COALESCE(SUM(amount_cents), 0)").
		From(donationIntentTableName).
		Where(sq.Eq{"need_id": needID, "payment_status": types.DonationPaymentStatusFinalized}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("failed to generate finalized amount by need query: %w", err)
	}

	var amountCents int
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&amountCents); err != nil {
		return 0, fmt.Errorf("failed to fetch finalized amount by need: %w", err)
	}

	return amountCents, nil
}

func (r *DonationIntentRepository) FinalizedAmountsByNeedIDs(ctx context.Context, needIDs []string) (map[string]int, error) {
	amountsByNeedID := make(map[string]int)
	if len(needIDs) == 0 {
		return amountsByNeedID, nil
	}

	query, args, err := psql().
		Select("need_id", "COALESCE(SUM(amount_cents), 0) AS total_amount_cents").
		From(donationIntentTableName).
		Where(sq.Eq{"payment_status": types.DonationPaymentStatusFinalized}).
		Where(sq.Eq{"need_id": needIDs}).
		GroupBy("need_id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate finalized amounts by need ids query: %w", err)
	}

	rows := make([]struct {
		NeedID           string `db:"need_id"`
		TotalAmountCents int    `db:"total_amount_cents"`
	}, 0)
	if err := pgxscan.Select(ctx, r.pool, &rows, query, args...); err != nil {
		if !pgxscan.NotFound(err) {
			return nil, fmt.Errorf("failed to fetch finalized amounts by need ids: %w", err)
		}
	}

	for _, row := range rows {
		amountsByNeedID[row.NeedID] = row.TotalAmountCents
	}

	return amountsByNeedID, nil
}

func (r *DonationIntentRepository) DonationIntentsByDonorUserID(ctx context.Context, donorUserID string) ([]*types.DonationIntent, error) {
	query, args, err := psql().
		Select(donationIntentColumns...).
		From(donationIntentTableName).
		Where(sq.Eq{"donor_user_id": donorUserID}).
		OrderBy("created_at desc").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate donation intents by donor user id query: %w", err)
	}

	intents := make([]*types.DonationIntent, 0)
	err = pgxscan.Select(ctx, r.pool, &intents, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return intents, nil
		}
		return nil, fmt.Errorf("failed to fetch donation intents by donor user id: %w", err)
	}

	return intents, nil
}

func (r *DonationIntentRepository) DonorImpactStats(ctx context.Context, donorUserID string) (types.DonorImpactStats, error) {
	query := fmt.Sprintf(`
		WITH donor_finalized AS (
			SELECT need_id, amount_cents
			FROM %s
			WHERE donor_user_id = $1 AND payment_status = $2
		),
		totals AS (
			SELECT
				COALESCE(SUM(amount_cents), 0) AS total_given_cents,
				COUNT(DISTINCT need_id) AS needs_supported
			FROM donor_finalized
		),
		funded AS (
			SELECT COUNT(*) AS needs_funded
			FROM (
				SELECT n.id
				FROM %s n
				WHERE n.deleted_at IS NULL
				  AND EXISTS (SELECT 1 FROM donor_finalized df WHERE df.need_id = n.id)
				  AND n.amount_needed_cents > 0
				  AND n.amount_raised_cents >= n.amount_needed_cents
			) funded_needs
		)
		SELECT totals.total_given_cents, totals.needs_supported, funded.needs_funded
		FROM totals, funded
	`, donationIntentTableName, needTableName)

	stats := types.DonorImpactStats{}
	err := r.pool.QueryRow(ctx, query, donorUserID, types.DonationPaymentStatusFinalized).Scan(
		&stats.TotalGivenCents,
		&stats.NeedsSupported,
		&stats.NeedsFunded,
	)
	if err != nil {
		return types.DonorImpactStats{}, fmt.Errorf("failed to fetch donor impact stats: %w", err)
	}

	return stats, nil
}

func (r *DonationIntentRepository) HomeImpactStats(ctx context.Context) (types.StatsData, error) {
	query := fmt.Sprintf(`
		WITH finalized AS (
			SELECT need_id, amount_cents
			FROM %s
			WHERE payment_status = $1
		),
		totals AS (
			SELECT COALESCE(SUM(amount_cents), 0) AS total_raised
			FROM finalized
		),
		funded AS (
			SELECT COUNT(*) AS needs_funded
			FROM (
				SELECT n.id
				FROM %s n
				JOIN finalized f ON f.need_id = n.id
				WHERE n.status <> $2
				GROUP BY n.id, n.amount_needed_cents
				HAVING COALESCE(SUM(f.amount_cents), 0) >= n.amount_needed_cents
			) funded_needs
		),
		changed AS (
			SELECT COUNT(DISTINCT n.user_id) AS lives_changed
			FROM %s n
			JOIN finalized f ON f.need_id = n.id
			WHERE n.status <> $2
		)
		SELECT totals.total_raised, funded.needs_funded, changed.lives_changed
		FROM totals, funded, changed
	`, donationIntentTableName, needTableName, needTableName)

	stats := types.StatsData{}
	err := r.pool.QueryRow(ctx, query, types.DonationPaymentStatusFinalized, types.NeedStatusDraft).Scan(
		&stats.TotalRaised,
		&stats.NeedsFunded,
		&stats.LivesChanged,
	)
	if err != nil {
		return types.StatsData{}, fmt.Errorf("failed to fetch home impact stats: %w", err)
	}

	return stats, nil
}
