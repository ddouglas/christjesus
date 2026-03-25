package server

import (
	"christjesus/pkg/types"
	"net/http"
)

func (s *Service) handleGetGuidelines(w http.ResponseWriter, r *http.Request) {
	data := &types.BasePageData{Title: "Community Guidelines"}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "page.guidelines", data); err != nil {
		s.logger.WithError(err).Error("failed to render guidelines page")
		s.internalServerError(w)
		return
	}
}
