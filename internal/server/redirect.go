package server

import "net/http"

func (s *Service) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.route(RouteLogin, nil), http.StatusSeeOther)
}
