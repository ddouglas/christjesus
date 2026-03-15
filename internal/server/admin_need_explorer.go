package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"christjesus/pkg/types"
)

const (
	adminNeedExplorerPageSize = 20
	adminExplorerSortUpdated  = "updated_desc"
)

func (s *Service) handleGetAdminNeedExplorer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	selectedStatus, statusFilter := canonicalAdminExplorerStatus(r.URL.Query().Get("status"))
	selectedSort := canonicalAdminExplorerSort(r.URL.Query().Get("sort"))
	statusCounts, err := s.needsRepo.AdminExplorerNeedsCountByStatus(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch grouped status counts for admin explorer")
		s.internalServerError(w)
		return
	}

	totalNeeds, err := s.needsRepo.AdminExplorerNeedsCount(ctx, statusFilter)
	if err != nil {
		s.logger.WithError(err).Error("failed to count needs for admin explorer")
		s.internalServerError(w)
		return
	}

	totalPages := totalNeeds / adminNeedExplorerPageSize
	if totalNeeds%adminNeedExplorerPageSize != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	needs, err := s.needsRepo.AdminExplorerNeedsPage(ctx, page, adminNeedExplorerPageSize, statusFilter, selectedSort)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch needs for admin explorer")
		s.internalServerError(w)
		return
	}

	if err := s.applyFinalizedRaisedAmounts(ctx, needs); err != nil {
		s.logger.WithError(err).Error("failed to apply finalized raised amounts for admin explorer")
		s.internalServerError(w)
		return
	}

	items := make([]*types.AdminNeedExplorerItem, 0, len(needs))
	for _, need := range needs {
		if need == nil {
			continue
		}

		needID := strings.TrimSpace(need.ID)
		if needID == "" {
			continue
		}

		publishedAt := "-"
		if need.PublishedAt != nil {
			publishedAt = need.PublishedAt.Format(time.DateOnly)
		}

		fundingPercent := fundingPercentFromCents(need.AmountRaisedCents, need.AmountNeededCents)
		items = append(items, &types.AdminNeedExplorerItem{
			NeedID:            needID,
			Status:            need.Status,
			AmountRaisedCents: need.AmountRaisedCents,
			AmountNeededCents: need.AmountNeededCents,
			FundingPercent:    fundingPercent,
			ActivityLabel:     fundingActivityLabel(fundingPercent),
			UpdatedAt:         need.UpdatedAt.Format(time.DateOnly),
			PublishedAt:       publishedAt,
			ReviewHref:        s.route(RouteAdminNeedReview, map[string]string{"id": needID}),
			DetailHref:        s.route(RouteNeedDetail, map[string]string{"id": needID}),
			CanViewDetail:     adminExplorerCanViewPublicDetail(need.Status),
		})
	}

	buildPageHref := func(nextPage int) string {
		v := url.Values{}
		v.Set("page", strconv.Itoa(nextPage))
		if selectedStatus != "" {
			v.Set("status", selectedStatus)
		}
		if selectedSort != "" {
			v.Set("sort", selectedSort)
		}
		return s.routeWithQuery(RouteAdminNeedExplorer, nil, v)
	}

	prevHref := ""
	if page > 1 {
		prevHref = buildPageHref(page - 1)
	}

	nextHref := ""
	if page < totalPages {
		nextHref = buildPageHref(page + 1)
	}

	statusCards := make([]*types.AdminNeedStatusCard, 0, len(adminExplorerStatusOptions()))
	for _, option := range adminExplorerStatusOptions() {
		status := types.NeedStatus(option.Value)
		count := totalNeeds
		if strings.TrimSpace(option.Value) != "" {
			count = statusCounts[status]
		}

		v := url.Values{}
		if strings.TrimSpace(option.Value) != "" {
			v.Set("status", option.Value)
		}
		if selectedSort != "" {
			v.Set("sort", selectedSort)
		}

		statusCards = append(statusCards, &types.AdminNeedStatusCard{
			Status:   status,
			Label:    option.Label,
			Count:    count,
			Href:     s.routeWithQuery(RouteAdminNeedExplorer, nil, v),
			IsActive: selectedStatus == option.Value,
		})
	}

	data := &types.AdminNeedExplorerPageData{
		BasePageData:      types.BasePageData{Title: "Admin Need Explorer"},
		Needs:             items,
		StatusCards:       statusCards,
		Page:              page,
		PageSize:          adminNeedExplorerPageSize,
		TotalNeeds:        totalNeeds,
		TotalPages:        totalPages,
		PrevHref:          prevHref,
		NextHref:          nextHref,
		SelectedStatus:    selectedStatus,
		SelectedSort:      selectedSort,
		FilterAction:      s.route(RouteAdminNeedExplorer, nil),
		StatusOptions:     adminExplorerStatusOptions(),
		SortOptions:       adminExplorerSortOptions(),
		BackHref:          s.route(RouteAdmin, nil),
		QueueHref:         s.route(RouteAdminNeeds, nil),
		CurrentStatusText: adminExplorerStatusLabelByValue(selectedStatus),
	}

	if err := s.renderTemplate(w, r, "page.admin.need.explorer", data); err != nil {
		s.logger.WithError(err).Error("failed to render admin need explorer page")
		s.internalServerError(w)
		return
	}
}

