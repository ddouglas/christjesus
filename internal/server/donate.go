package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"christjesus/internal/utils"
	"christjesus/pkg/types"

	"github.com/stripe/stripe-go/v84"
)

var donatePresetAmounts = []int{25, 50, 100, 250}

func (s *Service) handleGetNeedDonate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("id")

	data, err := s.buildNeedDonatePageData(ctx, needID, &types.NeedDonatePageData{PresetAmounts: donatePresetAmounts})
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to build donate page data")
		s.internalServerError(w)
		return
	}

	if err := s.renderTemplate(w, r, "page.need-donate", data); err != nil {
		s.logger.WithError(err).Error("failed to render need donate page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostNeedDonate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to parse donate form")
		s.internalServerError(w)
		return
	}

	selectedPreset, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("preset_amount")))
	customAmount := strings.TrimSpace(r.FormValue("custom_amount"))
	privateMessage := strings.TrimSpace(r.FormValue("private_message"))
	isAnonymous := r.FormValue("is_anonymous") == "on"

	data := &types.NeedDonatePageData{
		PresetAmounts:  donatePresetAmounts,
		SelectedPreset: selectedPreset,
		CustomAmount:   customAmount,
		PrivateMessage: privateMessage,
		IsAnonymous:    isAnonymous,
	}

	data, err := s.buildNeedDonatePageData(ctx, needID, data)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to build donate page data")
		s.internalServerError(w)
		return
	}

	amountCents := 0
	if customAmount != "" {
		amountCents, err = parseDonationAmountCents(customAmount)
		if err != nil || amountCents <= 0 {
			data.Error = "Enter a valid custom amount in whole dollars."
			if renderErr := s.renderTemplate(w, r, "page.need-donate", data); renderErr != nil {
				s.logger.WithError(renderErr).Error("failed to render need donate page with validation error")
				s.internalServerError(w)
			}
			return
		}
	} else if selectedPreset > 0 {
		amountCents = selectedPreset * 100
	}

	if amountCents <= 0 {
		data.Error = "Select a preset amount or enter a custom amount."
		if renderErr := s.renderTemplate(w, r, "page.need-donate", data); renderErr != nil {
			s.logger.WithError(renderErr).Error("failed to render need donate page with validation error")
			s.internalServerError(w)
		}
		return
	}

	if len(privateMessage) > 1000 {
		data.Error = "Private message must be 1000 characters or fewer."
		if renderErr := s.renderTemplate(w, r, "page.need-donate", data); renderErr != nil {
			s.logger.WithError(renderErr).Error("failed to render need donate page with validation error")
			s.internalServerError(w)
		}
		return
	}

	var donorUserID *string
	if userID, ok := ctx.Value(contextKeyUserID).(string); ok {
		trimmed := strings.TrimSpace(userID)
		if trimmed != "" {
			donorUserID = &trimmed
		}
	}
	if donorUserID == nil {
		s.setRedirectCookie(w, r.URL.Path, time.Minute*5)
		http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
		return
	}

	intent := &types.DonationIntent{
		ID:              utils.NanoID(),
		NeedID:          needID,
		DonorUserID:     donorUserID,
		AmountCents:     amountCents,
		IsAnonymous:     isAnonymous,
		PaymentProvider: types.DonationPaymentProviderStripe,
		PaymentStatus:   types.DonationPaymentStatusPending,
	}
	if privateMessage != "" {
		intent.PrivateMessage = &privateMessage
	}

	if err := s.donationIntentRepo.Create(ctx, intent); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to create donation intent")
		data.Error = "Unable to save your donation right now. Please try again."
		if renderErr := s.renderTemplate(w, r, "page.need-donate", data); renderErr != nil {
			s.logger.WithError(renderErr).Error("failed to render need donate page after persistence failure")
			s.internalServerError(w)
		}
		return
	}

	if s.stripeClient == nil {
		if _, markErr := s.donationIntentRepo.MarkIntentFailedByID(ctx, intent.ID, nil, nil); markErr != nil {
			s.logger.WithError(markErr).WithField("intent_id", intent.ID).Warn("failed to mark donation intent failed after stripe client unavailable")
		}
		data.Error = "Payments are not configured yet. Please try again later."
		if renderErr := s.renderTemplate(w, r, "page.need-donate", data); renderErr != nil {
			s.logger.WithError(renderErr).Error("failed to render need donate page with stripe config error")
			s.internalServerError(w)
		}
		return
	}

	successQuery := make(url.Values)
	successQuery.Set("intent_id", intent.ID)
	successURL := s.absoluteRoute(RouteNeedDonateConfirmation, map[string]string{"id": needID}, successQuery)
	cancelURL := s.absoluteRoute(RouteNeedDonate, map[string]string{"id": needID}, nil)

	donorEmail := s.resolveDonorCheckoutEmail(ctx, r, donorUserID)

	checkoutParams := &stripe.CheckoutSessionCreateParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		PaymentIntentData: &stripe.CheckoutSessionCreatePaymentIntentDataParams{
			Metadata: map[string]string{
				"donation_intent_id": intent.ID,
				"need_id":            needID,
			},
		},
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
					Currency:   stripe.String(string(stripe.CurrencyUSD)),
					UnitAmount: stripe.Int64(int64(amountCents)),
					ProductData: &stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{
						Name:        stripe.String(fmt.Sprintf("Support %s", data.OwnerName)),
						Description: stripe.String(fmt.Sprintf("Donation for need %s", needID)),
					},
				},
			},
		},
		ClientReferenceID: stripe.String(intent.ID),
		Metadata: map[string]string{
			"donation_intent_id": intent.ID,
			"need_id":            needID,
		},
	}
	if donorEmail != "" {
		checkoutParams.CustomerEmail = stripe.String(donorEmail)
	}

	checkoutSession, err := s.stripeClient.V1CheckoutSessions.Create(ctx, checkoutParams)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to create stripe checkout session")
		if _, markErr := s.donationIntentRepo.MarkIntentFailedByID(ctx, intent.ID, nil, nil); markErr != nil {
			s.logger.WithError(markErr).WithField("intent_id", intent.ID).Warn("failed to mark donation intent failed after stripe checkout create failure")
		}
		data.Error = "Unable to start Stripe checkout right now. Please try again."
		if renderErr := s.renderTemplate(w, r, "page.need-donate", data); renderErr != nil {
			s.logger.WithError(renderErr).Error("failed to render need donate page after checkout create failure")
			s.internalServerError(w)
		}
		return
	}

	if checkoutSession == nil || strings.TrimSpace(checkoutSession.ID) == "" || strings.TrimSpace(checkoutSession.URL) == "" {
		if _, markErr := s.donationIntentRepo.MarkIntentFailedByID(ctx, intent.ID, nil, nil); markErr != nil {
			s.logger.WithError(markErr).WithField("intent_id", intent.ID).Warn("failed to mark donation intent failed after invalid checkout session response")
		}
		data.Error = "Unable to start Stripe checkout right now. Please try again."
		if renderErr := s.renderTemplate(w, r, "page.need-donate", data); renderErr != nil {
			s.logger.WithError(renderErr).Error("failed to render need donate page after invalid checkout session")
			s.internalServerError(w)
		}
		return
	}

	if err := s.donationIntentRepo.SetCheckoutSessionID(ctx, intent.ID, checkoutSession.ID); err != nil {
		checkoutSessionID := checkoutSession.ID
		if _, markErr := s.donationIntentRepo.MarkIntentFailedByID(ctx, intent.ID, &checkoutSessionID, nil); markErr != nil {
			s.logger.WithError(markErr).WithField("intent_id", intent.ID).Warn("failed to mark donation intent failed after checkout session persistence failure")
		}
		s.logger.WithError(err).WithField("intent_id", intent.ID).Error("failed to persist checkout session id on donation intent")
		data.Error = "Unable to start Stripe checkout right now. Please try again."
		if renderErr := s.renderTemplate(w, r, "page.need-donate", data); renderErr != nil {
			s.logger.WithError(renderErr).Error("failed to render need donate page after checkout session persistence failure")
			s.internalServerError(w)
		}
		return
	}

	http.Redirect(w, r, checkoutSession.URL, http.StatusSeeOther)

}

