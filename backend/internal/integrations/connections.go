package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/oauth2"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/leads"
	"github.com/faiz-gh/fronko/backend/internal/orgs"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
	"github.com/faiz-gh/fronko/backend/internal/platform/netguard"
	"github.com/faiz-gh/fronko/backend/internal/platform/secrets"
)

// errorThreshold is how many failures in a row (each after its retries)
// put a connection into error.
const errorThreshold = 3

// maxNameLen caps a connection's name.
const maxNameLen = 80

// Errors the handler turns into responses.
var (
	ErrUnknownProvider = errors.New("unknown integration")
	ErrUnavailable     = errors.New("this integration isn't available on this server")
	ErrForbidden       = errors.New("you can't manage this connection")
	ErrOnlyOne         = errors.New("you already have a connection to this integration")
	ErrBadToken        = errors.New("invalid or revoked token")
	ErrNotReady        = errors.New("finish setting up this connection first")
	ErrNoSecretsKey    = errors.New("SECRETS_KEY isn't set on this server, so secrets can't be stored")
)

// LeadReader loads leads for lead sync (the leads store).
type LeadReader interface {
	SyncDetails(ctx context.Context, leadID int64) (*leads.SyncDetails, error)
}

// OrgReader loads organisations (the orgs store).
type OrgReader interface {
	GetOrganization(ctx context.Context, orgID int64) (*orgs.Organization, error)
}

// Options configure the service from the server's settings.
type Options struct {
	// PublicURL is PUBLIC_URL, or "": where people reach the site.
	PublicURL string
	// APIURL is PUBLIC_API_URL, or "": where the API is reached. OAuth
	// callbacks are built on it. Empty means PublicURL.
	APIURL string
	// Box seals secrets; nil when SECRETS_KEY isn't set.
	Box *secrets.Box
	// AllowPrivate lets providers call private addresses (development only).
	AllowPrivate bool
	// HTTP overrides the outbound client (tests).
	HTTP *http.Client
}

// Service manages connections and runs providers for them.
type Service struct {
	store    *Store
	registry *Registry
	leads    LeadReader
	orgs     OrgReader
	opts     Options
	http     *http.Client
	// oauthKey signs OAuth state.
	oauthKey []byte
}

// NewService returns the integrations service. stateKey signs OAuth state;
// derive it from a server secret.
func NewService(store *Store, registry *Registry, leadReader LeadReader, orgReader OrgReader, stateKey []byte, opts Options) *Service {
	client := opts.HTTP
	if client == nil {
		client = netguard.Client(netguard.Options{AllowPrivate: opts.AllowPrivate, Timeout: 20 * time.Second})
	}
	return &Service{store: store, registry: registry, leads: leadReader, orgs: orgReader, opts: opts, http: client, oauthKey: stateKey}
}

// vault is what a connection's sealed secrets hold.
type vault struct {
	Fields map[string]string `json:"fields,omitempty"`
	Token  *oauth2.Token     `json:"token,omitempty"`
}

func (v vault) empty() bool { return len(v.Fields) == 0 && v.Token == nil }

func secretsAAD(id int64) []byte { return fmt.Appendf(nil, "integration_connection:%d", id) }

func (s *Service) open(c *Connection) (vault, error) {
	var v vault
	if len(c.sealed) == 0 {
		return v, nil
	}
	if s.opts.Box == nil {
		return v, ErrNoSecretsKey
	}
	plain, err := s.opts.Box.Open(c.sealed, secretsAAD(c.ID))
	if err != nil {
		return v, fmt.Errorf("opening secrets of connection %d: %w", c.ID, err)
	}
	return v, json.Unmarshal(plain, &v)
}

func (s *Service) seal(id int64, v vault) ([]byte, error) {
	if v.empty() {
		return nil, nil
	}
	if s.opts.Box == nil {
		return nil, ErrNoSecretsKey
	}
	plain, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return s.opts.Box.Seal(plain, secretsAAD(id))
}

