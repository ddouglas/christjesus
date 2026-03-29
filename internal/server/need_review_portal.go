package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"christjesus/pkg/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
)

const userNeedReviewMessageCooldown = time.Minute

func (s *Service) handleGetProfileNeedReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))
	if needID == "" {
		http.NotFound(w, r)
		return
	}

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	shared, err := s.loadNeedReviewSharedData(ctx, needID)
	if err != nil {
		if errors.Is(err, types.ErrNeedNotFound) {
			http.NotFound(w, r)
			return
		}
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to load shared need review data for review portal")
		s.internalServerError(w)
		return
	}

	need := shared.Need
	if need.UserID != userID {
		s.redirectProfileWithError(w, r, "You do not have permission to access that need.")
		return
	}

	actions, err := s.progressRepo.ModerationActionsByNeed(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch moderation actions for review portal")
		s.internalServerError(w)
		return
	}

	rejectionReason, rejectionNote, rejectedDocumentByID := latestRejectedFeedback(actions)
	documentStatusByID := latestDocumentStatuses(actions)

	docFeedback := make([]types.NeedReviewDocumentFeedback, 0, len(shared.Documents))
	for _, doc := range shared.Documents {
		status := "Pending Review"
		if value, ok := documentStatusByID[doc.ID]; ok {
			status = value
		}

		reason := ""
		note := ""
		if feedback, ok := rejectedDocumentByID[doc.ID]; ok {
			reason = feedback.reason
			note = feedback.note
		}

		docFeedback = append(docFeedback, types.NeedReviewDocumentFeedback{
			DocumentID: doc.ID,
			FileName:   doc.FileName,
			TypeLabel:  documentTypeLabel(doc.DocumentType),
			Status:     status,
			Reason:     reason,
			Note:       note,
			ViewHref:   s.route(RouteProfileNeedDocumentView, Param("needID", needID), Param("documentID", doc.ID)),
		})
	}

	messages, err := s.needReviewMessageRepo.MessagesByNeed(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need review messages")
		s.internalServerError(w)
		return
	}

	data := &types.NeedReviewPortalPageData{
		BasePageData:        types.BasePageData{Title: "Need Review Portal"},
		Need:                need,
		Story:               shared.Story,
		PrimaryCategory:     shared.PrimaryCategory,
		SecondaryCategories: shared.SecondaryCategories,
		RejectionReason:     rejectionReason,
		RejectionNote:       rejectionNote,
		Documents:           docFeedback,
		Messages:            buildNeedReviewMessageViews(messages, userID),
		PostMessageAction:   s.route(RouteProfileNeedReviewPost, Param("needID", needID)),
		SetReadyAction:      s.route(RouteProfileNeedReviewSetReady, Param("needID", needID)),
		PullBackAction:      s.route(RouteProfileNeedReviewPullBack, Param("needID", needID)),
		BackHref:            s.route(RouteProfile),
		EditNeedHref:        s.route(RouteProfileNeedEdit, Param("needID", needID)),
		CanEditNeed:         need.Status == types.NeedStatusSubmitted || need.Status == types.NeedStatusChangesRequested,
		CanSetReady:         need.Status == types.NeedStatusSubmitted || need.Status == types.NeedStatusChangesRequested,
		CanPullBack:         need.Status == types.NeedStatusReadyForReview,
		CanSendMessage:      isNeedOwnerMessagingAllowedStatus(need.Status),
		Notice:              strings.TrimSpace(r.URL.Query().Get("notice")),
		Error:               strings.TrimSpace(r.URL.Query().Get("error")),
	}

	if err := s.renderTemplate(w, r, "page.profile.need.review", data); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to render need review portal")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostProfileNeedReviewMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))
	if needID == "" {
		http.NotFound(w, r)
		return
	}

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		if errors.Is(err, types.ErrNeedNotFound) {
			http.NotFound(w, r)
			return
		}
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need before posting review message")
		s.internalServerError(w)
		return
	}

	if need.UserID != userID {
		s.redirectProfileWithError(w, r, "You do not have permission to access that need.")
		return
	}

	if !isNeedOwnerMessagingAllowedStatus(need.Status) {
		s.redirectProfileNeedReviewWithError(w, r, needID, "Messages are not available for this need status.")
		return
	}

	if err := r.ParseForm(); err != nil {
		s.redirectProfileNeedReviewWithError(w, r, needID, "Invalid form submission.")
		return
	}

	body, validationErr := validateNeedReviewMessageBody(r.FormValue("message"))
	if validationErr != nil {
		s.redirectProfileNeedReviewWithError(w, r, needID, validationErr.Error())
		return
	}

	latestMessage, err := s.needReviewMessageRepo.LatestMessageByNeedAndSender(ctx, needID, userID, types.NeedReviewMessageSenderRoleUser)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).WithField("user_id", userID).Error("failed to check user message cooldown")
		s.internalServerError(w)
		return
	}
	if latestMessage != nil {
		elapsed := time.Since(latestMessage.CreatedAt)
		if elapsed < userNeedReviewMessageCooldown {
			remaining := int((userNeedReviewMessageCooldown - elapsed).Round(time.Second).Seconds())
			if remaining < 1 {
				remaining = 1
			}
			s.logger.WithField("need_id", needID).WithField("user_id", userID).WithField("cooldown_seconds_remaining", remaining).Info("need review message blocked by user cooldown")
			s.redirectProfileNeedReviewWithError(w, r, needID, "Please wait before sending another message.")
			return
		}
	}

	err = s.needReviewMessageRepo.CreateMessage(ctx, &types.NeedReviewMessage{
		NeedID:       needID,
		SenderUserID: userID,
		SenderRole:   types.NeedReviewMessageSenderRoleUser,
		Body:         body,
	})
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to create need review message")
		s.internalServerError(w)
		return
	}

	s.redirectProfileNeedReviewWithNotice(w, r, needID, "Message sent to admin reviewers.")
}

