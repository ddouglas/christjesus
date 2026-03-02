package store

import (
	"christjesus/internal/utils"
	"christjesus/pkg/types"
	"context"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5/pgxpool"
)

const userTableName = "christjesus.users"

var userColumns = utils.StructTagValues(types.User{})

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) User(ctx context.Context, userID string) (*types.User, error) {
	query, args, err := psql().
		Select(userColumns...).
		From(userTableName).
		Where(sq.Eq{"id": userID}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate user query: %w", err)
	}

	var user types.User
	err = pgxscan.Get(ctx, r.pool, &user, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return nil, types.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	return &user, nil
}

func (r *UserRepository) UsersByIDs(ctx context.Context, userIDs []string) ([]*types.User, error) {
	if len(userIDs) == 0 {
		return []*types.User{}, nil
	}

	query, args, err := psql().
		Select(userColumns...).
		From(userTableName).
		Where(sq.Eq{"id": userIDs}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate users-by-ids query: %w", err)
	}

	var users []*types.User
	err = pgxscan.Select(ctx, r.pool, &users, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch users by ids: %w", err)
	}

	return users, nil
}

func (r *UserRepository) Create(ctx context.Context, user *types.User) error {
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	query, args, err := psql().
		Insert(userTableName).
		SetMap(utils.StructToMap(user)).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate create user query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (r *UserRepository) Update(ctx context.Context, userID string, user *types.User) error {
	user.ID = userID
	user.UpdatedAt = time.Now()

	query, args, err := psql().
		Update(userTableName).
		SetMap(utils.StructToMap(user)).
		Where(sq.Eq{"id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate update user query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

func (r *UserRepository) UpsertIdentity(ctx context.Context, userID, email, givenName, familyName string) error {
	now := time.Now()

	var emailPtr *string
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail != "" {
		emailPtr = &trimmedEmail
	}

	var givenNamePtr *string
	trimmedGivenName := strings.TrimSpace(givenName)
	if trimmedGivenName != "" {
		givenNamePtr = &trimmedGivenName
	}

	var familyNamePtr *string
	trimmedFamilyName := strings.TrimSpace(familyName)
	if trimmedFamilyName != "" {
		familyNamePtr = &trimmedFamilyName
	}

	query, args, err := psql().
		Insert(userTableName).
		Columns("id", "email", "given_name", "family_name", "created_at", "updated_at").
		Values(userID, emailPtr, givenNamePtr, familyNamePtr, now, now).
		Suffix("ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email, given_name = EXCLUDED.given_name, family_name = EXCLUDED.family_name, updated_at = EXCLUDED.updated_at").
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate upsert identity user query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to upsert user identity fields: %w", err)
	}

	return nil
}
