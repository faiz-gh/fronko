package files

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/faiz-gh/fronko/backend/internal/auth"
)

func TestUploadArea(t *testing.T) {
	member := auth.Principal{Role: auth.RoleMember, Teams: []auth.TeamRef{{ID: 7, Role: auth.TeamRoleMember}}}
	lead := auth.Principal{Role: auth.RoleMember, Teams: []auth.TeamRef{{ID: 7, Role: auth.TeamRoleLead}}}
	admin := auth.Principal{Role: auth.RoleAdmin}
	cases := []struct {
		name      string
		who       auth.Principal
		requested string
		team      int64
		want      string
		allowed   bool
	}{
		{"member default", member, "", 0, AreaPersonal, true},
		{"member personal", member, AreaPersonal, 0, AreaPersonal, true},
		{"member shared", member, AreaShared, 0, "", false},
		{"member org", member, AreaOrg, 0, "", false},
		{"member own team", member, AreaTeam, 7, "", false},
		{"lead own team", lead, AreaTeam, 7, AreaTeam, true},
		{"lead other team", lead, AreaTeam, 8, "", false},
		{"lead org", lead, AreaOrg, 0, "", false},
		{"lead team without id", lead, AreaTeam, 0, "", false},
		{"admin default", admin, "", 0, AreaOrg, true},
		{"admin shared", admin, AreaShared, 0, AreaShared, true},
		{"admin any team", admin, AreaTeam, 8, AreaTeam, true},
		{"admin personal", admin, AreaPersonal, 0, "", false},
		{"admin bogus", admin, "bogus", 0, "", false},
	}
	for _, c := range cases {
		area, msg := uploadArea(c.who, c.requested, c.team)
		assert.Equal(t, c.allowed, msg == "", c.name)
		assert.Equal(t, c.want, area, c.name)
	}
}

func TestMoveAllowed(t *testing.T) {
	member := auth.Principal{UserID: 1, Role: auth.RoleMember, Teams: []auth.TeamRef{{ID: 7, Role: auth.TeamRoleMember}}}
	lead := auth.Principal{UserID: 2, Role: auth.RoleMember, Teams: []auth.TeamRef{{ID: 7, Role: auth.TeamRoleLead}}}
	admin := auth.Principal{UserID: 3, Role: auth.RoleAdmin}
	own := &File{UserID: 2, Area: AreaPersonal}
	others := &File{UserID: 1, Area: AreaPersonal}
	cases := []struct {
		name    string
		who     auth.Principal
		file    *File
		area    string
		team    int64
		allowed bool
	}{
		{"member into team", member, others, AreaTeam, 7, false},
		{"lead own file into led team", lead, own, AreaTeam, 7, true},
		{"lead into other team", lead, own, AreaTeam, 8, false},
		{"lead someone else's personal file", lead, others, AreaTeam, 7, false},
		{"lead into shared", lead, own, AreaShared, 0, false},
		{"admin into shared", admin, others, AreaShared, 0, true},
		{"admin into any team", admin, others, AreaTeam, 8, true},
		{"admin into team without id", admin, others, AreaTeam, 0, false},
		{"nobody into personal", admin, own, AreaPersonal, 0, false},
	}
	for _, c := range cases {
		assert.Equal(t, c.allowed, moveAllowed(c.who, c.file, c.area, c.team) == "", c.name)
	}
}
