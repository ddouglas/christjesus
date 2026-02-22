package types

import "time"

type NeedStory struct {
	NeedID    string    `db:"need_id"`
	Current   *string   `db:"current" form:"storyCurrent"`
	Need      *string   `db:"need" form:"storyNeed"`
	Outcome   *string   `db:"outcome" form:"storyOutcome"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
