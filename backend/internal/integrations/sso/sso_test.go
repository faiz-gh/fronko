package sso

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/faiz-gh/fronko/backend/internal/integrations"
)

func TestNormalizeDomain(t *testing.T) {
	for in, want := range map[string]string{
		"Example.COM":             "example.com",
		"ada@acme.co.uk":          "acme.co.uk",
		"https://www.acme.io/x":   "acme.io",
		"mail.acme-corp.example.": "mail.acme-corp.example",
	} {
		got, ok := normalizeDomain(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
	for _, bad := range []string{"", "localhost", "-acme.com", "acme..com", "acme.c", "acme .com"} {
		_, ok := normalizeDomain(bad)
		assert.False(t, ok, bad)
	}
}

func TestSafeReturn(t *testing.T) {
	assert.Equal(t, "/dashboard/leads?x=1", safeReturn("/dashboard/leads?x=1"))
	for _, bad := range []string{"", "https://evil.example", "//evil.example", "/\\evil.example", "dashboard"} {
		assert.Equal(t, homePath, safeReturn(bad), bad)
	}
}

func TestStateCookieIsSignedAndExpires(t *testing.T) {
	h := &Handler{stateKey: []byte("k1")}
	raw := h.sign(signInState{ConnectionID: 4, RequestID: "id-1", ReturnTo: "/dashboard", Expires: time.Now().Add(time.Minute).Unix()})
	st, ok := h.parse(raw)
	require.True(t, ok)
	assert.Equal(t, "id-1", st.RequestID)

	_, ok = (&Handler{stateKey: []byte("k2")}).parse(raw)
	assert.False(t, ok, "another key")
	body, sig, _ := strings.Cut(raw, ".")
	_, ok = h.parse(body + "x." + sig)
	assert.False(t, ok, "tampered")
	_, ok = h.parse(h.sign(signInState{Expires: time.Now().Add(-time.Second).Unix()}))
	assert.False(t, ok, "expired")
}

func assertion(nameID string, attrs map[string]string) *saml.Assertion {
	st := saml.AttributeStatement{}
	for k, v := range attrs {
		st.Attributes = append(st.Attributes, saml.Attribute{Name: k, Values: []saml.AttributeValue{{Value: v}}})
	}
	return &saml.Assertion{Subject: &saml.Subject{NameID: &saml.NameID{Value: nameID}},
		AttributeStatements: []saml.AttributeStatement{st}}
}

func TestReadAssertion(t *testing.T) {
	// Entra ID's default claims: the name ID is the UPN, which may not be the mail address.
	entra := readAssertion(assertion("ada@contoso.onmicrosoft.com", map[string]string{
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress": "Ada@Contoso.com",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname":    "Ada",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname":      "Lovelace",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name":         "ada@contoso.onmicrosoft.com",
	}))
	assert.Equal(t, "ada@contoso.com", entra.email)
	assert.Equal(t, "Ada Lovelace", entra.name)

	// Okta with the attribute statements from the setup steps.
	okta := readAssertion(assertion("bob@acme.com", map[string]string{"firstName": "Bob", "lastName": "Ng"}))
	assert.Equal(t, "bob@acme.com", okta.email, "the email name ID is used")
	assert.Equal(t, "Bob Ng", okta.name)

	none := readAssertion(assertion("00u1abc", nil))
	assert.Empty(t, none.email)
}

func TestKeyPairAndEndpoints(t *testing.T) {
	p := All()[0].(*Provider)
	cfg := integrations.Settings{Values: map[string]any{}, Secrets: map[string]string{}}
	require.NoError(t, p.Init(context.Background(), &cfg))
	block, _ := pem.Decode([]byte(cfg.Secrets[secretCert]))
	require.NotNil(t, block)
	key, cert, err := keyPair(cfg)
	require.NoError(t, err)
	assert.True(t, cert.NotAfter.After(time.Now().AddDate(9, 0, 0)))
	assert.Equal(t, 2048, key.N.BitLen())

	endpoints := func(env integrations.Env) map[string]string {
		values := map[string]string{}
		for _, e := range p.Endpoints(&integrations.Connection{ID: 12}, env) {
			values[e.Key] = e.Value
		}
		return values
	}
	same := endpoints(integrations.Env{PublicURL: "https://cards.example.com", APIURL: "https://cards.example.com", OrgHandle: "acme"})
	assert.Equal(t, "https://cards.example.com/auth/saml/12/acs", same["acs_url"])
	assert.Equal(t, "https://cards.example.com/auth/saml/12/metadata", same["entity_id"])
	assert.Equal(t, "https://cards.example.com/login/sso/acme", same["sign_in_url"])

	// With the API on its own domain, the identity provider talks to the API
	// and people are sent to the site.
	split := endpoints(integrations.Env{PublicURL: "https://example.com", APIURL: "https://api.example.com", OrgHandle: "acme"})
	assert.Equal(t, "https://api.example.com/auth/saml/12/acs", split["acs_url"])
	assert.Equal(t, "https://api.example.com/auth/saml/12/metadata", split["metadata_url"])
	assert.Equal(t, "https://example.com/login/sso/acme", split["sign_in_url"])

	assert.Empty(t, p.Endpoints(&integrations.Connection{ID: 12}, integrations.Env{}), "no PUBLIC_URL, no URLs")
}

func TestRedirectsGoToTheSite(t *testing.T) {
	h := &Handler{publicURL: "https://example.com", apiURL: "https://api.example.com"}
	w := httptest.NewRecorder()
	h.fail(w, httptest.NewRequest(http.MethodPost, "https://api.example.com/auth/saml/1/acs", nil), "Nope.")
	assert.Equal(t, "https://example.com/login?sso_error=Nope.", w.Header().Get("Location"))

	h = &Handler{}
	w = httptest.NewRecorder()
	h.fail(w, httptest.NewRequest(http.MethodGet, "/auth/sso/acme", nil), "Nope.")
	assert.Equal(t, "/login?sso_error=Nope.", w.Header().Get("Location"), "relative without PUBLIC_URL")
}

const idpMetadata = `<?xml version="1.0"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="http://idp.example.com/metadata">
  <md:IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="http://idp.example.com/sso"/>
  </md:IDPSSODescriptor>
</md:EntityDescriptor>`

func TestValidateMetadata(t *testing.T) {
	p := All()[2].(*Provider)
	ctx := context.Background()
	err := p.Validate(ctx, integrations.Settings{Values: map[string]any{}})
	var fe *integrations.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "metadata_url", fe.Field)

	assert.NoError(t, p.Validate(ctx, integrations.Settings{Values: map[string]any{"metadata_xml": idpMetadata}}))
	assert.NoError(t, p.Validate(ctx, integrations.Settings{Values: map[string]any{"metadata_url": "https://idp.example.com/md"}}))

	err = p.Validate(ctx, integrations.Settings{Values: map[string]any{"metadata_xml": "<html>nope</html>"}})
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "metadata_xml", fe.Field)

	sp := strings.Replace(idpMetadata, "IDPSSODescriptor", "SPSSODescriptor", 2)
	assert.Error(t, p.Validate(ctx, integrations.Settings{Values: map[string]any{"metadata_xml": sp}}))

	md, err := parseIDPMetadata([]byte(idpMetadata))
	require.NoError(t, err)
	assert.Equal(t, "http://idp.example.com/sso", ssoLocation(md))
}
