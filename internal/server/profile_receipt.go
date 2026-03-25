package server

import (
	"bytes"
	"christjesus/pkg/types"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/go-pdf/fpdf"
)

func (s *Service) handleGetProfileDonationReceipt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	intentID := strings.TrimSpace(r.PathValue("intentID"))
	if intentID == "" {
		s.redirectProfileWithError(w, r, "Donation not found.")
		return
	}

	intent, err := s.donationIntentRepo.ByID(ctx, intentID)
	if err != nil {
		s.logger.WithError(err).WithField("intent_id", intentID).Error("failed to fetch donation intent for receipt")
		s.internalServerError(w)
		return
	}
	if intent == nil || intent.DonorUserID == nil || strings.TrimSpace(*intent.DonorUserID) != userID {
		s.redirectProfileWithError(w, r, "Donation not found.")
		return
	}

	if strings.TrimSpace(strings.ToLower(intent.PaymentStatus)) != types.DonationPaymentStatusFinalized {
		s.redirectProfileWithError(w, r, "Receipt is available after payment is finalized.")
		return
	}

	need, err := s.needsRepo.Need(ctx, intent.NeedID)
	if err != nil && !errors.Is(err, types.ErrNeedNotFound) {
		s.logger.WithError(err).WithField("need_id", intent.NeedID).Error("failed to fetch need for donation receipt")
		s.internalServerError(w)
		return
	}
	if errors.Is(err, types.ErrNeedNotFound) {
		s.redirectProfileWithError(w, r, "Need not found for this donation receipt.")
		return
	}

	session, ok := sessionFromRequest(r)
	if !ok {
		s.logger.Error("session not found on context")
		s.internalServerError(w)
		return
	}

	needLabel := strings.TrimSpace(derefString(need.ShortDescription))
	if needLabel == "" {
		needLabel = "Need request"
	}

	receiptKey := fmt.Sprintf("receipts/donations/%s.pdf", intent.ID)
	filename := fmt.Sprintf("christjesus-receipt-%s.pdf", intent.ID)

	cachedReceipt, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.S3BucketName),
		Key:    aws.String(receiptKey),
	})
	if err == nil {
		defer cachedReceipt.Body.Close()
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
		w.Header().Set("Cache-Control", "private, no-store")
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, cachedReceipt.Body); err != nil {
			s.logger.WithError(err).WithField("intent_id", intent.ID).Error("failed to stream cached donation receipt pdf")
		}
		return
	}

	if !isS3NotFoundError(err) {
		s.logger.WithError(err).WithField("intent_id", intent.ID).Error("failed to load cached donation receipt from s3")
		s.internalServerError(w)
		return
	}

	pdfBytes, err := buildDonationReceiptPDF(types.ProfileDonationSummary{
		IntentID:    intent.ID,
		NeedID:      intent.NeedID,
		NeedLabel:   needLabel,
		Amount:      formatUSDFromCents(intent.AmountCents),
		Status:      formatDonationStatus(intent.PaymentStatus),
		IsAnonymous: intent.IsAnonymous,
		CreatedAt:   intent.CreatedAt.Format("Jan 2, 2006 3:04 PM MST"),
	}, session.DisplayName, session.Email, s.config.AppBaseURL)
	if err != nil {
		s.logger.WithError(err).WithField("intent_id", intent.ID).Error("failed to generate donation receipt pdf")
		s.internalServerError(w)
		return
	}

	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.config.S3BucketName),
		Key:         aws.String(receiptKey),
		Body:        bytes.NewReader(pdfBytes),
		ContentType: aws.String("application/pdf"),
	})
	if err != nil {
		s.logger.WithError(err).WithField("intent_id", intent.ID).Error("failed to store donation receipt pdf in s3")
		s.internalServerError(w)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(pdfBytes); err != nil {
		s.logger.WithError(err).WithField("intent_id", intent.ID).Error("failed to write donation receipt pdf response")
		return
	}
}

func buildDonationReceiptPDF(summary types.ProfileDonationSummary, donorName, donorEmail, appBaseURL string) ([]byte, error) {
	safeIntentID := pdfSafeText(summary.IntentID)
	safeCreatedAt := pdfSafeText(summary.CreatedAt)
	safeAmount := pdfSafeText(summary.Amount)
	safeStatus := pdfSafeText(summary.Status)
	safeNeedLabel := pdfSafeText(summary.NeedLabel)
	safeDonorName := pdfSafeText(donorName)
	safeDonorEmail := pdfSafeText(donorEmail)

	pdf := fpdf.New("P", "mm", "Letter", "")
	pdf.SetAutoPageBreak(true, 12)
	pdf.SetTitle("ChristJesus Donation Receipt "+safeIntentID, false)
	pdf.SetAuthor("ChristJesus", false)
	pdf.SetSubject("Donation receipt", false)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 18)
	pdf.CellFormat(0, 12, "Donation Receipt", "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(70, 70, 70)
	pdf.CellFormat(0, 7, "ChristJesus", "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 7, "Generated: "+time.Now().Format("Jan 2, 2006 3:04 PM MST"), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetTextColor(20, 20, 20)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Receipt Details", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(0, 7, "Receipt ID: "+safeIntentID, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 7, "Donation Date: "+safeCreatedAt, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 7, "Amount: "+safeAmount, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 7, "Status: "+safeStatus, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 7, "Anonymous Donation: "+map[bool]string{true: "Yes", false: "No"}[summary.IsAnonymous], "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Need", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.MultiCell(0, 7, safeNeedLabel, "", "L", false)
	needPath, err := BuildRoute(RouteNeedDetail, map[string]string{"needID": summary.NeedID})
	if err != nil {
		needPath = "/need/" + summary.NeedID
	}

	needURL := needPath
	if baseURL := strings.TrimRight(strings.TrimSpace(appBaseURL), "/"); baseURL != "" {
		needURL = baseURL + needPath
	}
	pdf.SetTextColor(20, 20, 20)
	pdf.CellFormat(22, 7, "Need Link:", "", 0, "L", false, 0, "")
	pdf.SetTextColor(37, 99, 235)
	pdf.CellFormat(0, 7, "View need", "", 1, "L", false, 0, needURL)
	pdf.SetTextColor(20, 20, 20)
	pdf.Ln(2)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Donor", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(0, 7, "Name: "+safeDonorName, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 7, "Email: "+safeDonorEmail, "", 1, "L", false, 0, "")
	pdf.Ln(4)

	pdf.SetTextColor(90, 90, 90)
	pdf.SetFont("Helvetica", "", 9)
	pdf.MultiCell(0, 5, "This document is a donation receipt generated by ChristJesus from payment records. Keep this receipt for your records.", "", "L", false)

	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

func pdfSafeText(value string) string {
	clean := strings.ToValidUTF8(value, "")
	var b strings.Builder
	b.Grow(len(clean))
	for _, r := range clean {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteRune(r)
		case r >= 32 && r <= 255:
			b.WriteRune(r)
		default:
			b.WriteRune('?')
		}
	}

	return strings.TrimSpace(b.String())
}

func isS3NotFoundError(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}

	code := strings.TrimSpace(apiErr.ErrorCode())
	return code == "NoSuchKey" || code == "NotFound"
}
