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
	selectedStatus := strings.TrimSpace(r.URL.Query().Get("status"))
	selectedSort := strings.TrimSpace(r.URL.Query().Get("sort"))
	if selectedSort == "" {
		selectedSort = adminExplorerSortUpdated
	}

	statusFilter := adminExplorerStatusFilter(selectedStatus)
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

		fundingPercent := needFundingPercent(need.AmountRaisedCents, need.AmountNeededCents)
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

	data := &types.AdminNeedExplorerPageData{
		BasePageData:     types.BasePageData{Title: "Admin Need Explorer"},
		Needs:            items,
		Page:             page,
		PageSize:         adminNeedExplorerPageSize,
		TotalNeeds:       totalNeeds,
		TotalPages:       totalPages,
		PrevHref:         prevHref,
		NextHref:         nextHref,
		SelectedStatus:   selectedStatus,
		SelectedSort:     selectedSort,
		FilterAction:     s.route(RouteAdminNeedExplorer, nil),
		StatusOptions:    adminExplorerStatusOptions(),
		SortOptions:      adminExplorerSortOptions(),
		BackHref:         s.route(RouteAdmin, nil),
		QueueHref:        s.route(RouteAdminNeeds, nil),
		CurrentStatusText: adminExplorerStatusLabel(selectedStatus),
	}

	if err := s.renderTemplate(w, r, "page.admin.need.explorer", data); err != nil {
		s.logger.WithError(err).Error("failed to render admin need explorer page")
		s.internalServerError(w)
		return
	}
}

func adminExplorerStatusFilter(raw string) *types.NeedStatus {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
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

func adminExplorerStatusLabel(raw string) string {
	status := adminExplorerStatusFilter(raw)
	if status == nil {
		return "All post-approval statuses"
	}
	return string(*status)
}

func adminExplorerStatusOptions() []types.AdminExplorerOption {
	return []types.AdminExplorerOption{
		{Value: "", Label: "All post-approval"},
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

func needFundingPercent(raisedCents, neededCents int) int {
	if neededCents <= 0 {
		return 0
	}
	percent := (raisedCents * 100) / neededCents
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
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
