package server

import (
	"net/http"
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

		submittedAt := "-"
		if need.SubmittedAt != nil {
			submittedAt = need.SubmittedAt.Format(time.DateOnly)
		}

		items = append(items, &types.AdminNeedQueueItem{
			NeedID:      need.ID,
			Status:      need.Status,
			CreatedAt:   need.CreatedAt.Format(time.DateOnly),
			SubmittedAt: submittedAt,
			ReviewHref:  s.route(RouteAdminNeedReview, map[string]string{"id": need.ID}),
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
