package sso

import (
	"context"
	"errors"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/integrations"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// Policy answers what other modules need to know about an organisation's
// single sign-on: whether a password may be used (account), and how to
// treat people a directory provisions (directory).
type Policy struct {
	svc       *integrations.Service
	store     *Store
	publicURL string
}

func NewPolicy(svc *integrations.Service, store *Store, publicURL string) *Policy {
	return &Policy{svc: svc, store: store, publicURL: publicURL}
}

// signInPath is where the organisation's people start single sign-on, or
// "" when it has none.
func (p *Policy) signInPath(ctx context.Context, orgID int64) (path string, enforced bool, err error) {
	link, err := p.svc.OrgConnection(ctx, orgID, integrations.CategorySSO)
	if errors.Is(err, database.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	org, err := p.store.orgByID(ctx, orgID)
	if err != nil {
		return "", false, err
	}
	return "/auth/sso/" + org.Handle, link.Settings.Bool("enforce"), nil
}

// PasswordBlocked reports whether u may not sign in (or reset their
// password) with a password, and where to sign in instead: people without a
// password always use single sign-on, and when the organisation requires it
// everyone but the owner does.
func (p *Policy) PasswordBlocked(ctx context.Context, u *users.User) (signInPath string, blocked bool, err error) {
	path, enforced, err := p.signInPath(ctx, u.OrgID)
	if err != nil || path == "" {
		return "", false, err
	}
	if !u.HasPassword() || (enforced && u.Role != auth.RoleOwner) {
		return path, true, nil
	}
	return "", false, nil
}

// SignInURL is the page on the site that starts an organisation's single
// sign-on, for people to bookmark and for emails. It's a site page rather
// than the API's /auth/sso/{handle}, so it stays on the site's domain when
// the API has its own.
func SignInURL(publicURL, handle string) string {
	return publicURL + "/login/sso/" + handle
}

// SSOSignInURL is the absolute address people sign in at, for emails, or
// "" when the organisation has no single sign-on.
func (p *Policy) SSOSignInURL(ctx context.Context, orgID int64) (string, error) {
	path, _, err := p.signInPath(ctx, orgID)
	if err != nil || path == "" {
		return "", err
	}
	return SignInURL(p.publicURL, strings.TrimPrefix(path, "/auth/sso/")), nil
}

// VerifiedEmail reports whether email is on one of the organisation's
// verified domains.
func (p *Policy) VerifiedEmail(ctx context.Context, orgID int64, email string) (bool, error) {
	return p.store.domainVerified(ctx, orgID, domainOf(email))
}
