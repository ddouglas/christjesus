package server

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	internalemail "christjesus/internal/email"
	"christjesus/internal/utils"
	"christjesus/pkg/types"
)

// sendEmailOption configures a sendEmail call.
type sendEmailOption func(*sendEmailConfig)

// sendEmailConfig holds optional parameters for sendEmail.
type sendEmailConfig struct{}

// sendEmail is the generic email dispatch helper.
//
// It checks the suppression list, inserts an email_messages record (queued),
// calls the configured Sender, then updates the status to "sent".
// Callers are responsible for creating any association records (e.g.
// donation_intent_emails, user_emails) using the returned EmailMessage.
func (s *Service) sendEmail(ctx context.Context, msg internalemail.Message, emailType string, opts ...sendEmailOption) (*types.EmailMessage, error) {
	suppressed, err := s.emailRepo.IsEmailSuppressed(ctx, msg.To)
	if err != nil {
		return nil, fmt.Errorf("check email suppression: %w", err)
	}
	if suppressed {
		s.logger.WithFields(map[string]any{
			"email_type": emailType,
			"recipient":  msg.To,
		}).Info("email recipient is suppressed — skipping send")
		return nil, nil
	}

	record := &types.EmailMessage{
		ID:        utils.NanoID(),
		Recipient: msg.To,
		EmailType: emailType,
		Subject:   msg.Subject,
		Provider:  "resend",
		Status:    types.EmailStatusQueued,
	}
	if err := s.emailRepo.InsertEmailMessage(ctx, record); err != nil {
		return nil, fmt.Errorf("insert email message: %w", err)
	}

	result, err := s.emailSender.Send(ctx, msg)
	if err != nil {
		s.logger.WithError(err).WithFields(map[string]any{
			"email_message_id": record.ID,
			"email_type":       emailType,
			"recipient":        msg.To,
		}).Error("failed to send email via provider")
		return nil, fmt.Errorf("send email: %w", err)
	}

	if err := s.emailRepo.UpdateEmailMessageStatus(ctx, record.ID, types.EmailStatusSent, &result.ProviderMessageID); err != nil {
		// Non-fatal: the email was sent; just log the tracking failure.
		s.logger.WithError(err).WithField("email_message_id", record.ID).Warn("failed to update email message status to sent")
	}

	return record, nil
}

type donationReceiptTemplateData struct {
	DonorName     string
	AmountDollars float64
	ReceiptURL    string
}

// sendDonationReceiptEmail fetches the intent and donor, then sends a receipt.
// Returns nil without sending if the donation was anonymous.
func (s *Service) sendDonationReceiptEmail(ctx context.Context, intentID string) error {
	intent, err := s.donationIntentRepo.ByID(ctx, intentID)
	if err != nil {
		return fmt.Errorf("fetch donation intent for receipt: %w", err)
	}
	if intent == nil || intent.DonorUserID == nil {
		return nil
	}

	user, err := s.userRepo.User(ctx, *intent.DonorUserID)
	if err != nil {
		return fmt.Errorf("fetch donor user for receipt: %w", err)
	}
	if user == nil {
		return fmt.Errorf("donor user %s not found", *intent.DonorUserID)
	}
	if user.Email == nil {
		return fmt.Errorf("donor user %s has no email address", user.ID)
	}

	from := strings.TrimSpace(s.config.EmailFromAddress)
	if from == "" {
		from = "noreply@christjesus.app"
	}

	receiptURL := fmt.Sprintf("%s/profile/donations/%s/receipt", strings.TrimRight(s.config.AppBaseURL, "/"), intent.ID)

	donorName := ""
	if user.GivenName != nil {
		donorName = *user.GivenName
	}

	var htmlBuf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&htmlBuf, "email.donation-receipt", donationReceiptTemplateData{
		DonorName:     donorName,
		AmountDollars: float64(intent.AmountCents) / 100.0,
		ReceiptURL:    receiptURL,
	}); err != nil {
		return fmt.Errorf("render donation receipt template: %w", err)
	}

	textBody := fmt.Sprintf("Thank you for your donation of $%.2f.\n\nView your receipt: %s\n\nChristJesus.app",
		float64(intent.AmountCents)/100.0, receiptURL)

	record, err := s.sendEmail(ctx, internalemail.Message{
		From:     from,
		To:       *user.Email,
		Subject:  "Thank you for your donation!",
		HTMLBody: htmlBuf.String(),
		TextBody: textBody,
	}, types.EmailTypeDonationReceipt)
	if err != nil {
		return err
	}
	if record == nil {
		// Recipient was suppressed; nothing to link.
		return nil
	}

	if err := s.emailRepo.InsertDonationIntentEmail(ctx, &types.DonationIntentEmail{
		ID:               utils.NanoID(),
		DonationIntentID: intent.ID,
		EmailMessageID:   record.ID,
		EmailType:        types.EmailTypeDonationReceipt,
	}); err != nil {
		return fmt.Errorf("link receipt email to donation intent: %w", err)
	}

	if err := s.emailRepo.InsertUserEmail(ctx, &types.UserEmail{
		ID:             utils.NanoID(),
		UserID:         user.ID,
		EmailMessageID: record.ID,
		EmailType:      types.EmailTypeDonationReceipt,
	}); err != nil {
		return fmt.Errorf("link receipt email to user: %w", err)
	}

	return nil
}
