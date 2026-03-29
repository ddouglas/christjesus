package store

import (
	"christjesus/pkg/types"
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5/pgxpool"
)

const savedNeedTableName = "christjesus.saved_needs"

type SavedNeedRepository struct {
	pool *pgxpool.Pool
}

func NewSavedNeedRepository(pool *pgxpool.Pool) *SavedNeedRepository {
	return &SavedNeedRepository{pool: pool}
}

func (r *SavedNeedRepository) Save(ctx context.Context, userID, needID string) error {
	query, args, err := psql().
		Insert(savedNeedTableName).
		Columns("user_id", "need_id").
		Values(userID, needID).
		Suffix("ON CONFLICT DO NOTHING").
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate save need query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	return err
}

func (r *SavedNeedRepository) Unsave(ctx context.Context, userID, needID string) error {
	query, args, err := psql().
		Delete(savedNeedTableName).
		Where(sq.Eq{"user_id": userID, "need_id": needID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate unsave need query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	return err
}

func (r *SavedNeedRepository) IsSaved(ctx context.Context, userID, needID string) (bool, error) {
	query, args, err := psql().
		Select("1").
		From(savedNeedTableName).
		Where(sq.Eq{"user_id": userID, "need_id": needID}).
		Limit(1).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("failed to generate is-saved query: %w", err)
	}

	var dummy int
	err = pgxscan.Get(ctx, r.pool, &dummy, query, args...)
	if pgxscan.NotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *SavedNeedRepository) SavedNeedsByUser(ctx context.Context, userID string) ([]*types.SavedNeed, error) {
	query, args, err := psql().
		Select("user_id", "need_id", "created_at").
		From(savedNeedTableName).
		Where(sq.Eq{"user_id": userID}).
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate saved needs by user query: %w", err)
	}

	var saved []*types.SavedNeed
	err = pgxscan.Select(ctx, r.pool, &saved, query, args...)
	if err != nil {
		return nil, err
	}
	return saved, nil
}
