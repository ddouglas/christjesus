package server

import (
	"net/http"

	"christjesus/pkg/types"
)

func (s *Service) handleGetAdminDashboard(w http.ResponseWriter, r *http.Request) {
	data := &types.AdminDashboardPageData{
		BasePageData: types.BasePageData{Title: "Admin"},
	}

	if err := s.renderTemplate(w, r, "page.admin.dashboard", data); err != nil {
		s.logger.WithError(err).Error("failed to render admin dashboard")
		s.internalServerError(w)
		return
	}
}
