package server

import (
	"bytes"
	"net/http"
	"strings"

	"christjesus/pkg/types"

	"github.com/gorilla/csrf"
)

func (s *Service) renderTemplate(w http.ResponseWriter, r *http.Request, templateName string, data any) error {
	userID, _ := r.Context().Value(contextKeyUserID).(string)
	userEmail, _ := r.Context().Value(contextKeyEmail).(string)
	userName, _ := r.Context().Value(contextKeyUserName).(string)
	isAdmin, _ := r.Context().Value(contextKeyIsAdmin).(bool)

	if userName == "" {
		userName = displayNameFromEmail(userEmail)
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

func displayNameFromEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return "Friend"
	}

	local := email
	if at := strings.Index(local, "@"); at > 0 {
		local = local[:at]
	}

	local = strings.ReplaceAll(local, ".", " ")
	local = strings.ReplaceAll(local, "_", " ")
	local = strings.ReplaceAll(local, "-", " ")
	local = strings.TrimSpace(local)
	if local == "" {
		return "Friend"
	}

	parts := strings.Fields(strings.ToLower(local))
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}

	return strings.Join(parts, " ")
}
