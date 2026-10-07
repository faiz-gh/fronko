package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptionalQuota(t *testing.T) {
	var req struct {
		Quota optionalQuota `json:"quota"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{}`), &req))
	assert.False(t, req.Quota.Set, "absent")

	require.NoError(t, json.Unmarshal([]byte(`{"quota": null}`), &req))
	assert.True(t, req.Quota.Set)
	assert.Nil(t, req.Quota.Value, "null is unlimited")
	assert.True(t, req.Quota.valid())

	require.NoError(t, json.Unmarshal([]byte(`{"quota": 1048576}`), &req))
	require.NotNil(t, req.Quota.Value)
	assert.EqualValues(t, 1<<20, *req.Quota.Value)
	assert.True(t, req.Quota.valid())

	require.NoError(t, json.Unmarshal([]byte(`{"quota": -1}`), &req))
	assert.False(t, req.Quota.valid())
	assert.Error(t, json.Unmarshal([]byte(`{"quota": "big"}`), &req))
}

func TestUploadArea(t *testing.T) {
	member := middleware.Principal{Role: models.RoleMember, Teams: []models.TeamRef{{ID: 7, Role: models.TeamRoleMember}}}
	lead := middleware.Principal{Role: models.RoleMember, Teams: []models.TeamRef{{ID: 7, Role: models.TeamRoleLead}}}
	admin := middleware.Principal{Role: models.RoleAdmin}
	cases := []struct {
		name      string
		who       middleware.Principal
		requested string
		team      int64
		want      string
		allowed   bool
	}{
		{"member default", member, "", 0, models.AreaPersonal, true},
		{"member personal", member, models.AreaPersonal, 0, models.AreaPersonal, true},
		{"member shared", member, models.AreaShared, 0, "", false},
		{"member org", member, models.AreaOrg, 0, "", false},
		{"member own team", member, models.AreaTeam, 7, "", false},
		{"lead own team", lead, models.AreaTeam, 7, models.AreaTeam, true},
		{"lead other team", lead, models.AreaTeam, 8, "", false},
		{"lead org", lead, models.AreaOrg, 0, "", false},
		{"lead team without id", lead, models.AreaTeam, 0, "", false},
		{"admin default", admin, "", 0, models.AreaOrg, true},
		{"admin shared", admin, models.AreaShared, 0, models.AreaShared, true},
		{"admin any team", admin, models.AreaTeam, 8, models.AreaTeam, true},
		{"admin personal", admin, models.AreaPersonal, 0, "", false},
		{"admin bogus", admin, "bogus", 0, "", false},
	}
	for _, c := range cases {
		area, msg := uploadArea(c.who, c.requested, c.team)
		assert.Equal(t, c.allowed, msg == "", c.name)
		assert.Equal(t, c.want, area, c.name)
	}
}

func TestMoveAllowed(t *testing.T) {
	member := middleware.Principal{UserID: 1, Role: models.RoleMember, Teams: []models.TeamRef{{ID: 7, Role: models.TeamRoleMember}}}
	lead := middleware.Principal{UserID: 2, Role: models.RoleMember, Teams: []models.TeamRef{{ID: 7, Role: models.TeamRoleLead}}}
	admin := middleware.Principal{UserID: 3, Role: models.RoleAdmin}
	own := &models.File{UserID: 2, Area: models.AreaPersonal}
	others := &models.File{UserID: 1, Area: models.AreaPersonal}
	cases := []struct {
		name    string
		who     middleware.Principal
		file    *models.File
		area    string
		team    int64
		allowed bool
	}{
		{"member into team", member, others, models.AreaTeam, 7, false},
		{"lead own file into led team", lead, own, models.AreaTeam, 7, true},
		{"lead into other team", lead, own, models.AreaTeam, 8, false},
		{"lead someone else's personal file", lead, others, models.AreaTeam, 7, false},
		{"lead into shared", lead, own, models.AreaShared, 0, false},
		{"admin into shared", admin, others, models.AreaShared, 0, true},
		{"admin into any team", admin, others, models.AreaTeam, 8, true},
		{"admin into team without id", admin, others, models.AreaTeam, 0, false},
		{"nobody into personal", admin, own, models.AreaPersonal, 0, false},
	}
	for _, c := range cases {
		assert.Equal(t, c.allowed, moveAllowed(c.who, c.file, c.area, c.team) == "", c.name)
	}
}

func TestValidOrgName(t *testing.T) {
	assert.True(t, validOrgName("Acme Inc."))
	assert.True(t, validOrgName("Café Ñandú"))
	assert.False(t, validOrgName(""))
	assert.False(t, validOrgName("Acme\r\nBcc: x@example.com"))
	assert.False(t, validOrgName(strings.Repeat("a", 81)))
}

func TestValidateBranding(t *testing.T) {
	ok := func(b models.OrgBranding) models.OrgBranding {
		t.Helper()
		assert.Empty(t, validateBranding(&b))
		return b
	}
	bad := func(b models.OrgBranding) {
		t.Helper()
		assert.NotEmpty(t, validateBranding(&b))
	}

	b := ok(models.OrgBranding{})
	assert.Equal(t, models.LogoOptional, b.LogoPolicy, "defaults to optional")
	blank := " "
	b = ok(models.OrgBranding{LogoFile: &blank, LogoPolicy: models.LogoRequired})
	assert.Nil(t, b.LogoFile, "a blank logo is no logo")
	b = ok(models.OrgBranding{Signature: models.OrgSignature{
		LockedTemplate: "corporate", BrandColor: "#1A2b3C", Disclaimer: "  Confidential.  ", BannerURL: "https://acme.test/x",
	}})
	assert.Equal(t, "Confidential.", b.Signature.Disclaimer)

	bad(models.OrgBranding{LogoPolicy: "sometimes"})
	bad(models.OrgBranding{Signature: models.OrgSignature{LockedTemplate: "fancy"}})
	bad(models.OrgBranding{Signature: models.OrgSignature{BrandColor: "red"}})
	bad(models.OrgBranding{Signature: models.OrgSignature{BrandColor: "#fff"}})
	bad(models.OrgBranding{Signature: models.OrgSignature{Disclaimer: strings.Repeat("a", 1001)}})
	bad(models.OrgBranding{Signature: models.OrgSignature{BannerURL: "javascript:alert(1)"}})
	bad(models.OrgBranding{Signature: models.OrgSignature{BannerURL: "acme.test"}})
}

func TestValidHandle(t *testing.T) {
	for _, h := range []string{"acme", "acme-corp", "a1b", "abcdefghijklmnopqrstuvwxyz123456"} {
		assert.True(t, validHandle(h), h)
	}
	for _, h := range []string{"", "ab", "Acme", "acme_corp", "-acme", "acme-", "ac--me", "abcdefghijklmnopqrstuvwxyz1234567", "acme/john"} {
		assert.False(t, validHandle(h), h)
	}
}
