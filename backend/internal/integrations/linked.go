package integrations

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
)

// This file is the API that the packages built on the integrations core
// use: single sign-on (sso), SCIM (directory) and the card's booking
// button (cards).

// tokenPrefix marks Fronko's inbound tokens, so secret scanners and people
// can recognise one.
const tokenPrefix = "fronko_scim_"

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// RotateToken generates a new token for a scim_token connection, revoking
// the previous one, and returns it. It's the only time the token is shown.
func (s *Service) RotateToken(ctx context.Context, p auth.Principal, id int64) (string, *View, error) {
	c, prov, err := s.get(ctx, p, id)
	if err != nil {
		return "", nil, err
	}
	m := prov.Manifest()
	if m.Auth != AuthSCIMToken {
		return "", nil, userErr("this integration doesn't use a token")
	}
	token := tokenPrefix + rand.Text() + rand.Text()
	if err := s.store.addToken(ctx, c.ID, hashToken(token), token[len(token)-4:], p.UserID); err != nil {
		return "", nil, err
	}
	summary := "Token generated"
	if c.token != nil {
		summary = "Token replaced; the previous one stopped working"
	}
	s.logActivity(ctx, c.ID, &Activity{Kind: ActivitySetup, Outcome: OutcomeSuccess, Summary: summary, userID: p.UserID})
	if c, err = s.store.GetConnection(ctx, p.OrgID, c.ID); err != nil {
		return "", nil, err
	}
	v, err := s.open(c)
	if err != nil {
		return "", nil, err
	}
	if c.Status == StatusPending && ready(m, Settings{Values: c.Config, Secrets: v.Fields}, v, c) {
		c.Status = StatusActive
		if err := s.store.UpdateConnection(ctx, c); err != nil {
			return "", nil, err
		}
	}
	view, err := s.view(ctx, c)
	return token, view, err
}

// Authenticate returns the enabled connection in category that token
// belongs to. Any other token gets ErrBadToken.
func (s *Service) Authenticate(ctx context.Context, token string, category Category) (*Connection, error) {
	if !strings.HasPrefix(token, tokenPrefix) {
		return nil, ErrBadToken
	}
	c, err := s.store.connectionByToken(ctx, hashToken(token))
	if errors.Is(err, database.ErrNotFound) {
		return nil, ErrBadToken
	}
	if err != nil {
		return nil, err
	}
	if c.Category != category || !c.Enabled {
		return nil, ErrBadToken
	}
	return c, nil
}

// Linked is a connection opened for the package that serves it.
type Linked struct {
	Connection *Connection
	Provider   Provider
	Settings   Settings
}

// OrgConnection returns the organisation's enabled, active connection in a
// single-connection category (sso, directory), or database.ErrNotFound.
func (s *Service) OrgConnection(ctx context.Context, orgID int64, category Category) (*Linked, error) {
	conns, err := s.store.FindConnections(ctx, orgID, 0, category, false)
	if err != nil {
		return nil, err
	}
	if len(conns) == 0 {
		return nil, database.ErrNotFound
	}
	return s.Open(conns[0])
}

// ConnectionByID returns a connection by id, for packages handling a
// request that names it (a SAML response). Only enabled ones are returned.
func (s *Service) ConnectionByID(ctx context.Context, id int64) (*Linked, error) {
	c, err := s.store.getConnectionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !c.Enabled {
		return nil, database.ErrNotFound
	}
	return s.Open(c)
}

// Open decrypts a connection's settings.
func (s *Service) Open(c *Connection) (*Linked, error) {
	prov, ok := s.registry.Get(c.Provider)
	if !ok {
		return nil, ErrUnknownProvider
	}
	v, err := s.open(c)
	if err != nil {
		return nil, err
	}
	return &Linked{
		Connection: c,
		Provider:   prov,
		Settings:   Settings{Values: c.Config, Secrets: v.Fields, AllowPrivate: s.opts.AllowPrivate},
	}, nil
}

// Event is something a connection did, for its activity log.
type Event struct {
	Kind    string
	Outcome string
	Summary string
	Detail  map[string]any
	// UserID is who did it, or 0.
	UserID int64
}

// Record adds an entry to a connection's activity log. Successes also mark
// the connection as working and synced now.
func (s *Service) Record(ctx context.Context, connectionID int64, e Event) {
	s.logActivity(ctx, connectionID, &Activity{Kind: e.Kind, Outcome: e.Outcome, Summary: e.Summary, Detail: e.Detail, userID: e.UserID})
	if e.Outcome == OutcomeSuccess {
		if err := s.store.markSuccess(context.WithoutCancel(ctx), connectionID, true); err != nil {
			logf("marking connection %d synced: %v", connectionID, err)
		}
	}
}

// Bookings are the booking pages of an organisation's cards.
type Bookings struct {
	org    *Booking
	byUser map[int64]*Booking
}

// For returns the booking page for a card held by holderID (0: the
// organisation): their own, or else the organisation's default. It may be nil.
func (b *Bookings) For(holderID int64) *Booking {
	if b == nil {
		return nil
	}
	if bk, ok := b.byUser[holderID]; ok && holderID != 0 {
		return bk
	}
	return b.org
}

// Bookings loads the booking pages connected in an organisation.
func (s *Service) Bookings(ctx context.Context, orgID int64) (*Bookings, error) {
	conns, err := s.store.connectionsInCategory(ctx, orgID, CategoryCalendar)
	if err != nil {
		return nil, err
	}
	out := &Bookings{byUser: map[int64]*Booking{}}
	for _, c := range conns {
		prov, ok := s.registry.Get(c.Provider)
		if !ok {
			continue
		}
		linker, ok := prov.(BookingLinker)
		if !ok {
			continue
		}
		bk := linker.Booking(Settings{Values: c.Config})
		if bk == nil {
			continue
		}
		bk.Scope = c.Scope()
		if c.UserID == 0 {
			out.org = bk
		} else {
			out.byUser[c.UserID] = bk
		}
	}
	return out, nil
}
