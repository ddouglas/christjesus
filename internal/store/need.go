package store

import (
	"christjesus/internal/utils"
	"christjesus/pkg/types"
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const needTableName = "christjesus.needs"

var needColumns = utils.StructTagValues(types.Need{})

type NeedRepository struct {
	pool *pgxpool.Pool
}

type needExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func NewNeedRepository(pool *pgxpool.Pool) *NeedRepository {
	return &NeedRepository{pool: pool}
}

func (r *NeedRepository) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	return r.pool.BeginTx(ctx, txOptions)
}

func (r *NeedRepository) Need(ctx context.Context, needID string) (*types.Need, error) {

	query, args, err := psql().Select(needColumns...).From(needTableName).
		Where(sq.Eq{"id": needID}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate draft need query: %w", err)
	}

	var need = new(types.Need)
	err = pgxscan.Get(ctx, r.pool, need, query, args...)
	if err != nil && !pgxscan.NotFound(err) {
		return nil, err
	}

	if err != nil {
		return nil, types.ErrNeedNotFound
	}

	return need, nil

}

func (r *NeedRepository) BrowseNeeds(ctx context.Context) ([]*types.Need, error) {
	query, args, err := psql().Select(needColumns...).From(needTableName).
		Where(sq.Eq{"status": []types.NeedStatus{types.NeedStatusActive, types.NeedStatusFunded}}).
		Where(sq.Eq{"deleted_at": nil}).
		OrderBy("created_at desc").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate browse needs query: %w", err)
	}

	needs := make([]*types.Need, 0)
	err = pgxscan.Select(ctx, r.pool, &needs, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return needs, nil
		}
		return nil, fmt.Errorf("failed to fetch browse needs: %w", err)
	}

	return needs, nil
}

func (r *NeedRepository) ModerationQueueNeeds(ctx context.Context) ([]*types.Need, error) {
	return r.ModerationQueueNeedsPage(ctx, 1, 500)
}

func (r *NeedRepository) ModerationQueueNeedsPage(ctx context.Context, page, pageSize int) ([]*types.Need, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	offset := uint64((page - 1) * pageSize)

	query, args, err := psql().Select(needColumns...).From(needTableName).
		Where(sq.Eq{"status": []types.NeedStatus{types.NeedStatusReadyForReview, types.NeedStatusUnderReview}}).
		Where(sq.Eq{"deleted_at": nil}).
		OrderBy("submitted_at desc nulls last", "created_at desc").
		Limit(uint64(pageSize)).
		Offset(offset).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate moderation queue needs query: %w", err)
	}

	needs := make([]*types.Need, 0)
	err = pgxscan.Select(ctx, r.pool, &needs, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return needs, nil
		}
		return nil, fmt.Errorf("failed to fetch moderation queue needs: %w", err)
	}

	return needs, nil
}

func (r *NeedRepository) ModerationQueueNeedsCount(ctx context.Context) (int, error) {
	query, args, err := psql().
		Select("COUNT(*)").
		From(needTableName).
		Where(sq.Eq{"status": []types.NeedStatus{types.NeedStatusReadyForReview, types.NeedStatusUnderReview}}).
		Where(sq.Eq{"deleted_at": nil}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("failed to generate moderation queue needs count query: %w", err)
	}

	var total int
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to count moderation queue needs: %w", err)
	}

	return total, nil
}

func (r *NeedRepository) AdminExplorerNeedsPage(ctx context.Context, page, pageSize int, statusFilter *types.NeedStatus, sortBy string) ([]*types.Need, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	offset := uint64((page - 1) * pageSize)

	queryBuilder := psql().Select(needColumns...).From(needTableName).
		Where(sq.Eq{"deleted_at": nil})

	if statusFilter != nil {
		queryBuilder = queryBuilder.Where(sq.Eq{"status": *statusFilter})
	}

	switch sortBy {
	case "updated_asc":
		queryBuilder = queryBuilder.OrderBy("updated_at asc", "id asc")
	case "raised_desc":
		queryBuilder = queryBuilder.OrderBy("amount_raised_cents desc", "updated_at desc", "id asc")
	case "needed_desc":
		queryBuilder = queryBuilder.OrderBy("amount_needed_cents desc", "updated_at desc", "id asc")
	case "progress_desc":
		queryBuilder = queryBuilder.OrderBy("CASE WHEN amount_needed_cents > 0 THEN (amount_raised_cents::float / amount_needed_cents::float) ELSE 0 END desc", "updated_at desc", "id asc")
	default:
		queryBuilder = queryBuilder.OrderBy("updated_at desc", "id asc")
	}

	query, args, err := queryBuilder.
		Limit(uint64(pageSize)).
		Offset(offset).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate admin explorer needs query: %w", err)
	}

	needs := make([]*types.Need, 0)
	err = pgxscan.Select(ctx, r.pool, &needs, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return needs, nil
		}
		return nil, fmt.Errorf("failed to fetch admin explorer needs: %w", err)
	}

	return needs, nil
}

