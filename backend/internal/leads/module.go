package leads

import (
	"context"
	"log"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/app"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
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
	r.Admin("DELETE /api/me/leads/{id}", h.DeleteLead)
	r.Admin("POST /api/me/leads/delete", h.DeleteLeads)
}

// Tasks deletes leads older than their organisation's retention period.
func (h *LeadHandler) Tasks() []jobs.Task {
	return []jobs.Task{{
		Name:  "lead retention",
		Every: 6 * time.Hour,
		Run: func(ctx context.Context) error {
			n, err := h.store.PurgeExpired(ctx)
			if n > 0 {
				log.Printf("lead retention: deleted %d leads", n)
			}
			return err
		},
	}}
}
