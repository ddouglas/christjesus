package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"christjesus/internal/store"
	"christjesus/pkg/types"

	"github.com/jackc/pgx/v5"
)

func (s *Service) handleGetAdminNeedReview(w http.ResponseWriter, r *http.Request) {
	needID := strings.TrimSpace(r.PathValue("needID"))
	if needID == "" {
		http.NotFound(w, r)
		return
	}

	ctx := r.Context()

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		if errors.Is(err, types.ErrNeedNotFound) {
			http.NotFound(w, r)
			return
		}
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need for admin review")
		s.internalServerError(w)
		return
	}

	story, err := s.storyRepo.GetStoryByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need story for admin review")
		s.internalServerError(w)
		return
	}

	assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need categories for admin review")
		s.internalServerError(w)
		return
	}

	categoryIDs := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		categoryID := strings.TrimSpace(assignment.CategoryID)
		if categoryID != "" {
			categoryIDs = append(categoryIDs, categoryID)
		}
	}

	categoryByID := make(map[string]*types.NeedCategory)
	if len(categoryIDs) > 0 {
		categories, err := s.categoryRepo.CategoriesByIDs(ctx, categoryIDs)
		if err != nil {
			s.logger.WithError(err).WithField("need_id", needID).Error("failed to load category records for admin review")
			s.internalServerError(w)
			return
		}

		for _, category := range categories {
			if category == nil || strings.TrimSpace(category.ID) == "" {
				continue
			}
			categoryByID[category.ID] = category
		}
	}

	var primaryCategory *types.NeedCategory
	secondaryCategories := make([]*types.NeedCategory, 0)
	for _, assignment := range assignments {
		category := categoryByID[assignment.CategoryID]
		if category == nil {
			continue
		}

		if assignment.IsPrimary {
			primaryCategory = category
			continue
		}

		secondaryCategories = append(secondaryCategories, category)
	}

	var selectedAddress *types.UserAddress
	if need.UserAddressID != nil {
		selectedAddressID := strings.TrimSpace(*need.UserAddressID)
		if selectedAddressID != "" {
			selectedAddress, err = s.userAddressRepo.ByIDAndUserID(ctx, selectedAddressID, need.UserID)
			if err != nil {
				s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch selected address for admin review")
				s.internalServerError(w)
				return
			}
		}
	}

	if selectedAddress == nil {
		selectedAddress, err = s.userAddressRepo.PrimaryByUserID(ctx, need.UserID)
		if err != nil {
			s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch primary address for admin review")
			s.internalServerError(w)
			return
		}
	}

	_, _, cityState := browseCityStateParts(selectedAddress)

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

	documentStatusByID := latestDocumentStatuses(actions)

	reviewDocuments := make([]*types.AdminNeedReviewDocument, 0, len(documents))
	for _, document := range documents {
		documentParams := map[string]string{"needID": needID, "documentID": document.ID}

		status := "Pending Review"
		if value, ok := documentStatusByID[document.ID]; ok {
			status = value
		}

		mimeType := strings.TrimSpace(document.MimeType)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		reviewDocuments = append(reviewDocuments, &types.AdminNeedReviewDocument{
			ID:          document.ID,
			FileName:    document.FileName,
			TypeLabel:   documentTypeLabel(document.DocumentType),
			UploadedAt:  document.UploadedAt.Format("2006-01-02 15:04"),
			Status:      status,
			MimeType:    mimeType,
			FileSize:    formatFileSize(document.FileSizeBytes),
			PreviewHref: s.route(RouteAdminNeedDocument, documentParams),
		})
	}

	moderationTimeline, err := s.progressRepo.ModerationTimelineByNeed(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need timeline for admin review")
		s.internalServerError(w)
		return
	}

	messages, err := s.needReviewMessageRepo.MessagesByNeed(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need review messages for admin review")
		s.internalServerError(w)
		return
	}

	session, ok := sessionFromRequest(r)
	if !ok {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need review messages for admin review")
		s.internalServerError(w)
		return
	}

	viewerUserID := session.UserID

	timeline := make([]*types.AdminNeedTimelineItem, 0, len(moderationTimeline))
	for _, item := range moderationTimeline {
		if item == nil || item.Event == nil {
			continue
		}

		event := item.Event
		action := item.Action

		actor := "-"
		if event.ActorUserID != nil && strings.TrimSpace(*event.ActorUserID) != "" {
			actor = *event.ActorUserID
		}
		if action != nil && strings.TrimSpace(action.ActorUserID) != "" {
			actor = action.ActorUserID
		}

		actionType := "-"
		reason := ""
		note := ""
		documentID := ""
		if action != nil {
			actionType = string(action.ActionType)
			if action.Reason != nil {
				reason = strings.TrimSpace(*action.Reason)
			}
			if action.Note != nil {
				note = strings.TrimSpace(*action.Note)
			}
			if action.DocumentID != nil {
				documentID = strings.TrimSpace(*action.DocumentID)
			}
		}

		timeline = append(timeline, &types.AdminNeedTimelineItem{
			When:       event.CreatedAt.Format("2006-01-02 15:04"),
			Step:       event.Step,
			Actor:      actor,
			Source:     string(event.EventSource),
			ActionType: actionType,
			Reason:     reason,
			Note:       note,
			DocumentID: documentID,
		})
	}

	for left, right := 0, len(timeline)-1; left < right; left, right = left+1, right-1 {
		timeline[left], timeline[right] = timeline[right], timeline[left]
	}

	data := &types.AdminNeedReviewPageData{
		BasePageData:        types.BasePageData{Title: "Admin Need Review"},
		Need:                need,
		Story:               story,
		PrimaryCategory:     primaryCategory,
		SecondaryCategories: secondaryCategories,
		SelectedAddress:     selectedAddress,
		CityState:           cityState,
		Documents:           reviewDocuments,
		Timeline:            timeline,
		BackHref:            s.route(RouteAdminNeeds, nil),
		ModerateAction:      s.route(RouteAdminNeedModerate, map[string]string{"needID": needID}),
		AcceptReviewAction:  s.route(RouteAdminNeedModerate, map[string]string{"needID": needID}),
		CanAcceptReview:     need.Status == types.NeedStatusReadyForReview,
		CanSubmitModeration: need.Status == types.NeedStatusUnderReview,
		DeleteAction:        s.route(RouteAdminNeedDelete, map[string]string{"needID": needID}),
		RestoreAction:       s.route(RouteAdminNeedRestore, map[string]string{"needID": needID}),
		IsDeleted:           need.DeletedAt != nil,
		DeletedAt:           formatOptionalDateTime(need.DeletedAt),
		DeletedByUserID:     formatOptionalString(need.DeletedByUserID),
		DeleteReason:        formatOptionalString(need.DeleteReason),
		Messages:            buildNeedReviewMessageViews(messages, strings.TrimSpace(viewerUserID)),
		MessageAction:       s.route(RouteAdminNeedMessage, map[string]string{"needID": needID}),
		Notice:              strings.TrimSpace(r.URL.Query().Get("notice")),
		Error:               strings.TrimSpace(r.URL.Query().Get("error")),
	}

	if err := s.renderTemplate(w, r, "page.admin.need.review", data); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to render admin need review page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostAdminNeedModerate(w http.ResponseWriter, r *http.Request) {
	needID := strings.TrimSpace(r.PathValue("needID"))
	if needID == "" {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.redirectAdminNeedReviewWithError(w, r, needID, "invalid form submission")
		return
	}

	need, err := s.needsRepo.Need(r.Context(), needID)
	if err != nil {
		if errors.Is(err, types.ErrNeedNotFound) {
			http.NotFound(w, r)
			return
		}
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need before moderation action")
		s.internalServerError(w)
		return
	}
	if need.DeletedAt != nil {
		s.redirectAdminNeedReviewWithError(w, r, needID, "cannot moderate a deleted need; restore it first")
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
	case "accept_review":
		if need.Status != types.NeedStatusReadyForReview {
			s.redirectAdminNeedReviewWithError(w, r, needID, "need must be ready for review before accepting")
			return
		}
		status := types.NeedStatusUnderReview
		newStatus = &status
		actionType = types.NeedModerationActionTypeReviewStarted
		notice = "Review accepted"
	case "approve":
		if need.Status != types.NeedStatusUnderReview {
			s.redirectAdminNeedReviewWithError(w, r, needID, "need must be under review before approval")
			return
		}
		actionType = types.NeedModerationActionTypeReviewApproved
		notice = "Need approved and published"
	case "reject":
		if need.Status != types.NeedStatusUnderReview {
			s.redirectAdminNeedReviewWithError(w, r, needID, "need must be under review before rejection")
			return
		}
		status := types.NeedStatusRejected
		newStatus = &status
		actionType = types.NeedModerationActionTypeReviewRejected
		notice = "Need rejected"
	case "request_changes":
		if need.Status != types.NeedStatusUnderReview {
			s.redirectAdminNeedReviewWithError(w, r, needID, "need must be under review before requesting changes")
			return
		}
		status := types.NeedStatusChangesRequested
		newStatus = &status
		actionType = types.NeedModerationActionTypeChangesRequested
		notice = "Changes requested"
	case "verify_document":
		if need.Status != types.NeedStatusUnderReview {
			s.redirectAdminNeedReviewWithError(w, r, needID, "need must be under review before document verification")
			return
		}
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
		if need.Status != types.NeedStatusUnderReview {
			s.redirectAdminNeedReviewWithError(w, r, needID, "need must be under review before document rejection")
			return
		}
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

	session, ok := sessionFromRequest(r)
	if !ok {
		s.logger.Error("session not found on context")
		s.redirectAdminNeedReviewWithError(w, r, needID, "missing actor identity")
		return
	}

	actorUserID := session.UserID

	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	var notePtr *string
	if note != "" {
		notePtr = &note
	}

	if err := store.WithTx(r.Context(), s.needsRepo, func(tx pgx.Tx) error {
		if action == "approve" {
			if err := s.needsRepo.PublishNeedTx(r.Context(), tx, needID); err != nil {
				return err
			}
		} else if newStatus != nil {
			if err := s.needsRepo.SetNeedStatusTx(r.Context(), tx, needID, *newStatus); err != nil {
				return err
			}
		}

		_, err := s.progressRepo.RecordModerationActionEventTx(r.Context(), tx, needID, actionType, actorUserID, reasonPtr, notePtr, moderationDocumentID)
		return err
	}); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to atomically apply moderation action")
		s.redirectAdminNeedReviewWithError(w, r, needID, "moderation action failed")
		return
	}

	v := url.Values{}
	v.Set("notice", notice)
	http.Redirect(w, r, s.routeWithQuery(RouteAdminNeedReview, map[string]string{"needID": needID}, v), http.StatusSeeOther)
}

