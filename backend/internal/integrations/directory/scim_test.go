package directory

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFilter(t *testing.T) {
	f, err := parseFilter(`userName eq "ada@example.com"`, userFilters)
	require.Nil(t, err)
	assert.Equal(t, &filter{attr: "username", value: "ada@example.com"}, f)

	f, err = parseFilter(`urn:ietf:params:scim:schemas:core:2.0:User:externalId EQ "a \"quoted\" id"`, userFilters)
	require.Nil(t, err)
	assert.Equal(t, &filter{attr: "externalid", value: `a "quoted" id`}, f)

	f, err = parseFilter("", userFilters)
	assert.Nil(t, err)
	assert.Nil(t, f)

	_, err = parseFilter(`userName co "ada"`, userFilters)
	require.NotNil(t, err)
	assert.Equal(t, "invalidFilter", err.ScimType)
	_, err = parseFilter(`title eq "CEO"`, userFilters)
	require.NotNil(t, err)
	_, err = parseFilter(`displayName eq "Sales"`, groupFilters)
	assert.Nil(t, err)
}

func ops(t *testing.T, raw string) []patchOp {
	t.Helper()
	var req patchRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &req))
	var out []patchOp
	for _, op := range req.Operations {
		expanded, err := op.expand()
		require.Nil(t, err)
		out = append(out, expanded...)
	}
	return out
}

// The requests below are the shapes Microsoft Entra ID sends.
func TestApplyEntraUserPatch(t *testing.T) {
	email, full := "ada@example.com", "Ada Lovelace"
	d := draftOf(&person{Email: &email, FullName: &full, Username: "ada"})
	for _, op := range ops(t, `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[
		{"op":"Replace","path":"active","value":"False"},
		{"op":"Replace","path":"emails[type eq \"work\"].value","value":"ada.king@example.com"},
		{"op":"Replace","path":"name.familyName","value":"King"},
		{"op":"Add","path":"externalId","value":"0d1c2b3a"},
		{"op":"Replace","path":"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User:department","value":"R&D"},
		{"op":"Replace","path":"title","value":"Countess"}
	]}`) {
		require.Nil(t, applyUserOp(d, op))
	}
	assert.False(t, d.active, `"False" as a string deactivates`)
	assert.Equal(t, "ada.king@example.com", d.email)
	assert.Equal(t, "0d1c2b3a", *d.externalID)
	name, err := d.fullName()
	require.Nil(t, err)
	assert.Equal(t, "Ada King", *name, "the given name is kept, the family name replaced")
}

func TestPatchWithoutPath(t *testing.T) {
	d := draftOf(&person{Username: "bob"})
	for _, op := range ops(t, `{"Operations":[{"op":"replace","value":{"active":true,"displayName":"Bob B","name":{"givenName":"Robert"}}}]}`) {
		require.Nil(t, applyUserOp(d, op))
	}
	assert.True(t, d.active)
	name, _ := d.fullName()
	assert.Equal(t, "Bob B", *name, "a display name in the request wins")

	_, err := patchOp{Op: "move", Path: "active"}.kind()
	assert.NotNil(t, err)
}

func TestDraftFromPost(t *testing.T) {
	var in userIn
	require.NoError(t, json.Unmarshal([]byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],
		"externalId":"x1","userName":"ada@contoso.com","active":true,"displayName":"Ada Lovelace",
		"emails":[{"primary":true,"type":"work","value":"ada@contoso.com"}],
		"name":{"formatted":"Ada Lovelace","familyName":"Lovelace","givenName":"Ada"}}`), &in))
	d := draftFrom(in)
	assert.Equal(t, "ada@contoso.com", d.email)
	name, _ := d.fullName()
	assert.Equal(t, "Ada Lovelace", *name)
	assert.True(t, d.active)
}

func TestMemberFilterAndRefs(t *testing.T) {
	id, ok := memberFilter(`members[value eq "42"]`)
	assert.True(t, ok)
	assert.Equal(t, "42", id)
	_, ok = memberFilter("members")
	assert.False(t, ok)
	assert.Equal(t, []int64{4, 5}, refIDs([]ref{{Value: "4"}, {Value: "nope"}, {Value: "5"}, {Value: "-1"}}))
}

func TestPrimaryEmail(t *testing.T) {
	assert.Equal(t, "b@x.com", primaryEmail([]emailAttr{{Value: "a@x.com", Type: "home"}, {Value: "b@x.com", Primary: true}}))
	assert.Equal(t, "w@x.com", primaryEmail([]emailAttr{{Value: "h@x.com", Type: "home"}, {Value: "w@x.com", Type: "work"}}))
	assert.Equal(t, "", primaryEmail(nil))
}
