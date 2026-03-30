package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const resendDefaultBaseURL = "https://api.resend.com"

type resendSenderOptions struct {
	baseURL    string
	httpClient *http.Client
}

// SenderOption configures a ResendSender.
type SenderOption func(*resendSenderOptions)

// WithBaseURL overrides the Resend API base URL. Primarily used in tests.
func WithBaseURL(url string) SenderOption {
	return func(o *resendSenderOptions) {
		o.baseURL = strings.TrimRight(url, "/")
	}
}

// ResendSender sends transactional email via the Resend API.
type ResendSender struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewResendSender returns a new ResendSender. Returns an error if apiKey is empty.
func NewResendSender(apiKey string, opts ...SenderOption) (*ResendSender, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("resend: API key is required")
	}
	o := &resendSenderOptions{
		baseURL:    resendDefaultBaseURL,
		httpClient: &http.Client{},
	}
	for _, opt := range opts {
		opt(o)
	}
	return &ResendSender{
		apiKey:  apiKey,
		baseURL: o.baseURL,
		client:  o.httpClient,
	}, nil
}

type resendSendRequest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html,omitempty"`
	Text    string `json:"text,omitempty"`
	ReplyTo string `json:"reply_to,omitempty"`
}

type resendSendResponse struct {
	ID string `json:"id"`
}

type resendErrorResponse struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// Send sends a single transactional email via the Resend API.
func (s *ResendSender) Send(ctx context.Context, msg Message) (SendResult, error) {
	payload := resendSendRequest{
		From:    msg.From,
		To:      msg.To,
		Subject: msg.Subject,
		HTML:    msg.HTMLBody,
		Text:    msg.TextBody,
		ReplyTo: msg.ReplyTo,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return SendResult{}, fmt.Errorf("resend: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return SendResult{}, fmt.Errorf("resend: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return SendResult{}, fmt.Errorf("resend: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var resendErr resendErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&resendErr)
		return SendResult{}, fmt.Errorf("resend: API error %d: %s: %s", resp.StatusCode, resendErr.Name, resendErr.Message)
	}

	var result resendSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return SendResult{}, fmt.Errorf("resend: decode response: %w", err)
	}

	return SendResult{ProviderMessageID: result.ID}, nil
}
