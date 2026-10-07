package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
	_ "time/tzdata" // reports take IANA time zones; the alpine image has no zoneinfo

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
)

// Card is what Collect needs to know about the card a beacon is for.
type Card struct {
	ID           int64
	OrgID        int64
	OrgSuspended bool
}

// CardResolver finds a card by its public link, /p/{handle}/{slug}. The
// cards module provides it, so analytics doesn't depend on cards.
type CardResolver interface {
	ResolveCard(ctx context.Context, handle, slug string) (Card, error)
}

type AnalyticsHandler struct {
	store  *Store
	cards  CardResolver
	events *EventRecorder
	// zones caches how each requested time zone is passed to the database.
	zones sync.Map
}

func NewAnalyticsHandler(store *Store, cards CardResolver, events *EventRecorder) *AnalyticsHandler {
	return &AnalyticsHandler{store: store, cards: cards, events: events}
}

type eventRequest struct {
	Session string `json:"session"`
	Source  string `json:"source"`
	Events  []struct {
		Type   string `json:"type"`
		Target string `json:"target"`
		Label  string `json:"label"`
		Value  int    `json:"value"`
	} `json:"events"`
}

// Public: POST /api/profiles/{org}/{slug}/events. The public card's tracker sends
// batches here with navigator.sendBeacon (as text/plain). It answers 204
// whether or not anything was stored, so it reveals nothing about the card.
func (h *AnalyticsHandler) Collect(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxEventBodyBytes)
	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Events) > maxEventsPerBatch {
		req.Events = req.Events[:maxEventsPerBatch]
	}
	events := make([]CardEvent, 0, len(req.Events))
	for _, e := range req.Events {
		if ev, ok := cleanEvent(CardEvent{Type: e.Type, Target: e.Target, Label: e.Label, Value: e.Value}); ok {
			events = append(events, ev)
		}
	}
	if len(events) == 0 || isBot(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	profile, err := h.cards.ResolveCard(r.Context(), r.PathValue("org"), r.PathValue("slug"))
	if err != nil || profile.OrgSuspended {
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			log.Printf("collect events: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	h.events.Record(r, profile.ID, profile.OrgID, req.Source, req.Session, events...)
	w.WriteHeader(http.StatusNoContent)
}

// analyticsFilter reads the shared query parameters: from, to (RFC 3339; the
// last 30 days by default), tz, profile_id, user_id and team_id. Filtering by
// a team needs an admin or that team's lead.
func analyticsFilter(w http.ResponseWriter, r *http.Request) (AnalyticsFilter, bool) {
	q := r.URL.Query()
	principal := auth.PrincipalFrom(r.Context())
	f := AnalyticsFilter{To: time.Now(), TZ: "UTC"}

	parseTime := func(name string) (time.Time, bool) {
		t, err := time.Parse(time.RFC3339, q.Get(name))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, name+" must be an RFC 3339 timestamp")
			return t, false
		}
		return t, true
	}
	var ok bool
	if q.Get("to") != "" {
		if f.To, ok = parseTime("to"); !ok {
			return f, false
		}
	}
	f.From = f.To.Add(-defaultAnalyticsRange)
	if q.Get("from") != "" {
		if f.From, ok = parseTime("from"); !ok {
			return f, false
		}
	}
	if span := f.To.Sub(f.From); span <= 0 || span > maxAnalyticsRange {
		httpx.WriteError(w, http.StatusBadRequest, "the period must be between a moment and 366 days long")
		return f, false
	}
	if tz := q.Get("tz"); tz != "" {
		if _, err := time.LoadLocation(tz); err != nil || len(tz) > 64 {
			httpx.WriteError(w, http.StatusBadRequest, "invalid tz")
			return f, false
		}
		f.TZ = tz
	}

	parseID := func(name string) (int64, bool) {
		raw := q.Get(name)
		if raw == "" {
			return 0, true
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			httpx.WriteError(w, http.StatusBadRequest, "invalid "+name)
			return 0, false
		}
		return id, true
	}
	if f.ProfileID, ok = parseID("profile_id"); !ok {
		return f, false
	}
	if f.UserID, ok = parseID("user_id"); !ok {
		return f, false
	}
	if f.TeamID, ok = parseID("team_id"); !ok {
		return f, false
	}
	// Cards and people outside the scope simply match nothing; a team filter
	// is refused outright so its existence isn't a guessing game.
	if f.TeamID != 0 && !principal.IsAdmin() && !principal.LeadsTeam(f.TeamID) {
		httpx.WriteError(w, http.StatusForbidden, "you can only see analytics for teams you lead")
		return f, false
	}
	return f, true
}