func adminExplorerStatusFilter(raw string) *types.NeedStatus {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case string(types.NeedStatusDraft):
		status := types.NeedStatusDraft
		return &status
	case string(types.NeedStatusSubmitted):
		status := types.NeedStatusSubmitted
		return &status
	case string(types.NeedStatusReadyForReview):
		status := types.NeedStatusReadyForReview
		return &status
	case string(types.NeedStatusUnderReview):
		status := types.NeedStatusUnderReview
		return &status
	case string(types.NeedStatusChangesRequested):
		status := types.NeedStatusChangesRequested
		return &status
	case string(types.NeedStatusRejected):
		status := types.NeedStatusRejected
		return &status
	case string(types.NeedStatusApproved):
		status := types.NeedStatusApproved
		return &status
	case string(types.NeedStatusActive):
		status := types.NeedStatusActive
		return &status
	case string(types.NeedStatusFunded):
		status := types.NeedStatusFunded
		return &status
	default:
		return nil
	}
}

func canonicalAdminExplorerStatus(raw string) (string, *types.NeedStatus) {
	status := adminExplorerStatusFilter(raw)
	if status == nil {
		return "", nil
	}

	canonical := string(*status)
	return canonical, status
}

func canonicalAdminExplorerSort(raw string) string {
	selected := strings.TrimSpace(raw)
	if selected == "" {
		return adminExplorerSortUpdated
	}

	for _, option := range adminExplorerSortOptions() {
		if option.Value == selected {
			return selected
		}
	}

	return adminExplorerSortUpdated
}

func adminExplorerStatusLabelByValue(value string) string {
	for _, option := range adminExplorerStatusOptions() {
		if option.Value == value {
			return option.Label
		}
	}
	return "All statuses"
}

func adminExplorerStatusOptions() []types.AdminExplorerOption {
	return []types.AdminExplorerOption{
		{Value: "", Label: "All statuses"},
		{Value: string(types.NeedStatusDraft), Label: "Draft"},
		{Value: string(types.NeedStatusSubmitted), Label: "Submitted"},
		{Value: string(types.NeedStatusReadyForReview), Label: "Ready For Review"},
		{Value: string(types.NeedStatusUnderReview), Label: "Under Review"},
		{Value: string(types.NeedStatusChangesRequested), Label: "Changes Requested"},
		{Value: string(types.NeedStatusRejected), Label: "Rejected"},
		{Value: string(types.NeedStatusApproved), Label: "Approved"},
		{Value: string(types.NeedStatusActive), Label: "Active"},
		{Value: string(types.NeedStatusFunded), Label: "Funded"},
	}
}

func adminExplorerSortOptions() []types.AdminExplorerOption {
	return []types.AdminExplorerOption{
		{Value: "updated_desc", Label: "Recently Updated"},
		{Value: "updated_asc", Label: "Oldest Updated"},
		{Value: "progress_desc", Label: "Highest % Funded"},
		{Value: "raised_desc", Label: "Most Raised"},
		{Value: "needed_desc", Label: "Largest Goal"},
	}
}

func adminExplorerCanViewPublicDetail(status types.NeedStatus) bool {
	switch status {
	case types.NeedStatusActive, types.NeedStatusFunded:
		return true
	default:
		return false
	}
}

func fundingActivityLabel(percent int) string {
	switch {
	case percent >= 75:
		return "High"
	case percent >= 25:
		return "Medium"
	default:
		return "Low"
	}
}
