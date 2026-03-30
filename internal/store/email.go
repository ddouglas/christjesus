package store

import (
	"context"
	"fmt"
	"time"

	"christjesus/internal/utils"
	"christjesus/pkg/types"

	sq "github.com/Masterminds/squirrel"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	emailMessagesTable         = "christjesus.email_messages"
	emailEventsTable           = "christjesus.email_events"
	emailSuppressionsTable     = "christjesus.email_suppressions"
	donationIntentEmailsTable  = "christjesus.donation_intent_emails"
	userEmailsTable            = "christjesus.user_emails"
)

var (
	emailMessageColumns        = utils.StructTagValues(types.EmailMessage{})
	emailEventColumns          = utils.StructTagValues(types.EmailEvent{})
	emailSuppressionColumns    = utils.StructTagValues(types.EmailSuppression{})
	donationIntentEmailColumns = utils.StructTagValues(types.DonationIntentEmail{})
	userEmailColumns           = utils.StructTagValues(types.UserEmail{})
)

type EmailRepository struct {
	pool *pgxpool.Pool
}

func NewEmailRepository(pool *pgxpool.Pool) *EmailRepository {
	return &EmailRepository{pool: pool}
}

// InsertEmailMessage inserts a new email message record with status "queued".
func (r *EmailRepository) InsertEmailMessage(ctx context.Context, msg *types.EmailMessage) error {
	now := time.Now()
	msg.CreatedAt = now
	msg.UpdatedAt = now

	query, args, err := psql().
		Insert(emailMessagesTable).
		SetMap(utils.StructToMap(msg)).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert email message query: %w", err)
	}

	if _, err = r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert email message: %w", err)
	}
	return nil
}

// UpdateEmailMessageStatus updates the status and optionally the provider message ID.
func (r *EmailRepository) UpdateEmailMessageStatus(ctx context.Context, id, status string, providerMessageID *string) error {
	set := sq.Eq{
		"status":     status,
		"updated_at": time.Now(),
	}
	if providerMessageID != nil {
		set["provider_message_id"] = *providerMessageID
	}

	query, args, err := psql().
		Update(emailMessagesTable).
		SetMap(set).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update email message status query: %w", err)
	}

	if _, err = r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("update email message status: %w", err)
	}
	return nil
}

// InsertEmailEvent records a webhook event. Returns (true, nil) if inserted,
// (false, nil) if the provider_event_id already exists (idempotent).
func (r *EmailRepository) InsertEmailEvent(ctx context.Context, event *types.EmailEvent) (bool, error) {
	event.CreatedAt = time.Now()

	query, args, err := psql().
		Insert(emailEventsTable).
		SetMap(utils.StructToMap(event)).
		Suffix("ON CONFLICT (provider_event_id) DO NOTHING").
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build insert email event query: %w", err)
	}

	result, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("insert email event: %w", err)
	}
	return result.RowsAffected() == 1, nil
}

// EmailEventByProviderEventID returns the event for the given provider event ID, or nil if not found.
func (r *EmailRepository) EmailEventByProviderEventID(ctx context.Context, providerEventID string) (*types.EmailEvent, error) {
	query, args, err := psql().
		Select(emailEventColumns...).
		From(emailEventsTable).
		Where(sq.Eq{"provider_event_id": providerEventID}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build email event by provider event id query: %w", err)
	}

	var event types.EmailEvent
	if err := pgxscan.Get(ctx, r.pool, &event, query, args...); err != nil {
		if pgxscan.NotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("fetch email event: %w", err)
	}
	return &event, nil
}

// EmailMessageByProviderMessageID returns the message for the given provider message ID, or nil if not found.
func (r *EmailRepository) EmailMessageByProviderMessageID(ctx context.Context, providerMessageID string) (*types.EmailMessage, error) {
	query, args, err := psql().
		Select(emailMessageColumns...).
		From(emailMessagesTable).
		Where(sq.Eq{"provider_message_id": providerMessageID}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build email message by provider message id query: %w", err)
	}

	var msg types.EmailMessage
	if err := pgxscan.Get(ctx, r.pool, &msg, query, args...); err != nil {
		if pgxscan.NotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("fetch email message: %w", err)
	}
	return &msg, nil
}

// UpsertEmailSuppression adds an address to the suppression list.
// If the address already has an active suppression, it is a no-op.
func (r *EmailRepository) UpsertEmailSuppression(ctx context.Context, suppression *types.EmailSuppression) error {
	suppression.CreatedAt = time.Now()

	query, args, err := psql().
		Insert(emailSuppressionsTable).
		SetMap(utils.StructToMap(suppression)).
		Suffix("ON CONFLICT (email_address) WHERE removed_at IS NULL DO NOTHING").
		ToSql()
	if err != nil {
		return fmt.Errorf("build upsert email suppression query: %w", err)
	}

	if _, err = r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert email suppression: %w", err)
	}
	return nil
}

// IsEmailSuppressed returns true if the address has an active suppression.
func (r *EmailRepository) IsEmailSuppressed(ctx context.Context, emailAddress string) (bool, error) {
	query, args, err := psql().
		Select("1").
		From(emailSuppressionsTable).
		Where(sq.Eq{"email_address": emailAddress}).
		Where("removed_at IS NULL").
		Limit(1).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build is email suppressed query: %w", err)
	}

	var placeholder int
	if err := pgxscan.Get(ctx, r.pool, &placeholder, query, args...); err != nil {
		if pgxscan.NotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("check email suppression: %w", err)
	}
	return true, nil
}

// InsertDonationIntentEmail links an email message to a donation intent.
func (r *EmailRepository) InsertDonationIntentEmail(ctx context.Context, link *types.DonationIntentEmail) error {
	link.CreatedAt = time.Now()

	query, args, err := psql().
		Insert(donationIntentEmailsTable).
		SetMap(utils.StructToMap(link)).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert donation intent email query: %w", err)
	}

	if _, err = r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert donation intent email: %w", err)
	}
	return nil
}

// InsertUserEmail links an email message to a user.
func (r *EmailRepository) InsertUserEmail(ctx context.Context, link *types.UserEmail) error {
	link.CreatedAt = time.Now()

	query, args, err := psql().
		Insert(userEmailsTable).
		SetMap(utils.StructToMap(link)).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert user email query: %w", err)
	}

	if _, err = r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert user email: %w", err)
	}
	return nil
}
