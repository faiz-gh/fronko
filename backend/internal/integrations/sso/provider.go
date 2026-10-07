package sso

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/crewjam/saml"
	xrv "github.com/mattermost/xml-roundtrip-validator"

	"github.com/faiz-gh/fronko/backend/internal/integrations"
)

// Internal secrets of a SAML connection: Fronko's own key pair, made when
// the connection is created. The certificate is in the SP metadata.
const (
	secretKey  = "sp_key"
	secretCert = "sp_cert"
)

// maxMetadataLen allows for identity providers that list many certificates.
const maxMetadataLen = 100_000

// Provider is a SAML 2.0 identity provider. Okta and Microsoft Entra ID are
// the generic provider with their own wording and setup steps.
type Provider struct{ m integrations.Manifest }

func (p *Provider) Manifest() integrations.Manifest { return p.m }

type preset struct {
	id, name, description, urlLabel, urlHelp string
	steps                                    []string
	keywords                                 []string
}

func newProvider(ps preset) *Provider {
	return &Provider{m: integrations.Manifest{
		ID:          ps.id,
		Name:        ps.name,
		Category:    integrations.CategorySSO,
		Description: ps.description,
		Scopes:      []integrations.Scope{integrations.ScopeOrg},
		Auth:        integrations.AuthSAML,
		Status:      integrations.Available,
		Fields: []integrations.Field{
			{Key: "metadata_url", Label: ps.urlLabel, Type: integrations.FieldURL, Help: ps.urlHelp,
				Placeholder: "https://…"},
			{Key: "metadata_xml", Label: "Or paste the metadata XML", Type: integrations.FieldTextarea,
				MaxLength: maxMetadataLen,
				Help:      "Only if your identity provider doesn't publish a metadata URL. The URL is preferred: certificates then renew by themselves."},
			{Key: "jit", Label: "Create accounts on first sign-in", Type: integrations.FieldBool, Default: true,
				Help: "People your identity provider lets in get a Fronko account (as members) the first time they sign in."},
			{Key: "enforce", Label: "Require single sign-on", Type: integrations.FieldBool, Default: false,
				Help: "Everyone signs in through your identity provider; passwords stop working. The owner can still use their password, so they can fix things if sign-in breaks."},
			{Key: "idp_initiated", Label: "Allow sign-in from the identity provider's app launcher", Type: integrations.FieldBool, Default: false,
				Help: "Lets people start from their app dashboard (IdP-initiated). Less safe than starting at Fronko; leave it off unless you need it."},
		},
		SetupSteps: ps.steps,
		DocsURL:    fmt.Sprintf("https://github.com/faiz-gh/fronko/blob/master/docs/integrations/%s.md", ps.id),
		Requires:   []integrations.Requirement{integrations.RequiresPublicURL, integrations.RequiresSecretsKey},
		Keywords:   append(ps.keywords, "saml", "sso", "single sign-on", "login"),
	}}
}

