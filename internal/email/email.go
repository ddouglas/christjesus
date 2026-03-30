package email

import "context"

// Message is a provider-agnostic email message.
type Message struct {
	To       string
	From     string
	ReplyTo  string
	Subject  string
	HTMLBody string
	TextBody string
}

// SendResult is returned by a successful Send call.
type SendResult struct {
	ProviderMessageID string
}

// Sender is the provider-agnostic interface for sending transactional email.
type Sender interface {
	Send(ctx context.Context, msg Message) (SendResult, error)
}
