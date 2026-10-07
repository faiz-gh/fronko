package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // reports take IANA time zones; the alpine image has no zoneinfo
	"unicode/utf8"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
)

const (
	// maxEventsPerBatch caps one beacon; the tracker flushes every few seconds.
	maxEventsPerBatch = 20
	maxEventBodyBytes = 16 << 10
	maxEventTargetLen = 300
	maxEventLabelLen  = 120
	// maxTimeOnCardMs keeps a tab left open overnight from skewing durations.
	maxTimeOnCardMs = 60 * 60 * 1000

	defaultAnalyticsRange = 30 * 24 * time.Hour
	maxAnalyticsRange     = 366 * 24 * time.Hour
	defaultActivityLimit  = 15
	maxActivityLimit      = 50
)

var (
	sessionIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	// Crawlers, link unfurlers and scripts; they never count as visits.
	botPattern = regexp.MustCompile(`(?i)bot|crawl|spider|slurp|preview|headless|lighthouse|facebookexternalhit|` +
		`curl|wget|python-requests|go-http-client|okhttp|axios|node-fetch|java/`)

	// clientEventTypes are what the browser may report. Contact saves and sent
	// forms are recorded by the server when they happen, so they can't be faked
	// or counted twice.
	clientEventTypes = map[string]bool{
		models.EventView: true, models.EventClick: true, models.EventScroll: true,
		models.EventDocOpen: true, models.EventGalleryOpen: true, models.EventFormOpen: true,
		models.EventShare: true, models.EventLeave: true,
	}
)

// normalizeSource maps the ?via= marker to a source; anything else is a plain link.
func normalizeSource(s string) string {
	switch s {
	case models.SourceNFC, models.SourceQR:
		return s
	}
	return models.SourceLink
}

// deviceClass is a rough phone / tablet / computer split from the user agent.
func deviceClass(ua string) string {
	switch {
	case strings.Contains(ua, "iPad") || strings.Contains(ua, "Tablet") ||
		(strings.Contains(ua, "Android") && !strings.Contains(ua, "Mobile")):
		return "tablet"
	case strings.Contains(ua, "Mobi") || strings.Contains(ua, "iPhone") || strings.Contains(ua, "Android"):
		return "mobile"
	}
	return "desktop"
}

// truncate cuts s to at most n runes.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// cleanEvent validates one event from the browser, returning ok=false to drop it.
func cleanEvent(e repository.CardEvent) (repository.CardEvent, bool) {
	if !clientEventTypes[e.Type] {
		return e, false
	}
	e.Target = truncate(e.Target, maxEventTargetLen)
	e.Label = truncate(e.Label, maxEventLabelLen)
	switch e.Type {
	case models.EventScroll:
		// Only the tracker's thresholds are meaningful.
		switch e.Value {
		case 25, 50, 75, 100:
		default:
			return e, false
		}
	case models.EventLeave:
		e.Value = min(max(e.Value, 0), maxTimeOnCardMs)
	default:
		e.Value = 0
	}
	return e, true
}

// EventRecorder stores what visitors do on public cards without identifying
// them: the visitor hash mixes the address and browser with a random salt
// that changes every day and is then deleted.
type EventRecorder struct {
	repo       *repository.Repository
	auth       *auth.Service
	sessions   middleware.SessionChecker
	trustProxy bool

	mu      sync.Mutex
	saltDay string
	salt    []byte
}

func NewEventRecorder(repo *repository.Repository, authService *auth.Service, sessions middleware.SessionChecker, trustProxy bool) *EventRecorder {
	return &EventRecorder{repo: repo, auth: authService, sessions: sessions, trustProxy: trustProxy}
}

func (rec *EventRecorder) todaysSalt(ctx context.Context) ([]byte, error) {
	now := time.Now().UTC()
	day := now.Format(time.DateOnly)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.saltDay == day {
		return rec.salt, nil
	}
	salt, err := rec.repo.AnalyticsSalt(ctx, now)
	if err != nil {
		return nil, err
	}
	rec.saltDay, rec.salt = day, salt
	return salt, nil
}

