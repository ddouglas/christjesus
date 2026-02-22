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
	"github.com/k0kubun/pp"
)

const needTableName = "christjesus.needs"

var needColumns = utils.StructTagValues(types.Need{})

type NeedRepository struct {
	pool *pgxpool.Pool
}

func NewNeedRepository(pool *pgxpool.Pool) *NeedRepository {
	return &NeedRepository{pool: pool}
}

func (r *NeedRepository) Need(ctx context.Context, needID string) (*types.Need, error) {

	query, args, err := psql().Select(needColumns...).From(needTableName).
		Where(sq.Eq{"id": needID}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate draft need query: %w", err)
	}

	var need = new(types.Need)
	err = pgxscan.Get(ctx, r.pool, need, query, args...)
	if err != nil && !pgxscan.NotFound(err) {
		return nil, err
	}

	if err != nil {
		return nil, types.ErrNeedNotFound
	}

	// Initialize NeedLocation if nil (pgxscan won't create pointer when fields are NULL)
	if need.NeedLocation == nil {
		need.NeedLocation = &types.NeedLocation{}
	}

	return need, nil

}

func (r *NeedRepository) NeedsByUser(ctx context.Context, userID string) ([]*types.Need, error) {

	query, args, err := psql().Select(needColumns...).From(needTableName).
		Where(sq.Eq{"user_id": userID}).
		OrderBy("created_at desc").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate draft need query: %w", err)
	}

	var needs = make([]*types.Need, 0)
	err = pgxscan.Get(ctx, r.pool, &needs, query, args...)
	if err != nil && !pgxscan.NotFound(err) {
		return nil, err
	}

	if err != nil {
		return nil, types.ErrNeedNotFound
	}

	// Initialize NeedLocation for each need if nil
	for _, need := range needs {
		if need.NeedLocation == nil {
			need.NeedLocation = &types.NeedLocation{}
		}
	}

	return needs, nil
}

func (r *NeedRepository) NeedsByStatus(ctx context.Context, userID string) ([]*types.Need, error) {

	query, args, err := psql().Select(needColumns...).From(needTableName).
		Where(sq.Eq{"status": "draft"}).
		OrderBy("created_at desc").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate draft need query: %w", err)
	}

	var needs = make([]*types.Need, 0)
	err = pgxscan.Get(ctx, r.pool, &needs, query, args...)
	if err != nil && !pgxscan.NotFound(err) {
		return nil, err
	}

	if err != nil {
		return nil, types.ErrNeedNotFound
	}

	// Initialize NeedLocation for each need if nil
	for _, need := range needs {
		if need.NeedLocation == nil {
			need.NeedLocation = &types.NeedLocation{}
		}
	}

	return needs, nil
}

func (r *NeedRepository) DraftNeedsByUser(ctx context.Context, userID string) (*types.Need, error) {

	query, args, err := psql().Select(needColumns...).From(needTableName).
		Where(sq.Eq{"user_id": userID, "status": "draft"}).
		OrderBy("created_at desc").
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate draft need query: %w", err)
	}

	var need = new(types.Need)
	err = pgxscan.Get(ctx, r.pool, need, query, args...)
	if err != nil && !pgxscan.NotFound(err) {
		return nil, err
	}

	if err != nil {
		return nil, types.ErrNeedNotFound
	}

	// Initialize NeedLocation if nil
	if need.NeedLocation == nil {
		need.NeedLocation = &types.NeedLocation{}
	}

	return need, nil
}

func (r *NeedRepository) CreateNeed(ctx context.Context, need *types.Need) error {

	now := time.Now()
	need.ID = utils.NanoID()
	need.UpdatedAt = now
	need.CreatedAt = now

	needMap := utils.StructToMap(need)

	query, args, err := psql().Insert(needTableName).SetMap(needMap).ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate insert need query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	return utils.ErrorWrapOrNil(err, "failed to create need")

}

func (r *NeedRepository) UpdateNeed(ctx context.Context, needID string, need *types.Need) error {

	now := time.Now()
	need.ID = needID
	need.UpdatedAt = now

	needMap := utils.StructToMap(need)

	pp.Print(needMap)

	query, args, err := psql().Update(needTableName).SetMap(needMap).Where(sq.Eq{"id": needID}).ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate update need query for need %s: %w", needID, err)
	}

	_, err = r.pool.Exec(ctx, query, args...)

	return utils.ErrorWrapOrNil(err, "failed to update need")

}

func (r *NeedRepository) DeleteNeed(ctx context.Context, needID string) error {

	query, args, err := psql().Delete(needTableName).Where(sq.Eq{"id": needID}).ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate delete need query for need %s: %w", needID, err)
	}

	_, err = r.pool.Exec(ctx, query, args...)

	return utils.ErrorWrapOrNil(err, "failed to update need")

}
