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
	// Booking is the booking page the card chose (data.booking_connection_id), if it can still show it.
	Booking *Booking `json:"booking"`
	// BookingOptions are the pages the card can choose from: its holder's
	// and the organisation's. Only filled for a single card.
	BookingOptions []Booking `json:"booking_options,omitempty"`
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
	// ConnectionID is the integration connection; Label its name ("30-min intro").
	ConnectionID int64  `json:"connection_id"`
	Label        string `json:"label"`
	// Provider is the integration's id ("calendly"), Name its display name.
	Provider string `json:"provider"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	// Prefill maps the page's query parameters to what a visitor gave the
	// card's lead form: "name", "first_name", "last_name" or "email".
	Prefill map[string]string `json:"prefill,omitempty"`
	// Scope is "user" for a person's own page, "org" for the organisation's.
	Scope string `json:"scope"`
}

// BookingPages are the booking pages connected in an organisation.
type BookingPages interface {
	// For returns the page connectionID if a card held by holderID (0: the
	// organisation) may show it, or nil.
	For(holderID, connectionID int64) *Booking
	// Options are the pages a card held by holderID can choose from.
	Options(holderID int64) []Booking
}

// BookingFinder loads an organisation's booking pages. It's the
// integrations module, wired in by the server.
type BookingFinder func(ctx context.Context, orgID int64) (BookingPages, error)

// bookingConnectionID is the booking page the card data chose, or 0.
func bookingConnectionID(data json.RawMessage) int64 {
	var d struct {
		BookingConnectionID *int64 `json:"booking_connection_id"`
	}
	if json.Unmarshal(data, &d) != nil || d.BookingConnectionID == nil {
		return 0
	}
	return *d.BookingConnectionID
}

// bookingID is the booking page the card chose, or 0.
func (p *Profile) bookingID() int64 { return bookingConnectionID(p.Data) }

// holderID is who holds the card, or 0 for the organisation.
func (p *Profile) holderID() int64 {
	if p.AssignedUserID == nil {
		return 0
	}
	return *p.AssignedUserID
}