// onlyOneError is ErrOnlyOne with a message for the category.
type onlyOneError struct{ msg string }

func (e *onlyOneError) Error() string        { return e.msg }
func (e *onlyOneError) Is(target error) bool { return target == ErrOnlyOne }

var onlyOneMessages = map[Category]map[Scope]string{
	CategoryCalendar: {
		ScopeUser: "you already have a booking page connected; remove it before adding another",
		ScopeOrg:  "your organisation already has a default booking page; remove it before adding another",
	},
	CategoryDirectory: {ScopeOrg: "your organisation already imports people from a directory; remove that connection first"},
	CategorySSO:       {ScopeOrg: "your organisation already has single sign-on set up; remove that connection first"},
}

// Unavailable explains why a provider can't be connected on this server,
// or returns "" when it can.
func (s *Service) Unavailable(m Manifest) string {
	if m.Status == ComingSoon {
		return "Coming soon"
	}
	needsKey := m.Auth == AuthOAuth2
	for _, f := range m.Fields {
		needsKey = needsKey || f.Type == FieldSecret
	}
	for _, r := range m.Requires {
		switch r {
		case RequiresPublicURL:
			if s.opts.PublicURL == "" || s.apiURL() == "" {
				return "Needs FRONTEND_URL (or PUBLIC_URL) to be set on the server"
			}
		case RequiresSecretsKey:
			needsKey = true
		}
	}
	if needsKey && s.opts.Box == nil {
		return "Needs SECRETS_KEY to be set on the server"
	}
	return ""
}

// apiURL is where the API is reached from outside: PUBLIC_API_URL, or
// PUBLIC_URL when the API shares the site's origin.
func (s *Service) apiURL() string {
	if s.opts.APIURL != "" {
		return s.opts.APIURL
	}
	return s.opts.PublicURL
}

// OAuthRedirectURL is the redirect URL to register in an OAuth app. It's on
// the API's address, where the browser's session cookie and the OAuth
// cookie live.
func (s *Service) OAuthRedirectURL() string {
	if s.apiURL() == "" {
		return ""
	}
	return s.apiURL() + oauthCallbackPath
}

// View is a connection as the API shows it: secrets are only reported as
// set or not.
type View struct {
	*Connection
	Scope Scope `json:"scope"`
	// Secrets says which secret fields have a value.
	Secrets map[string]bool `json:"secrets"`
	// Authorized is true once an OAuth connection has a token.
	Authorized bool `json:"authorized"`
	// Missing lists required settings that still need a value.
	Missing []string `json:"missing"`
	// Endpoints are values to enter in the provider's own settings.
	Endpoints []Endpoint `json:"endpoints"`
	// Token describes the token the provider calls Fronko with, for
	// scim_token providers; nil until one is generated.
	Token *TokenInfo `json:"token"`
}

func (s *Service) view(ctx context.Context, c *Connection) (*View, error) {
	v, err := s.open(c)
	if err != nil && !errors.Is(err, ErrNoSecretsKey) {
		return nil, err
	}
	out := &View{Connection: c, Scope: c.Scope(), Secrets: map[string]bool{}, Authorized: v.Token != nil,
		Endpoints: []Endpoint{}, Token: c.token}
	p, ok := s.registry.Get(c.Provider)
	if !ok {
		return out, nil
	}
	m := p.Manifest()
	for _, f := range m.Fields {
		if f.Type == FieldSecret {
			out.Secrets[f.Key] = v.Fields[f.Key] != ""
		}
	}
	out.Missing = missingFields(m, Settings{Values: c.Config, Secrets: v.Fields})
	if out.Missing == nil {
		out.Missing = []string{}
	}
	if d, ok := p.(Describer); ok {
		env, err := s.env(ctx, c.OrgID)
		if err != nil {
			return nil, err
		}
		out.Endpoints = d.Endpoints(c, env)
	}
	return out, nil
}

