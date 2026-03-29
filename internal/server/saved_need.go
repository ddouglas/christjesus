package server

import (
	"net/http"
)

func (s *Service) handlePostNeedSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	session, ok := sessionFromRequest(r)
	if !ok {
		s.internalServerError(w)
		return
	}

	if err := s.savedNeedRepo.Save(ctx, session.UserID, needID); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to save need")
		s.internalServerError(w)
		return
	}

	http.Redirect(w, r, s.route(RouteNeedDetail, Param("needID", needID)), http.StatusSeeOther)
}

func (s *Service) handlePostNeedUnsave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	session, ok := sessionFromRequest(r)
	if !ok {
		s.internalServerError(w)
		return
	}

	if err := s.savedNeedRepo.Unsave(ctx, session.UserID, needID); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to unsave need")
		s.internalServerError(w)
		return
	}

	// Check if request came from profile page
	referer := r.Header.Get("Referer")
	if referer != "" {
		http.Redirect(w, r, referer, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, s.route(RouteNeedDetail, Param("needID", needID)), http.StatusSeeOther)
}
