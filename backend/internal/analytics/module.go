package analytics

import (
	"context"
	"log"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/app"
	"github.com/faiz-gh/fronko/backend/internal/platform/schedule"
)

// Card analytics beacons: a visit sends a handful, a few seconds apart.
const (
	eventBurst    = 30
	eventInterval = 2 * time.Second
)

// Module is card analytics: the public beacon endpoint, the reports, and
// the retention sweep.
type Module struct {
	Handler *AnalyticsHandler
	Store   *Store
	// Retention is how long events are kept (ANALYTICS_RETENTION_DAYS).
	Retention time.Duration
}

func (m Module) Routes(r *app.Routes) {
	h := m.Handler
	r.Public("POST /api/profiles/{org}/{slug}/events", r.RateLimit(eventInterval, eventBurst)(h.Collect))

	r.User("GET /api/me/analytics/summary", h.Summary)
	r.User("GET /api/me/analytics/timeseries", h.Timeseries)
	r.User("GET /api/me/analytics/content", h.Content)
	r.User("GET /api/me/analytics/cards", h.Cards)
	r.User("GET /api/me/analytics/teams", h.Teams)
	r.User("GET /api/me/analytics/members", h.Members)
	r.User("GET /api/me/analytics/activity", h.Activity)
}

// Tasks deletes events older than the retention period (and stale
// visitor-hash salts) every hour.
func (m Module) Tasks() []schedule.Task {
	return []schedule.Task{{
		Name:  "analytics retention",
		Every: time.Hour,
		Run: func(ctx context.Context) error {
			n, err := m.Store.PurgeAnalytics(ctx, time.Now().Add(-m.Retention))
			if n > 0 {
				log.Printf("analytics retention: deleted %d events", n)
			}
			return err
		},
	}}
}
