package branding

import "github.com/faiz-gh/fronko/backend/internal/app"

// Routes registers the organisation's branding settings. Everyone reads them,
// since cards and signatures show the logo; admins change them.
func (h *Handler) Routes(r *app.Routes) {
	r.User("GET /api/org/branding", h.Get)
	r.Admin("PUT /api/org/branding", h.Update)
}
