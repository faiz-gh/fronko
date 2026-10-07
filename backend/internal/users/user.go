package users

import (
	"time"

	"github.com/faiz-gh/fronko/backend/internal/auth"
)

type User struct {
	ID       int64  `json:"id"`
	OrgID    int64  `json:"-"`
	Role     string `json:"role"`
	Username string `json:"username"`
	// PasswordHash is "" for accounts that only sign in with single sign-on.
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
	SessionVersion int `json:"-"`
	// FullName is the person's name as their identity provider knows it.
	FullName *string `json:"full_name"`
	// ExternalID and ExternalUsername are the identity provider's id and
	// user name for the person (SCIM externalId and userName).
	ExternalID       *string `json:"-"`
	ExternalUsername *string `json:"-"`
	// ProvisionedBy is what created the account: nil for a person, or
	// ProvisionedSCIM or ProvisionedSSO.
	ProvisionedBy *string   `json:"provisioned_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// What created an account, besides a person.
const (
	ProvisionedSCIM = "scim"
	ProvisionedSSO  = "sso"
)

// HasPassword reports whether the user can sign in with a password.
func (u *User) HasPassword() bool { return u.PasswordHash != "" }

// IsAdmin reports whether the user can manage the organisation.
func (u *User) IsAdmin() bool { return u.Role == auth.RoleOwner || u.Role == auth.RoleAdmin }

// OrgUser is a user as the organisation's Users page lists them.
type OrgUser struct {
	User
	CardCount int64          `json:"card_count"`
	LeadCount int64          `json:"lead_count"`
	UsedBytes int64          `json:"used_bytes"`
	Teams     []auth.TeamRef `json:"teams"`
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
