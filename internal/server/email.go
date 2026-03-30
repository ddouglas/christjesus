package server

import (
	"context"
	"fmt"
	"strings"

	internalemail "christjesus/internal/email"
	"christjesus/internal/utils"
	"christjesus/pkg/types"
)

// sendEmail is the generic email dispatch helper.
//
// It checks the suppression list, inserts an email_messages record (queued),
// optionally links the message to a donation intent and/or user, calls the
// configured Sender, then updates the status to "sent". If the Sender is not
// configured the call is a no-op.
func (s *Service) sendEmail(
	ctx context.Context,
	msg internalemail.Message,
	emailType string,
	donationIntentID *string,
	userID *string,
) error {
	if s.emailSender == nil {
		s.logger.WithField("email_type", emailType).Debug("email sender not configured — skipping send")
		return nil
	}

	suppressed, err := s.emailRepo.IsEmailSuppressed(ctx, msg.To)
	if err != nil {
		return fmt.Errorf("check email suppression: %w", err)
	}
	if suppressed {
		s.logger.WithFields(map[string]any{
			"email_type": emailType,
			"recipient":  msg.To,
		}).Info("email recipient is suppressed — skipping send")
		return nil
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
		return fmt.Errorf("insert email message: %w", err)
	}

	if donationIntentID != nil {
		link := &types.DonationIntentEmail{
			ID:               utils.NanoID(),
			DonationIntentID: *donationIntentID,
			EmailMessageID:   record.ID,
			EmailType:        emailType,
		}
		if err := s.emailRepo.InsertDonationIntentEmail(ctx, link); err != nil {
			return fmt.Errorf("link email to donation intent: %w", err)
		}
	}

	if userID != nil {
		link := &types.UserEmail{
			ID:             utils.NanoID(),
			UserID:         *userID,
			EmailMessageID: record.ID,
			EmailType:      emailType,
		}
		if err := s.emailRepo.InsertUserEmail(ctx, link); err != nil {
			return fmt.Errorf("link email to user: %w", err)
		}
	}

	result, err := s.emailSender.Send(ctx, msg)
	if err != nil {
		s.logger.WithError(err).WithFields(map[string]any{
			"email_message_id": record.ID,
			"email_type":       emailType,
			"recipient":        msg.To,
		}).Error("failed to send email via provider")
		return fmt.Errorf("send email: %w", err)
	}

	if err := s.emailRepo.UpdateEmailMessageStatus(ctx, record.ID, types.EmailStatusSent, &result.ProviderMessageID); err != nil {
		// Non-fatal: the email was sent; just log the tracking failure.
		s.logger.WithError(err).WithField("email_message_id", record.ID).Warn("failed to update email message status to sent")
	}

	return nil
}

// maybeSendDonationReceiptEmail fetches the intent and donor details, then
// sends a receipt email if the donor is identifiable and has an email address.
// It is safe to call even when the email sender is not configured.
func (s *Service) maybeSendDonationReceiptEmail(ctx context.Context, intentID string) error {
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
	if user == nil || user.Email == nil {
		return nil
	}

	donorName := ""
	if user.GivenName != nil {
		donorName = *user.GivenName
	}

	return s.sendDonationReceiptEmail(ctx, intent, *user.Email, donorName)
}

// sendDonationReceiptEmail sends a donation receipt to the donor.
func (s *Service) sendDonationReceiptEmail(ctx context.Context, intent *types.DonationIntent, donorEmail, donorName string) error {
	from := strings.TrimSpace(s.config.EmailFromAddress)
	if from == "" {
		from = "noreply@christjesus.app"
	}

	receiptURL := fmt.Sprintf("%s/profile/donations/%s/receipt", strings.TrimRight(s.config.AppBaseURL, "/"), intent.ID)
	amountDollars := float64(intent.AmountCents) / 100.0

	subject := "Thank you for your donation!"

	greeting := "Hello"
	if trimmed := strings.TrimSpace(donorName); trimmed != "" {
		greeting = "Hello, " + trimmed
	}

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family:sans-serif;max-width:600px;margin:0 auto;padding:24px;color:#1a1a1a">
  <h2 style="color:#C9A84C">Thank you for your donation!</h2>
  <p>%s,</p>
  <p>Your donation of <strong>$%.2f</strong> has been received. We are grateful for your generosity and support.</p>
  <p><a href="%s" style="display:inline-block;padding:12px 24px;background:#C9A84C;color:#0D1B2A;text-decoration:none;border-radius:4px;font-weight:bold">View your receipt</a></p>
  <p style="color:#666;font-size:14px">If the button above does not work, copy and paste this link into your browser:<br>%s</p>
  <hr style="border:none;border-top:1px solid #eee;margin:24px 0">
  <p style="color:#999;font-size:12px">ChristJesus.app — connecting donors with verified needs</p>
</body>
</html>`, greeting, amountDollars, receiptURL, receiptURL)

	textBody := fmt.Sprintf("%s,\n\nThank you for your donation of $%.2f.\n\nView your receipt: %s\n\nChristJesus.app — connecting donors with verified needs",
		greeting, amountDollars, receiptURL)

	msg := internalemail.Message{
		From:     from,
		To:       donorEmail,
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: textBody,
	}

	return s.sendEmail(ctx, msg, types.EmailTypeDonationReceipt, &intent.ID, intent.DonorUserID)
}
