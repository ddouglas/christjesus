package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"christjesus/pkg/types"
)

const adminNeedsPageSize = 20

func (s *Service) handleGetAdminNeeds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)

	totalNeeds, err := s.needsRepo.ModerationQueueNeedsCount(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to count needs for admin queue")
		s.internalServerError(w)
		return
	}

	totalPages := totalNeeds / adminNeedsPageSize
	if totalNeeds%adminNeedsPageSize != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	needs, err := s.needsRepo.ModerationQueueNeedsPage(ctx, page, adminNeedsPageSize)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch needs for admin queue")
		s.internalServerError(w)
		return
	}

	items := make([]*types.AdminNeedQueueItem, 0, len(needs))
	for _, need := range needs {
		if need == nil {
			continue
		}

		needID := strings.TrimSpace(need.ID)
		if needID == "" {
			s.logger.Warn("skipping moderation queue row with empty need id")
			continue
		}

		submittedAt := "-"
		if need.SubmittedAt != nil {
			submittedAt = need.SubmittedAt.Format(time.DateOnly)
		}

		items = append(items, &types.AdminNeedQueueItem{
			NeedID:      needID,
			Status:      need.Status,
			CreatedAt:   need.CreatedAt.Format(time.DateOnly),
			SubmittedAt: submittedAt,
			ReviewHref:  s.route(RouteAdminNeedReview, Param("needID", needID)),
		})
	}

	prevHref := ""
	if page > 1 {
		v := url.Values{}
		v.Set("page", strconv.Itoa(page-1))
		prevHref = s.routeWithQuery(RouteAdminNeeds, v)
	}

	nextHref := ""
	if page < totalPages {
		v := url.Values{}
		v.Set("page", strconv.Itoa(page+1))
		nextHref = s.routeWithQuery(RouteAdminNeeds, v)
	}

	data := &types.AdminNeedsPageData{
		BasePageData: types.BasePageData{Title: "Admin Needs"},
		Needs:        items,
		Page:         page,
		PageSize:     adminNeedsPageSize,
		TotalNeeds:   totalNeeds,
		TotalPages:   totalPages,
		PrevHref:     prevHref,
		NextHref:     nextHref,
	}

	if err := s.renderTemplate(w, r, "page.admin.needs", data); err != nil {
		s.logger.WithError(err).Error("failed to render admin needs page")
		s.internalServerError(w)
		return
	}
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
