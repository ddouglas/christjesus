package server

import (
	"christjesus/pkg/types"
	"net/http"
	"net/url"
	"strings"
)

func (s *Service) handleGetProfileDonorPreferences(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	categories, err := s.categoryRepo.Categories(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to load categories for profile donor preferences")
		s.internalServerError(w)
		return
	}

	pref, err := s.donorPreferenceRepo.ByUserID(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("failed to load donor preferences for profile")
		s.internalServerError(w)
		return
	}

	assignments, err := s.donorPreferenceAssignRepo.AssignmentsByUserID(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("failed to load donor preference category assignments for profile")
		s.internalServerError(w)
		return
	}

	selectedCategoryIDs := make(map[string]bool)
	for _, a := range assignments {
		selectedCategoryIDs[a.CategoryID] = true
	}

	data := &types.ProfileDonorPreferencesPageData{
		BasePageData:            types.BasePageData{Title: "My Preferences"},
		SidebarItems:            buildProfileSidebar(string(types.UserTypeDonor)),
		Categories:              categories,
		SelectedCategoryIDs:     selectedCategoryIDs,
		UpdatePreferencesAction: s.route(RouteProfileDonorPreferences),
		Notice:                  strings.TrimSpace(r.URL.Query().Get("notice")),
		Error:                   strings.TrimSpace(r.URL.Query().Get("error")),
	}
	if pref != nil {
		if pref.ZipCode != nil {
			data.ZipCode = *pref.ZipCode
		}
		if pref.Radius != nil {
			data.Radius = *pref.Radius
		}
		if pref.DonationRange != nil {
			data.DonationRange = *pref.DonationRange
		}
		if pref.NotificationFrequency != nil {
			data.NotificationFrequency = *pref.NotificationFrequency
		}
	}

	err = s.renderTemplate(w, r, "page.profile.donor.preferences", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render profile donor preferences page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostProfileDonorPreferences(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).Error("failed to parse profile donor preferences form")
		s.internalServerError(w)
		return
	}

	cleanOptional := func(value string) *string {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil
		}
		return &trimmed
	}

	selectedCategoryIDs := make([]string, 0, len(r.Form["categories"]))
	seen := make(map[string]bool)
	for _, categoryID := range r.Form["categories"] {
		categoryID = strings.TrimSpace(categoryID)
		if categoryID == "" || seen[categoryID] {
			continue
		}
		selectedCategoryIDs = append(selectedCategoryIDs, categoryID)
		seen[categoryID] = true
	}

	if len(selectedCategoryIDs) > 0 {
		validCategories, err := s.categoryRepo.CategoriesByIDs(ctx, selectedCategoryIDs)
		if err != nil {
			s.logger.WithError(err).Error("failed to validate donor preference categories")
			s.internalServerError(w)
			return
		}
		if len(validCategories) != len(selectedCategoryIDs) {
			s.logger.WithField("selected_count", len(selectedCategoryIDs)).
				WithField("valid_count", len(validCategories)).
				Warn("profile donor preferences contained invalid category ids")
			s.internalServerError(w)
			return
		}
	}

	existingPref, err := s.donorPreferenceRepo.ByUserID(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch existing donor preferences for profile update")
		s.internalServerError(w)
		return
	}

	if existingPref == nil {
		newPref := &types.DonorPreference{
			UserID:                userID,
			ZipCode:               cleanOptional(r.FormValue("zipCode")),
			Radius:                cleanOptional(r.FormValue("radius")),
			DonationRange:         cleanOptional(r.FormValue("donationRange")),
			NotificationFrequency: cleanOptional(r.FormValue("notificationFrequency")),
		}
		err = s.donorPreferenceRepo.Create(ctx, newPref)
		if err != nil {
			s.logger.WithError(err).Error("failed to create donor preferences from profile")
			s.internalServerError(w)
			return
		}
	} else {
		existingPref.ZipCode = cleanOptional(r.FormValue("zipCode"))
		existingPref.Radius = cleanOptional(r.FormValue("radius"))
		existingPref.DonationRange = cleanOptional(r.FormValue("donationRange"))
		existingPref.NotificationFrequency = cleanOptional(r.FormValue("notificationFrequency"))

		err = s.donorPreferenceRepo.Update(ctx, userID, existingPref)
		if err != nil {
			s.logger.WithError(err).Error("failed to update donor preferences from profile")
			s.internalServerError(w)
			return
		}
	}

	err = s.donorPreferenceAssignRepo.ReplaceAssignments(ctx, userID, selectedCategoryIDs)
	if err != nil {
		s.logger.WithError(err).Error("failed to replace donor preference category assignments from profile")
		s.internalServerError(w)
		return
	}

	v := url.Values{}
	v.Set("notice", "Preferences saved.")
	http.Redirect(w, r, s.routeWithQuery(RouteProfileDonorPreferences, v), http.StatusSeeOther)
}
