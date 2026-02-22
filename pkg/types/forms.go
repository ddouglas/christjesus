package types

import "time"

type PrayerRequest struct {
	ID          string    `db:"id"`
	Name        string    `db:"name"`
	Email       *string   `db:"email"`
	RequestBody string    `db:"request_body"`
	CreatedAt   time.Time `db:"created_at"`
}

type EmailSignup struct {
	Email     string    `db:"email"`
	City      *string   `db:"city"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type CategoryData struct {
	Name  string
	Slug  string
	Count int
	Icon  string
}

type StatsData struct {
	TotalRaised  int64
	NeedsFunded  int
	LivesChanged int
}

type StepData struct {
	Number      int
	Title       string
	Description string
}
