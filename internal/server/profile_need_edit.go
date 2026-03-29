package server

import (
	"net/http"
	"strings"
)

func (s *Service) handleGetProfileNeedEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))
	if needID == "" {
		http.NotFound(w, r)
		return
	}

	if _, err := s.profileEditableNeed(ctx, needID); err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	http.Redirect(w, r, s.route(RouteProfileNeedEditLocation, Param("needID", needID)), http.StatusSeeOther)
}
