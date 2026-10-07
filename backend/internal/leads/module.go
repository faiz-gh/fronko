package leads

import (
	"time"

	"github.com/faiz-gh/fronko/backend/internal/app"
)

// The public contact form: a few at once per address, then one more every interval.
const (
	leadBurst    = 5
	leadInterval = 15 * time.Second
)

// Routes registers the public contact form and the lead inbox.
func (h *LeadHandler) Routes(r *app.Routes) {
	r.Public("POST /api/profiles/{id}/leads", r.RateLimit(leadInterval, leadBurst)(h.SubmitLead))

	r.User("GET /api/me/profiles/{id}/leads", h.GetLeads)
	r.User("GET /api/me/leads", h.ListLeads)
}
