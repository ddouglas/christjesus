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

const assignmentTableName = "christjesus.need_category_assignments"

var assignmentColumns = utils.StructTagValues(types.NeedCategoryAssignment{})

type AssignmentRepository struct {
	pool *pgxpool.Pool
}

func NewAssignmentRepository(pool *pgxpool.Pool) *AssignmentRepository {
	return &AssignmentRepository{pool: pool}
}

// GetAssignmentsByNeedID returns all category assignments for a need
func (r *AssignmentRepository) GetAssignmentsByNeedID(ctx context.Context, needID string) ([]*types.NeedCategoryAssignment, error) {
	query, args, err := psql().
		Select(assignmentColumns...).
		From(assignmentTableName).
		Where(sq.Eq{"need_id": needID}).
		OrderBy("is_primary DESC", "created_at ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate assignments query: %w", err)
	}

	var assignments []*types.NeedCategoryAssignment
	err = pgxscan.Select(ctx, r.pool, &assignments, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch assignments: %w", err)
	}

	return assignments, nil
}

// CreateAssignment creates a new category assignment
func (r *AssignmentRepository) CreateAssignment(ctx context.Context, assignment *types.NeedCategoryAssignment) error {
	now := time.Now()
	assignment.CreatedAt = now

	query, args, err := psql().
		Insert(assignmentTableName).
		SetMap(utils.StructToMap(assignment)).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate insert query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert assignment: %w", err)
	}

	return nil
}

// CreateAssignment creates a new category assignment
func (r *AssignmentRepository) CreateAssignments(ctx context.Context, assignments []*types.NeedCategoryAssignment) error {
	now := time.Now()

	// Use a transaction to insert all assignments
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	builder := psql().
		Insert(assignmentTableName).Columns(assignmentColumns...)

	for _, assignment := range assignments {
		assignment.CreatedAt = now
		builder = builder.Values(assignment.NeedID, assignment.CategoryID, assignment.IsPrimary, assignment.CreatedAt)
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate insert query: %w", err)
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert assignment: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteAssignment deletes a specific category assignment
func (r *AssignmentRepository) DeleteAssignment(ctx context.Context, needID, categoryID string) error {
	query, args, err := psql().
		Delete(assignmentTableName).
		Where(sq.Eq{"need_id": needID, "category_id": categoryID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate delete query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete assignment: %w", err)
	}

	return nil
}

// DeleteAllAssignmentsByNeedID deletes all category assignments for a need
func (r *AssignmentRepository) DeleteAllAssignmentsByNeedID(ctx context.Context, needID string) error {
	query, args, err := psql().
		Delete(assignmentTableName).
		Where(sq.Eq{"need_id": needID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate delete query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete assignments: %w", err)
	}

	return nil
}
