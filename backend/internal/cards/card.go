package cards

import (
	"context"
	"encoding/json"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/branding"
	"github.com/faiz-gh/fronko/backend/internal/files"
)

type Profile struct {
	ID     int64 `json:"id"`
	OrgID  int64 `json:"-"`
	UserID int64 `json:"user_id"` // the account that created the card
	// AssignedUserID is the one user who works on the card; nil means the organisation holds it.
	AssignedUserID *int64          `json:"-"`
	AssignedUser   *auth.UserRef   `json:"assigned_user"`
	Slug           string          `json:"slug"`
	Data           json.RawMessage `json:"data"` // JSONB block data
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	LeadCount      int64           `json:"lead_count"` // only populated when listing a user's profiles
	// Booking is the booking page the card shows: its holder's, or else the organisation's.
	Booking *Booking `json:"booking"`
	// OrgHandle and OrgSuspended are only populated by the public link lookup.
	OrgHandle    string `json:"-"`
	OrgSuspended bool   `json:"-"`
}

// PublicProfile is the shape served to anonymous visitors; it omits owner details.
type PublicProfile struct {
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
	// OrgHandle is the organisation's part of the card's link, /p/{org_handle}/{slug}.
	OrgHandle string              `json:"org_handle"`
	Data      json.RawMessage     `json:"data"`
	Files     []files.PublicFile  `json:"files"` // library files the card references
	Org       *branding.PublicOrg `json:"org"`
	// Booking is the "Book a meeting" button's page, or null.
	Booking *Booking `json:"booking"`
}

// Booking is a booking page connected in Integrations (Calendar Booking).
type Booking struct {
	// Provider is the integration's id ("calendly"), Name its display name.
	Provider string `json:"provider"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	// Prefill maps the page's query parameters to what a visitor gave the
	// card's lead form: "name", "first_name", "last_name" or "email".
	Prefill map[string]string `json:"prefill,omitempty"`
	// Scope is "user" for the holder's own page, "org" for the organisation's default.
	Scope string `json:"scope"`
}

// BookingFinder loads the booking pages connected in an organisation and
// returns a lookup by card holder (0: the organisation). It's the
// integrations module, wired in by the server.
type BookingFinder func(ctx context.Context, orgID int64) (func(holderID int64) *Booking, error)

// holderID is who holds the card, or 0 for the organisation.
func (p *Profile) holderID() int64 {
	if p.AssignedUserID == nil {
		return 0
	}
	return *p.AssignedUserID
}
