package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"christjesus/internal/utils"
	"christjesus/pkg/types"
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
		data.Error = "Unable to save your donation intent right now. Please try again."
		if renderErr := s.renderTemplate(w, r, "page.need-donate", data); renderErr != nil {
			s.logger.WithError(renderErr).Error("failed to render need donate page after persistence failure")
			s.internalServerError(w)
		}
		return
	}

	v := url.Values{}
	v.Set("intent_id", intent.ID)
	http.Redirect(w, r, fmt.Sprintf("/need/%s/donate/confirmation?%s", needID, v.Encode()), http.StatusSeeOther)
}

func (s *Service) handleGetNeedDonateConfirmation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("id")
	intentID := strings.TrimSpace(r.URL.Query().Get("intent_id"))
	if intentID == "" {
		http.Redirect(w, r, fmt.Sprintf("/need/%s/donate", needID), http.StatusSeeOther)
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
		BasePageData:    types.BasePageData{Title: "Donation Confirmation"},
		NeedID:          need.ID,
		IntentID:        intent.ID,
		OwnerName:       ownerName,
		AmountCents:     intent.AmountCents,
		IsAnonymous:     intent.IsAnonymous,
		PrimaryCategory: primaryCategory,
	}

	if err := s.renderTemplate(w, r, "page.need-donate-confirmation", data); err != nil {
		s.logger.WithError(err).Error("failed to render donate confirmation page")
		s.internalServerError(w)
		return
	}
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