func (r *NeedRepository) AdminExplorerNeedsCount(ctx context.Context, statusFilter *types.NeedStatus) (int, error) {
	queryBuilder := psql().
		Select("COUNT(*)").
		From(needTableName).
		Where(sq.Eq{"deleted_at": nil})

	if statusFilter != nil {
		queryBuilder = queryBuilder.Where(sq.Eq{"status": *statusFilter})
	}

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return 0, fmt.Errorf("failed to generate admin explorer needs count query: %w", err)
	}

	var total int
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to count admin explorer needs: %w", err)
	}

	return total, nil
}

func (r *NeedRepository) AdminExplorerNeedsCountByStatus(ctx context.Context) (map[types.NeedStatus]int, error) {
	query, args, err := psql().
		Select("status", "COUNT(*)").
		From(needTableName).
		Where(sq.Eq{"deleted_at": nil}).
		GroupBy("status").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate admin explorer grouped count query: %w", err)
	}

	type statusCountRow struct {
		Status types.NeedStatus `db:"status"`
		Count  int              `db:"count"`
	}

	rows := make([]statusCountRow, 0)
	if err := pgxscan.Select(ctx, r.pool, &rows, query, args...); err != nil {
		if pgxscan.NotFound(err) {
			return map[types.NeedStatus]int{}, nil
		}
		return nil, fmt.Errorf("failed to fetch admin explorer grouped counts: %w", err)
	}

	counts := make(map[types.NeedStatus]int, len(rows))
	for _, row := range rows {
		counts[row.Status] = row.Count
	}

	return counts, nil
}

func (r *NeedRepository) LatestNeeds(ctx context.Context, limit int) ([]*types.Need, error) {
	if limit <= 0 {
		limit = 5
	}

	query, args, err := psql().Select(needColumns...).From(needTableName).
		Where(sq.NotEq{"status": types.NeedStatusDraft}).
		Where(sq.Eq{"deleted_at": nil}).
		OrderBy("created_at desc").
		Limit(uint64(limit)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate latest needs query: %w", err)
	}

	needs := make([]*types.Need, 0)
	err = pgxscan.Select(ctx, r.pool, &needs, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return needs, nil
		}
		return nil, fmt.Errorf("failed to fetch latest needs: %w", err)
	}

	return needs, nil
}

func (r *NeedRepository) NeedsByUser(ctx context.Context, userID string) ([]*types.Need, error) {

	query, args, err := psql().Select(needColumns...).From(needTableName).
		Where(sq.Eq{"user_id": userID}).
		OrderBy("created_at desc").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate draft need query: %w", err)
	}

	var needs = make([]*types.Need, 0)
	err = pgxscan.Select(ctx, r.pool, &needs, query, args...)
	if err != nil && !pgxscan.NotFound(err) {
		return nil, err
	}

	if err != nil {
		return nil, types.ErrNeedNotFound
	}

	return needs, nil
}

func (r *NeedRepository) NeedsByIDs(ctx context.Context, needIDs []string) ([]*types.Need, error) {
	if len(needIDs) == 0 {
		return []*types.Need{}, nil
	}

	query, args, err := psql().Select(needColumns...).From(needTableName).
		Where(sq.Eq{"id": needIDs}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate needs by ids query: %w", err)
	}

	needs := make([]*types.Need, 0)
	err = pgxscan.Select(ctx, r.pool, &needs, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return needs, nil
		}
		return nil, fmt.Errorf("failed to fetch needs by ids: %w", err)
	}

	return needs, nil
}

func (r *NeedRepository) NeedsByStatus(ctx context.Context, userID string) ([]*types.Need, error) {

	query, args, err := psql().Select(needColumns...).From(needTableName).
		Where(sq.Eq{"status": "draft"}).
		OrderBy("created_at desc").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate draft need query: %w", err)
	}

	var needs = make([]*types.Need, 0)
	err = pgxscan.Get(ctx, r.pool, &needs, query, args...)
	if err != nil && !pgxscan.NotFound(err) {
		return nil, err
	}

	if err != nil {
		return nil, types.ErrNeedNotFound
	}

	return needs, nil
}

func (r *NeedRepository) DraftNeedsByUser(ctx context.Context, userID string) (*types.Need, error) {

	query, args, err := psql().Select(needColumns...).From(needTableName).
		Where(sq.Eq{"user_id": userID, "status": "draft"}).
		OrderBy("created_at desc").
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate draft need query: %w", err)
	}

	var need = new(types.Need)
	err = pgxscan.Get(ctx, r.pool, need, query, args...)
	if err != nil && !pgxscan.NotFound(err) {
		return nil, err
	}

	if err != nil {
		return nil, types.ErrNeedNotFound
	}

	return need, nil
}

func (r *NeedRepository) CreateNeed(ctx context.Context, need *types.Need) error {

	now := time.Now()
	need.ID = utils.NanoID()
	need.UpdatedAt = now
	need.CreatedAt = now

	needMap := utils.StructToMap(need)

	query, args, err := psql().Insert(needTableName).SetMap(needMap).ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate insert need query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	return utils.ErrorWrapOrNil(err, "failed to create need")

}

