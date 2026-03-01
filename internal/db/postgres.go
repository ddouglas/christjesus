package db

import (
	"context"
	"fmt"
	"time"

	"christjesus/pkg/types"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, config *types.Config) (*pgxpool.Pool, error) {

	poolConfig, err := pgxpool.ParseConfig(config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	if _, ok := poolConfig.ConnConfig.RuntimeParams["search_path"]; !ok {
		poolConfig.ConnConfig.RuntimeParams["search_path"] = "christjesus"
	}

	poolConfig.MaxConnIdleTime = 15 * time.Minute
	poolConfig.MaxConnLifetime = 45 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
