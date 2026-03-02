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

const donorPreferenceAssignmentTableName = "christjesus.donor_preference_category_assignments"

var donorPreferenceAssignmentColumns = utils.StructTagValues(types.DonorPreferenceCategoryAssignment{})

type DonorPreferenceAssignmentRepository struct {
	pool *pgxpool.Pool
}

func NewDonorPreferenceAssignmentRepository(pool *pgxpool.Pool) *DonorPreferenceAssignmentRepository {
	return &DonorPreferenceAssignmentRepository{pool: pool}
}

func (r *DonorPreferenceAssignmentRepository) AssignmentsByUserID(ctx context.Context, userID string) ([]*types.DonorPreferenceCategoryAssignment, error) {
	query, args, err := psql().
		Select(donorPreferenceAssignmentColumns...).
		From(donorPreferenceAssignmentTableName).
		Where(sq.Eq{"user_id": userID}).
		OrderBy("created_at ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate donor preference assignments query: %w", err)
	}

	var assignments []*types.DonorPreferenceCategoryAssignment
	err = pgxscan.Select(ctx, r.pool, &assignments, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch donor preference assignments: %w", err)
	}

	return assignments, nil
}

func (r *DonorPreferenceAssignmentRepository) ReplaceAssignments(ctx context.Context, userID string, categoryIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin donor preference assignment transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	deleteQuery, deleteArgs, err := psql().
		Delete(donorPreferenceAssignmentTableName).
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate donor preference assignment delete query: %w", err)
	}

	if _, err := tx.Exec(ctx, deleteQuery, deleteArgs...); err != nil {
		return fmt.Errorf("failed to delete donor preference assignments: %w", err)
	}

	if len(categoryIDs) > 0 {
		now := time.Now()
		insertBuilder := psql().
			Insert(donorPreferenceAssignmentTableName).
			Columns(donorPreferenceAssignmentColumns...)

		for _, categoryID := range categoryIDs {
			insertBuilder = insertBuilder.Values(userID, categoryID, now)
		}

		insertQuery, insertArgs, err := insertBuilder.ToSql()
		if err != nil {
			return fmt.Errorf("failed to generate donor preference assignment insert query: %w", err)
		}

		if _, err := tx.Exec(ctx, insertQuery, insertArgs...); err != nil {
			return fmt.Errorf("failed to insert donor preference assignments: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit donor preference assignment transaction: %w", err)
	}

	return nil
}
