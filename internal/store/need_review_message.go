package store

import (
	"christjesus/internal/utils"
	"christjesus/pkg/types"
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	sq "github.com/Masterminds/squirrel"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5/pgxpool"
)

const needReviewMessagesTableName = "christjesus.need_review_messages"

var needReviewMessagesColumns = utils.StructTagValues(types.NeedReviewMessage{})

type NeedReviewMessageRepository struct {
	pool *pgxpool.Pool
}

func NewNeedReviewMessageRepository(pool *pgxpool.Pool) *NeedReviewMessageRepository {
	return &NeedReviewMessageRepository{pool: pool}
}

func (r *NeedReviewMessageRepository) MessagesByNeed(ctx context.Context, needID string) ([]*types.NeedReviewMessage, error) {
	query, args, err := psql().
		Select(needReviewMessagesColumns...).
		From(needReviewMessagesTableName).
		Where(sq.Eq{"need_id": needID}).
		OrderBy("created_at ASC", "id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate need review messages query: %w", err)
	}

	messages := make([]*types.NeedReviewMessage, 0)
	err = pgxscan.Select(ctx, r.pool, &messages, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return messages, nil
		}
		return nil, utils.ErrorWrapOrNil(err, "failed to load need review messages")
	}

	return messages, nil
}

func (r *NeedReviewMessageRepository) LatestMessageByNeedAndSender(ctx context.Context, needID, senderUserID string, senderRole types.NeedReviewMessageSenderRole) (*types.NeedReviewMessage, error) {
	query, args, err := psql().
		Select(needReviewMessagesColumns...).
		From(needReviewMessagesTableName).
		Where(sq.Eq{
			"need_id":        needID,
			"sender_user_id": senderUserID,
			"sender_role":    senderRole,
		}).
		OrderBy("created_at DESC", "id DESC").
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to generate latest need review message query: %w", err)
	}

	message := new(types.NeedReviewMessage)
	err = pgxscan.Get(ctx, r.pool, message, query, args...)
	if err != nil {
		if pgxscan.NotFound(err) {
			return nil, nil
		}
		return nil, utils.ErrorWrapOrNil(err, "failed to load latest need review message")
	}

	return message, nil
}

func (r *NeedReviewMessageRepository) CreateMessage(ctx context.Context, message *types.NeedReviewMessage) error {
	if message == nil {
		return fmt.Errorf("message is required")
	}

	message.NeedID = strings.TrimSpace(message.NeedID)
	if message.NeedID == "" {
		return fmt.Errorf("need id is required")
	}

	message.SenderUserID = strings.TrimSpace(message.SenderUserID)
	if message.SenderUserID == "" {
		return fmt.Errorf("sender user id is required")
	}

	if message.SenderRole != types.NeedReviewMessageSenderRoleUser && message.SenderRole != types.NeedReviewMessageSenderRoleAdmin {
		return fmt.Errorf("invalid sender role")
	}

	message.ID = utils.NanoID()
	message.Body = strings.TrimSpace(message.Body)
	if message.Body == "" {
		return fmt.Errorf("message body is required")
	}
	if utf8.RuneCountInString(message.Body) > types.NeedReviewMessageMaxChars {
		return fmt.Errorf("message body exceeds max length")
	}
	message.CreatedAt = time.Now()

	query, args, err := psql().
		Insert(needReviewMessagesTableName).
		Columns(needReviewMessagesColumns...).
		Values(message.ID, message.NeedID, message.SenderUserID, message.SenderRole, message.Body, message.CreatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to generate create need review message query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	return utils.ErrorWrapOrNil(err, "failed to create need review message")
}
