package store

import (
	"christjesus/internal/utils"
	"christjesus/pkg/types"
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5/pgxpool"
)

const categoryTableName = "christjesus.need_categories"

var categoryColumns = utils.StructTagValues(types.NeedCategory{})

type CategoryRepository struct {
	pool *pgxpool.Pool
}

func NewCategoryRepository(pool *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{pool: pool}
}

func (r *CategoryRepository) AllCategories(ctx context.Context) ([]*types.NeedCategory, error) {
	query, args, err := psql().
		Select(categoryColumns...).
		From(categoryTableName).
		Where(sq.Eq{"is_active": true}).
		OrderBy("display_order ASC", "name ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate categories query: %w", err)
	}

	var categories []*types.NeedCategory
	err = pgxscan.Select(ctx, r.pool, &categories, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch categories: %w", err)
	}

	return categories, nil
}

func (r *CategoryRepository) CategoryByID(ctx context.Context, id string) (*types.NeedCategory, error) {
	query, args, err := psql().
		Select(categoryColumns...).
		From(categoryTableName).
		Where(sq.Eq{"id": id}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate category query: %w", err)
	}

	var category types.NeedCategory
	err = pgxscan.Get(ctx, r.pool, &category, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch category: %w", err)
	}

	return &category, nil
}

func (r *CategoryRepository) CategoryBySlug(ctx context.Context, slug string) (*types.NeedCategory, error) {
	query, args, err := psql().
		Select(categoryColumns...).
		From(categoryTableName).
		Where(sq.Eq{"slug": slug}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate category query: %w", err)
	}

	var category types.NeedCategory
	err = pgxscan.Get(ctx, r.pool, &category, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return nil, nil // Category not found, return nil without error
		}
		return nil, fmt.Errorf("failed to fetch category: %w", err)
	}

	return &category, nil
}

func (r *CategoryRepository) AllCategoriesUnfiltered(ctx context.Context) ([]*types.NeedCategory, error) {
	query, args, err := psql().
		Select(categoryColumns...).
		From(categoryTableName).
		OrderBy("display_order ASC", "name ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate categories query: %w", err)
	}

	var categories []*types.NeedCategory
	err = pgxscan.Select(ctx, r.pool, &categories, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch categories: %w", err)
	}

	return categories, nil
}

func (r *CategoryRepository) CreateCategory(ctx context.Context, category *types.NeedCategory) error {
	query, args, err := psql().
		Insert(categoryTableName).
		SetMap(utils.StructToMap(category)).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate insert query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert category: %w", err)
	}

	return nil
}

func (r *CategoryRepository) UpsertCategory(ctx context.Context, category *types.NeedCategory) error {
	categoryMap := utils.StructToMap(category)

	// Build the INSERT statement
	insertBuilder := psql().
		Insert(categoryTableName).
		SetMap(categoryMap)

	// Build the UPDATE SET clause for ON CONFLICT
	// Exclude id and created_at from updates
	updateMap := make(map[string]interface{})
	for k, v := range categoryMap {
		if k != "id" && k != "created_at" {
			updateMap[k] = v
		}
	}

	// Generate SQL with ON CONFLICT clause
	query, args, err := insertBuilder.
		Suffix("ON CONFLICT (id) DO UPDATE SET " + buildUpdateClause(updateMap)).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate upsert query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to upsert category: %w", err)
	}

	return nil
}

func (r *CategoryRepository) DeleteCategory(ctx context.Context, id string) error {
	query, args, err := psql().
		Delete(categoryTableName).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate delete query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	return nil
}

// buildUpdateClause creates the SET clause for ON CONFLICT DO UPDATE
// e.g., "name = EXCLUDED.name, slug = EXCLUDED.slug, ..."
func buildUpdateClause(fields map[string]interface{}) string {
	var clause string
	first := true
	for field := range fields {
		if !first {
			clause += ", "
		}
		clause += fmt.Sprintf("%s = EXCLUDED.%s", field, field)
		first = false
	}
	return clause
}
