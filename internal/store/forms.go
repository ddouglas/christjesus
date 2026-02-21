package store

import (
	"context"
	"fmt"

	"christjesus/pkg/types"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5/pgxpool"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

type FormsRepository struct {
	pool *pgxpool.Pool
}

func NewFormsRepository(pool *pgxpool.Pool) *FormsRepository {
	return &FormsRepository{pool: pool}
}

func (r *FormsRepository) CreatePrayerRequest(ctx context.Context, name, email, requestBody string) error {
	id, err := gonanoid.New(21)
	if err != nil {
		return fmt.Errorf("generate id: %w", err)
	}

	query, args, err := psql().
		Insert("prayer_requests").
		Columns("id", "name", "email", "request_body").
		Values(id, name, nullable(email), requestBody).
		ToSql()
	if err != nil {
		return fmt.Errorf("build prayer insert: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("insert prayer request: %w", err)
	}

	return nil
}

func (r *FormsRepository) UpsertEmailSignup(ctx context.Context, email, city string) error {
	query := `
		INSERT INTO email_signups (email, city)
		VALUES ($1, $2)
		ON CONFLICT (email)
		DO UPDATE SET city = EXCLUDED.city, updated_at = now()`

	_, err := r.pool.Exec(ctx, query, email, nullable(city))
	if err != nil {
		return fmt.Errorf("upsert email signup: %w", err)
	}

	return nil
}

func (r *FormsRepository) LatestPrayerRequests(ctx context.Context, limit uint64) ([]*types.PrayerRequest, error) {
	query, args, err := psql().
		Select("id", "name", "email", "request_body", "created_at").
		From("prayer_requests").
		OrderBy("created_at DESC").
		Limit(limit).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build latest prayer query: %w", err)
	}

	out := make([]*types.PrayerRequest, 0)
	if err := pgxscan.Select(ctx, r.pool, &out, query, args...); err != nil {
		return nil, fmt.Errorf("select latest prayer requests: %w", err)
	}

	return out, nil
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}
