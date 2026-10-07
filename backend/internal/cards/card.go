package cards

import (
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
}
