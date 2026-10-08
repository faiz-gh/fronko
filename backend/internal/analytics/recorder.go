package analytics

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // reports take IANA time zones; the alpine image has no zoneinfo
	"unicode/utf8"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/ratelimit"
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
		EventView: true, EventClick: true, EventScroll: true,
		EventDocOpen: true, EventGalleryOpen: true, EventFormOpen: true,
		EventShare: true, EventLeave: true,
	}
)

// NormalizeSource maps the ?via= marker to a source; anything else is a plain link.
func NormalizeSource(s string) string {
	switch s {
	case SourceNFC, SourceQR:
		return s
	}
	return SourceLink
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
func cleanEvent(e CardEvent) (CardEvent, bool) {
	if !clientEventTypes[e.Type] {
		return e, false
	}
	e.Target = truncate(e.Target, maxEventTargetLen)
	e.Label = truncate(e.Label, maxEventLabelLen)
	switch e.Type {
	case EventScroll:
		// Only the tracker's thresholds are meaningful.
		switch e.Value {
		case 25, 50, 75, 100:
		default:
			return e, false
		}
	case EventLeave:
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
	store      *Store
	auth       *auth.Service
	sessions   auth.SessionChecker
	trustProxy bool

	mu      sync.Mutex
	saltDay string
	salt    []byte
}

func NewEventRecorder(store *Store, authService *auth.Service, sessions auth.SessionChecker, trustProxy bool) *EventRecorder {
	return &EventRecorder{store: store, auth: authService, sessions: sessions, trustProxy: trustProxy}
}

func (rec *EventRecorder) todaysSalt(ctx context.Context) ([]byte, error) {
	now := time.Now().UTC()
	day := now.Format(time.DateOnly)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.saltDay == day {
		return rec.salt, nil
	}
	salt, err := rec.store.AnalyticsSalt(ctx, now)
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
	cookie, err := r.Cookie(auth.SessionCookieName)
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

// optedOut reports whether the visitor's browser asks not to be tracked:
// Global Privacy Control or Do Not Track.
func optedOut(r *http.Request) bool {
	return r.Header.Get("Sec-GPC") == "1" || r.Header.Get("DNT") == "1"
}

// Record stores events for a card's visit. Bots, visitors who opted out of
// tracking and the card's own organisation are skipped; failures are
// logged, never shown to visitors.
func (rec *EventRecorder) Record(r *http.Request, profileID, orgID int64, source, session string, events ...CardEvent) {
	if rec == nil || len(events) == 0 || isBot(r) || optedOut(r) {
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
	err = rec.store.RecordCardEvents(ctx, EventBatch{
		ProfileID:   profileID,
		SessionID:   session,
		VisitorHash: visitorHash(salt, ratelimit.ClientIP(r, rec.trustProxy), r.UserAgent(), profileID),
		Source:      NormalizeSource(source),
		Device:      deviceClass(r.UserAgent()),
		Referrer:    referrerHost(r),
		Events:      events,
	})
	if err != nil {
		log.Printf("record card events: %v", err)
	}
}
