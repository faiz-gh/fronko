package platformadmin

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/app"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
)

// snapshotInterval is how often today's usage snapshot is refreshed.
const snapshotInterval = time.Hour

// Routes registers the platform admin panel: its own sign-in, and the API
// behind the admin session.
func (h *AdminHandler) Routes(r *app.Routes) {
	r.Public("POST /auth/admin/login", r.AuthLimit(h.Login))
	r.Public("POST /auth/admin/logout", h.Logout)

	r.PlatformAdmin("GET /api/admin/me", h.Me)
	r.PlatformAdmin("GET /api/admin/summary", h.Summary)
	r.PlatformAdmin("GET /api/admin/trends", h.PlatformTrend)
	r.PlatformAdmin("GET /api/admin/orgs", h.ListOrgs)
	r.PlatformAdmin("GET /api/admin/orgs/{id}", h.GetOrg)
	r.PlatformAdmin("GET /api/admin/orgs/{id}/trends", h.OrgTrend)
	r.PlatformAdmin("POST /api/admin/orgs/{id}/suspend", h.SuspendOrg)
	r.PlatformAdmin("POST /api/admin/orgs/{id}/reinstate", h.ReinstateOrg)
	r.PlatformAdmin("GET /api/admin/feedback", h.ListFeedback)
	r.PlatformAdmin("GET /api/admin/feedback/{id}", h.GetFeedback)
	r.PlatformAdmin("PATCH /api/admin/feedback/{id}", h.SetFeedbackStatus)
	r.PlatformAdmin("POST /api/admin/feedback/{id}/replies", h.ReplyFeedback)
	r.PlatformAdmin("GET /api/admin/audit", h.ListAudit)
}

// Guard authenticates platform admins for the /api/admin/ routes.
func (h *AdminHandler) Guard() func(http.Handler) http.Handler {
	return AdminMiddleware(h.authService, func(ctx context.Context, id int64) (*PlatformAdmin, error) {
		a, err := h.store.GetPlatformAdmin(ctx, id)
		if errors.Is(err, database.ErrNotFound) {
			err = ErrAdminNotFound
		}
		return a, err
	})
}

// Tasks refreshes today's usage snapshot every hour for the admin trends.
// Each run rewrites today's row, so today's point stays current and the last
// run of a day becomes that day's final value. Missed runs (the server was
// down) only leave gaps in the trend.
func (h *AdminHandler) Tasks() []jobs.Task {
	return []jobs.Task{{
		Name:  "usage snapshot",
		Every: snapshotInterval,
		Run:   func(ctx context.Context) error { return h.store.TakeUsageSnapshot(ctx, time.Now()) },
	}}
}