func (s *Service) handleGetNeedDonateConfirmation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("id")
	intentID := strings.TrimSpace(r.URL.Query().Get("intent_id"))
	if intentID == "" {
		http.Redirect(w, r, s.route(RouteNeedDonate, map[string]string{"id": needID}), http.StatusSeeOther)
		return
	}

	intent, err := s.donationIntentRepo.ByID(ctx, intentID)
	if err != nil {
		s.logger.WithError(err).WithField("intent_id", intentID).Error("failed to fetch donation intent")
		s.internalServerError(w)
		return
	}
	if intent == nil || intent.NeedID != needID {
		http.NotFound(w, r)
		return
	}

	need, ownerName, primaryCategory, err := s.loadNeedDonateSummary(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to build donate confirmation page data")
		s.internalServerError(w)
		return
	}

	data := &types.NeedDonateConfirmationPageData{
		BasePageData:       types.BasePageData{Title: "Donation Confirmation"},
		NeedID:             need.ID,
		IntentID:           intent.ID,
		OwnerName:          ownerName,
		AmountCents:        intent.AmountCents,
		IsAnonymous:        intent.IsAnonymous,
		PrimaryCategory:    primaryCategory,
		PaymentStatus:      intent.PaymentStatus,
		StatusLabel:        donationStatusLabel(intent.PaymentStatus),
		StatusTitle:        donationStatusTitle(intent.PaymentStatus, ownerName),
		StatusDescription:  donationStatusDescription(intent.PaymentStatus),
		StatusGuidance:     donationStatusGuidance(intent.PaymentStatus),
		ShowRetryCTA:       intent.PaymentStatus == types.DonationPaymentStatusFailed || intent.PaymentStatus == types.DonationPaymentStatusCanceled,
		ShowReceiptDetails: intent.PaymentStatus == types.DonationPaymentStatusFinalized,
		DonationDate:       donationConfirmationDate(intent),
	}

	if err := s.renderTemplate(w, r, "page.need-donate-confirmation", data); err != nil {
		s.logger.WithError(err).Error("failed to render donate confirmation page")
		s.internalServerError(w)
		return
	}
}

