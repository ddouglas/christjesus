package store

import (
	"context"
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
