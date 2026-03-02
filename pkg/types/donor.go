package types

import "time"

type DonorPreference struct {
	UserID                string    `db:"user_id"`
	ZipCode               *string   `db:"zip_code"`
	Radius                *string   `db:"radius"`
	DonationRange         *string   `db:"donation_range"`
	NotificationFrequency *string   `db:"notification_frequency"`
	CreatedAt             time.Time `db:"created_at"`
	UpdatedAt             time.Time `db:"updated_at"`
}

type DonorPreferenceCategoryAssignment struct {
	UserID     string    `db:"user_id"`
	CategoryID string    `db:"category_id"`
	CreatedAt  time.Time `db:"created_at"`
}
