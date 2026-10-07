package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
	"github.com/stretchr/testify/assert"
)

func TestCleanEvent(t *testing.T) {
	_, ok := cleanEvent(repository.CardEvent{Type: models.EventVCard})
	assert.False(t, ok, "contact saves are server-side only")
	_, ok = cleanEvent(repository.CardEvent{Type: models.EventFormSubmit})
	assert.False(t, ok, "sent forms are server-side only")
	_, ok = cleanEvent(repository.CardEvent{Type: "hack"})
	assert.False(t, ok)

	_, ok = cleanEvent(repository.CardEvent{Type: models.EventScroll, Value: 33})
	assert.False(t, ok, "only the tracker's thresholds")
	e, ok := cleanEvent(repository.CardEvent{Type: models.EventScroll, Value: 75})
	assert.True(t, ok)
	assert.Equal(t, 75, e.Value)

	e, _ = cleanEvent(repository.CardEvent{Type: models.EventLeave, Value: 1 << 30})
	assert.Equal(t, maxTimeOnCardMs, e.Value)
	e, _ = cleanEvent(repository.CardEvent{Type: models.EventLeave, Value: -5})
	assert.Zero(t, e.Value)
	e, _ = cleanEvent(repository.CardEvent{Type: models.EventClick, Value: 9, Target: strings.Repeat("é", 400), Label: " Site "})
	assert.Zero(t, e.Value, "values only mean something for scroll and leave")
	assert.Equal(t, maxEventTargetLen, len([]rune(e.Target)))
	assert.Equal(t, "Site", e.Label)
}

func TestDeviceAndSource(t *testing.T) {
	assert.Equal(t, "mobile", deviceClass("Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) Mobile/15E148"))
	assert.Equal(t, "mobile", deviceClass("Mozilla/5.0 (Linux; Android 15; Pixel 9) Chrome/130 Mobile Safari/537.36"))
	assert.Equal(t, "tablet", deviceClass("Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X)"))
	assert.Equal(t, "tablet", deviceClass("Mozilla/5.0 (Linux; Android 15; SM-X910) Chrome/130 Safari/537.36"))
	assert.Equal(t, "desktop", deviceClass("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1.15"))

	assert.Equal(t, models.SourceNFC, normalizeSource("nfc"))
	assert.Equal(t, models.SourceQR, normalizeSource("qr"))
	assert.Equal(t, models.SourceLink, normalizeSource(""))
	assert.Equal(t, models.SourceLink, normalizeSource("email"))
}

func TestVisitorHash(t *testing.T) {
	a := visitorHash([]byte("salt"), "1.2.3.4", "UA", 7)
	assert.Equal(t, a, visitorHash([]byte("salt"), "1.2.3.4", "UA", 7), "stable within a day")
	assert.NotEqual(t, a, visitorHash([]byte("next"), "1.2.3.4", "UA", 7), "a new salt unlinks it")
	assert.NotEqual(t, a, visitorHash([]byte("salt"), "1.2.3.4", "UA", 8), "and so does another card")
	assert.NotEqual(t, a, visitorHash([]byte("salt"), "1.2.3.4U", "A", 7), "fields are separated")
}

func TestIsBot(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	for _, ua := range []string{"", "Googlebot/2.1", "facebookexternalhit/1.1", "curl/8.0", "Mozilla/5.0 HeadlessChrome/130"} {
		r.Header.Set("User-Agent", ua)
		assert.True(t, isBot(r), ua)
	}
	r.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) Mobile/15E148")
	assert.False(t, isBot(r))
}

func TestCollectRejectsBadInput(t *testing.T) {
	h := NewAnalyticsHandler(nil, nil)
	w := httptest.NewRecorder()
	h.Collect(w, httptest.NewRequest(http.MethodPost, "/api/profiles/x/events", strings.NewReader("{")))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Nothing valid to store: answered without touching the database.
	w = httptest.NewRecorder()
	h.Collect(w, httptest.NewRequest(http.MethodPost, "/api/profiles/x/events",
		strings.NewReader(`{"source":"qr","events":[{"type":"vcard"},{"type":"form_submit"}]}`)))
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAnalyticsFilter(t *testing.T) {
	member := middleware.Principal{UserID: 2, OrgID: 1, Role: models.RoleMember,
		Teams: []models.TeamRef{{ID: 5, Role: models.TeamRoleLead}, {ID: 6, Role: models.TeamRoleMember}}}
	run := func(p middleware.Principal, query string) (repository.AnalyticsFilter, int) {
		r := httptest.NewRequest(http.MethodGet, "/api/me/analytics/summary?"+query, nil)
		r = r.WithContext(middleware.WithPrincipal(r.Context(), p))
		w := httptest.NewRecorder()
		f, ok := analyticsFilter(w, r)
		if ok {
			return f, 0
		}
		return f, w.Code
	}

	f, code := run(member, "")
	assert.Zero(t, code)
	assert.Equal(t, defaultAnalyticsRange, f.To.Sub(f.From))
	assert.Equal(t, "UTC", f.TZ)

	f, code = run(member, "from=2026-01-01T00:00:00Z&to=2026-02-01T00:00:00Z&tz=Asia/Kolkata&team_id=5&profile_id=3")
	assert.Zero(t, code)
	assert.EqualValues(t, 5, f.TeamID)
	assert.EqualValues(t, 3, f.ProfileID)
	assert.Equal(t, "Asia/Kolkata", f.TZ)

	_, code = run(member, "team_id=6")
	assert.Equal(t, http.StatusForbidden, code, "members of a team can't see its analytics")
	_, code = run(middleware.Principal{OrgID: 1, Role: models.RoleAdmin}, "team_id=6")
	assert.Zero(t, code, "admins see every team")

	for _, q := range []string{
		"from=yesterday",
		"from=2026-02-01T00:00:00Z&to=2026-01-01T00:00:00Z",
		"from=2024-01-01T00:00:00Z&to=2026-01-01T00:00:00Z",
		"tz=Mars/Olympus",
		"profile_id=0",
		"user_id=abc",
	} {
		_, code = run(member, q)
		assert.Equal(t, http.StatusBadRequest, code, q)
	}
}

func TestTeamsAnalyticsNeedsAdminOrLead(t *testing.T) {
	h := NewAnalyticsHandler(nil, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/me/analytics/teams", nil)
	r = r.WithContext(middleware.WithPrincipal(r.Context(), middleware.Principal{UserID: 2, OrgID: 1, Role: models.RoleMember,
		Teams: []models.TeamRef{{ID: 6, Role: models.TeamRoleMember}}}))
	w := httptest.NewRecorder()
	h.Teams(w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPosixZone(t *testing.T) {
	assert.Equal(t, "<+0530>-05:30", posixZone(5*3600+30*60))
	assert.Equal(t, "<-0400>+04:00", posixZone(-4*3600))
	assert.Equal(t, "<+0000>-00:00", posixZone(0))
}
