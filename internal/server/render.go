package server

import (
	"bytes"
	"net/http"

	"christjesus/pkg/types"

	"github.com/gorilla/csrf"
)

func (s *Service) renderTemplate(w http.ResponseWriter, r *http.Request, templateName string, data any) error {
	userID, _ := r.Context().Value(contextKeyUserID).(string)
	userEmail, _ := r.Context().Value(contextKeyEmail).(string)
	userName, _ := r.Context().Value(contextKeyUserName).(string)
	isAdmin, _ := r.Context().Value(contextKeyIsAdmin).(bool)

	if userName == "" {
		userName = "Friend"
	}

	if setter, ok := data.(types.CSRFFieldSetter); ok {
		setter.SetCSRFField(csrf.TemplateField(r))
	}

	if setter, ok := data.(types.NavbarDataSetter); ok {
		setter.SetNavbarData(types.NavbarData{
			IsAuthenticated: userID != "",
			IsAdmin:         isAdmin,
			UserID:          userID,
			UserEmail:       userEmail,
			UserName:        userName,
			AvatarURL:       "/static/avatar-placeholder.svg",
		})
	}

	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		return err
	}

	_, err := buf.WriteTo(w)
	return err
}