func (s *Service) env(ctx context.Context, orgID int64) (Env, error) {
	org, err := s.orgs.GetOrganization(ctx, orgID)
	if err != nil {
		return Env{}, err
	}
	return Env{PublicURL: s.opts.PublicURL, APIURL: s.apiURL(), OrgHandle: org.Handle}, nil
}

// ready reports whether a connection has everything it needs to work.
func ready(m Manifest, settings Settings, v vault, c *Connection) bool {
	return len(missingFields(m, settings)) == 0 &&
		(m.Auth != AuthOAuth2 || v.Token != nil) &&
		(m.Auth != AuthSCIMToken || c.token != nil)
}

// canManage reports whether the user may see and change a connection:
// admins manage the organisation's, people their own.
func canManage(p auth.Principal, c *Connection) bool {
	if c.UserID != 0 {
		return c.UserID == p.UserID
	}
	return p.IsAdmin()
}

// List returns the connections a user manages: the organisation's (admins
// only) and/or their own.
func (s *Service) List(ctx context.Context, p auth.Principal, scope Scope, provider string) ([]*View, error) {
	f := ConnectionFilter{Provider: provider}
	switch scope {
	case ScopeOrg:
		if !p.IsAdmin() {
			return nil, ErrForbidden
		}
		f.Org = true
	case ScopeUser:
		f.UserID = p.UserID
	default:
		f.Org = p.IsAdmin()
		f.UserID = p.UserID
	}
	conns, err := s.store.ListConnections(ctx, p.OrgID, f)
	if err != nil {
		return nil, err
	}
	out := make([]*View, 0, len(conns))
	for _, c := range conns {
		v, err := s.view(ctx, c)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// get loads a connection the user manages; others look missing.
func (s *Service) get(ctx context.Context, p auth.Principal, id int64) (*Connection, Provider, error) {
	c, err := s.store.GetConnection(ctx, p.OrgID, id)
	if err != nil {
		return nil, nil, err
	}
	if !canManage(p, c) {
		return nil, nil, database.ErrNotFound
	}
	prov, ok := s.registry.Get(c.Provider)
	if !ok {
		return nil, nil, ErrUnknownProvider
	}
	return c, prov, nil
}

// Get returns one connection the user manages.
func (s *Service) Get(ctx context.Context, p auth.Principal, id int64) (*View, error) {
	c, _, err := s.get(ctx, p, id)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, c)
}

// CreateInput is a new connection.
type CreateInput struct {
	Provider string            `json:"provider"`
	Scope    Scope             `json:"scope"`
	Name     string            `json:"name"`
	Values   map[string]any    `json:"config"`
	Secrets  map[string]string `json:"secrets"`
}

// Create adds a connection. It starts pending until every required
// setting is filled in (and, for OAuth, it is authorised).
func (s *Service) Create(ctx context.Context, p auth.Principal, in CreateInput) (*View, error) {
	prov, ok := s.registry.Get(in.Provider)
	if !ok {
		return nil, ErrUnknownProvider
	}
	m := prov.Manifest()
	if s.Unavailable(m) != "" {
		return nil, ErrUnavailable
	}
	if in.Scope == "" {
		in.Scope = m.Scopes[0]
	}
	if !m.AllowsScope(in.Scope) {
		return nil, userErr(fmt.Sprintf("%s can't be connected for %s", m.Name, map[Scope]string{ScopeOrg: "the organisation", ScopeUser: "one person"}[in.Scope]))
	}
	c := &Connection{
		OrgID:     p.OrgID,
		Provider:  m.ID,
		Category:  m.Category,
		Enabled:   true,
		CreatedBy: &auth.UserRef{ID: p.UserID},
	}
	if in.Scope == ScopeUser {
		c.UserID = p.UserID
	} else if !p.IsAdmin() {
		return nil, ErrForbidden
	}
	if m.Category.Single() {
		n, err := s.store.CountInCategory(ctx, p.OrgID, c.UserID, m.Category)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return nil, &onlyOneError{msg: onlyOneMessages[m.Category][in.Scope]}
		}
	} else if !m.Multiple {
		n, err := s.store.CountConnections(ctx, p.OrgID, c.UserID, m.ID)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return nil, ErrOnlyOne
		}
	}
	name, err := cleanName(in.Name, m.Name)
	if err != nil {
		return nil, err
	}
	c.Name = name

	settings, err := mergeSettings(m, Settings{AllowPrivate: s.opts.AllowPrivate}, in.Values, in.Secrets, true)
	if err != nil {
		return nil, err
	}
	if err := prov.Validate(ctx, settings); err != nil {
		return nil, asUserError(err)
	}
	if init, ok := prov.(Initializer); ok {
		if err := init.Init(ctx, &settings); err != nil {
			return nil, err
		}
	}
	v := vault{Fields: settings.Secrets}
	c.Config = settings.Values
	c.Status = StatusPending
	if ready(m, settings, v, c) {
		c.Status = StatusActive
	}
	if err := s.store.CreateConnection(ctx, c, func(id int64) ([]byte, error) { return s.seal(id, v) }); err != nil {
		return nil, err
	}
	s.logActivity(ctx, c.ID, &Activity{Kind: ActivitySetup, Outcome: OutcomeSuccess, Summary: "Connection created", userID: p.UserID})
	return s.Get(ctx, p, c.ID)
}

