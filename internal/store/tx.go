package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TxStarter is implemented by repositories that can open transactions.
type TxStarter interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// WithTx runs fn in a transaction and commits only if fn succeeds.
func WithTx(ctx context.Context, starter TxStarter, fn func(tx pgx.Tx) error) error {
	tx, err := starter.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit tx: %w", err)
	}

	return nil
}
