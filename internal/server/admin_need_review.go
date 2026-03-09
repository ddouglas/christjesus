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

	ctx := r.Context()

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		if err == types.ErrNeedNotFound {
			http.NotFound(w, r)
			return
		}
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need for admin review")
		s.internalServerError(w)
		return
	}

	documents, err := s.documentRepo.DocumentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need documents for admin review")
		s.internalServerError(w)
		return
	}

	actions, err := s.progressRepo.ModerationActionsByNeed(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch moderation actions for admin review")
		s.internalServerError(w)
		return
	}

	documentStatusByID := make(map[string]string)
	for _, action := range actions {
		if action == nil || action.DocumentID == nil {
			continue
		}

		documentID := strings.TrimSpace(*action.DocumentID)
		if documentID == "" {
			continue
		}

		switch action.ActionType {
		case types.NeedModerationActionTypeDocumentVerified:
			documentStatusByID[documentID] = "Verified"
		case types.NeedModerationActionTypeDocumentRejected:
			documentStatusByID[documentID] = "Rejected"
		}
	}

	reviewDocuments := make([]*types.AdminNeedReviewDocument, 0, len(documents))
	for _, document := range documents {
		status := "Pending Review"
		if value, ok := documentStatusByID[document.ID]; ok {
			status = value
		}

		reviewDocuments = append(reviewDocuments, &types.AdminNeedReviewDocument{
			ID:         document.ID,
			FileName:   document.FileName,
			TypeLabel:  documentTypeLabel(document.DocumentType),
			UploadedAt: document.UploadedAt.Format("2006-01-02 15:04"),
			Status:     status,
		})
	}

	events, err := s.progressRepo.EventsByNeed(ctx, needID)
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
		BasePageData:   types.BasePageData{Title: "Admin Need Review"},
		Need:           need,
		Documents:      reviewDocuments,
		Timeline:       timeline,
		BackHref:       s.route(RouteAdminNeeds, nil),
		ModerateAction: s.route(RouteAdminNeedModerate, map[string]string{"id": needID}),
		Notice:         strings.TrimSpace(r.URL.Query().Get("notice")),
		Error:          strings.TrimSpace(r.URL.Query().Get("error")),
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
	documentID := strings.TrimSpace(r.FormValue("document_id"))
	reason := strings.TrimSpace(r.FormValue("reason"))
	note := strings.TrimSpace(r.FormValue("note"))

	var newStatus *types.NeedStatus
	var actionType types.NeedModerationActionType
	var moderationDocumentID *string
	var notice string

	switch action {
	case "approve":
		status := types.NeedStatusApproved
		newStatus = &status
		actionType = types.NeedModerationActionTypeReviewApproved
		notice = "Need approved"
	case "reject":
		status := types.NeedStatusRejected
		newStatus = &status
		actionType = types.NeedModerationActionTypeReviewRejected
		notice = "Need rejected"
	case "request_changes":
		status := types.NeedStatusChangesRequested
		newStatus = &status
		actionType = types.NeedModerationActionTypeChangesRequested
		notice = "Changes requested"
	case "verify_document":
		if documentID == "" {
			s.redirectAdminNeedReviewWithError(w, r, needID, "missing document id")
			return
		}

		if _, err := s.documentRepo.DocumentByNeedIDAndID(r.Context(), needID, documentID); err != nil {
			s.logger.WithError(err).WithField("need_id", needID).WithField("document_id", documentID).Warn("invalid document verification target")
			s.redirectAdminNeedReviewWithError(w, r, needID, "invalid document selection")
			return
		}

		actionType = types.NeedModerationActionTypeDocumentVerified
		moderationDocumentID = &documentID
		notice = "Document verified"
	case "reject_document":
		if documentID == "" {
			s.redirectAdminNeedReviewWithError(w, r, needID, "missing document id")
			return
		}

		if _, err := s.documentRepo.DocumentByNeedIDAndID(r.Context(), needID, documentID); err != nil {
			s.logger.WithError(err).WithField("need_id", needID).WithField("document_id", documentID).Warn("invalid document rejection target")
			s.redirectAdminNeedReviewWithError(w, r, needID, "invalid document selection")
			return
		}

		actionType = types.NeedModerationActionTypeDocumentRejected
		moderationDocumentID = &documentID
		notice = "Document rejected"
	default:
		s.redirectAdminNeedReviewWithError(w, r, needID, "unknown moderation action")
		return
	}

	actorUserID, ok := r.Context().Value(contextKeyUserID).(string)
	if !ok || strings.TrimSpace(actorUserID) == "" {
		s.redirectAdminNeedReviewWithError(w, r, needID, "missing actor identity")
		return
	}

	if newStatus != nil {
		if err := s.needsRepo.SetNeedStatus(r.Context(), needID, *newStatus); err != nil {
			s.logger.WithError(err).WithField("need_id", needID).Error("failed to set need status")
			s.redirectAdminNeedReviewWithError(w, r, needID, "failed to update need status")
			return
		}
	}

	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	var notePtr *string
	if note != "" {
		notePtr = &note
	}

	if _, err := s.progressRepo.RecordModerationActionEvent(r.Context(), needID, actionType, actorUserID, reasonPtr, notePtr, moderationDocumentID); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to record moderation action event")
		s.redirectAdminNeedReviewWithError(w, r, needID, "moderation action failed to record")
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
