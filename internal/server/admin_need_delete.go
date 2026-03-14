package server

import (
	"net/http"
	"net/url"
	"strings"

	"christjesus/pkg/types"
)

func (s *Service) handlePostAdminNeedDelete(w http.ResponseWriter, r *http.Request) {
	s.handlePostAdminNeedDeleteOrRestore(w, r, true)
}

func (s *Service) handlePostAdminNeedRestore(w http.ResponseWriter, r *http.Request) {
	s.handlePostAdminNeedDeleteOrRestore(w, r, false)
}

func (s *Service) handlePostAdminNeedDeleteOrRestore(w http.ResponseWriter, r *http.Request, deleting bool) {
	needID := strings.TrimSpace(r.PathValue("id"))
	if needID == "" {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.redirectAdminNeedReviewWithError(w, r, needID, "invalid form submission")
		return
	}

	reason := strings.TrimSpace(r.FormValue("reason"))
	if reason == "" {
		if deleting {
			s.redirectAdminNeedReviewWithError(w, r, needID, "delete reason is required")
			return
		}
		s.redirectAdminNeedReviewWithError(w, r, needID, "restore reason is required")
		return
	}

	need, err := s.needsRepo.Need(r.Context(), needID)
	if err != nil {
		if err == types.ErrNeedNotFound {
			http.NotFound(w, r)
			return
		}
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need for admin delete/restore")
		s.internalServerError(w)
		return
	}

	actorUserID, ok := r.Context().Value(contextKeyUserID).(string)
	if !ok || strings.TrimSpace(actorUserID) == "" {
		s.redirectAdminNeedReviewWithError(w, r, needID, "missing actor identity")
		return
	}

	if deleting {
		if need.DeletedAt != nil {
			s.redirectAdminNeedReviewWithError(w, r, needID, "need is already deleted")
			return
		}

		if err := s.needsRepo.SoftDeleteNeed(r.Context(), needID, actorUserID, reason); err != nil {
			s.logger.WithError(err).WithField("need_id", needID).Error("failed to soft-delete need")
			s.redirectAdminNeedReviewWithError(w, r, needID, "failed to delete need")
			return
		}

		reasonPtr := reason
		if _, err := s.progressRepo.RecordModerationActionEvent(r.Context(), needID, types.NeedModerationActionTypeSoftDeleted, actorUserID, &reasonPtr, nil, nil); err != nil {
			s.logger.WithError(err).WithField("need_id", needID).Error("failed to record need delete event")
			s.redirectAdminNeedReviewWithError(w, r, needID, "need was deleted but audit event failed")
			return
		}

		v := url.Values{}
		v.Set("notice", "Need deleted")
		http.Redirect(w, r, s.routeWithQuery(RouteAdminNeedReview, map[string]string{"id": needID}, v), http.StatusSeeOther)
		return
	}

	if need.DeletedAt == nil {
		s.redirectAdminNeedReviewWithError(w, r, needID, "need is not deleted")
		return
	}

	if err := s.needsRepo.RestoreNeed(r.Context(), needID); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to restore need")
		s.redirectAdminNeedReviewWithError(w, r, needID, "failed to restore need")
		return
	}

	reasonPtr := reason
	if _, err := s.progressRepo.RecordModerationActionEvent(r.Context(), needID, types.NeedModerationActionTypeRestored, actorUserID, &reasonPtr, nil, nil); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to record need restore event")
		s.redirectAdminNeedReviewWithError(w, r, needID, "need was restored but audit event failed")
		return
	}

	v := url.Values{}
	v.Set("notice", "Need restored")
	http.Redirect(w, r, s.routeWithQuery(RouteAdminNeedReview, map[string]string{"id": needID}, v), http.StatusSeeOther)
}
