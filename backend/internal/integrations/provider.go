package integrations

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

// Provider is one outside service. Only Manifest and Validate are required;
// what else it can do is expressed by the optional interfaces below.
type Provider interface {
	Manifest() Manifest
	// Validate checks a connection's settings beyond what the manifest's
	// fields already enforce (required, type, options). Return a
	// *FieldError to point at the field at fault.
	Validate(ctx context.Context, cfg Settings) error
}

// Settings is a connection's configuration as a provider sees it.
type Settings struct {
	// Values holds the non-secret fields.
	Values map[string]any
	// Secrets holds the secret fields, decrypted.
	Secrets map[string]string
	// AllowPrivate is set in development, where providers may call private
	// addresses (a webhook receiver on localhost).
	AllowPrivate bool
}

// String returns a text setting, or "".
func (s Settings) String(key string) string {
	v, _ := s.Values[key].(string)
	return v
}

// Bool returns a boolean setting, or false.
func (s Settings) Bool(key string) bool {
	v, _ := s.Values[key].(bool)
	return v
}

// Call is what a provider gets when it does work for a connection.
type Call struct {
	Connection *Connection
	Settings   Settings
	// HTTP is the client to reach the provider with. It refuses private
	// addresses (outside development) and, for OAuth providers, signs
	// requests with the connection's token and refreshes it as needed.
	HTTP *http.Client
	// PublicURL is PUBLIC_URL, or "".
	PublicURL string
}

// Result is what a provider reports about work it did, for the activity log.
type Result struct {
	// Summary is one line for people, e.g. "Created contact ada@example.com".
	Summary string
	// Detail is shown when someone expands the entry, e.g. the HTTP status.
	Detail map[string]any
}

// LeadPusher sends leads somewhere (lead sync). PushLead may be called more
// than once for the same lead (retries are at least once), so it should be
// idempotent, for example by upserting on the email address. Return an
// error wrapped with jobs.Permanent when retrying can't help (bad
// credentials, a rejected payload).
type LeadPusher interface {
	PushLead(ctx context.Context, call *Call, lead *Lead) (Result, error)
}

// Tester checks a connection works, for the "Test" button of providers that
// don't push leads.
type Tester interface {
	Test(ctx context.Context, call *Call) (Result, error)
}

// OAuthSpec is how to authorise with a provider using the organisation's
// own OAuth app, whose client ID and secret are the connection's
// "client_id" and "client_secret" fields.
type OAuthSpec struct {
	AuthURL  string
	TokenURL string
	Scopes   []string
	// AuthStyle is how the client secret is sent to TokenURL; zero lets
	// the library work it out.
	AuthStyle oauth2.AuthStyle
	// PKCE adds a code challenge, for providers that support it.
	PKCE bool
	// AuthParams are extra query parameters for AuthURL.
	AuthParams map[string]string
}

// OAuthProvider is implemented by providers with Auth: AuthOAuth2.
type OAuthProvider interface {
	OAuth(cfg Settings) OAuthSpec
}

// Lead is the lead a LeadPusher sends: version 1 of the payload, which the
// webhook provider sends as is. Add fields; never rename or remove them.
type Lead struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	// Phone is E.164 ("+919876543210"), or "".
	Phone string `json:"phone"`
	Notes string `json:"notes"`
	// Source is how the visitor reached the card: nfc, qr, link or "".
	Source string   `json:"source"`
	Card   LeadCard `json:"card"`
	// Owner held the card when the lead arrived; nil means the organisation did.
	Owner        *LeadOwner `json:"owner"`
	Organisation LeadOrg    `json:"organisation"`
}

// LeadCard is the card a lead came in through.
type LeadCard struct {
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
	// Name is the name on the card.
	Name string `json:"name"`
	// URL is the card's public link; empty when PUBLIC_URL isn't set.
	URL string `json:"url"`
}

// LeadOwner is the person who held the card.
type LeadOwner struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// LeadOrg is the organisation the lead belongs to.
type LeadOrg struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Handle string `json:"handle"`
}

// FieldError is a validation error about one setting.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Message }

// fieldErr is a shorthand for providers and the core.
func fieldErr(field, msg string) *FieldError { return &FieldError{Field: field, Message: msg} }

// NewFieldError returns a validation error about one setting.
func NewFieldError(field, msg string) error { return fieldErr(field, msg) }
