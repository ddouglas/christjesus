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

const userAddressTableName = "christjesus.user_addresses"

var userAddressColumns = utils.StructTagValues(types.UserAddress{})

type UserAddressRepository struct {
	pool *pgxpool.Pool
}

func NewUserAddressRepository(pool *pgxpool.Pool) *UserAddressRepository {
	return &UserAddressRepository{pool: pool}
}

func (r *UserAddressRepository) PrimaryByUserID(ctx context.Context, userID string) (*types.UserAddress, error) {
	query, args, err := psql().
		Select(userAddressColumns...).
		From(userAddressTableName).
		Where(sq.Eq{"user_id": userID, "is_primary": true}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate primary user address query: %w", err)
	}

	var address types.UserAddress
	err = pgxscan.Get(ctx, r.pool, &address, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch primary user address: %w", err)
	}

	return &address, nil
}

func (r *UserAddressRepository) ByIDAndUserID(ctx context.Context, id, userID string) (*types.UserAddress, error) {
	query, args, err := psql().
		Select(userAddressColumns...).
		From(userAddressTableName).
		Where(sq.Eq{"id": id, "user_id": userID}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate user address query: %w", err)
	}

	var address types.UserAddress
	err = pgxscan.Get(ctx, r.pool, &address, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch user address: %w", err)
	}

	return &address, nil
}

func (r *UserAddressRepository) ByIDs(ctx context.Context, ids []string) ([]*types.UserAddress, error) {
	if len(ids) == 0 {
		return []*types.UserAddress{}, nil
	}

	query, args, err := psql().
		Select(userAddressColumns...).
		From(userAddressTableName).
		Where(sq.Eq{"id": ids}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate user addresses by ids query: %w", err)
	}

	var addresses []*types.UserAddress
	err = pgxscan.Select(ctx, r.pool, &addresses, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user addresses by ids: %w", err)
	}

	return addresses, nil
}

func (r *UserAddressRepository) PrimaryByUserIDs(ctx context.Context, userIDs []string) ([]*types.UserAddress, error) {
	if len(userIDs) == 0 {
		return []*types.UserAddress{}, nil
	}

	query, args, err := psql().
		Select(userAddressColumns...).
		From(userAddressTableName).
		Where(sq.Eq{"user_id": userIDs, "is_primary": true}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate primary user addresses by user ids query: %w", err)
	}

	var addresses []*types.UserAddress
	err = pgxscan.Select(ctx, r.pool, &addresses, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch primary user addresses by user ids: %w", err)
	}

	return addresses, nil
}

func (r *UserAddressRepository) AddressesByUserID(ctx context.Context, userID string) ([]*types.UserAddress, error) {
	query, args, err := psql().
		Select(userAddressColumns...).
		From(userAddressTableName).
		Where(sq.Eq{"user_id": userID}).
		OrderBy("is_primary DESC", "created_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate user addresses query: %w", err)
	}

	var addresses []*types.UserAddress
	err = pgxscan.Select(ctx, r.pool, &addresses, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user addresses: %w", err)
	}

	return addresses, nil
}

func (r *UserAddressRepository) Create(ctx context.Context, address *types.UserAddress) error {
	now := time.Now()
	address.CreatedAt = now
	address.UpdatedAt = now

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx for user address create: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if address.IsPrimary {
		clearPrimaryQuery, clearPrimaryArgs, err := psql().
			Update(userAddressTableName).
			Set("is_primary", false).
			Set("updated_at", now).
			Where(sq.Eq{"user_id": address.UserID, "is_primary": true}).
			ToSql()
		if err != nil {
			return fmt.Errorf("failed to generate clear primary query: %w", err)
		}

		_, err = tx.Exec(ctx, clearPrimaryQuery, clearPrimaryArgs...)
		if err != nil {
			return fmt.Errorf("failed to clear current primary address: %w", err)
		}
	}

	insertQuery, insertArgs, err := psql().
		Insert(userAddressTableName).
		Columns(userAddressColumns...).
		Values(
			address.ID,
			address.UserID,
			address.Address,
			address.AddressExt,
			address.City,
			address.State,
			address.ZipCode,
			address.PrivacyDisplay,
			address.ContactMethods,
			address.PreferredContactTime,
			address.IsPrimary,
			address.CreatedAt,
			address.UpdatedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate user address insert: %w", err)
	}

	_, err = tx.Exec(ctx, insertQuery, insertArgs...)
	if err != nil {
		return fmt.Errorf("failed to insert user address: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit user address create tx: %w", err)
	}

	return nil
}

func (r *UserAddressRepository) SetPrimaryByID(ctx context.Context, userID, addressID string) error {
	now := time.Now()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx for user address primary update: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	clearPrimaryQuery, clearPrimaryArgs, err := psql().
		Update(userAddressTableName).
		Set("is_primary", false).
		Set("updated_at", now).
		Where(sq.Eq{"user_id": userID, "is_primary": true}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate clear primary query: %w", err)
	}

	_, err = tx.Exec(ctx, clearPrimaryQuery, clearPrimaryArgs...)
	if err != nil {
		return fmt.Errorf("failed to clear current primary address: %w", err)
	}

	setPrimaryQuery, setPrimaryArgs, err := psql().
		Update(userAddressTableName).
		Set("is_primary", true).
		Set("updated_at", now).
		Where(sq.Eq{"id": addressID, "user_id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate set primary query: %w", err)
	}

	_, err = tx.Exec(ctx, setPrimaryQuery, setPrimaryArgs...)
	if err != nil {
		return fmt.Errorf("failed to set address as primary: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit user address primary update tx: %w", err)
	}

	return nil
}

func (r *UserAddressRepository) CreateAsPrimary(ctx context.Context, address *types.UserAddress) error {
	address.IsPrimary = true
	return r.Create(ctx, address)
}
