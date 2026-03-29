package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"christjesus/pkg/types"
)

const adminUsersPageSize = 20

func (s *Service) handleGetAdminUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	selectedType := strings.TrimSpace(r.URL.Query().Get("type"))

	totalUsers, err := s.userRepo.CountUsers(ctx, search, selectedType)
	if err != nil {
		s.logger.WithError(err).Error("failed to count users for admin list")
		s.internalServerError(w)
		return
	}

	totalPages := totalUsers / adminUsersPageSize
	if totalUsers%adminUsersPageSize != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	users, err := s.userRepo.ListUsers(ctx, page, adminUsersPageSize, search, selectedType)
	if err != nil {
		s.logger.WithError(err).Error("failed to list users for admin")
		s.internalServerError(w)
		return
	}

	items := make([]*types.AdminUserListItem, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}

		email := ""
		if user.Email != nil {
			email = *user.Email
		}
		givenName := ""
		if user.GivenName != nil {
			givenName = *user.GivenName
		}
		familyName := ""
		if user.FamilyName != nil {
			familyName = *user.FamilyName
		}
		userType := "-"
		if user.UserType != nil && *user.UserType != "" {
			userType = *user.UserType
		}

		items = append(items, &types.AdminUserListItem{
			UserID:     user.ID,
			Email:      email,
			GivenName:  givenName,
			FamilyName: familyName,
			UserType:   userType,
			CreatedAt:  user.CreatedAt.Format(time.DateOnly),
			DetailHref: s.route(RouteAdminUserDetail, Param("userID", user.ID)),
		})
	}

	buildPageHref := func(p int) string {
		v := url.Values{}
		v.Set("page", strconv.Itoa(p))
		if search != "" {
			v.Set("search", search)
		}
		if selectedType != "" {
			v.Set("type", selectedType)
		}
		return s.routeWithQuery(RouteAdminUsers, v)
	}

	prevHref := ""
	if page > 1 {
		prevHref = buildPageHref(page - 1)
	}
	nextHref := ""
	if page < totalPages {
		nextHref = buildPageHref(page + 1)
	}

	data := &types.AdminUsersPageData{
		BasePageData: types.BasePageData{Title: "Admin Users"},
		Users:        items,
		Page:         page,
		PageSize:     adminUsersPageSize,
		TotalUsers:   totalUsers,
		TotalPages:   totalPages,
		PrevHref:     prevHref,
		NextHref:     nextHref,
		Search:       search,
		SelectedType: selectedType,
		FilterAction: s.route(RouteAdminUsers),
		BackHref:     s.route(RouteAdmin),
	}

	if err := s.renderTemplate(w, r, "page.admin.users", data); err != nil {
		s.logger.WithError(err).Error("failed to render admin users page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetAdminUserDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := strings.TrimSpace(r.PathValue("userID"))
	if userID == "" {
		http.NotFound(w, r)
		return
	}

	user, err := s.userRepo.User(ctx, userID)
	if err != nil {
		s.logger.WithError(err).WithField("user_id", userID).Error("failed to fetch user for admin detail")
		http.NotFound(w, r)
		return
	}

	email := ""
	if user.Email != nil {
		email = *user.Email
	}
	givenName := ""
	if user.GivenName != nil {
		givenName = *user.GivenName
	}
	familyName := ""
	if user.FamilyName != nil {
		familyName = *user.FamilyName
	}
	authSubject := ""
	if user.AuthSubject != nil {
		authSubject = *user.AuthSubject
	}
	userType := "-"
	if user.UserType != nil && *user.UserType != "" {
		userType = *user.UserType
	}

	data := &types.AdminUserDetailPageData{
		BasePageData: types.BasePageData{Title: "User: " + givenName + " " + familyName},
		UserID:       user.ID,
		Email:        email,
		GivenName:    givenName,
		FamilyName:   familyName,
		AuthSubject:  authSubject,
		UserType:     userType,
		CreatedAt:    user.CreatedAt.Format(time.DateTime),
		UpdatedAt:    user.UpdatedAt.Format(time.DateTime),
		BackHref:     s.route(RouteAdminUsers),
	}

	if userType == string(types.UserTypeRecipient) {
		s.populateAdminRecipientData(ctx, data, userID)
	} else if userType == string(types.UserTypeDonor) {
		s.populateAdminDonorData(ctx, data, userID)
	}

	if err := s.renderTemplate(w, r, "page.admin.user.detail", data); err != nil {
		s.logger.WithError(err).Error("failed to render admin user detail page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) populateAdminRecipientData(ctx context.Context, data *types.AdminUserDetailPageData, userID string) {
	data.IsRecipient = true

	needs, err := s.needsRepo.NeedsByUser(ctx, userID)
	if err != nil {
		s.logger.WithError(err).WithField("user_id", userID).Error("failed to fetch needs for admin user detail")
		return
	}

	var activeCount int
	var totalRaisedCents int
	items := make([]*types.AdminUserNeedItem, 0, len(needs))
	for _, need := range needs {
		if need == nil {
			continue
		}

		if need.Status == types.NeedStatusActive {
			activeCount++
		}
		totalRaisedCents += need.AmountRaisedCents

		fundingPercent := 0
		if need.AmountNeededCents > 0 {
			fundingPercent = need.AmountRaisedCents * 100 / need.AmountNeededCents
		}

		desc := ""
		if need.ShortDescription != nil {
			desc = *need.ShortDescription
		}

		items = append(items, &types.AdminUserNeedItem{
			NeedID:           need.ID,
			ShortDescription: desc,
			Status:           need.Status,
			AmountNeeded:     formatUSDFromCents(need.AmountNeededCents),
			AmountRaised:     formatUSDFromCents(need.AmountRaisedCents),
			FundingPercent:   fundingPercent,
			CreatedAt:        need.CreatedAt.Format(time.DateOnly),
			ReviewHref:       s.route(RouteAdminNeedReview, Param("needID", need.ID)),
		})
	}

	data.Needs = items
	data.HasNeeds = len(items) > 0
	data.TotalNeeds = len(items)
	data.NeedsSummary = fmt.Sprintf("%d needs, %d active, %s raised", len(items), activeCount, formatUSDFromCents(totalRaisedCents))
}

func (s *Service) populateAdminDonorData(ctx context.Context, data *types.AdminUserDetailPageData, userID string) {
	data.IsDonor = true

	intents, err := s.donationIntentRepo.DonationIntentsByDonorUserID(ctx, userID)
	if err != nil {
		s.logger.WithError(err).WithField("user_id", userID).Error("failed to fetch donations for admin user detail")
		return
	}

	// Collect distinct need IDs for labels
	distinctNeedIDs := make([]string, 0, len(intents))
	seenNeedIDs := make(map[string]bool)
	for _, intent := range intents {
		if intent == nil {
			continue
		}
		needID := strings.TrimSpace(intent.NeedID)
		if needID == "" || seenNeedIDs[needID] {
			continue
		}
		seenNeedIDs[needID] = true
		distinctNeedIDs = append(distinctNeedIDs, needID)
	}

	needLabelByID := make(map[string]string, len(distinctNeedIDs))
	if len(distinctNeedIDs) > 0 {
		needs, err := s.needsRepo.NeedsByIDs(ctx, distinctNeedIDs)
		if err != nil {
			s.logger.WithError(err).WithField("user_id", userID).Error("failed to batch fetch needs for admin donor detail")
		} else {
			for _, need := range needs {
				if need == nil {
					continue
				}
				label := "Need request"
				if need.ShortDescription != nil && strings.TrimSpace(*need.ShortDescription) != "" {
					label = *need.ShortDescription
				}
				needLabelByID[need.ID] = label
			}
		}
	}

	var totalCents int
	items := make([]*types.AdminUserDonationItem, 0, len(intents))
	for _, intent := range intents {
		if intent == nil {
			continue
		}

		needLabel := needLabelByID[intent.NeedID]
		if strings.TrimSpace(needLabel) == "" {
			needLabel = "Need request"
		}

		if strings.ToLower(intent.PaymentStatus) == types.DonationPaymentStatusFinalized {
			totalCents += intent.AmountCents
		}

		items = append(items, &types.AdminUserDonationItem{
			IntentID:    intent.ID,
			NeedID:      intent.NeedID,
			NeedLabel:   needLabel,
			Amount:      formatUSDFromCents(intent.AmountCents),
			Status:      formatDonationStatus(intent.PaymentStatus),
			IsAnonymous: intent.IsAnonymous,
			CreatedAt:   intent.CreatedAt.Format(time.DateOnly),
		})
	}

	data.Donations = items
	data.HasDonations = len(items) > 0
	data.TotalDonations = len(items)
	data.DonationsSummary = fmt.Sprintf("%d donations, %s finalized total", len(items), formatUSDFromCents(totalCents))
}
