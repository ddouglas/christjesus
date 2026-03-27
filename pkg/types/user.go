package types

import "time"

type UserType string

const (
	UserTypeRecipient UserType = "recipient"
	UserTypeDonor     UserType = "donor"
	UserTypeSponsor   UserType = "sponsor"
)

type User struct {
	ID          string    `db:"id"`
	AuthSubject *string   `db:"auth_subject"`
	UserType    *string   `db:"user_type"`
	Email       *string   `db:"email"`
	GivenName   *string   `db:"given_name"`
	FamilyName  *string   `db:"family_name"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}
