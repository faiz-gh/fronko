package integrations

import (
	"context"
	"log"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/app"
	"github.com/faiz-gh/fronko/backend/internal/platform/events"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
)

// Tests make outbound calls: a few at once per address, then one more every interval.
const (
	testBurst    = 5
	testInterval = 10 * time.Second
)

// Module is the integrations feature: the catalog and connections API, lead
// dispatch, and the activity log's retention.
type Module struct {
	Service *Service
	Handler *Handler
}

func (m Module) Routes(r *app.Routes) {
	h := m.Handler
	r.User("GET /api/integrations/catalog", h.Catalog)
	r.User("GET /api/integrations/connections", h.ListConnections)
	r.User("POST /api/integrations/connections", h.CreateConnection)
	r.User("GET /api/integrations/connections/{id}", h.GetConnection)
	r.User("PATCH /api/integrations/connections/{id}", h.UpdateConnection)
	r.User("DELETE /api/integrations/connections/{id}", h.DeleteConnection)
	r.User("POST /api/integrations/connections/{id}/test", r.RateLimit(testInterval, testBurst)(h.TestConnection))
	r.User("GET /api/integrations/connections/{id}/activity", h.ListActivity)
	r.User("POST /api/integrations/connections/{id}/tokens", r.AuthLimit(h.RotateToken))
	r.User("GET /api/integrations/connections/{id}/oauth/start", h.StartOAuth)
	r.User("GET "+oauthCallbackPath, h.OAuthCallback)
}

func (m Module) Subscribe(b *events.Bus) { m.Service.Subscribe(b) }

func (m Module) JobHandlers() map[string]jobs.Handler { return m.Service.JobHandlers() }

// Tasks deletes activity older than 90 days.
func (m Module) Tasks() []jobs.Task {
	return []jobs.Task{{
		Name:  "integrations: activity retention",
		Every: time.Hour,
		Run: func(ctx context.Context) error {
			n, err := m.Service.store.purgeActivity(ctx, time.Now().Add(-activityRetention))
			if n > 0 {
				log.Printf("integrations: deleted %d old activity entries", n)
			}
			return err
		},
	}}
}
