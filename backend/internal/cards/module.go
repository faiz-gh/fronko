package cards

import "github.com/faiz-gh/fronko/backend/internal/app"

// Routes registers the public card lookups and card management.
func (h *ProfileHandler) Routes(r *app.Routes) {
	r.Public("GET /api/profiles/{org}/{slug}", h.GetPublicProfile)
	r.Public("GET /api/profiles/{org}/{slug}/vcard", h.VCard)

	r.User("GET /api/me/profiles", h.GetMyProfiles)
	r.Admin("POST /api/me/profiles", h.CreateProfile)
	r.User("GET /api/me/profiles/{id}", h.GetMyProfile)
	r.User("PUT /api/me/profiles/{id}", h.UpdateProfile)
	r.Admin("DELETE /api/me/profiles/{id}", h.DeleteProfile)
	r.Admin("PUT /api/org/profiles/{id}/assignee", h.SetAssignee)
}
