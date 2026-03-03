package types

import "time"

const (
	DonationPaymentProviderStripe  = "stripe"
	DonationPaymentStatusPending   = "pending"
	DonationPaymentStatusFinalized = "finalized"
	DonationPaymentStatusFailed    = "failed"
	DonationPaymentStatusCanceled  = "canceled"
)

type DonationIntent struct {
	ID                string    `db:"id"`
	NeedID            string    `db:"need_id"`
	DonorUserID       *string   `db:"donor_user_id"`
	CheckoutSessionID *string   `db:"checkout_session_id"`
	PaymentIntentID   *string   `db:"payment_intent_id"`
	AmountCents       int       `db:"amount_cents"`
	PrivateMessage    *string   `db:"private_message"`
	IsAnonymous       bool      `db:"is_anonymous"`
	PaymentProvider   string    `db:"payment_provider"`
	PaymentStatus     string    `db:"payment_status"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}
