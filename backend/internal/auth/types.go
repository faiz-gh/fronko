package auth

// Roles within an organisation. The owner registered it; admins manage it
// with the owner; members only work on the cards assigned to them.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// Roles within a team. Leads look after the team's files and see their
// teammates' cards and leads; members see the team's files.
const (
	TeamRoleLead   = "lead"
	TeamRoleMember = "member"
)

// TeamRef names a team alongside someone's role in it (or a file in it).
type TeamRef struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Role  string `json:"role,omitempty"`
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
