package models

import (
	"encoding/json"
	"time"
)

// Roles within an organisation. The owner registered it; admins manage it
// with the owner; members only work on the cards assigned to them.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// File areas: a user's own files, the organisation's private files, and the
// area every user in the organisation can see.
const (
	AreaPersonal = "personal"
	AreaOrg      = "org"
	AreaShared   = "shared"
)

type Organization struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// DefaultQuotaBytes is the storage limit given to new users; nil is unlimited.
	DefaultQuotaBytes *int64    `json:"default_quota_bytes"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	// SuspendedAt is set while a platform admin has suspended the organisation.
	SuspendedAt     *time.Time `json:"-"`
	SuspendedReason *string    `json:"-"`
}

type User struct {
	ID           int64  `json:"id"`
	OrgID        int64  `json:"-"`
	Role         string `json:"role"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"` // Never exposed in JSON
	// Email is nil only for accounts created before emails were required.
	Email           *string    `json:"email"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	// MustChangePassword is set while the password is one the organisation chose.
	MustChangePassword bool `json:"must_change_password"`
	// StorageQuotaBytes limits the user's personal files; nil is unlimited.
	StorageQuotaBytes *int64     `json:"storage_quota_bytes"`
	SuspendedAt       *time.Time `json:"suspended_at"`
	CreatedBy         *int64     `json:"-"`
	LastLoginAt       *time.Time `json:"last_login_at"`
	// SessionVersion is carried in session tokens; bumping it revokes them all.
	SessionVersion int       `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// IsAdmin reports whether the user can manage the organisation.
func (u *User) IsAdmin() bool { return u.Role == RoleOwner || u.Role == RoleAdmin }

// OrgUser is a user as the organisation's Users page lists them.
type OrgUser struct {
	User
	CardCount int64 `json:"card_count"`
	LeadCount int64 `json:"lead_count"`
	UsedBytes int64 `json:"used_bytes"`
}

// UserRef names a user alongside something they hold (a card, lead or file).
type UserRef struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

// SessionState is the live account state behind a session token, checked on every request.
type SessionState struct {
	Version            int
	Verified           bool
	OrgID              int64
	Role               string
	Suspended          bool
	MustChangePassword bool
	// OrgSuspended is set when a platform admin suspended the whole organisation.
	OrgSuspended       bool
	OrgSuspendedReason string
}

// EmailCode is a one-time code sent by email. Only its HMAC is stored.
type EmailCode struct {
	UserID  int64
	Purpose string
	// Email is the address a change_email code was sent to; nil for other purposes.
	Email     *string
	CodeHash  string
	Attempts  int
	ExpiresAt time.Time
	CreatedAt time.Time
}

type Profile struct {
	ID     int64 `json:"id"`
	OrgID  int64 `json:"-"`
	UserID int64 `json:"user_id"` // the account that created the card
	// AssignedUserID is the one user who works on the card; nil means the organisation holds it.
	AssignedUserID *int64          `json:"-"`
	AssignedUser   *UserRef        `json:"assigned_user"`
	Slug           string          `json:"slug"`
	Data           json.RawMessage `json:"data"` // JSONB block data
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	LeadCount      int64           `json:"lead_count"` // only populated when listing a user's profiles
	// OrgSuspended is only populated by the public slug lookup.
	OrgSuspended bool `json:"-"`
}

// PublicProfile is the shape served to anonymous visitors; it omits owner details.
type PublicProfile struct {
	ID    int64           `json:"id"`
	Slug  string          `json:"slug"`
	Data  json.RawMessage `json:"data"`
	Files []PublicFile    `json:"files"` // library files the card references
}

// StorageSettings is a user's bucket configuration. The key fields hold
// ciphertext (see package secrets) and never leave the server.
type StorageSettings struct {
	UserID             int64
	Provider           string
	Endpoint           string
	Region             string
	Bucket             string
	PathStyle          bool
	AccessKeyIDEnc     []byte
	SecretAccessKeyEnc []byte
	AccessKeyHint      string
	VerifiedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// File is an upload in a user's library. PublicID is the only identifier exposed to visitors.
type File struct {
	ID       int64  `json:"-"`
	PublicID string `json:"id"`
	OrgID    int64  `json:"-"`
	UserID   int64  `json:"-"` // the uploader; for personal files, the owner
	Area     string `json:"area"`
	// Owner is the user a personal file belongs to (or who uploaded an org or shared file).
	Owner *UserRef `json:"owner,omitempty"`
	// FormerOwner is the username of a deleted user whose personal file this was.
	FormerOwner  *string   `json:"former_owner,omitempty"`
	Bucket       string    `json:"-"`
	ObjectKey    string    `json:"-"`
	Kind         string    `json:"kind"` // "image" or "pdf"
	ContentType  string    `json:"content_type"`
	SizeBytes    int64     `json:"size_bytes"`
	OriginalName string    `json:"name"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
}

// PublicFile is the file metadata shown on a public card.
type PublicFile struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
}

type Lead struct {
	ID        int64  `json:"id"`
	ProfileID int64  `json:"profile_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	// Optional; both set or both empty. Dial code with "+" ("+91") and the
	// national number, digits only.
	PhoneCountryCode string    `json:"phone_country_code,omitempty"`
	PhoneNumber      string    `json:"phone_number,omitempty"`
	Notes            string    `json:"notes"`
	CreatedAt        time.Time `json:"created_at"`
	// AssignedUser held the card when the lead arrived; nil means the organisation did.
	AssignedUser *UserRef `json:"assigned_user"`
}