func (s *Service) redirectAdminNeedReviewWithError(w http.ResponseWriter, r *http.Request, needID, message string) {
	v := url.Values{}
	v.Set("error", message)
	http.Redirect(w, r, s.routeWithQuery(RouteAdminNeedReview, map[string]string{"needID": needID}, v), http.StatusSeeOther)
}

func (s *Service) handlePostAdminNeedMessage(w http.ResponseWriter, r *http.Request) {
	needID := strings.TrimSpace(r.PathValue("needID"))
	if needID == "" {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.redirectAdminNeedReviewWithError(w, r, needID, "invalid message submission")
		return
	}

	body, validationErr := validateNeedReviewMessageBody(r.FormValue("message"))
	if validationErr != nil {
		s.redirectAdminNeedReviewWithError(w, r, needID, validationErr.Error())
		return
	}

	session, ok := sessionFromRequest(r)
	if !ok {
		s.logger.Error("session not found on context")
		s.redirectAdminNeedReviewWithError(w, r, needID, "missing actor identity")
		return
	}

	actorUserID := session.UserID

	if _, err := s.needsRepo.Need(r.Context(), needID); err != nil {
		if errors.Is(err, types.ErrNeedNotFound) {
			http.NotFound(w, r)
			return
		}
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need before creating admin review message")
		s.internalServerError(w)
		return
	}

	err := s.needReviewMessageRepo.CreateMessage(r.Context(), &types.NeedReviewMessage{
		NeedID:       needID,
		SenderUserID: actorUserID,
		SenderRole:   types.NeedReviewMessageSenderRoleAdmin,
		Body:         body,
	})
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to create admin review message")
		s.redirectAdminNeedReviewWithError(w, r, needID, "failed to send message")
		return
	}

	v := url.Values{}
	v.Set("notice", "Message sent to need owner")
	http.Redirect(w, r, s.routeWithQuery(RouteAdminNeedReview, map[string]string{"needID": needID}, v), http.StatusSeeOther)
}

func formatFileSize(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}

	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}

	if unit == 0 {
		return fmt.Sprintf("%d %s", bytes, units[unit])
	}

	return fmt.Sprintf("%.1f %s", size, units[unit])
}

func latestDocumentStatuses(actions []*types.NeedModerationAction) map[string]string {
	documentStatusByID := make(map[string]string)
	for _, action := range actions {
		if action == nil || action.DocumentID == nil {
			continue
		}

		documentID := strings.TrimSpace(*action.DocumentID)
		if documentID == "" {
			continue
		}

		// actions are loaded newest-first; keep the first status seen per document.
		if _, exists := documentStatusByID[documentID]; exists {
			continue
		}

		switch action.ActionType {
		case types.NeedModerationActionTypeDocumentVerified:
			documentStatusByID[documentID] = "Verified"
		case types.NeedModerationActionTypeDocumentRejected:
			documentStatusByID[documentID] = "Rejected"
		}
	}

	return documentStatusByID
}

func formatOptionalDateTime(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return value.Format("2006-01-02 15:04")
}

func formatOptionalString(value *string) string {
	if value == nil {
		return "-"
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return "-"
	}
	return trimmed
}