func donationStatusLabel(status string) string {
	switch strings.TrimSpace(status) {
	case types.DonationPaymentStatusFinalized:
		return "Payment Complete"
	case types.DonationPaymentStatusFailed:
		return "Payment Failed"
	case types.DonationPaymentStatusCanceled:
		return "Payment Canceled"
	default:
		return "Payment Processing"
	}
}

func donationStatusTitle(status, ownerName string) string {
	switch strings.TrimSpace(status) {
	case types.DonationPaymentStatusFinalized:
		return fmt.Sprintf("Thank you for supporting %s", ownerName)
	case types.DonationPaymentStatusFailed:
		return "We couldn't complete your donation"
	case types.DonationPaymentStatusCanceled:
		return "Donation was canceled"
	default:
		return "Thanks — we captured your donation"
	}
}

func donationStatusDescription(status string) string {
	switch strings.TrimSpace(status) {
	case types.DonationPaymentStatusFinalized:
		return "Your donation is finalized and has been applied to this need."
	case types.DonationPaymentStatusFailed:
		return "Your payment did not complete. No finalized donation was recorded for this attempt."
	case types.DonationPaymentStatusCanceled:
		return "You exited checkout before completion, so no finalized donation was recorded."
	default:
		return "We received your donation and payment is still processing."
	}
}

