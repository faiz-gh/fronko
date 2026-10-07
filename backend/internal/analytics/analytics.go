package analytics

import "github.com/faiz-gh/fronko/backend/internal/auth"

// Event types recorded on public cards.
const (
	EventView        = "view"
	EventClick       = "click"
	EventScroll      = "scroll"
	EventDocOpen     = "doc_open"
	EventGalleryOpen = "gallery_open"
	EventVCard       = "vcard"
	EventFormOpen    = "form_open"
	EventFormSubmit  = "form_submit"
	EventShare       = "share"
	EventLeave       = "leave"
)

// Visit sources: the NFC tag, the QR code, or any other link to the card.
const (
	SourceNFC  = "nfc"
	SourceQR   = "qr"
	SourceLink = "link"
)

// AnalyticsTotals are the headline numbers for a period.
type AnalyticsTotals struct {
	Views int64 `json:"views"`
	// UniqueVisitors counts distinct visitors per day, summed over the period:
	// visitor hashes rotate daily, so the same person on two days counts twice.
	UniqueVisitors int64 `json:"unique_visitors"`
	Sessions       int64 `json:"sessions"`
	NFCViews       int64 `json:"nfc_views"`
	QRViews        int64 `json:"qr_views"`
	LinkViews      int64 `json:"link_views"`
	Saves          int64 `json:"saves"`
	FormOpens      int64 `json:"form_opens"`
	Leads          int64 `json:"leads"`
	DocOpens       int64 `json:"doc_opens"`
	Clicks         int64 `json:"clicks"`
	GalleryOpens   int64 `json:"gallery_opens"`
	Shares         int64 `json:"shares"`
	// EngagedSessions clicked, saved, opened something, shared or scrolled at least half way.
	EngagedSessions int64 `json:"engaged_sessions"`
	// ActionSessions saved the contact or opened the contact form.
	ActionSessions int64 `json:"action_sessions"`
	// RepeatVisitors came back for another visit on the same day.
	RepeatVisitors int64 `json:"repeat_visitors"`
	// AvgScrollDepth is the mean of each visit's deepest scroll, in percent.
	AvgScrollDepth float64 `json:"avg_scroll_depth"`
	// MedianTimeMs is the median time a visit kept the card on screen.
	MedianTimeMs int64 `json:"median_time_ms"`
}

// AnalyticsSummary is a period's totals, the period before it (for changes),
// and how the visits break down.
type AnalyticsSummary struct {
	From     string          `json:"from"`
	To       string          `json:"to"`
	Current  AnalyticsTotals `json:"current"`
	Previous AnalyticsTotals `json:"previous"`
	// Devices counts visits per device class.
	Devices map[string]int64 `json:"devices"`
	// LeadSources counts leads by how the visitor arrived (unknown for older leads).
	LeadSources map[string]int64 `json:"lead_sources"`
	// Heatmap is views by weekday (0 = Sunday) and hour, in the requested time zone.
	Heatmap [7][24]int64 `json:"heatmap"`
	// ScrollDepths counts visits by deepest scroll: 0, 25, 50, 75 and 100 percent.
	ScrollDepths [5]int64 `json:"scroll_depths"`
	// TimeBuckets counts visits by time on card: <10s, 10–30s, 30s–1m, 1–3m, 3m+.
	TimeBuckets [5]int64 `json:"time_buckets"`
}

// AnalyticsPoint is one day of activity.
type AnalyticsPoint struct {
	Date           string `json:"date"`
	Views          int64  `json:"views"`
	NFCViews       int64  `json:"nfc_views"`
	QRViews        int64  `json:"qr_views"`
	LinkViews      int64  `json:"link_views"`
	UniqueVisitors int64  `json:"unique_visitors"`
	Saves          int64  `json:"saves"`
	Leads          int64  `json:"leads"`
}

// ContentStat is how often one link, quick action, document or image was used.
type ContentStat struct {
	Type    string `json:"type"`
	Target  string `json:"target"`
	Label   string `json:"label"`
	Count   int64  `json:"count"`
	Unique  int64  `json:"unique"`
	Profile int64  `json:"profile_id,omitempty"`
}

// CardStat is one card's activity in a period.
type CardStat struct {
	ProfileID      int64         `json:"profile_id"`
	Slug           string        `json:"slug"`
	Name           string        `json:"name"`
	AssignedUser   *auth.UserRef `json:"assigned_user"`
	Views          int64         `json:"views"`
	UniqueVisitors int64         `json:"unique_visitors"`
	NFCViews       int64         `json:"nfc_views"`
	QRViews        int64         `json:"qr_views"`
	LinkViews      int64         `json:"link_views"`
	Saves          int64         `json:"saves"`
	FormOpens      int64         `json:"form_opens"`
	Leads          int64         `json:"leads"`
	DocOpens       int64         `json:"doc_opens"`
	// LastViewedAt is the card's latest view ever, not just in the period.
	LastViewedAt *string `json:"last_viewed_at"`
}

// TeamStat is one team's activity in a period, with the period before it.
type TeamStat struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Color          string `json:"color"`
	Members        int64  `json:"members"`
	Cards          int64  `json:"cards"`
	ActiveCards    int64  `json:"active_cards"`
	Views          int64  `json:"views"`
	UniqueVisitors int64  `json:"unique_visitors"`
	Sessions       int64  `json:"sessions"`
	Engaged        int64  `json:"engaged_sessions"`
	Saves          int64  `json:"saves"`
	FormOpens      int64  `json:"form_opens"`
	Leads          int64  `json:"leads"`
	PrevViews      int64  `json:"prev_views"`
	PrevSaves      int64  `json:"prev_saves"`
	PrevLeads      int64  `json:"prev_leads"`
}

// MemberStat is one person's activity in a period: the leaderboard row.
type MemberStat struct {
	UserID         int64  `json:"user_id"`
	Username       string `json:"username"`
	Cards          int64  `json:"cards"`
	Views          int64  `json:"views"`
	UniqueVisitors int64  `json:"unique_visitors"`
	Saves          int64  `json:"saves"`
	FormOpens      int64  `json:"form_opens"`
	DocOpens       int64  `json:"doc_opens"`
	Leads          int64  `json:"leads"`
}

// ActivityItem is one notable thing that happened on a card, for the overview feed.
type ActivityItem struct {
	Type         string        `json:"type"`
	Source       string        `json:"source"`
	Label        string        `json:"label"`
	ProfileID    int64         `json:"profile_id"`
	CardName     string        `json:"card_name"`
	Slug         string        `json:"slug"`
	AssignedUser *auth.UserRef `json:"assigned_user"`
	CreatedAt    string        `json:"created_at"`
}
