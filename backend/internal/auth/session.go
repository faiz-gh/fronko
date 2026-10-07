package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

type contextKey string

const (
	principalKey contextKey = "principal"
	stateKey     contextKey = "sessionState"
)

// Error codes the SPA acts on.
const (
	// CodeEmailUnverified marks the 403 sent to signed-in users who haven't
	// verified their email yet; the SPA sends them to the verification screen.
	CodeEmailUnverified = "email_unverified"
	// CodePasswordChangeRequired marks the 403 sent while the user still has
	// the password their organisation set; the SPA asks them to choose one.
	CodePasswordChangeRequired = "password_change_required"
	// CodeAccountSuspended marks the 401 for accounts the organisation suspended.
	CodeAccountSuspended = "account_suspended"
	// CodeOrgSuspended marks responses for an organisation the platform
	// suspended; they carry the reason the admin gave.
	CodeOrgSuspended = "org_suspended"
)

// OrgSuspendedMessage is the error shown to members of a suspended organisation.
const OrgSuspendedMessage = "your organisation has been suspended"

// Principal is the signed-in user and where they stand in their organisation.
type Principal struct {
	UserID int64
	OrgID  int64
	Role   string
	// Teams the user is in, with their role in each.
	Teams []TeamRef
}

// IsAdmin is true for the organisation's owner and admins.
func (p Principal) IsAdmin() bool { return p.Role == RoleOwner || p.Role == RoleAdmin }

// IsOwner is true only for the account that registered the organisation.
func (p Principal) IsOwner() bool { return p.Role == RoleOwner }

// InTeam reports whether the user is in the team, as a member or a lead.
func (p Principal) InTeam(teamID int64) bool {
	for _, t := range p.Teams {
		if t.ID == teamID {
			return true
		}
	}
	return false
}

// LeadsTeam reports whether the user is a lead of the team.
func (p Principal) LeadsTeam(teamID int64) bool {
	for _, t := range p.Teams {
		if t.ID == teamID && t.Role == TeamRoleLead {
			return true
		}
	}
	return false
}

// LeadsAnyTeam reports whether the user leads at least one team.
func (p Principal) LeadsAnyTeam() bool {
	for _, t := range p.Teams {
		if t.Role == TeamRoleLead {
			return true
		}
	}
	return false
}

// PrincipalFrom returns the authenticated principal. Only call it from
// handlers mounted behind JWTMiddleware.
func PrincipalFrom(ctx context.Context) Principal {
	p, _ := ctx.Value(principalKey).(Principal)
	return p
}

// WithPrincipal returns ctx carrying p, as JWTMiddleware does. For tests and
// handlers that authenticate some other way.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// SessionCookieName is the HttpOnly cookie that carries the JWT.
const SessionCookieName = "fronko_session"

// UserID returns the authenticated user's ID. Only call it from handlers
// mounted behind JWTMiddleware.
func UserID(ctx context.Context) int64 {
	return PrincipalFrom(ctx).UserID
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}

func writeErrorCode(w http.ResponseWriter, status int, msg, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `","code":"` + code + `"}`))
}

// writeOrgSuspended sends the org_suspended error. The reason is free text an
// admin typed, so it goes through the JSON encoder.
func writeOrgSuspended(w http.ResponseWriter, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": OrgSuspendedMessage, "code": CodeOrgSuspended, "reason": reason})
}

// ErrSessionUserNotFound is what a SessionChecker returns for a deleted account.
var ErrSessionUserNotFound = errors.New("user not found")

// SessionChecker looks up the live state behind a token: its current session
// version (bumped to revoke every session), whether the email is verified,
// and the user's organisation and role.
type SessionChecker interface {
	GetSessionState(ctx context.Context, userID int64) (SessionState, error)
}

// SessionCheckerFunc adapts a function to SessionChecker.
type SessionCheckerFunc func(ctx context.Context, userID int64) (SessionState, error)

func (f SessionCheckerFunc) GetSessionState(ctx context.Context, userID int64) (SessionState, error) {
	return f(ctx, userID)
}

func sessionState(ctx context.Context) SessionState {
	s, _ := ctx.Value(stateKey).(SessionState)
	return s
}

// EmailVerified reports whether the signed-in user has verified their email.
func EmailVerified(ctx context.Context) bool {
	return sessionState(ctx).Verified
}

// MustChangePassword reports whether the user still has a password their organisation set.
func MustChangePassword(ctx context.Context) bool {
	return sessionState(ctx).MustChangePassword
}

// RequireVerified rejects users who haven't verified their email. Mount it
// inside JWTMiddleware.
func RequireVerified(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !EmailVerified(r.Context()) {
			writeErrorCode(w, http.StatusForbidden, "verify your email to continue", CodeEmailUnverified)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequirePasswordSet rejects users who must still replace the password their
// organisation gave them. Mount it inside JWTMiddleware.
func RequirePasswordSet(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if MustChangePassword(r.Context()) {
			writeErrorCode(w, http.StatusForbidden, "choose a new password to continue", CodePasswordChangeRequired)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin only lets the organisation's owner and admins through.
func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !PrincipalFrom(r.Context()).IsAdmin() {
			writeError(w, http.StatusForbidden, "only your organisation's admins can do this")
			return
		}
		next(w, r)
	}
}

// RequireOwner only lets the organisation's owner through.
func RequireOwner(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !PrincipalFrom(r.Context()).IsOwner() {
			writeError(w, http.StatusForbidden, "only your organisation's owner can do this")
			return
		}
		next(w, r)
	}
}

// JWTMiddleware authenticates the session cookie. Besides the signature and
// expiry, it checks the token's session version against the database, so a
// password change or reset signs out every older session, and turns away
// suspended accounts.
func JWTMiddleware(authService *Service, sessions SessionChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				writeError(w, http.StatusUnauthorized, "not signed in")
				return
			}

			userID, version, err := authService.ValidateJWT(cookie.Value)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "session expired, please sign in again")
				return
			}

			state, err := sessions.GetSessionState(r.Context(), userID)
			if err != nil {
				if errors.Is(err, ErrSessionUserNotFound) {
					writeError(w, http.StatusUnauthorized, "account no longer exists")
					return
				}
				log.Printf("session lookup: %v", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			// Checked before the version, which suspending also bumps, so the
			// user is told why rather than that their session expired.
			if state.OrgSuspended {
				writeOrgSuspended(w, state.OrgSuspendedReason)
				return
			}
			if version != state.Version {
				writeError(w, http.StatusUnauthorized, "session expired, please sign in again")
				return
			}
			if state.Suspended {
				writeErrorCode(w, http.StatusUnauthorized, "this account is suspended; contact your organisation", CodeAccountSuspended)
				return
			}

			ctx := context.WithValue(r.Context(), principalKey, Principal{UserID: userID, OrgID: state.OrgID, Role: state.Role, Teams: state.Teams})
			ctx = context.WithValue(ctx, stateKey, state)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
