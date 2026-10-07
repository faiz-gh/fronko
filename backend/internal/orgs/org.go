package orgs

import (
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Organization struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Handle is the organisation's part of every card link, /p/{handle}/{slug}.
	Handle string `json:"handle"`
	// DefaultQuotaBytes is the storage limit given to new users; nil is unlimited.
	DefaultQuotaBytes *int64    `json:"default_quota_bytes"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	// SuspendedAt is set while a platform admin has suspended the organisation.
	SuspendedAt     *time.Time `json:"-"`
	SuspendedReason *string    `json:"-"`
}

const maxOrgNameLen = 80

// ValidName checks an organisation name: 1-80 characters, no control
// characters (it appears in email subjects and bodies).
func ValidName(name string) bool {
	n := utf8.RuneCountInString(name)
	return n > 0 && n <= maxOrgNameLen && !strings.ContainsFunc(name, unicode.IsControl)
}

// handlePattern is the same as a card slug's: lowercase letters, digits and
// single hyphens.
var handlePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// validHandle checks an organisation's link handle: the same characters as
// a card slug, 3-32 long.
func validHandle(handle string) bool {
	return len(handle) >= MinHandleLen && len(handle) <= MaxHandleLen && handlePattern.MatchString(handle)
}
