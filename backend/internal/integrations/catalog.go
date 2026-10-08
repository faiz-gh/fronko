// Package integrations connects Fronko to outside services: CRMs that
// receive leads, booking pages, directories that provision people, and
// single sign-on.
//
// Each service is a Provider, described by a Manifest. The manifest is the
// single source of truth for the catalog the dashboard shows and for the
// settings form it renders, so adding a provider needs no frontend change
// beyond (optionally) a logo. Providers live in integrations/providers/<id>
// and are registered in providers/all.go.
//
// An organisation or a person connects a provider by creating a Connection:
// its settings, its sealed secrets and its health. What providers can do
// beyond the manifest is expressed by optional interfaces (LeadPusher,
// Tester, OAuthProvider) that the core finds by type assertion.
package integrations

import "slices"

// Category groups providers by what they do; each has its own section in
// the dashboard.
type Category string

const (
	CategoryLeadSync  Category = "lead_sync"
	CategoryCalendar  Category = "calendar"
	CategoryDirectory Category = "directory"
	CategorySSO       Category = "sso"
)

// Categories in the order the dashboard shows them.
var Categories = []Category{CategoryLeadSync, CategoryCalendar, CategoryDirectory, CategorySSO}

// Single reports whether an owner may have only one connection in the
// category, whichever provider it's to: one directory and one sign-in
// method per organisation. Booking pages aren't: people connect as many as
// they like and each card picks one.
func (c Category) Single() bool {
	return c == CategoryDirectory || c == CategorySSO
}

// Scope is who a connection belongs to.
type Scope string

const (
	// ScopeOrg connections belong to the organisation; admins manage them.
	ScopeOrg Scope = "org"
	// ScopeUser connections belong to one person, who manages them.
	ScopeUser Scope = "user"
)

// AuthKind is how Fronko authenticates with the provider.
type AuthKind string

const (
	AuthNone   AuthKind = "none"
	AuthAPIKey AuthKind = "api_key"
	// AuthOAuth2 uses the organisation's own OAuth app: its client ID and
	// secret are fields on the connection, and someone authorises it in
	// the browser.
	AuthOAuth2 AuthKind = "oauth2"
	// AuthLink is a link the person pastes, such as a booking page.
	AuthLink AuthKind = "link"
	AuthSAML AuthKind = "saml"
	// AuthSCIMToken means the provider calls Fronko with a token Fronko issued.
	AuthSCIMToken AuthKind = "scim_token"
)

// Availability is how ready a provider is.
type Availability string

const (
	Available  Availability = "available"
	Beta       Availability = "beta"
	ComingSoon Availability = "coming_soon"
)

// Requirement is server configuration a provider needs.
type Requirement string

const (
	// RequiresPublicURL: callbacks or endpoints need PUBLIC_URL.
	RequiresPublicURL Requirement = "public_url"
	// RequiresSecretsKey: secrets are sealed with SECRETS_KEY.
	RequiresSecretsKey Requirement = "secrets_key"
)

// FieldType is how a setting is entered and checked.
type FieldType string

const (
	FieldText     FieldType = "text"
	FieldURL      FieldType = "url"
	FieldSecret   FieldType = "secret"
	FieldSelect   FieldType = "select"
	FieldTextarea FieldType = "textarea"
	FieldBool     FieldType = "bool"
)

// Option is one choice of a select field.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Field is one setting on a connection. Secret fields are sealed and never
// sent back; the API only says whether each is set.
type Field struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required,omitempty"`
	Help        string    `json:"help,omitempty"`
	Placeholder string    `json:"placeholder,omitempty"`
	Options     []Option  `json:"options,omitempty"`
	// Default fills the field on a new connection.
	Default any `json:"default,omitempty"`
	// Generate makes the form offer a random value (for a secret both
	// sides need to know, such as a signing secret).
	Generate bool `json:"generate,omitempty"`
	// MaxLength overrides the default limit for text and textarea fields.
	MaxLength int `json:"max_length,omitempty"`
}

// Manifest describes a provider: what the catalog shows and what a
// connection to it needs.
type Manifest struct {
	// ID is stable and stored on connections: lowercase, digits and dashes.
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    Category `json:"category"`
	Description string   `json:"description"`
	// Scopes says who may connect it; the first is the default.
	Scopes []Scope      `json:"scopes"`
	Auth   AuthKind     `json:"auth"`
	Status Availability `json:"status"`
	// Multiple allows more than one connection per owner.
	Multiple   bool          `json:"multiple"`
	Fields     []Field       `json:"fields"`
	SetupSteps []string      `json:"setup_steps,omitempty"`
	DocsURL    string        `json:"docs_url,omitempty"`
	Requires   []Requirement `json:"requires,omitempty"`
	// Keywords help people find it in search ("crm", "zapier").
	Keywords []string `json:"keywords,omitempty"`
}

// AllowsScope reports whether connections with scope s are allowed.
func (m Manifest) AllowsScope(s Scope) bool { return slices.Contains(m.Scopes, s) }

// Field returns the field with the given key.
func (m Manifest) Field(key string) (Field, bool) {
	for _, f := range m.Fields {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}
