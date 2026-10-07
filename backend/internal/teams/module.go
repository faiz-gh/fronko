package teams

import "github.com/faiz-gh/fronko/backend/internal/app"

// Routes registers team management. Everyone lists the teams they're in;
// admins create and change them.
func (h *TeamHandler) Routes(r *app.Routes) {
	r.User("GET /api/org/teams", h.List)
	r.Admin("POST /api/org/teams", h.Create)
	r.User("GET /api/org/teams/{id}", h.Get)
	r.Admin("PATCH /api/org/teams/{id}", h.Update)
	r.Admin("DELETE /api/org/teams/{id}", h.Delete)
	r.Admin("PUT /api/org/teams/{id}/members", h.SetMembers)
	r.Admin("PUT /api/org/users/{id}/teams", h.SetUserTeams)
}
