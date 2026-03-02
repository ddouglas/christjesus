package store

import (
	"christjesus/internal/utils"
	"christjesus/pkg/types"
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5/pgxpool"
)

const donorPreferenceTableName = "christjesus.donor_preferences"

var donorPreferenceColumns = utils.StructTagValues(types.DonorPreference{})

type DonorPreferenceRepository struct {
	pool *pgxpool.Pool
}

func NewDonorPreferenceRepository(pool *pgxpool.Pool) *DonorPreferenceRepository {
	return &DonorPreferenceRepository{pool: pool}
}

func (r *DonorPreferenceRepository) ByUserID(ctx context.Context, userID string) (*types.DonorPreference, error) {
	query, args, err := psql().
		Select(donorPreferenceColumns...).
		From(donorPreferenceTableName).
		Where(sq.Eq{"user_id": userID}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate donor preference query: %w", err)
	}

	var pref types.DonorPreference
	err = pgxscan.Get(ctx, r.pool, &pref, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch donor preference: %w", err)
	}

	return &pref, nil
}

func (r *DonorPreferenceRepository) Create(ctx context.Context, pref *types.DonorPreference) error {
	now := time.Now()
	pref.CreatedAt = now
	pref.UpdatedAt = now
	prefMap := utils.StructToMap(pref)

	query, args, err := psql().
		Insert(donorPreferenceTableName).
		SetMap(prefMap).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate donor preference create query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to create donor preference: %w", err)
	}

	return nil
}

func (r *DonorPreferenceRepository) Update(ctx context.Context, userID string, pref *types.DonorPreference) error {
	pref.UserID = userID
	pref.UpdatedAt = time.Now()
	prefMap := utils.StructToMap(pref)

	query, args, err := psql().
		Update(donorPreferenceTableName).
		SetMap(prefMap).
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate donor preference update query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update donor preference: %w", err)
	}

	return nil
}