// All returns the SAML providers in catalog order.
func All() []integrations.Provider {
	return []integrations.Provider{
		newProvider(preset{
			id: "okta-saml", name: "Okta",
			description: "Sign in to Fronko with Okta, and create accounts for new people as they arrive.",
			urlLabel:    "Okta metadata URL",
			urlHelp:     "In the app's Sign On tab, under SAML 2.0, copy the Metadata URL.",
			steps: []string{
				"Save this connection to get Fronko's sign-in URLs (shown below).",
				"In the Okta admin console, go to Applications → Create App Integration → SAML 2.0, and name it Fronko.",
				"Set Single sign-on URL to the ACS URL and Audience URI (SP Entity ID) to the Entity ID shown here. Name ID format: EmailAddress; Application username: Email.",
				"Add attribute statements email → user.email, firstName → user.firstName and lastName → user.lastName.",
				"Finish, then copy the Metadata URL from the Sign On tab into the field below and save.",
				"Assign people or groups to the app, then try signing in with SSO. Verify your email domain so people can sign in with just their email.",
			},
		}),
		newProvider(preset{
			id: "entra-saml", name: "Microsoft Entra ID",
			description: "Sign in to Fronko with Microsoft Entra ID (Azure AD), and create accounts for new people as they arrive.",
			urlLabel:    "App Federation Metadata URL",
			urlHelp:     "In the app's Single sign-on page, section SAML Certificates, copy the App Federation Metadata Url.",
			steps: []string{
				"Save this connection to get Fronko's sign-in URLs (shown below).",
				"In the Microsoft Entra admin center, go to Enterprise applications → New application → Create your own application (non-gallery), named Fronko.",
				"Open Single sign-on → SAML. Under Basic SAML Configuration, set Identifier (Entity ID) and Reply URL (ACS URL) to the values shown here, and Sign on URL to the Sign-in URL.",
				"Keep the default claims (emailaddress, givenname, surname); make sure people have a mail address.",
				"Copy the App Federation Metadata Url into the field below and save.",
				"Assign people or groups under Users and groups, then try signing in with SSO. Verify your email domain so people can sign in with just their email.",
			},
			keywords: []string{"azure", "azure ad", "microsoft", "office 365"},
		}),
		newProvider(preset{
			id: "saml", name: "Other SAML provider",
			description: "Sign in to Fronko with any SAML 2.0 identity provider, such as OneLogin, JumpCloud, Google Workspace or ADFS.",
			urlLabel:    "Identity provider metadata URL",
			urlHelp:     "Where your identity provider publishes its SAML metadata.",
			steps: []string{
				"Save this connection to get Fronko's sign-in URLs (shown below), or give your identity provider the SP metadata URL.",
				"In your identity provider, create a SAML 2.0 app with the ACS URL and Entity ID shown here. Send the email address as the Name ID or as an email attribute, and optionally firstName and lastName (or displayName).",
				"Paste its metadata URL (or the metadata XML) below and save.",
				"Assign people to the app, then try signing in with SSO.",
			},
			keywords: []string{"onelogin", "jumpcloud", "adfs", "google", "ping", "keycloak"},
		}),
	}
}

// Validate checks there's a way to get the identity provider's metadata,
// and that pasted metadata is usable.
func (p *Provider) Validate(_ context.Context, cfg integrations.Settings) error {
	xml := strings.TrimSpace(cfg.String("metadata_xml"))
	if cfg.String("metadata_url") == "" && xml == "" {
		return integrations.NewFieldError("metadata_url", "enter the metadata URL, or paste the metadata XML")
	}
	if xml != "" {
		if _, err := parseIDPMetadata([]byte(xml)); err != nil {
			return integrations.NewFieldError("metadata_xml", err.Error())
		}
	}
	return nil
}

// parseIDPMetadata reads identity provider metadata and checks it can be
// used to sign in.
func parseIDPMetadata(data []byte) (*saml.EntityDescriptor, error) {
	md, err := decodeMetadata(data)
	if err != nil {
		return nil, errors.New("the metadata isn't valid SAML metadata XML")
	}
	if len(md.IDPSSODescriptors) == 0 {
		return nil, errors.New("the metadata doesn't describe an identity provider (no IDPSSODescriptor)")
	}
	hasSSO := false
	for _, d := range md.IDPSSODescriptors {
		for _, s := range d.SingleSignOnServices {
			hasSSO = hasSSO || s.Binding == saml.HTTPRedirectBinding || s.Binding == saml.HTTPPostBinding
		}
	}
	if !hasSSO {
		return nil, errors.New("the metadata has no sign-in address Fronko can use (HTTP-Redirect or HTTP-POST)")
	}
	return md, nil
}

