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
