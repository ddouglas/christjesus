package store

import (
	"christjesus/internal/utils"
	"christjesus/pkg/types"
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const needProgressEventsTableName = "christjesus.need_progress_events"
const needModerationActionsTableName = "christjesus.need_moderation_actions"

var needProgressEventsColumns = utils.StructTagValues(types.NeedProgressEvent{})
var needModerationActionsColumns = utils.StructTagValues(types.NeedModerationAction{})

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
		Columns("id", "need_id", "step", "event_source").
		Values(id, needID, step, types.NeedProgressEventSourceUser).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate insert progress event query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	return utils.ErrorWrapOrNil(err, "failed to record progress event")
}

func (r *NeedProgressRepository) RecordModerationEvent(
	ctx context.Context,
	needID string,
	step types.NeedProgressEventStep,
	actorUserID string,
	moderationActionID *string,
) error {
	id := utils.NanoID()

	query, args, err := psql().
		Insert(needProgressEventsTableName).
		Columns("id", "need_id", "step", "event_source", "actor_user_id", "moderation_action_id").
		Values(id, needID, step, types.NeedProgressEventSourceAdmin, actorUserID, moderationActionID).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate insert moderation event query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	return utils.ErrorWrapOrNil(err, "failed to record moderation event")
}

func (r *NeedProgressRepository) CreateModerationAction(ctx context.Context, action *types.NeedModerationAction) (string, error) {
	if action == nil {
		return "", fmt.Errorf("action is required")
	}

	id := utils.NanoID()
	action.ID = id

	query, args, err := psql().
		Insert(needModerationActionsTableName).
		Columns("id", "need_id", "action_type", "actor_user_id", "reason", "note", "document_id").
		Values(action.ID, action.NeedID, action.ActionType, action.ActorUserID, action.Reason, action.Note, action.DocumentID).
		ToSql()
	if err != nil {
		return "", fmt.Errorf("failed to generate create moderation action query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return "", utils.ErrorWrapOrNil(err, "failed to create moderation action")
	}

	return id, nil
}

func (r *NeedProgressRepository) ModerationActionsByNeed(ctx context.Context, needID string) ([]*types.NeedModerationAction, error) {
	query, args, err := psql().
		Select(needModerationActionsColumns...).
		From(needModerationActionsTableName).
		Where(sq.Eq{"need_id": needID}).
		OrderBy("created_at ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate moderation actions query: %w", err)
	}

	var actions []*types.NeedModerationAction
	err = pgxscan.Select(ctx, r.pool, &actions, query, args...)
	if err != nil {
		return nil, utils.ErrorWrapOrNil(err, "failed to get moderation actions")
	}

	return actions, nil
}

func (r *NeedProgressRepository) RecordModerationActionEvent(
	ctx context.Context,
	needID string,
	actionType types.NeedModerationActionType,
	actorUserID string,
	reason *string,
	note *string,
	documentID *string,
) (*types.NeedModerationAction, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin moderation event tx: %w", err)
	}
	defer tx.Rollback(ctx)

	action := &types.NeedModerationAction{
		ID:          utils.NanoID(),
		NeedID:      needID,
		ActionType:  actionType,
		ActorUserID: actorUserID,
		Reason:      reason,
		Note:        note,
		DocumentID:  documentID,
	}

	actionQuery, actionArgs, err := psql().
		Insert(needModerationActionsTableName).
		Columns("id", "need_id", "action_type", "actor_user_id", "reason", "note", "document_id").
		Values(action.ID, action.NeedID, action.ActionType, action.ActorUserID, action.Reason, action.Note, action.DocumentID).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate create moderation action query: %w", err)
	}

	if _, err := tx.Exec(ctx, actionQuery, actionArgs...); err != nil {
		return nil, utils.ErrorWrapOrNil(err, "failed to create moderation action")
	}

	eventID := utils.NanoID()
	eventQuery, eventArgs, err := psql().
		Insert(needProgressEventsTableName).
		Columns("id", "need_id", "step", "event_source", "actor_user_id", "moderation_action_id").
		Values(eventID, needID, types.NeedProgressEventStep(actionType), types.NeedProgressEventSourceAdmin, actorUserID, action.ID).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate insert moderation event query: %w", err)
	}

	if _, err := tx.Exec(ctx, eventQuery, eventArgs...); err != nil {
		return nil, utils.ErrorWrapOrNil(err, "failed to record moderation event")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit moderation event tx: %w", err)
	}

	return action, nil
}

func (r *NeedProgressRepository) ModerationTimelineByNeed(ctx context.Context, needID string) ([]*types.NeedModerationTimelineEvent, error) {
	events, err := r.EventsByNeed(ctx, needID)
	if err != nil {
		return nil, err
	}

	actions, err := r.ModerationActionsByNeed(ctx, needID)
	if err != nil {
		return nil, err
	}

	actionByID := make(map[string]*types.NeedModerationAction, len(actions))
	for _, action := range actions {
		if action == nil {
			continue
		}
		actionByID[action.ID] = action
	}

	timeline := make([]*types.NeedModerationTimelineEvent, 0, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}

		item := &types.NeedModerationTimelineEvent{Event: event}
		if event.ModerationActionID != nil {
			if action, ok := actionByID[*event.ModerationActionID]; ok {
				item.Action = action
			}
		}

		timeline = append(timeline, item)
	}

	return timeline, nil
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
