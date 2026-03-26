package server

import (
	"christjesus/internal/utils"
	"christjesus/pkg/types"
	"net/http"
	"strings"

	"github.com/k0kubun/pp/v3"
)

func (s *Service) handleGetOnboardingNeedLocation(w http.ResponseWriter, r *http.Request) {
	var ctx = r.Context()

	needID := r.PathValue("needID")
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}

	if s.redirectIfNeedSubmitted(w, r, need) {
		return
	}

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	addresses, err := s.userAddressRepo.AddressesByUserID(ctx, userID)
	if err != nil {
		s.logger.WithError(err).WithField("user_id", userID).Error("failed to fetch user addresses")
		s.internalServerError(w)
		return
	}

	selectedAddressID := ""
	if need.UserAddressID != nil && *need.UserAddressID != "" {
		selectedAddressID = *need.UserAddressID
	} else if len(addresses) > 0 {
		selectedAddressID = addresses[0].ID
	} else {
		selectedAddressID = "new"
	}

	showSetSelectedPrimary := false
	for _, address := range addresses {
		if address.ID == selectedAddressID {
			showSetSelectedPrimary = !address.IsPrimary
			break
		}
	}

	data := &types.NeedLocationPageData{
		BasePageData:      types.BasePageData{Title: "Need Location"},
		ID:                needID,
		Addresses:         addresses,
		HasAddresses:      len(addresses) > 0,
		SelectedAddressID: selectedAddressID,
		ShowSetPrimary:    showSetSelectedPrimary,
		NewAddress:        &types.UserAddressForm{},
		FormAction:        s.route(RouteOnboardingNeedLocation, map[string]string{"needID": needID}),
		BackHref:          s.route(RouteOnboardingNeedWelcome, map[string]string{"needID": needID}),
	}

	err = s.renderTemplate(w, r, "page.onboarding.need.location", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need location page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostOnboardingNeedLocation(w http.ResponseWriter, r *http.Request) {

	var ctx = r.Context()

	needID := r.PathValue("needID")
	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}

	if s.redirectIfNeedSubmitted(w, r, need) {
		return
	}

	err = r.ParseForm()
	if err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		return
	}

	addresses, err := s.userAddressRepo.AddressesByUserID(ctx, userID)
	if err != nil {
		s.logger.WithError(err).WithField("user_id", userID).Error("failed to fetch user addresses")
		s.internalServerError(w)
		return
	}

	selection := strings.TrimSpace(r.FormValue("address_selection"))
	if selection == "" && len(addresses) > 0 {
		selection = addresses[0].ID
	}
	if selection == "" && len(addresses) == 0 {
		selection = "new"
	}

	var selectedAddress *types.UserAddress
	usesNonPrimaryAddress := false

	pp.Print(selection)

	if selection != "new" {
		addressID := selection
		if addressID == "" {
			s.logger.Error("missing selected address id")
			s.internalServerError(w)
			return
		}

		selectedAddress, err = s.userAddressRepo.ByIDAndUserID(ctx, addressID, userID)
		if err != nil {
			s.logger.WithError(err).WithField("address_id", addressID).Error("failed to fetch selected user address")
			s.internalServerError(w)
			return
		}

		if selectedAddress == nil {
			s.logger.WithField("address_id", addressID).Error("selected user address not found")
			s.internalServerError(w)
			return
		}

		setSelectedAsPrimary := r.FormValue("set_selected_as_primary") == "on"
		if setSelectedAsPrimary && !selectedAddress.IsPrimary {
			err = s.userAddressRepo.SetPrimaryByID(ctx, userID, selectedAddress.ID)
			if err != nil {
				s.logger.WithError(err).WithField("address_id", selectedAddress.ID).Error("failed to promote selected address to primary")
				s.internalServerError(w)
				return
			}
			selectedAddress.IsPrimary = true
		}

		usesNonPrimaryAddress = !selectedAddress.IsPrimary
	} else {
		location := new(types.UserAddressForm)
		err = decoder.Decode(location, r.Form)
		if err != nil {
			s.logger.WithError(err).Error("failed to decode form onto location form")
			s.internalServerError(w)
			return
		}

		pp.Print("Location :: ", location)

		if location.Address == nil || strings.TrimSpace(*location.Address) == "" ||
			location.City == nil || strings.TrimSpace(*location.City) == "" ||
			location.State == nil || strings.TrimSpace(*location.State) == "" ||
			location.ZipCode == nil || strings.TrimSpace(*location.ZipCode) == "" {
			s.logger.Error("new address submission missing required fields")
			s.internalServerError(w)
			return
		}

		setNewAsPrimary := len(addresses) == 0 || r.FormValue("set_new_as_primary") == "on"

		selectedAddress = &types.UserAddress{
			ID:                   utils.NanoID(),
			UserID:               userID,
			Address:              location.Address,
			AddressExt:           location.AddressExt,
			City:                 location.City,
			State:                location.State,
			ZipCode:              location.ZipCode,
			PrivacyDisplay:       location.PrivacyDisplay,
			ContactMethods:       location.ContactMethods,
			PreferredContactTime: location.PreferredContactTime,
			IsPrimary:            setNewAsPrimary,
		}

		if validationErr := s.validateAndStandardizeAddress(ctx, selectedAddress); validationErr != "" {
			data := &types.NeedLocationPageData{
				BasePageData:      types.BasePageData{Title: "Need Location"},
				ID:                needID,
				Addresses:         addresses,
				HasAddresses:      len(addresses) > 0,
				SelectedAddressID: "new",
				NewAddress:        location,
				FormAction:        s.route(RouteOnboardingNeedLocation, map[string]string{"needID": needID}),
				BackHref:          s.route(RouteOnboardingNeedWelcome, map[string]string{"needID": needID}),
				Error:             validationErr,
			}
			if err := s.renderTemplate(w, r, "page.onboarding.need.location", data); err != nil {
				s.logger.WithError(err).Error("failed to render need location page with validation error")
				s.internalServerError(w)
			}
			return
		}

		err = s.userAddressRepo.Create(ctx, selectedAddress)
		if err != nil {
			s.logger.WithError(err).WithField("user_id", userID).Error("failed to create user address")
			s.internalServerError(w)
			return
		}

		usesNonPrimaryAddress = !setNewAsPrimary
	}

	need.CurrentStep = types.NeedStepLocation
	need.UserAddressID = &selectedAddress.ID
	need.UsesNonPrimaryAddress = usesNonPrimaryAddress

	err = s.needsRepo.UpdateNeed(ctx, needID, need)
	if err != nil {
		s.logger.WithError(err).Error("failed to update need with location data")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepLocation)

	http.Redirect(w, r, s.route(RouteOnboardingNeedCategories, map[string]string{"needID": need.ID}), http.StatusSeeOther)
}
