package teams

import (
	"time"

	"github.com/faiz-gh/fronko/backend/internal/auth"
)

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
	auth.UserRef
	Email *string `json:"email"`
	// OrgRole is the person's role in the organisation (owner, admin, member).
	OrgRole string    `json:"org_role"`
	Role    string    `json:"role"`
	AddedAt time.Time `json:"added_at"`
}

// TeamMembership is a person's place in one team, as sent when setting their teams.
type TeamMembership struct {
	TeamID int64  `json:"team_id"`
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
}