// UpdateInput changes a connection. Nil fields are left as they are; only
// the settings and secrets present are changed.
type UpdateInput struct {
	Name    *string           `json:"name"`
	Enabled *bool             `json:"enabled"`
	Values  map[string]any    `json:"config"`
	Secrets map[string]string `json:"secrets"`
}

// Update changes a connection. Saving settings gives a connection in error
// a fresh start.
func (s *Service) Update(ctx context.Context, p auth.Principal, id int64, in UpdateInput) (*View, error) {
	c, prov, err := s.get(ctx, p, id)
	if err != nil {
		return nil, err
	}
	m := prov.Manifest()
	v, err := s.open(c)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		if c.Name, err = cleanName(*in.Name, m.Name); err != nil {
			return nil, err
		}
	}
	settingsChanged := len(in.Values) > 0 || len(in.Secrets) > 0
	if settingsChanged {
		current := Settings{Values: c.Config, Secrets: v.Fields, AllowPrivate: s.opts.AllowPrivate}
		settings, err := mergeSettings(m, current, in.Values, in.Secrets, false)
		if err != nil {
			return nil, err
		}
		if err := prov.Validate(ctx, settings); err != nil {
			return nil, asUserError(err)
		}
		// New app credentials invalidate the token issued to the old ones.
		if m.Auth == AuthOAuth2 && (settings.String("client_id") != current.String("client_id") ||
			settings.Secrets["client_secret"] != current.Secrets["client_secret"]) {
			v.Token = nil
		}
		c.Config = settings.Values
		v.Fields = settings.Secrets
		if c.sealed, err = s.seal(c.ID, v); err != nil {
			return nil, err
		}
	}
	reenabled := in.Enabled != nil && *in.Enabled && !c.Enabled
	if in.Enabled != nil {
		c.Enabled = *in.Enabled
	}
	switch {
	case !ready(m, Settings{Values: c.Config, Secrets: v.Fields}, v, c):
		c.Status = StatusPending
	case c.Status == StatusPending || settingsChanged || reenabled:
		c.Status, c.FailureCount, c.LastError, c.LastErrorAt = StatusActive, 0, nil, nil
	}
	if err := s.store.UpdateConnection(ctx, c); err != nil {
		return nil, err
	}
	if settingsChanged {
		s.logActivity(ctx, c.ID, &Activity{Kind: ActivitySetup, Outcome: OutcomeSuccess, Summary: "Settings changed", userID: p.UserID})
	}
	return s.view(ctx, c)
}

