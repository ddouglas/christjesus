package server

import (
	"net/http"
	"strings"

	"christjesus/pkg/types"
)

func (s *Service) handleGetAdminNeedReview(w http.ResponseWriter, r *http.Request) {
	needID := strings.TrimSpace(r.PathValue("id"))
	if needID == "" {
		http.NotFound(w, r)
		return
	}

	need, err := s.needsRepo.Need(r.Context(), needID)
	if err != nil {
		if err == types.ErrNeedNotFound {
			http.NotFound(w, r)
			return
		}
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need for admin review")
		s.internalServerError(w)
		return
	}

	events, err := s.progressRepo.EventsByNeed(r.Context(), needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need timeline for admin review")
		s.internalServerError(w)
		return
	}

	timeline := make([]*types.AdminNeedTimelineItem, 0, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}

		actor := "-"
		if event.ActorUserID != nil && strings.TrimSpace(*event.ActorUserID) != "" {
			actor = *event.ActorUserID
		}

		timeline = append(timeline, &types.AdminNeedTimelineItem{
			When:   event.CreatedAt.Format("2006-01-02 15:04"),
			Step:   event.Step,
			Actor:  actor,
			Source: string(event.EventSource),
		})
	}

	data := &types.AdminNeedReviewPageData{
		BasePageData: types.BasePageData{Title: "Admin Need Review"},
		Need:         need,
		Timeline:     timeline,
		BackHref:     s.route(RouteAdminNeeds, nil),
	}

	if err := s.renderTemplate(w, r, "page.admin.need.review", data); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to render admin need review page")
		s.internalServerError(w)
		return
	}
}
