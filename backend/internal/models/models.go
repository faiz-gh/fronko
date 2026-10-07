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
	AreaTeam     = "team"
)

// Roles within a team. Leads look after the team's files and see their
// teammates' cards and leads; members see the team's files.
const (
	TeamRoleLead   = "lead"
	TeamRoleMember = "member"
)

// File purposes say what a file is for, so the library and pickers can sort
// logos from brochures.
const (
	PurposeLogo     = "logo"
	PurposeBanner   = "banner"
	PurposeAvatar   = "avatar"
	PurposeCover    = "cover"
	PurposeGallery  = "gallery"
	PurposeBrochure = "brochure"
	PurposeOther    = "other"
)

// Purposes lists every file purpose, in the order the library shows them.
var Purposes = []string{PurposeLogo, PurposeBanner, PurposeAvatar, PurposeCover, PurposeGallery, PurposeBrochure, PurposeOther}

// ValidPurpose reports whether p is a known file purpose.
func ValidPurpose(p string) bool {
	for _, v := range Purposes {
		if v == p {
			return true
		}
	}
	return false
}

// Team is a group of people in an organisation, with its totals.
type Team struct {
	ID          int64     `json:"id"`
	OrgID       int64     `json:"-"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	MemberCount int64     `json:"member_count"`
	LeadCount   int64     `json:"lead_count"`
	FileCount   int64     `json:"file_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TeamMember is someone in a team and their role in it.
type TeamMember struct {
	UserRef
	Email *string `json:"email"`
	// OrgRole is the person's role in the organisation (owner, admin, member).
	OrgRole string    `json:"org_role"`
	Role    string    `json:"role"`
	AddedAt time.Time `json:"added_at"`
}

// TeamRef names a team alongside someone's role in it (or a file in it).
type TeamRef struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Role  string `json:"role,omitempty"`
}

// TeamMembership is a person's place in one team, as sent when setting their teams.
type TeamMembership struct {
	TeamID int64  `json:"team_id"`
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
}

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

// Logo policies: whether every card and signature shows the organisation's
// logo, or each card chooses.
const (
	LogoRequired = "required"
	LogoOptional = "optional"
)

// OrgSignature is the organisation's email signature settings.
type OrgSignature struct {
	// LockedTemplate, when set, is the only signature template employees can use.
	LockedTemplate string `json:"locked_template,omitempty"`
	// BrandColor (#rrggbb), when set, replaces each card's accent in signatures.
	BrandColor string `json:"brand_color,omitempty"`
	Disclaimer string `json:"disclaimer,omitempty"`
	// BannerFile is an org image shown under every signature, linking to BannerURL.
	BannerFile string `json:"banner_file,omitempty"`
	BannerURL  string `json:"banner_url,omitempty"`
}

// OrgBranding is how an organisation appears on its cards and email signatures.
type OrgBranding struct {
	Name       string       `json:"name"`
	LogoFile   *string      `json:"logo_file"`
	LogoPolicy string       `json:"logo_policy"`
	Signature  OrgSignature `json:"signature"`
}

// PublicOrg is the branding a public card needs.
type PublicOrg struct {
	Name       string  `json:"name"`
	LogoFile   *string `json:"logo_file"`
	LogoPolicy string  `json:"logo_policy"`
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
	CardCount int64     `json:"card_count"`
	LeadCount int64     `json:"lead_count"`
	UsedBytes int64     `json:"used_bytes"`
	Teams     []TeamRef `json:"teams"`
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
	// Teams the user is in, with their role in each.
	Teams []TeamRef
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
	Org   *PublicOrg      `json:"org"`
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
	FormerOwner *string `json:"former_owner,omitempty"`
	// TeamID and Team are set for files in a team's area.
	TeamID       *int64   `json:"-"`
	Team         *TeamRef `json:"team,omitempty"`
	Bucket       string   `json:"-"`
	ObjectKey    string   `json:"-"`
	ThumbKey     *string  `json:"-"`
	Kind         string   `json:"kind"` // "image" or "pdf"
	Purpose      string   `json:"purpose"`
	ContentType  string   `json:"content_type"`
	SizeBytes    int64    `json:"size_bytes"`
	OriginalName string   `json:"name"`
	Title        string   `json:"title"`
	Width        *int     `json:"width"`
	Height       *int     `json:"height"`
	Pages        *int     `json:"pages"`
	HasThumb     bool     `json:"has_thumb"`
	// UseCount is how many places use the file: card slots, the logo and the signature banner.
	UseCount  int64     `json:"use_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FileUse is one card using a file, and where on the card.
type FileUse struct {
	ProfileID int64  `json:"profile_id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Slot      string `json:"slot"`
}

// FileUsage is everywhere a file is used that the viewer may know about.
type FileUsage struct {
	Cards []FileUse `json:"cards"`
	// HiddenCards counts cards using the file that the viewer can't see.
	HiddenCards int64 `json:"hidden_cards"`
	OrgLogo     bool  `json:"org_logo"`
	SigBanner   bool  `json:"signature_banner"`
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
	PhoneCountryCode string `json:"phone_country_code,omitempty"`
	PhoneNumber      string `json:"phone_number,omitempty"`
	Notes            string `json:"notes"`
	// Source is how the visitor reached the card: nfc, qr or link ("" for older leads).
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	// AssignedUser held the card when the lead arrived; nil means the organisation did.
	AssignedUser *UserRef `json:"assigned_user"`
}
