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
	member := middleware.Principal{Role: models.RoleMember}
	admin := middleware.Principal{Role: models.RoleAdmin}
	cases := []struct {
		who       middleware.Principal
		requested string
		want      string
		allowed   bool
	}{
		{member, "", models.AreaPersonal, true},
		{member, models.AreaPersonal, models.AreaPersonal, true},
		{member, models.AreaShared, "", false},
		{member, models.AreaOrg, "", false},
		{admin, "", models.AreaOrg, true},
		{admin, models.AreaShared, models.AreaShared, true},
		{admin, models.AreaPersonal, "", false},
		{admin, "bogus", "", false},
	}
	for _, c := range cases {
		area, msg := uploadArea(c.who, c.requested)
		assert.Equal(t, c.allowed, msg == "", "%s uploading to %q", c.who.Role, c.requested)
		assert.Equal(t, c.want, area, "%s uploading to %q", c.who.Role, c.requested)
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