// Delete removes a connection and its log.
func (s *Service) Delete(ctx context.Context, p auth.Principal, id int64) error {
	c, _, err := s.get(ctx, p, id)
	if err != nil {
		return err
	}
	return s.store.DeleteConnection(ctx, p.OrgID, c.ID)
}

// Activity returns a page of a connection's log.
func (s *Service) Activity(ctx context.Context, p auth.Principal, id, beforeID int64, limit int) ([]*Activity, error) {
	c, _, err := s.get(ctx, p, id)
	if err != nil {
		return nil, err
	}
	return s.store.ListActivity(ctx, c.ID, beforeID, limit)
}

// TestResult is the outcome of a test.
type TestResult struct {
	OK      bool           `json:"ok"`
	Summary string         `json:"summary"`
	Detail  map[string]any `json:"detail,omitempty"`
}

// Test checks a connection works: lead sync providers receive a sample
// lead; others run their own check.
func (s *Service) Test(ctx context.Context, p auth.Principal, id int64) (*TestResult, error) {
	c, prov, err := s.get(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if c.Status == StatusPending {
		return nil, ErrNotReady
	}
	call, err := s.newCall(ctx, c, prov)
	if err != nil {
		return nil, err
	}
	var res Result
	switch impl := prov.(type) {
	case LeadPusher:
		lead, err := s.sampleLead(ctx, c)
		if err != nil {
			return nil, err
		}
		res, err = impl.PushLead(ctx, call, lead)
		if err != nil {
			return s.testFailed(ctx, c, p, err), nil
		}
	case Tester:
		res, err = impl.Test(ctx, call)
		if err != nil {
			return s.testFailed(ctx, c, p, err), nil
		}
	default:
		return nil, userErr("this integration has nothing to test")
	}
	if res.Summary == "" {
		res.Summary = "Test succeeded"
	}
	s.logActivity(ctx, c.ID, &Activity{Kind: ActivityTest, Outcome: OutcomeSuccess, Summary: res.Summary, Detail: res.Detail, userID: p.UserID})
	if err := s.store.markSuccess(ctx, c.ID, false); err != nil {
		return nil, err
	}
	return &TestResult{OK: true, Summary: res.Summary, Detail: res.Detail}, nil
}

func (s *Service) testFailed(ctx context.Context, c *Connection, p auth.Principal, err error) *TestResult {
	summary := describeError(err)
	s.logActivity(ctx, c.ID, &Activity{Kind: ActivityTest, Outcome: OutcomeFailed, Summary: summary, Detail: errorDetail(err), userID: p.UserID})
	return &TestResult{OK: false, Summary: summary, Detail: errorDetail(err)}
}

// newCall prepares what a provider needs to work for a connection.
func (s *Service) newCall(ctx context.Context, c *Connection, prov Provider) (*Call, error) {
	v, err := s.open(c)
	if err != nil {
		return nil, err
	}
	settings := Settings{Values: c.Config, Secrets: v.Fields, AllowPrivate: s.opts.AllowPrivate}
	call := &Call{Connection: c, Settings: settings, HTTP: s.http, PublicURL: s.opts.PublicURL}
	if op, ok := prov.(OAuthProvider); ok && prov.Manifest().Auth == AuthOAuth2 {
		if v.Token == nil {
			return nil, jobs.Permanent(userErr("the connection isn't authorised yet; connect it first"))
		}
		cfg := s.oauthConfig(op.OAuth(settings), settings)
		base := cfg.TokenSource(context.WithValue(context.WithoutCancel(ctx), oauth2.HTTPClient, s.http), v.Token)
		src := &savingTokenSource{base: base, last: v.Token, save: func(t *oauth2.Token) error {
			v.Token = t
			sealed, err := s.seal(c.ID, v)
			if err != nil {
				return err
			}
			return s.store.saveSecrets(context.WithoutCancel(ctx), c.ID, sealed)
		}}
		call.HTTP = &http.Client{
			Transport:     &oauth2.Transport{Source: src, Base: s.http.Transport},
			Timeout:       s.http.Timeout,
			CheckRedirect: s.http.CheckRedirect,
		}
	}
	return call, nil
}

// savingTokenSource stores refreshed OAuth tokens, so the next run starts
// from the newest one (providers may rotate refresh tokens).
type savingTokenSource struct {
	mu   sync.Mutex
	base oauth2.TokenSource
	last *oauth2.Token
	save func(*oauth2.Token) error
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	t, err := s.base.Token()
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) && (re.ErrorCode == "invalid_grant" || re.Response != nil && re.Response.StatusCode == http.StatusUnauthorized) {
			return nil, jobs.Permanent(errors.New("the authorisation was revoked or expired; connect the integration again"))
		}
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil || t.AccessToken != s.last.AccessToken {
		s.last = t
		if err := s.save(t); err != nil {
			return nil, fmt.Errorf("saving refreshed token: %w", err)
		}
	}
	return t, nil
}

