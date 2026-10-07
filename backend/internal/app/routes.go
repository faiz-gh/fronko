package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/ratelimit"
)

// Handler wraps a handler with extra checks, such as a rate limit.
type Handler = func(http.HandlerFunc) http.HandlerFunc

// Routes is where modules register their endpoints. Each method mounts the
// handler behind the matching middleware chain, so a module only says who may
// call an endpoint, not how that is enforced.
//
// Patterns use net/http's syntax ("GET /api/me/leads"). Signed-in routes must
// sit under /api/<area>/ (for example /api/me/, /api/org/ or
// /api/integrations/); the area is mounted behind the session chain the first
// time it is used.
type Routes struct {
	ctx        context.Context
	trustProxy bool
	mux        *http.ServeMux
	external   *http.ServeMux
	user       *http.ServeMux
	platform   *http.ServeMux
	session    func(http.Handler) http.Handler
	userChain  http.Handler
	mounted    map[string]bool
	hasAdmin   bool

	// AuthLimit is the per-IP budget shared by every endpoint that checks a
	// password or sends an email code, so they can't be used to guess either.
	AuthLimit Handler
}

// Session authenticates a user's session cookie (auth.JWTMiddleware).
type Session = func(http.Handler) http.Handler

// NewRoutes returns an empty route table. session authenticates users, and
// AuthLimit allows authBurst requests at once, then one every authEvery.
func NewRoutes(ctx context.Context, session Session, trustProxy bool, authEvery time.Duration, authBurst int) *Routes {
	r := &Routes{
		ctx:        ctx,
		trustProxy: trustProxy,
		mux:        http.NewServeMux(),
		external:   http.NewServeMux(),
		user:       http.NewServeMux(),
		platform:   http.NewServeMux(),
		session:    session,
		mounted:    map[string]bool{},
	}
	// Protected routes: verified email, and a password the user chose.
	r.userChain = session(auth.RequireVerified(auth.RequirePasswordSet(r.user)))
	r.AuthLimit = r.RateLimit(authEvery, authBurst)
	return r
}

// RateLimit returns a new per-IP token bucket: burst requests at once, then
// one more every interval. Each call is a separate budget.
func (r *Routes) RateLimit(every time.Duration, burst int) Handler {
	return ratelimit.NewRateLimiter(r.ctx, rate.Every(every), burst, r.trustProxy).Limit
}

// Public serves anyone, signed in or not.
func (r *Routes) Public(pattern string, h http.HandlerFunc) {
	r.mux.HandleFunc(pattern, h)
}

// External serves other servers and sites that bring their own credentials:
// a SCIM client with its bearer token, or an identity provider posting a
// sign-in response from its own origin. These routes skip the same-origin
// check that protects the session cookie, so they must never act on the
// session cookie alone.
func (r *Routes) External(pattern string, h http.HandlerFunc) {
	r.external.HandleFunc(pattern, h)
}

// SignedIn needs a session but works before the email is verified, so the
// user can see who they are and finish verification. These patterns are
// more specific than the area mounts, so they take precedence.
func (r *Routes) SignedIn(pattern string, h http.HandlerFunc) {
	r.mux.Handle(pattern, r.session(h))
}

// Verified needs a session and a verified email, but works while the
// organisation's temporary password is still in place.
func (r *Routes) Verified(pattern string, h http.HandlerFunc) {
	r.mux.Handle(pattern, r.session(auth.RequireVerified(h)))
}

// User needs a session, a verified email and a password the user chose.
// Owners and admins see their whole organisation; members only their own
// things; the handler applies that with auth.ScopeOf.
func (r *Routes) User(pattern string, h http.HandlerFunc) {
	r.mountArea(pattern)
	r.user.HandleFunc(pattern, h)
}

// Admin is User for the organisation's owner and admins only.
func (r *Routes) Admin(pattern string, h http.HandlerFunc) {
	r.User(pattern, auth.RequireAdmin(h))
}

// Owner is User for the organisation's owner only.
func (r *Routes) Owner(pattern string, h http.HandlerFunc) {
	r.User(pattern, auth.RequireOwner(h))
}

// PlatformAdmin serves the platform admin panel's API under /api/admin/,
// behind the admin session (never a user session).
func (r *Routes) PlatformAdmin(pattern string, h http.HandlerFunc) {
	r.hasAdmin = true
	r.platform.HandleFunc(pattern, h)
}

// mountArea sends /api/<area> and everything under it through the session
// chain to the user mux.
func (r *Routes) mountArea(pattern string) {
	path := pattern
	if i := strings.IndexByte(path, ' '); i >= 0 {
		path = path[i+1:]
	}
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 3)
	if len(parts) < 2 || parts[0] != "api" {
		panic("app: signed-in route outside /api/<area>/: " + pattern)
	}
	area := "/api/" + parts[1]
	for _, p := range []string{area + "/", path} {
		// The exact area path ("GET /api/org") is only mounted when used.
		if p == area+"/" || p == area {
			if !r.mounted[p] {
				r.mounted[p] = true
				r.mux.Handle(p, r.userChain)
			}
		}
	}
}

// Handler returns the finished router. platformGuard authenticates platform
// admins; it is only used when a module registered PlatformAdmin routes.
// browser wraps every route but the External ones (the same-origin check).
func (r *Routes) Handler(platformGuard, browser func(http.Handler) http.Handler) http.Handler {
	if r.hasAdmin {
		r.mux.Handle("/api/admin/", platformGuard(r.platform))
	}
	// External patterns are more specific than "/", so they take precedence.
	r.external.Handle("/", browser(r.mux))
	return r.external
}
