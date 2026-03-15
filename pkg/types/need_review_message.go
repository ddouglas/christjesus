package types

import "time"

const NeedReviewMessageMaxChars = 2000

type NeedReviewMessageSenderRole string

const (
	NeedReviewMessageSenderRoleUser  NeedReviewMessageSenderRole = "user"
	NeedReviewMessageSenderRoleAdmin NeedReviewMessageSenderRole = "admin"
)

type NeedReviewMessage struct {
	ID           string                      `db:"id"`
	NeedID       string                      `db:"need_id"`
	SenderUserID string                      `db:"sender_user_id"`
	SenderRole   NeedReviewMessageSenderRole `db:"sender_role"`
	Body         string                      `db:"body"`
	CreatedAt    time.Time                   `db:"created_at"`
}