// decodeMetadata reads an <EntityDescriptor>, or the identity provider's
// entry in an <EntitiesDescriptor>. XML that wouldn't survive a round trip
// is refused, since signature checks rely on it.
func decodeMetadata(data []byte) (*saml.EntityDescriptor, error) {
	if err := xrv.Validate(bytes.NewReader(data)); err != nil {
		return nil, err
	}
	entity := &saml.EntityDescriptor{}
	err := xml.Unmarshal(data, entity)
	if err == nil {
		return entity, nil
	}
	entities := &saml.EntitiesDescriptor{}
	if xml.Unmarshal(data, entities) != nil {
		return nil, err
	}
	for i, e := range entities.EntityDescriptors {
		if len(e.IDPSSODescriptors) > 0 {
			return &entities.EntityDescriptors[i], nil
		}
	}
	return nil, errors.New("no identity provider in the metadata")
}

// Init makes the connection's own key pair, used to sign requests and to
// receive encrypted assertions.
func (p *Provider) Init(_ context.Context, cfg *integrations.Settings) error {
	key, cert, err := newKeyPair()
	if err != nil {
		return err
	}
	cfg.Secrets[secretKey], cfg.Secrets[secretCert] = key, cert
	return nil
}

func newKeyPair() (keyPEM, certPEM string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		return "", "", err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Fronko SAML service provider", Organization: []string{"Fronko"}},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	return keyPEM, certPEM, nil
}

// Endpoints are the values to enter in the identity provider.
func (p *Provider) Endpoints(c *integrations.Connection, env integrations.Env) []integrations.Endpoint {
	if env.PublicURL == "" {
		return nil
	}
	urls := urlsFor(env.PublicURL, c.ID)
	return []integrations.Endpoint{
		{Key: "acs_url", Label: "ACS URL (Reply URL)", Value: urls.acs,
			Help: "Where your identity provider sends people after they sign in."},
		{Key: "entity_id", Label: "Entity ID (Audience URI / Identifier)", Value: urls.metadata},
		{Key: "metadata_url", Label: "SP metadata URL", Value: urls.metadata,
			Help: "For identity providers that can read Fronko's settings from a URL."},
		{Key: "sign_in_url", Label: "Sign-in URL", Value: env.PublicURL + "/auth/sso/" + env.OrgHandle,
			Help: "Opens your identity provider's sign-in for this organisation. Use it as the app's Sign on URL, or share it."},
	}
}

// Test fetches the identity provider's metadata and reports what it found.
func (p *Provider) Test(ctx context.Context, call *integrations.Call) (integrations.Result, error) {
	md, err := loadMetadata(ctx, call.HTTP, call.Settings)
	if err != nil {
		return integrations.Result{}, err
	}
	detail := map[string]any{"entity_id": md.EntityID}
	if loc := ssoLocation(md); loc != "" {
		detail["sign_in_url"] = loc
	}
	summary := "The identity provider's metadata is valid"
	if exp := certExpiry(md); !exp.IsZero() {
		detail["certificate_expires"] = exp.Format(time.DateOnly)
		if days := int(time.Until(exp).Hours() / 24); days < 30 {
			summary += "; its signing certificate expires in " + strconv.Itoa(max(days, 0)) + " days"
		}
	}
	return integrations.Result{Summary: summary, Detail: detail}, nil
}

func ssoLocation(md *saml.EntityDescriptor) string {
	for _, d := range md.IDPSSODescriptors {
		for _, s := range d.SingleSignOnServices {
			if s.Binding == saml.HTTPRedirectBinding {
				return s.Location
			}
		}
	}
	return ""
}

// certExpiry is when the first signing certificate in the metadata expires.
func certExpiry(md *saml.EntityDescriptor) time.Time {
	for _, d := range md.IDPSSODescriptors {
		for _, kd := range d.KeyDescriptors {
			if kd.Use != "" && kd.Use != "signing" {
				continue
			}
			for _, c := range kd.KeyInfo.X509Data.X509Certificates {
				cert, err := parseCertData(c.Data)
				if err == nil {
					return cert.NotAfter
				}
			}
		}
	}
	return time.Time{}
}
