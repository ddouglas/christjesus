package server

import (
	"net/http"
	"strings"
	"time"

	"christjesus/pkg/types"
)

func (s *Service) handleGetAdminNeeds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	needs, err := s.needsRepo.ModerationQueueNeeds(ctx)
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
			ReviewHref:  s.route(RouteAdminNeedReview, map[string]string{"id": needID}),
		})
	}

	data := &types.AdminNeedsPageData{
		BasePageData: types.BasePageData{Title: "Admin Needs"},
		Needs:        items,
	}

	if err := s.renderTemplate(w, r, "page.admin.needs", data); err != nil {
		s.logger.WithError(err).Error("failed to render admin needs page")
		s.internalServerError(w)
		return
	}
}