func (s *Service) handlePostProfileNeedReviewSetReady(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))
	if needID == "" {
		http.NotFound(w, r)
		return
	}

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		if errors.Is(err, types.ErrNeedNotFound) {
			http.NotFound(w, r)
			return
		}
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need before ready-for-review transition")
		s.internalServerError(w)
		return
	}

	if need.UserID != userID {
		http.NotFound(w, r)
		return
	}

	if need.Status != types.NeedStatusSubmitted && need.Status != types.NeedStatusChangesRequested {
		s.redirectProfileNeedReviewWithError(w, r, needID, "This need cannot be marked ready for review in its current status.")
		return
	}

	if err := s.needsRepo.SetNeedStatus(ctx, needID, types.NeedStatusReadyForReview); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to set need status to ready for review")
		s.internalServerError(w)
		return
	}

	s.redirectProfileNeedReviewWithNotice(w, r, needID, "Need marked ready for review.")
}

func (s *Service) handlePostProfileNeedReviewPullBack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))
	if needID == "" {
		http.NotFound(w, r)
		return
	}

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		if errors.Is(err, types.ErrNeedNotFound) {
			http.NotFound(w, r)
			return
		}
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need before pull-back transition")
		s.internalServerError(w)
		return
	}

	if need.UserID != userID {
		http.NotFound(w, r)
		return
	}

	if need.Status != types.NeedStatusReadyForReview {
		s.redirectProfileNeedReviewWithError(w, r, needID, "This need cannot be pulled back in its current status.")
		return
	}

	if err := s.needsRepo.SetNeedStatus(ctx, needID, types.NeedStatusSubmitted); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to pull need back to submitted")
		s.internalServerError(w)
		return
	}

	s.redirectProfileNeedReviewWithNotice(w, r, needID, "Need pulled back to submitted.")
}

func isNeedOwnerMessagingAllowedStatus(status types.NeedStatus) bool {
	switch status {
	case types.NeedStatusSubmitted, types.NeedStatusReadyForReview, types.NeedStatusUnderReview, types.NeedStatusChangesRequested, types.NeedStatusRejected:
		return true
	default:
		return false
	}
}