// visitorHash is stable for one browser on one card for one day, and useless after that.
func visitorHash(salt []byte, ip, ua string, profileID int64) []byte {
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(ip))
	h.Write([]byte{0})
	h.Write([]byte(ua))
	h.Write([]byte{0})
	var id [8]byte
	binary.BigEndian.PutUint64(id[:], uint64(profileID))
	h.Write(id[:])
	return h.Sum(nil)
}

// isBot reports whether the request comes from a crawler or script.
func isBot(r *http.Request) bool {
	ua := r.UserAgent()
	return ua == "" || botPattern.MatchString(ua)
}

// viewerOrg is the organisation of the signed-in user making the request, or
// 0 for anonymous visitors. People previewing their own organisation's cards
// aren't counted.
func (rec *EventRecorder) viewerOrg(r *http.Request) int64 {
	if rec.auth == nil || rec.sessions == nil {
		return 0
	}
	cookie, err := r.Cookie(middleware.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return 0
	}
	userID, version, err := rec.auth.ValidateJWT(cookie.Value)
	if err != nil {
		return 0
	}
	state, err := rec.sessions.GetSessionState(r.Context(), userID)
	if err != nil || state.Version != version {
		return 0
	}
	return state.OrgID
}

// referrerHost is the host of a referring page on another site, if any.
func referrerHost(r *http.Request) string {
	ref, err := url.Parse(r.Referer())
	if err != nil || ref.Host == "" || strings.EqualFold(ref.Host, r.Host) {
		return ""
	}
	return truncate(strings.ToLower(ref.Hostname()), 255)
}

