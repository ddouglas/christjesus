package server

import (
	"net/http"
	"net/url"
	"strings"

	"christjesus/internal/store"
	"christjesus/pkg/types"

	"github.com/jackc/pgx/v5"
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

		reasonPtr := reason
		if err := store.WithTx(r.Context(), s.needsRepo, func(tx pgx.Tx) error {
			if err := s.needsRepo.SoftDeleteNeedTx(r.Context(), tx, needID, actorUserID, reason); err != nil {
				return err
			}

			_, err := s.progressRepo.RecordModerationActionEventTx(r.Context(), tx, needID, types.NeedModerationActionTypeSoftDeleted, actorUserID, &reasonPtr, nil, nil)
			return err
		}); err != nil {
			s.logger.WithError(err).WithField("need_id", needID).Error("failed to atomically delete need and record audit event")
			s.redirectAdminNeedReviewWithError(w, r, needID, "failed to delete need")
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

	reasonPtr := reason
	if err := store.WithTx(r.Context(), s.needsRepo, func(tx pgx.Tx) error {
		if err := s.needsRepo.RestoreNeedTx(r.Context(), tx, needID); err != nil {
			return err
		}

		_, err := s.progressRepo.RecordModerationActionEventTx(r.Context(), tx, needID, types.NeedModerationActionTypeRestored, actorUserID, &reasonPtr, nil, nil)
		return err
	}); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to atomically restore need and record audit event")
		s.redirectAdminNeedReviewWithError(w, r, needID, "failed to restore need")
		return
	}

	v := url.Values{}
	v.Set("notice", "Need restored")
	http.Redirect(w, r, s.routeWithQuery(RouteAdminNeedReview, map[string]string{"id": needID}, v), http.StatusSeeOther)
}