func (s *Service) handleGetProfileNeedDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))
	documentID := strings.TrimSpace(r.PathValue("documentID"))
	if needID == "" || documentID == "" {
		http.NotFound(w, r)
		return
	}

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		if errors.Is(err, types.ErrNeedNotFound) {
			http.NotFound(w, r)
			return
		}

		s.logger.WithError(err).
			WithField("need_id", needID).
			Error("failed to fetch need for profile document view")
		s.internalServerError(w)
		return
	}

	if need.UserID != userID {
		http.NotFound(w, r)
		return
	}

	doc, err := s.documentRepo.DocumentByNeedIDAndID(ctx, needID, documentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}

		s.logger.WithError(err).
			WithField("need_id", needID).
			WithField("document_id", documentID).
			Error("failed to fetch profile document record")
		s.internalServerError(w)
		return
	}

	storageKey := strings.TrimSpace(doc.StorageKey)
	if storageKey == "" {
		s.logger.WithField("need_id", needID).
			WithField("document_id", documentID).
			Error("profile document record missing storage key")
		s.internalServerError(w)
		return
	}

	response, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.S3BucketName),
		Key:    aws.String(storageKey),
	})
	if err != nil {
		if isS3NotFoundError(err) {
			http.NotFound(w, r)
			return
		}

		s.logger.WithError(err).
			WithField("need_id", needID).
			WithField("document_id", documentID).
			WithField("storage_key", storageKey).
			Error("failed to fetch profile document from s3")
		s.internalServerError(w)
		return
	}
	defer response.Body.Close()

	contentType := strings.TrimSpace(doc.MimeType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	s.logger.WithField("need_id", needID).
		WithField("document_id", documentID).
		WithField("user_id", userID).
		Info("profile need document viewed")

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", safeContentDispositionFilename(doc.FileName, doc.ID)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, response.Body); err != nil {
		s.logger.WithError(err).
			WithField("need_id", needID).
			WithField("document_id", documentID).
			Warn("failed to stream profile document response")
	}
}

func (s *Service) redirectProfileNeedReviewWithNotice(w http.ResponseWriter, r *http.Request, needID, notice string) {
	v := url.Values{}
	v.Set("notice", notice)
	http.Redirect(w, r, s.routeWithQuery(RouteProfileNeedReview, v, Param("needID", needID)), http.StatusSeeOther)
}

func (s *Service) redirectProfileNeedReviewWithError(w http.ResponseWriter, r *http.Request, needID, message string) {
	v := url.Values{}
	v.Set("error", message)
	http.Redirect(w, r, s.routeWithQuery(RouteProfileNeedReview, v, Param("needID", needID)), http.StatusSeeOther)
}

type rejectedDocumentFeedback struct {
	reason string
	note   string
}

func latestRejectedFeedback(actions []*types.NeedModerationAction) (string, string, map[string]rejectedDocumentFeedback) {
	needReason := ""
	needNote := ""
	rejectedDocuments := make(map[string]rejectedDocumentFeedback)

	for _, action := range actions {
		if action == nil {
			continue
		}

		reason := ""
		note := ""
		if action.Reason != nil {
			reason = strings.TrimSpace(*action.Reason)
		}
		if action.Note != nil {
			note = strings.TrimSpace(*action.Note)
		}

		switch action.ActionType {
		case types.NeedModerationActionTypeReviewRejected, types.NeedModerationActionTypeChangesRequested:
			if needReason == "" && needNote == "" {
				needReason = reason
				needNote = note
			}
		case types.NeedModerationActionTypeDocumentRejected:
			if action.DocumentID == nil {
				continue
			}

			documentID := strings.TrimSpace(*action.DocumentID)
			if documentID == "" {
				continue
			}
			if _, exists := rejectedDocuments[documentID]; exists {
				continue
			}
			rejectedDocuments[documentID] = rejectedDocumentFeedback{reason: reason, note: note}
		}
	}

	return needReason, needNote, rejectedDocuments
}

func buildNeedReviewMessageViews(messages []*types.NeedReviewMessage, viewerUserID string) []types.NeedReviewMessageView {
	views := make([]types.NeedReviewMessageView, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}

		if message.SenderRole != types.NeedReviewMessageSenderRoleAdmin && message.SenderRole != types.NeedReviewMessageSenderRoleUser {
			continue
		}

		isAdmin := message.SenderRole == types.NeedReviewMessageSenderRoleAdmin
		author := "Need Owner"
		if isAdmin {
			author = "Admin Reviewer"
		}

		views = append(views, types.NeedReviewMessageView{
			ID:           message.ID,
			AuthorLabel:  author,
			Body:         message.Body,
			CreatedAt:    message.CreatedAt.Format("2006-01-02 15:04"),
			IsFromAdmin:  isAdmin,
			IsFromViewer: message.SenderUserID == viewerUserID,
		})
	}

	return views
}

func validateNeedReviewMessageBody(raw string) (string, error) {
	body := strings.TrimSpace(raw)
	if body == "" {
		return "", fmt.Errorf("message cannot be empty")
	}

	if utf8.RuneCountInString(body) > types.NeedReviewMessageMaxChars {
		return "", fmt.Errorf("message cannot exceed %d characters", types.NeedReviewMessageMaxChars)
	}

	return body, nil
}
