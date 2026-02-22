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

const needProgressEventsTableName = "christjesus.need_progress_events"

var needProgressEventsColumns = utils.StructTagValues(types.NeedProgressEvent{})

type NeedProgressRepository struct {
	pool *pgxpool.Pool
}

func NewNeedProgressRepository(pool *pgxpool.Pool) *NeedProgressRepository {
	return &NeedProgressRepository{pool: pool}
}

// RecordStepCompletion logs that a step was completed (allows duplicates for edit tracking)
func (r *NeedProgressRepository) RecordStepCompletion(ctx context.Context, needID string, step types.NeedStep) error {
	id := utils.NanoID()

	query, args, err := psql().
		Insert(needProgressEventsTableName).
		Columns("id", "need_id", "step").
		Values(id, needID, step).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate insert progress event query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	return utils.ErrorWrapOrNil(err, "failed to record progress event")
}

// GetAllEvents returns all progress events for a need, ordered chronologically
func (r *NeedProgressRepository) EventsByNeed(ctx context.Context, needID string) ([]*types.NeedProgressEvent, error) {
	query, args, err := psql().
		Select(needProgressEventsColumns...).
		From(needProgressEventsTableName).
		Where(sq.Eq{"need_id": needID}).
		OrderBy("created_at ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate get events query: %w", err)
	}

	var events []*types.NeedProgressEvent
	err = pgxscan.Select(ctx, r.pool, &events, query, args...)
	if err != nil {
		return nil, utils.ErrorWrapOrNil(err, "failed to get progress events")
	}

	return events, nil
}
