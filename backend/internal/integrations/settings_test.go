package integrations

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testManifest = Manifest{
	ID: "test", Name: "Test", Category: CategoryLeadSync, Description: "d", Scopes: []Scope{ScopeOrg},
	Auth: AuthNone, Status: Available,
	Fields: []Field{
		{Key: "url", Label: "URL", Type: FieldURL, Required: true},
		{Key: "token", Label: "Token", Type: FieldSecret, Required: true},
		{Key: "mode", Label: "Mode", Type: FieldSelect, Options: []Option{{"a", "A"}, {"b", "B"}}, Default: "a"},
		{Key: "notify", Label: "Notify", Type: FieldBool},
		{Key: "note", Label: "Note", Type: FieldTextarea},
		{Key: "label", Label: "Label", Type: FieldText},
	},
}

func fieldOf(t *testing.T, err error) string {
	t.Helper()
	var fe *FieldError
	require.True(t, errors.As(err, &fe), "want a FieldError, got %v", err)
	return fe.Field
}

func TestMergeSettingsNewConnection(t *testing.T) {
	s, err := mergeSettings(testManifest, Settings{}, map[string]any{"url": " https://example.com/hook ", "notify": true},
		map[string]string{"token": " abc "}, true)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"url": "https://example.com/hook", "notify": true, "mode": "a"}, s.Values)
	assert.Equal(t, map[string]string{"token": "abc"}, s.Secrets)
	assert.Empty(t, missingFields(testManifest, s))
}

func TestMergeSettingsKeepsAndClears(t *testing.T) {
	current := Settings{Values: map[string]any{"url": "https://example.com/a", "label": "x"}, Secrets: map[string]string{"token": "old"}}
	s, err := mergeSettings(testManifest, current, map[string]any{"label": ""}, map[string]string{"token": ""}, false)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"url": "https://example.com/a"}, s.Values, "an empty value clears the setting")
	assert.Equal(t, "old", s.Secrets["token"], "an empty secret keeps the saved one")
	assert.Equal(t, "x", current.Values["label"], "the current settings are not modified")
}

func TestMergeSettingsRejects(t *testing.T) {
	cases := map[string]struct {
		values  map[string]any
		secrets map[string]string
		field   string
	}{
		"unknown setting":        {map[string]any{"nope": "x"}, nil, "nope"},
		"secret sent as a value": {map[string]any{"token": "x"}, nil, "token"},
		"value sent as a secret": {nil, map[string]string{"url": "x"}, "url"},
		"plain http":             {map[string]any{"url": "http://example.com"}, nil, "url"},
		"private address":        {map[string]any{"url": "https://192.168.1.10/hook"}, nil, "url"},
		"not a url":              {map[string]any{"url": "example"}, nil, "url"},
		"bad option":             {map[string]any{"mode": "c"}, nil, "mode"},
		"bool as text":           {map[string]any{"notify": "yes"}, nil, "notify"},
		"number as text":         {map[string]any{"label": 5}, nil, "label"},
		"multi-line text":        {map[string]any{"label": "a\nb"}, nil, "label"},
		"multi-line secret":      {nil, map[string]string{"token": "a\nb"}, "token"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := mergeSettings(testManifest, Settings{}, tc.values, tc.secrets, true)
			assert.Equal(t, tc.field, fieldOf(t, err))
		})
	}
}

func TestMergeSettingsAllowsPrivateInDevelopment(t *testing.T) {
	s, err := mergeSettings(testManifest, Settings{AllowPrivate: true}, map[string]any{"url": "http://localhost:9000/hook"}, nil, true)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9000/hook", s.Values["url"])
}

func TestMissingFields(t *testing.T) {
	assert.Equal(t, []string{"URL", "Token"}, missingFields(testManifest, Settings{}))
	assert.Equal(t, []string{"Token"}, missingFields(testManifest, Settings{Values: map[string]any{"url": "https://x.io"}}))
}

type fakeProvider struct {
	m Manifest
}

func (f fakeProvider) Manifest() Manifest                       { return f.m }
func (f fakeProvider) Validate(context.Context, Settings) error { return nil }

func TestRegistryRejectsBadManifests(t *testing.T) {
	good := testManifest
	r := NewRegistry()
	r.Register(fakeProvider{good})
	assert.Panics(t, func() { r.Register(fakeProvider{good}) }, "duplicate id")

	bad := func(edit func(m *Manifest)) {
		m := good
		m.ID = "other"
		m.Fields = append([]Field(nil), good.Fields...)
		edit(&m)
		assert.Panics(t, func() { NewRegistry().Register(fakeProvider{m}) })
	}
	bad(func(m *Manifest) { m.ID = "Bad ID" })
	bad(func(m *Manifest) { m.Category = "crm" })
	bad(func(m *Manifest) { m.Scopes = nil })
	bad(func(m *Manifest) { m.Fields = append(m.Fields, Field{Key: "url", Label: "Again", Type: FieldText}) })
	bad(func(m *Manifest) { m.Fields = append(m.Fields, Field{Key: "pick", Label: "Pick", Type: FieldSelect}) })
	bad(func(m *Manifest) { m.Auth = AuthOAuth2 }) // no OAuthProvider, no client fields

	// Placeholders only need the catalog fields.
	NewRegistry().Register(Placeholder{M: Manifest{ID: "soon", Name: "Soon", Description: "d", Category: CategorySSO,
		Scopes: []Scope{ScopeOrg}, Auth: AuthSAML}})
}

func TestOAuthStateIsSignedAndExpires(t *testing.T) {
	s := &Service{oauthKey: []byte("key")}
	raw, err := s.signState(oauthState{ConnectionID: 7, UserID: 3, Nonce: "n", Expires: 1 << 40})
	require.NoError(t, err)
	st, err := s.parseState(raw)
	require.NoError(t, err)
	assert.Equal(t, int64(7), st.ConnectionID)

	tampered := []byte(raw)
	tampered[2] ^= 1
	_, err = s.parseState(string(tampered))
	assert.ErrorIs(t, err, ErrOAuthState)
	_, err = (&Service{oauthKey: []byte("other")}).parseState(raw)
	assert.ErrorIs(t, err, ErrOAuthState)

	expired, err := s.signState(oauthState{ConnectionID: 7, Expires: 1})
	require.NoError(t, err)
	_, err = s.parseState(expired)
	assert.ErrorIs(t, err, ErrOAuthState)
}

func TestDescribeError(t *testing.T) {
	assert.Equal(t, "Ünïcode first", describeError(errors.New("ünïcode first")))
	assert.Equal(t, "The request timed out", describeError(context.DeadlineExceeded))
	detailed := WithDetail(errors.New("x"), map[string]any{"status": 500})
	assert.Equal(t, map[string]any{"status": 500}, errorDetail(detailed))
}
