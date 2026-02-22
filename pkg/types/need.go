package types

import (
	"time"
)

type NeedStatus string

const (
	NeedStatusDraft       NeedStatus = "DRAFT"
	NeedStatusSubmitted   NeedStatus = "SUBMITTED"
	NeedStatusUnderReview NeedStatus = "UNDER_REVIEW"
	NeedStatusApproved    NeedStatus = "APPROVED"
	NeedStatusRejected    NeedStatus = "REJECTED"
	NeedStatusActive      NeedStatus = "ACTIVE"
	NeedStatusFunded      NeedStatus = "FUNDED"
)

type Need struct {
	ID     string `db:"id"`
	UserID string `db:"user_id"`

	NeedLocation

	Title             *string    `db:"title"`
	AmountNeededCents int        `db:"amount_needed_cents"`
	AmountRaisedCents int        `db:"amount_raised_cents"`
	ShortDescription  *string    `db:"short_description"`
	Story             *string    `db:"story"`
	Status            NeedStatus `db:"status"`
	VerifiedAt        *time.Time `db:"verified_at"`
	VerifiedBy        *string    `db:"verified_by"`
	CurrentStep       string     `db:"current_step"`
	CompletedSteps    []string   `db:"completed_steps"` // jsonb array
	PublishedAt       *time.Time `db:"published_at"`
	ClosedAt          *time.Time `db:"closed_at"`
	IsFeatured        bool       `db:"is_featured"`
	SubmittedAt       *time.Time `db:"submitted_at"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
}

type NeedLocation struct {
	Address              string   `db:"address" form:"address"`
	AddressExt           string   `db:"address_ext" form:"address_ext"`
	City                 *string  `db:"city" form:"city"`
	State                *string  `db:"state" form:"state"`
	ZipCode              *string  `db:"zip_code" form:"zip_code"`
	PrivacyDisplay       string   `db:"privacy_display" form:"privacy_display"`
	ContactMethods       []string `db:"contact_methods" form:"contact_methods"`
	PreferredContactTime *string  `db:"preferred_contact_time" form:"preferred_contact_time"`
}

type UpdateNeedLocation struct {
	City    string
	State   string
	ZipCode string
}
