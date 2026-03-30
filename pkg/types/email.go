package types

import "time"

type EmailMessage struct {
	ID                string     `db:"id"`
	Recipient         string     `db:"recipient"`
	EmailType         string     `db:"email_type"`
	Subject           string     `db:"subject"`
	Provider          string     `db:"provider"`
	ProviderMessageID *string    `db:"provider_message_id"`
	Status            string     `db:"status"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
}

type EmailEvent struct {
	ID              string    `db:"id"`
	EmailMessageID  *string   `db:"email_message_id"`
	ProviderEventID string    `db:"provider_event_id"`
	EventType       string    `db:"event_type"`
	Payload         []byte    `db:"payload"`
	CreatedAt       time.Time `db:"created_at"`
}

type EmailSuppression struct {
	ID           string     `db:"id"`
	EmailAddress string     `db:"email_address"`
	Reason       string     `db:"reason"`
	SourceEventID *string   `db:"source_event_id"`
	CreatedAt    time.Time  `db:"created_at"`
	RemovedAt    *time.Time `db:"removed_at"`
}

type DonationIntentEmail struct {
	ID                 string    `db:"id"`
	DonationIntentID   string    `db:"donation_intent_id"`
	EmailMessageID     string    `db:"email_message_id"`
	EmailType          string    `db:"email_type"`
	CreatedAt          time.Time `db:"created_at"`
}

type UserEmail struct {
	ID             string    `db:"id"`
	UserID         string    `db:"user_id"`
	EmailMessageID string    `db:"email_message_id"`
	EmailType      string    `db:"email_type"`
	CreatedAt      time.Time `db:"created_at"`
}

// Email status values
const (
	EmailStatusQueued    = "queued"
	EmailStatusSent      = "sent"
	EmailStatusDelivered = "delivered"
	EmailStatusBounced   = "bounced"
	EmailStatusComplained = "complained"
)

// Email suppression reasons
const (
	EmailSuppressionReasonHardBounce = "hard_bounce"
	EmailSuppressionReasonComplaint  = "complaint"
	EmailSuppressionReasonManual     = "manual"
)