// Record stores events for a card's visit. Bots and the card's own
// organisation are skipped; failures are logged, never shown to visitors.
func (rec *EventRecorder) Record(r *http.Request, profileID, orgID int64, source, session string, events ...repository.CardEvent) {
	if rec == nil || len(events) == 0 || isBot(r) {
		return
	}
	if orgID != 0 && rec.viewerOrg(r) == orgID {
		return
	}
	if !sessionIDPattern.MatchString(session) {
		session = ""
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancel()
	salt, err := rec.todaysSalt(ctx)
	if err != nil {
		log.Printf("analytics salt: %v", err)
		return
	}
	err = rec.repo.RecordCardEvents(ctx, repository.EventBatch{
		ProfileID:   profileID,
		SessionID:   session,
		VisitorHash: visitorHash(salt, middleware.ClientIP(r, rec.trustProxy), r.UserAgent(), profileID),
		Source:      normalizeSource(source),
		Device:      deviceClass(r.UserAgent()),
		Referrer:    referrerHost(r),
		Events:      events,
	})
	if err != nil {
		log.Printf("record card events: %v", err)
	}
}

// ----------------------------------------------------------------------------
// Handlers
// ----------------------------------------------------------------------------

type AnalyticsHandler struct {
	repo   *repository.Repository
	events *EventRecorder
	// zones caches how each requested time zone is passed to the database.
	zones sync.Map
}

func NewAnalyticsHandler(repo *repository.Repository, events *EventRecorder) *AnalyticsHandler {
	return &AnalyticsHandler{repo: repo, events: events}
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
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Events) > maxEventsPerBatch {
		req.Events = req.Events[:maxEventsPerBatch]
	}
	events := make([]repository.CardEvent, 0, len(req.Events))
	for _, e := range req.Events {
		if ev, ok := cleanEvent(repository.CardEvent{Type: e.Type, Target: e.Target, Label: e.Label, Value: e.Value}); ok {
			events = append(events, ev)
		}
	}
	if len(events) == 0 || isBot(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	profile, err := h.repo.GetProfileByPath(r.Context(), r.PathValue("org"), r.PathValue("slug"))
	if err != nil || profile.OrgSuspended {
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
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
func analyticsFilter(w http.ResponseWriter, r *http.Request) (repository.AnalyticsFilter, bool) {
	q := r.URL.Query()
	principal := middleware.PrincipalFrom(r.Context())
	f := repository.AnalyticsFilter{To: time.Now(), TZ: "UTC"}

	parseTime := func(name string) (time.Time, bool) {
		t, err := time.Parse(time.RFC3339, q.Get(name))
		if err != nil {
			writeError(w, http.StatusBadRequest, name+" must be an RFC 3339 timestamp")
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
		writeError(w, http.StatusBadRequest, "the period must be between a moment and 366 days long")
		return f, false
	}
	if tz := q.Get("tz"); tz != "" {
		if _, err := time.LoadLocation(tz); err != nil || len(tz) > 64 {
			writeError(w, http.StatusBadRequest, "invalid tz")
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
			writeError(w, http.StatusBadRequest, "invalid "+name)
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
		writeError(w, http.StatusForbidden, "you can only see analytics for teams you lead")
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
func (h *AnalyticsHandler) filter(w http.ResponseWriter, r *http.Request) (repository.AnalyticsFilter, bool) {
	f, ok := analyticsFilter(w, r)
	if !ok || f.TZ == "UTC" {
		return f, ok
	}
	if zone, cached := h.zones.Load(f.TZ); cached {
		f.TZ = zone.(string)
		return f, true
	}
	known, err := h.repo.KnowsTimeZone(r.Context(), f.TZ)
	if err != nil {
		internalError("time zone", err).write(w)
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
	sum, err := h.repo.AnalyticsSummary(r.Context(), scopeOf(r), f)
	if err != nil {
		internalError("analytics summary", err).write(w)
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

// Protected: GET /api/me/analytics/timeseries
func (h *AnalyticsHandler) Timeseries(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r)
	if !ok {
		return
	}
	points, err := h.repo.AnalyticsTimeseries(r.Context(), scopeOf(r), f)
	if err != nil {
		internalError("analytics timeseries", err).write(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"points": points})
}

// Protected: GET /api/me/analytics/content
func (h *AnalyticsHandler) Content(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r)
	if !ok {
		return
	}
	items, err := h.repo.AnalyticsContent(r.Context(), scopeOf(r), f)
	if err != nil {
		internalError("analytics content", err).write(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// Protected: GET /api/me/analytics/cards
func (h *AnalyticsHandler) Cards(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r)
	if !ok {
		return
	}
	cards, err := h.repo.AnalyticsCards(r.Context(), scopeOf(r), f)
	if err != nil {
		internalError("analytics cards", err).write(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": cards})
}

// Protected: GET /api/me/analytics/teams. Admins compare every team; team
// leads the teams they lead. Other members are refused.
func (h *AnalyticsHandler) Teams(w http.ResponseWriter, r *http.Request) {
	principal := middleware.PrincipalFrom(r.Context())
	if !principal.IsAdmin() && !principal.LeadsAnyTeam() {
		writeError(w, http.StatusForbidden, "only admins and team leads can compare teams")
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
	teams, err := h.repo.AnalyticsTeams(r.Context(), principal.OrgID, leadUserID, f.TeamID, f.From, f.To)
	if err != nil {
		internalError("analytics teams", err).write(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"teams": teams})
}

// Protected: GET /api/me/analytics/members[?team_id=]. The people the user can
// see, ranked by their cards' activity.
func (h *AnalyticsHandler) Members(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r)
	if !ok {
		return
	}
	members, err := h.repo.AnalyticsMembers(r.Context(), scopeOf(r), f.TeamID, f.From, f.To)
	if err != nil {
		internalError("analytics members", err).write(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
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
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = n
	}
	items, err := h.repo.AnalyticsActivity(r.Context(), scopeOf(r), f, limit)
	if err != nil {
		internalError("analytics activity", err).write(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
