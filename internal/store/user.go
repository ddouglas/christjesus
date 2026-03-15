package store

import (
	"christjesus/internal/utils"
	"christjesus/pkg/types"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func (r *UserRepository) UpsertIdentity(ctx context.Context, authSubject, email, givenName, familyName string) (string, error) {
	authSubject = strings.TrimSpace(authSubject)
	if authSubject == "" {
		return "", fmt.Errorf("auth subject is required")
	}

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

	updateQuery, updateArgs, err := psql().
		Update(userTableName).
		Set("email", emailPtr).
		Set("given_name", givenNamePtr).
		Set("family_name", familyNamePtr).
		Set("updated_at", now).
		Where(sq.Eq{"auth_subject": authSubject}).
		Suffix("RETURNING id").
		ToSql()
	if err != nil {
		return "", fmt.Errorf("failed to generate update identity user query: %w", err)
	}

	var userID string
	err = r.pool.QueryRow(ctx, updateQuery, updateArgs...).Scan(&userID)
	if err == nil {
		return userID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("failed to update existing user identity fields: %w", err)
	}

	newUserID := types.NanoID()

	insertQuery, insertArgs, err := psql().
		Insert(userTableName).
		Columns("id", "auth_subject", "email", "given_name", "family_name", "created_at", "updated_at").
		Values(newUserID, authSubject, emailPtr, givenNamePtr, familyNamePtr, now, now).
		Suffix("RETURNING id").
		ToSql()
	if err != nil {
		return "", fmt.Errorf("failed to generate insert identity user query: %w", err)
	}

	err = r.pool.QueryRow(ctx, insertQuery, insertArgs...).Scan(&userID)
	if err == nil {
		return userID, nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		err = r.pool.QueryRow(ctx, updateQuery, updateArgs...).Scan(&userID)
		if err == nil {
			return userID, nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("user identity raced during insert but was not found on re-read")
		}
		return "", fmt.Errorf("failed to load raced user identity after unique violation: %w", err)
	}

	return "", fmt.Errorf("failed to insert user identity fields: %w", err)
}