// sampleLead is the lead a test sends: made up apart from the organisation,
// with every field filled so the payload looks like the real thing.
func (s *Service) sampleLead(ctx context.Context, c *Connection) (*Lead, error) {
	org, err := s.orgs.GetOrganization(ctx, c.OrgID)
	if err != nil {
		return nil, err
	}
	return &Lead{
		ID:        0,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		Name:      "Test Lead",
		Email:     "test.lead@example.com",
		Phone:     "+14155550100",
		Notes:     "This is a test lead sent from Fronko to check the connection.",
		Source:    "link",
		Card: LeadCard{
			ID:   0,
			Slug: "example-card",
			Name: "Example Card",
			URL:  s.cardURL(org.Handle, "example-card"),
		},
		Owner:        &LeadOwner{ID: 0, Username: "example-user", Email: "owner@example.com"},
		Organisation: LeadOrg{ID: org.ID, Name: org.Name, Handle: org.Handle},
	}, nil
}

func (s *Service) cardURL(handle, slug string) string {
	if s.opts.PublicURL == "" {
		return ""
	}
	return s.opts.PublicURL + "/p/" + handle + "/" + slug
}

// toLead converts a stored lead into the lead sync payload.
func (s *Service) toLead(d *leads.SyncDetails) *Lead {
	l := &Lead{
		ID:        d.ID,
		CreatedAt: d.CreatedAt.UTC(),
		Name:      d.Name,
		Email:     d.Email,
		Notes:     d.Notes,
		Source:    d.Source,
		Card: LeadCard{
			ID:   d.ProfileID,
			Slug: d.CardSlug,
			Name: d.CardName,
			URL:  s.cardURL(d.OrgHandle, d.CardSlug),
		},
		Organisation: LeadOrg{ID: d.OrgID, Name: d.OrgName, Handle: d.OrgHandle},
	}
	if d.PhoneNumber != "" {
		l.Phone = d.PhoneCountryCode + d.PhoneNumber
	}
	if d.AssignedUser != nil {
		l.Owner = &LeadOwner{ID: d.AssignedUser.ID, Username: d.AssignedUser.Username, Email: d.OwnerEmail}
	}
	return l
}

func (s *Service) logActivity(ctx context.Context, connectionID int64, a *Activity) {
	a.connectionID = connectionID
	if len(a.Summary) > 500 {
		a.Summary = strings.ToValidUTF8(a.Summary[:500], "") + "…"
	}
	if err := s.store.addActivity(context.WithoutCancel(ctx), a); err != nil {
		logf("recording activity for connection %d: %v", connectionID, err)
	}
}

// cleanName checks a connection name, defaulting to the provider's.
func cleanName(name, fallback string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = fallback
	}
	if utf8.RuneCountInString(name) > maxNameLen || strings.ContainsFunc(name, unicode.IsControl) {
		return "", fieldErr("name", fmt.Sprintf("the name must be at most %d characters on one line", maxNameLen))
	}
	return name, nil
}
