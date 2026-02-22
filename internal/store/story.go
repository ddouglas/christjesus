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

const storyTableName = "christjesus.need_stories"

var storyColumns = utils.StructTagValues(types.NeedStory{})

type StoryRepository struct {
	pool *pgxpool.Pool
}

func NewStoryRepository(pool *pgxpool.Pool) *StoryRepository {
	return &StoryRepository{pool: pool}
}

// GetStoryByNeedID returns the story for a need
func (r *StoryRepository) GetStoryByNeedID(ctx context.Context, needID string) (*types.NeedStory, error) {
	query, args, err := psql().
		Select(storyColumns...).
		From(storyTableName).
		Where(sq.Eq{"need_id": needID}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate story query: %w", err)
	}

	var story types.NeedStory
	err = pgxscan.Get(ctx, r.pool, &story, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return nil, nil // No story found, return nil without error
		}
		return nil, fmt.Errorf("failed to fetch story: %w", err)
	}

	return &story, nil
}

// CreateStory creates a new story for a need
func (r *StoryRepository) CreateStory(ctx context.Context, story *types.NeedStory) error {
	now := time.Now()
	story.CreatedAt = now
	story.UpdatedAt = now

	query, args, err := psql().
		Insert(storyTableName).
		SetMap(utils.StructToMap(story)).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate insert query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert story: %w", err)
	}

	return nil
}

// UpdateStory updates an existing story
func (r *StoryRepository) UpdateStory(ctx context.Context, needID string, story *types.NeedStory) error {
	story.UpdatedAt = time.Now()

	storyMap := utils.StructToMap(story)
	// Remove fields that shouldn't be updated
	delete(storyMap, "need_id")
	delete(storyMap, "created_at")

	query, args, err := psql().
		Update(storyTableName).
		SetMap(storyMap).
		Where(sq.Eq{"need_id": needID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate update query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update story: %w", err)
	}

	return nil
}

// UpsertStory creates or updates a story (insert if not exists, update if exists)
func (r *StoryRepository) UpsertStory(ctx context.Context, story *types.NeedStory) error {
	now := time.Now()
	story.CreatedAt = now
	story.UpdatedAt = now

	storyMap := utils.StructToMap(story)

	// Build the UPDATE SET clause for ON CONFLICT
	// Exclude need_id and created_at from updates
	updateMap := make(map[string]interface{})
	for k, v := range storyMap {
		if k != "need_id" && k != "created_at" {
			updateMap[k] = v
		}
	}

	// Generate SQL with ON CONFLICT clause
	query, args, err := psql().
		Insert(storyTableName).
		SetMap(storyMap).
		Suffix("ON CONFLICT (need_id) DO UPDATE SET " + buildUpdateClause(updateMap)).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate upsert query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to upsert story: %w", err)
	}

	return nil
}

// DeleteStory deletes a story for a need
func (r *StoryRepository) DeleteStory(ctx context.Context, needID string) error {
	query, args, err := psql().
		Delete(storyTableName).
		Where(sq.Eq{"need_id": needID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate delete query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete story: %w", err)
	}

	return nil
}
