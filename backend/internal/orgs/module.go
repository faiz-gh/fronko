package orgs

import "github.com/faiz-gh/fronko/backend/internal/app"

// Routes registers the organisation's settings and user management.
func (h *OrgHandler) Routes(r *app.Routes) {
	r.Admin("GET /api/org", h.Get)
	r.Owner("PUT /api/org", h.Update)
	r.Admin("PUT /api/org/handle", h.UpdateHandle)
	r.Admin("PUT /api/org/privacy", h.UpdatePrivacy)
	r.Admin("GET /api/org/users", h.ListUsers)
	r.Admin("POST /api/org/users", r.AuthLimit(h.CreateUser))
	r.Admin("GET /api/org/users/{id}", h.GetUser)
	r.Admin("PATCH /api/org/users/{id}", h.UpdateUser)
	r.Admin("POST /api/org/users/{id}/password", r.AuthLimit(h.ResetPassword))
	r.Admin("DELETE /api/org/users/{id}", h.DeleteUser)
}
