package server

import (
	"christjesus/pkg/types"
	"net/http"
)

func (s *Service) renderTemplate(w http.ResponseWriter, r *http.Request, templateName string, data any) error {
	userID, _ := r.Context().Value(contextKeyUserID).(string)
	userEmail, _ := r.Context().Value(contextKeyEmail).(string)

	if setter, ok := data.(types.NavbarDataSetter); ok {
		setter.SetNavbarData(types.NavbarData{
			IsAuthenticated: userID != "",
			UserID:          userID,
			UserEmail:       userEmail,
		})
	}

	return s.templates.ExecuteTemplate(w, templateName, data)
}
