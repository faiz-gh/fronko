package auth

import (
	"fmt"
	"net/http"
)

// Scope limits a query to what a signed-in user may see: everything in their
// organisation for owners and admins, or only their own things for members.
// Team leads also see the cards and leads of the people in the teams they lead.
type Scope struct {
	OrgID  int64
	UserID int64
	Admin  bool
}

// MemberID is the user id to filter by for members, or 0 for admins (no filter).
func (s Scope) MemberID() int64 {
	if s.Admin {
		return 0
	}
	return s.UserID
}

// ScopeOf is what the signed-in user may see: their whole organisation for
// owners and admins, only their own things for members.
func ScopeOf(r *http.Request) Scope {
	p := PrincipalFrom(r.Context())
	return Scope{OrgID: p.OrgID, UserID: p.UserID, Admin: p.IsAdmin()}
}

// VisibleTo is a SQL condition on a user id column: true when the cards and
// leads that user holds are visible to the user in parameter $n (MemberID).
// That's everyone for admins ($n = 0); otherwise themselves and the members of
// the teams they lead.
func VisibleTo(col string, n int) string {
	return fmt.Sprintf(`($%[1]d::bigint = 0 OR %[2]s = $%[1]d OR %[2]s IN (
		SELECT tm.user_id FROM team_members tm
		JOIN team_members ld ON ld.team_id = tm.team_id AND ld.role = 'lead'
		WHERE ld.user_id = $%[1]d))`, n, col)
}
