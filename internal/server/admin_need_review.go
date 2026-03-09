package server

import (
	"net/http"
	"net/url"
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
		ModerateAction: s.route(RouteAdminNeedModerate, map[string]string{"id": needID}),
		Notice:       strings.TrimSpace(r.URL.Query().Get("notice")),
		Error:        strings.TrimSpace(r.URL.Query().Get("error")),
	}

	if err := s.renderTemplate(w, r, "page.admin.need.review", data); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to render admin need review page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostAdminNeedModerate(w http.ResponseWriter, r *http.Request) {
	needID := strings.TrimSpace(r.PathValue("id"))
	if needID == "" {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.redirectAdminNeedReviewWithError(w, r, needID, "invalid form submission")
		return
	}

	action := strings.TrimSpace(r.FormValue("action"))
	reason := strings.TrimSpace(r.FormValue("reason"))
	note := strings.TrimSpace(r.FormValue("note"))

	var newStatus types.NeedStatus
	var actionType types.NeedModerationActionType
	var notice string

	switch action {
	case "approve":
		newStatus = types.NeedStatusApproved
		actionType = types.NeedModerationActionTypeReviewApproved
		notice = "Need approved"
	case "reject":
		newStatus = types.NeedStatusRejected
		actionType = types.NeedModerationActionTypeReviewRejected
		notice = "Need rejected"
	case "request_changes":
		newStatus = types.NeedStatusChangesRequested
		actionType = types.NeedModerationActionTypeChangesRequested
		notice = "Changes requested"
	default:
		s.redirectAdminNeedReviewWithError(w, r, needID, "unknown moderation action")
		return
	}

	actorUserID, ok := r.Context().Value(contextKeyUserID).(string)
	if !ok || strings.TrimSpace(actorUserID) == "" {
		s.redirectAdminNeedReviewWithError(w, r, needID, "missing actor identity")
		return
	}

	if err := s.needsRepo.SetNeedStatus(r.Context(), needID, newStatus); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to set need status")
		s.redirectAdminNeedReviewWithError(w, r, needID, "failed to update need status")
		return
	}

	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	var notePtr *string
	if note != "" {
		notePtr = &note
	}

	if _, err := s.progressRepo.RecordModerationActionEvent(r.Context(), needID, actionType, actorUserID, reasonPtr, notePtr, nil); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to record moderation action event")
		s.redirectAdminNeedReviewWithError(w, r, needID, "need updated but audit event failed")
		return
	}

	v := url.Values{}
	v.Set("notice", notice)
	http.Redirect(w, r, s.routeWithQuery(RouteAdminNeedReview, map[string]string{"id": needID}, v), http.StatusSeeOther)
}

func (s *Service) redirectAdminNeedReviewWithError(w http.ResponseWriter, r *http.Request, needID, message string) {
	v := url.Values{}
	v.Set("error", message)
	http.Redirect(w, r, s.routeWithQuery(RouteAdminNeedReview, map[string]string{"id": needID}, v), http.StatusSeeOther)
}
