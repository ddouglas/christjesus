package types

import "time"

type SavedNeed struct {
	UserID    string    `db:"user_id"`
	NeedID    string    `db:"need_id"`
	CreatedAt time.Time `db:"created_at"`
}