func donationStatusGuidance(status string) string {
	switch strings.TrimSpace(status) {
	case types.DonationPaymentStatusFinalized:
		return "You can keep this page as your confirmation summary and view your receipt from Profile → Donations."
	case types.DonationPaymentStatusFailed:
		return "Please try again using the retry button below. If this continues, contact support with your donation reference ID."
	case types.DonationPaymentStatusCanceled:
		return "You can return to the donation form and submit again whenever you're ready."
	default:
		return "If this remains in processing, refresh shortly or check your donation status from your profile."
	}
}

func donationConfirmationDate(intent *types.DonationIntent) string {
	if intent == nil {
		return ""
	}

	if strings.TrimSpace(intent.PaymentStatus) == types.DonationPaymentStatusFinalized {
		return intent.UpdatedAt.Local().Format("Jan 2, 2006 3:04 PM MST")
	}

	return intent.CreatedAt.Local().Format("Jan 2, 2006 3:04 PM MST")
}

func (s *Service) buildNeedDonatePageData(ctx context.Context, needID string, data *types.NeedDonatePageData) (*types.NeedDonatePageData, error) {
	need, ownerName, primaryCategory, err := s.loadNeedDonateSummary(ctx, needID)
	if err != nil {
		return nil, err
	}

	if data == nil {
		data = &types.NeedDonatePageData{}
	}

	data.BasePageData = types.BasePageData{Title: "Donate"}
	data.NeedID = need.ID
	data.OwnerName = ownerName
	data.PrimaryCategory = primaryCategory
	data.ShortDescription = need.ShortDescription
	data.AmountNeededCents = need.AmountNeededCents
	data.AmountRaisedCents = need.AmountRaisedCents
	if len(data.PresetAmounts) == 0 {
		data.PresetAmounts = donatePresetAmounts
	}

	return data, nil
}

func (s *Service) loadNeedDonateSummary(ctx context.Context, needID string) (*types.Need, string, string, error) {
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		return nil, "", "", err
	}

	if err := s.applyFinalizedRaisedAmount(ctx, need); err != nil {
		return nil, "", "", err
	}

	ownerName := "Anonymous"
	user, err := s.userRepo.User(ctx, need.UserID)
	if err == nil {
		ownerName = userDisplayName(user)
	} else if !errors.Is(err, types.ErrUserNotFound) {
		s.logger.WithError(err).WithField("user_id", need.UserID).Warn("failed to fetch need owner for donate page")
	}

	primaryCategoryName := "General Need"
	assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID(ctx, needID)
	if err != nil {
		return nil, "", "", err
	}

	for _, assignment := range assignments {
		if !assignment.IsPrimary {
			continue
		}
		category, err := s.categoryRepo.CategoryByID(ctx, assignment.CategoryID)
		if err != nil {
			return nil, "", "", err
		}
		if category != nil && strings.TrimSpace(category.Name) != "" {
			primaryCategoryName = category.Name
		}
		break
	}

	return need, ownerName, primaryCategoryName, nil
}

func parseDonationAmountCents(raw string) (int, error) {
	normalized := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(raw, "$", ""), ",", ""))
	if normalized == "" {
		return 0, fmt.Errorf("amount is empty")
	}

	amountDollars, err := strconv.Atoi(normalized)
	if err != nil {
		return 0, err
	}

	if amountDollars <= 0 {
		return 0, fmt.Errorf("amount must be positive")
	}

	return amountDollars * 100, nil
}

func (s *Service) resolveDonorCheckoutEmail(ctx context.Context, r *http.Request, donorUserID *string) string {
	if email, ok := ctx.Value(contextKeyEmail).(string); ok {
		trimmed := strings.TrimSpace(email)
		if trimmed != "" {
			return trimmed
		}
	}

	if claims, ok := s.authClaimsFromRequest(r); ok {
		trimmed := strings.TrimSpace(claims.Email)
		if trimmed != "" {
			return trimmed
		}
	}

	if donorUserID == nil {
		return ""
	}

	user, err := s.userRepo.User(ctx, *donorUserID)
	if err != nil || user == nil || user.Email == nil {
		return ""
	}

	return strings.TrimSpace(*user.Email)
}