func (r *NeedRepository) UpdateNeed(ctx context.Context, needID string, need *types.Need) error {

	now := time.Now()
	need.ID = needID
	need.UpdatedAt = now

	needMap := utils.StructToMap(need)

	query, args, err := psql().Update(needTableName).SetMap(needMap).Where(sq.Eq{"id": needID}).ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate update need query for need %s: %w", needID, err)
	}

	_, err = r.pool.Exec(ctx, query, args...)

	return utils.ErrorWrapOrNil(err, "failed to update need")

}

func (r *NeedRepository) SetNeedStatus(ctx context.Context, needID string, status types.NeedStatus) error {
	return r.setNeedStatusWithExec(ctx, r.pool, needID, status)
}

func (r *NeedRepository) SetNeedStatusTx(ctx context.Context, tx pgx.Tx, needID string, status types.NeedStatus) error {
	return r.setNeedStatusWithExec(ctx, tx, needID, status)
}

func (r *NeedRepository) PublishNeed(ctx context.Context, needID string) error {
	return r.publishNeedWithExec(ctx, r.pool, needID)
}

func (r *NeedRepository) PublishNeedTx(ctx context.Context, tx pgx.Tx, needID string) error {
	return r.publishNeedWithExec(ctx, tx, needID)
}

func (r *NeedRepository) setNeedStatusWithExec(ctx context.Context, execer needExecer, needID string, status types.NeedStatus) error {
	query, args, err := psql().
		Update(needTableName).
		Set("status", status).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": needID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate set need status query for need %s: %w", needID, err)
	}

	_, err = execer.Exec(ctx, query, args...)
	return utils.ErrorWrapOrNil(err, "failed to set need status")
}

func (r *NeedRepository) publishNeedWithExec(ctx context.Context, execer needExecer, needID string) error {
	query, args, err := psql().
		Update(needTableName).
		Set("status", types.NeedStatusActive).
		Set("published_at", sq.Expr("COALESCE(published_at, NOW())")).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": needID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate publish need query for need %s: %w", needID, err)
	}

	_, err = execer.Exec(ctx, query, args...)
	return utils.ErrorWrapOrNil(err, "failed to publish need")
}

func (r *NeedRepository) SoftDeleteNeed(ctx context.Context, needID, actorUserID, reason string) error {
	return r.softDeleteNeedWithExec(ctx, r.pool, needID, actorUserID, reason)
}

func (r *NeedRepository) SoftDeleteNeedTx(ctx context.Context, tx pgx.Tx, needID, actorUserID, reason string) error {
	return r.softDeleteNeedWithExec(ctx, tx, needID, actorUserID, reason)
}

func (r *NeedRepository) softDeleteNeedWithExec(ctx context.Context, execer needExecer, needID, actorUserID, reason string) error {
	now := time.Now()
	query, args, err := psql().
		Update(needTableName).
		Set("deleted_at", now).
		Set("deleted_by_user_id", actorUserID).
		Set("delete_reason", reason).
		Set("updated_at", now).
		Where(sq.Eq{"id": needID, "deleted_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate soft-delete need query for need %s: %w", needID, err)
	}

	tag, err := execer.Exec(ctx, query, args...)
	if err != nil {
		return utils.ErrorWrapOrNil(err, "failed to soft-delete need")
	}

	if tag.RowsAffected() == 0 {
		return types.ErrNeedAlreadyDeleted
	}

	return utils.ErrorWrapOrNil(err, "failed to soft-delete need")
}

func (r *NeedRepository) RestoreNeed(ctx context.Context, needID string) error {
	return r.restoreNeedWithExec(ctx, r.pool, needID)
}

func (r *NeedRepository) RestoreNeedTx(ctx context.Context, tx pgx.Tx, needID string) error {
	return r.restoreNeedWithExec(ctx, tx, needID)
}

func (r *NeedRepository) restoreNeedWithExec(ctx context.Context, execer needExecer, needID string) error {
	now := time.Now()
	query, args, err := psql().
		Update(needTableName).
		Set("deleted_at", nil).
		Set("deleted_by_user_id", nil).
		Set("delete_reason", nil).
		Set("updated_at", now).
		Where(sq.Eq{"id": needID}).
		Where(sq.NotEq{"deleted_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate restore need query for need %s: %w", needID, err)
	}

	tag, err := execer.Exec(ctx, query, args...)
	if err != nil {
		return utils.ErrorWrapOrNil(err, "failed to restore need")
	}

	if tag.RowsAffected() == 0 {
		return types.ErrNeedNotDeleted
	}

	return utils.ErrorWrapOrNil(err, "failed to restore need")
}

func (r *NeedRepository) DeleteNeed(ctx context.Context, needID string) error {

	query, args, err := psql().Delete(needTableName).Where(sq.Eq{"id": needID}).ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate delete need query for need %s: %w", needID, err)
	}

	_, err = r.pool.Exec(ctx, query, args...)

	return utils.ErrorWrapOrNil(err, "failed to update need")

}
