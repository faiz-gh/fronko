package orgs

import (
	"encoding/json"
	"strings"
	"testing"

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

func TestValidOrgName(t *testing.T) {
	assert.True(t, ValidName("Acme Inc."))
	assert.True(t, ValidName("Café Ñandú"))
	assert.False(t, ValidName(""))
	assert.False(t, ValidName("Acme\r\nBcc: x@example.com"))
	assert.False(t, ValidName(strings.Repeat("a", 81)))
}

func TestValidHandle(t *testing.T) {
	for _, h := range []string{"acme", "acme-corp", "a1b", "abcdefghijklmnopqrstuvwxyz123456"} {
		assert.True(t, validHandle(h), h)
	}
	for _, h := range []string{"", "ab", "Acme", "acme_corp", "-acme", "acme-", "ac--me", "abcdefghijklmnopqrstuvwxyz1234567", "acme/john"} {
		assert.False(t, validHandle(h), h)
	}
}
