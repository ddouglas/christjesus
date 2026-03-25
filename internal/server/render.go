package server

import (
	"bytes"
	"net/http"

	"christjesus/pkg/types"

	"github.com/gorilla/csrf"
)

func (s *Service) renderTemplate(w http.ResponseWriter, r *http.Request, templateName string, data any) error {

	var userID, userEmail, userName string
	var isAdmin bool
	if session, ok := sessionFromRequest(r); ok {
		userID, userEmail, userName = session.UserID, session.Email, session.DisplayName
		isAdmin = session.IsAdmin
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