// posixZone is a fixed offset in the POSIX form Postgres accepts, where the
// sign is inverted: UTC+05:30 is "<+0530>-05:30".
func posixZone(offsetSeconds int) string {
	sign, inv := '+', '-'
	if offsetSeconds < 0 {
		sign, inv, offsetSeconds = '-', '+', -offsetSeconds
	}
	h, m := offsetSeconds/3600, offsetSeconds%3600/60
	return fmt.Sprintf("<%c%02d%02d>%c%02d:%02d", sign, h, m, inv, h, m)
}

// filter reads the shared parameters and makes sure the database understands
// the time zone: a name it doesn't know falls back to that zone's current
// offset (exact except across a daylight-saving change).
func (h *AnalyticsHandler) filter(w http.ResponseWriter, r *http.Request) (AnalyticsFilter, bool) {
	f, ok := analyticsFilter(w, r)
	if !ok || f.TZ == "UTC" {
		return f, ok
	}
	if zone, cached := h.zones.Load(f.TZ); cached {
		f.TZ = zone.(string)
		return f, true
	}
	known, err := h.store.KnowsTimeZone(r.Context(), f.TZ)
	if err != nil {
		httpx.Internal("time zone", err).Write(w)
		return f, false
	}
	zone := f.TZ
	if !known {
		loc, _ := time.LoadLocation(f.TZ) // already validated
		_, offset := time.Now().In(loc).Zone()
		zone = posixZone(offset)
	}
	h.zones.Store(f.TZ, zone)
	f.TZ = zone
	return f, true
}

// Protected: GET /api/me/analytics/summary
func (h *AnalyticsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r)
	if !ok {
		return
	}
	sum, err := h.store.AnalyticsSummary(r.Context(), auth.ScopeOf(r), f)
	if err != nil {
		httpx.Internal("analytics summary", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sum)
}

// Protected: GET /api/me/analytics/timeseries
func (h *AnalyticsHandler) Timeseries(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r)
	if !ok {
		return
	}
	points, err := h.store.AnalyticsTimeseries(r.Context(), auth.ScopeOf(r), f)
	if err != nil {
		httpx.Internal("analytics timeseries", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"points": points})
}

// Protected: GET /api/me/analytics/content
func (h *AnalyticsHandler) Content(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r)
	if !ok {
		return
	}
	items, err := h.store.AnalyticsContent(r.Context(), auth.ScopeOf(r), f)
	if err != nil {
		httpx.Internal("analytics content", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

// Protected: GET /api/me/analytics/cards
func (h *AnalyticsHandler) Cards(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r)
	if !ok {
		return
	}
	cards, err := h.store.AnalyticsCards(r.Context(), auth.ScopeOf(r), f)
	if err != nil {
		httpx.Internal("analytics cards", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"cards": cards})
}

// Protected: GET /api/me/analytics/teams. Admins compare every team; team
// leads the teams they lead. Other members are refused.
func (h *AnalyticsHandler) Teams(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFrom(r.Context())
	if !principal.IsAdmin() && !principal.LeadsAnyTeam() {
		httpx.WriteError(w, http.StatusForbidden, "only admins and team leads can compare teams")
		return
	}
	f, ok := h.filter(w, r)
	if !ok {
		return
	}
	var leadUserID int64
	if !principal.IsAdmin() {
		leadUserID = principal.UserID
	}
	teams, err := h.store.AnalyticsTeams(r.Context(), principal.OrgID, leadUserID, f.TeamID, f.From, f.To)
	if err != nil {
		httpx.Internal("analytics teams", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"teams": teams})
}

// Protected: GET /api/me/analytics/members[?team_id=]. The people the user can
// see, ranked by their cards' activity.
func (h *AnalyticsHandler) Members(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r)
	if !ok {
		return
	}
	members, err := h.store.AnalyticsMembers(r.Context(), auth.ScopeOf(r), f.TeamID, f.From, f.To)
	if err != nil {
		httpx.Internal("analytics members", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"members": members})
}

// Protected: GET /api/me/analytics/activity[?limit=]. The latest visits,
// contact saves, sent forms and brochure opens.
func (h *AnalyticsHandler) Activity(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r)
	if !ok {
		return
	}
	limit := defaultActivityLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxActivityLimit {
			httpx.WriteError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = n
	}
	items, err := h.store.AnalyticsActivity(r.Context(), auth.ScopeOf(r), f, limit)
	if err != nil {
		httpx.Internal("analytics activity", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
