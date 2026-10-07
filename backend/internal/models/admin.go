package models

import (
	"encoding/json"
	"time"
)

// PlatformAdmin runs the Fronko service. Admins are not organisation users.
type PlatformAdmin struct {
	ID             int64      `json:"id"`
	Email          string     `json:"email"`
	PasswordHash   string     `json:"-"`
	SessionVersion int        `json:"-"`
	LastLoginAt    *time.Time `json:"last_login_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

// OrgUsage is what the admin panel shows about an organisation: aggregate
// counts only, never card contents, leads, files or member details.
type OrgUsage struct {
	ID                 int64      `json:"id"`
	Name               string     `json:"name"`
	Handle             string     `json:"handle"`
	CreatedAt          time.Time  `json:"created_at"`
	OwnerEmail         *string    `json:"owner_email"`
	SuspendedAt        *time.Time `json:"suspended_at"`
	SuspendedReason    *string    `json:"suspended_reason,omitempty"`
	UserCount          int64      `json:"user_count"`
	AdminCount         int64      `json:"admin_count"`
	MemberCount        int64      `json:"member_count"`
	SuspendedUserCount int64      `json:"suspended_user_count"`
	TeamCount          int64      `json:"team_count"`
	CardCount          int64      `json:"card_count"`
	LeadCount          int64      `json:"lead_count"`
	FileCount          int64      `json:"file_count"`
	StorageUsedBytes   int64      `json:"storage_used_bytes"`
	StorageConnected   bool       `json:"storage_connected"`
	StorageVerified    bool       `json:"storage_verified"`
	StorageProvider    *string    `json:"storage_provider"`
	DefaultQuotaBytes  *int64     `json:"default_quota_bytes"`
	LastActiveAt       *time.Time `json:"last_active_at"`
	// Branding, as yes/no settings only.
	LogoSet         bool   `json:"logo_set"`
	LogoPolicy      string `json:"logo_policy"`
	SignatureLocked bool   `json:"signature_locked"`
	// FilesByPurpose counts files per purpose (logo, brochure, …).
	FilesByPurpose map[string]int64 `json:"files_by_purpose"`
}

// PlatformSummary totals usage across every organisation.
type PlatformSummary struct {
	OrgCount          int64 `json:"org_count"`
	SuspendedOrgCount int64 `json:"suspended_org_count"`
	NewOrgs30d        int64 `json:"new_orgs_30d"`
	UserCount         int64 `json:"user_count"`
	CardCount         int64 `json:"card_count"`
	LeadCount         int64 `json:"lead_count"`
	FileCount         int64 `json:"file_count"`
	OrgsWithStorage   int64 `json:"orgs_with_storage"`
	StorageUsedBytes  int64 `json:"storage_used_bytes"`
	ActiveOrgs30d     int64 `json:"active_orgs_30d"`
	NewFeedback       int64 `json:"new_feedback"`
	TeamCount         int64 `json:"team_count"`
	OrgsWithTeams     int64 `json:"orgs_with_teams"`
	OrgsWithLogo      int64 `json:"orgs_with_logo"`
}

// UsagePoint is one day of a usage trend. Platform trends fill every field;
// organisation trends leave the platform-only ones zero.
type UsagePoint struct {
	Date             string `json:"date"` // YYYY-MM-DD
	OrgCount         int64  `json:"org_count,omitempty"`
	UserCount        int64  `json:"user_count"`
	CardCount        int64  `json:"card_count"`
	LeadCount        int64  `json:"lead_count"`
	FileCount        int64  `json:"file_count"`
	OrgsWithStorage  int64  `json:"orgs_with_storage,omitempty"`
	StorageUsedBytes int64  `json:"storage_used_bytes"`
	NewOrgs          int64  `json:"new_orgs,omitempty"`
	FeedbackCount    int64  `json:"feedback_count,omitempty"`
	TeamCount        int64  `json:"team_count"`
	OrgsWithTeams    int64  `json:"orgs_with_teams,omitempty"`
	OrgsWithLogo     int64  `json:"orgs_with_logo,omitempty"`
}

// Feedback categories and statuses.
const (
	FeedbackBug   = "bug"
	FeedbackIdea  = "idea"
	FeedbackOther = "other"

	FeedbackNew      = "new"
	FeedbackRead     = "read"
	FeedbackResolved = "resolved"
)

// Feedback is a message a signed-in user sent about the product.
type Feedback struct {
	ID          int64            `json:"id"`
	OrgID       *int64           `json:"org_id"`
	UserID      *int64           `json:"-"`
	SenderEmail string           `json:"sender_email"`
	OrgName     string           `json:"org_name"`
	Category    string           `json:"category"`
	Rating      *int16           `json:"rating"`
	Message     string           `json:"message"`
	PagePath    *string          `json:"page_path"`
	Status      string           `json:"status"`
	ReplyCount  int64            `json:"reply_count"`
	Replies     []*FeedbackReply `json:"replies,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// FeedbackReply is an answer a platform admin emailed to the sender.
type FeedbackReply struct {
	ID         int64     `json:"id"`
	FeedbackID int64     `json:"-"`
	AdminID    *int64    `json:"-"`
	AdminEmail *string   `json:"admin_email"`
	Body       string    `json:"body"`
	EmailSent  bool      `json:"email_sent"`
	CreatedAt  time.Time `json:"created_at"`
}

// AuditEntry records one thing a platform admin did.
type AuditEntry struct {
	ID         int64           `json:"id"`
	AdminID    *int64          `json:"-"`
	AdminEmail string          `json:"admin_email"`
	Action     string          `json:"action"`
	TargetType *string         `json:"target_type"`
	TargetID   *int64          `json:"target_id"`
	Detail     json.RawMessage `json:"detail"`
	CreatedAt  time.Time       `json:"created_at"`
}

// Audit actions.
const (
	AuditAdminLogin     = "admin.login"
	AuditOrgSuspend     = "org.suspend"
	AuditOrgReinstate   = "org.reinstate"
	AuditFeedbackReply  = "feedback.reply"
	AuditFeedbackStatus = "feedback.status"
)
