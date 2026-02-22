package types

import "time"

type NeedCategory struct {
	ID           string    `db:"id"`
	Name         string    `db:"name"`
	Slug         string    `db:"slug"`
	Description  *string   `db:"description"`
	Icon         *string   `db:"icon"`
	DisplayOrder int       `db:"display_order"`
	IsActive     bool      `db:"is_active"`
	CreatedAt    time.Time `db:"created_at"`
}

type NeedCategoryAssignment struct {
	NeedID     string    `db:"need_id"`
	CategoryID string    `db:"category_id"`
	IsPrimary  bool      `db:"is_primary"`
	CreatedAt  time.Time `db:"created_at"`

	Need     *Need         `db:"-"`
	Category *NeedCategory `db:"-"`
}
