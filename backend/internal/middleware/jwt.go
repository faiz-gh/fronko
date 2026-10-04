package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/faiz-gh/fronko/backend/internal/auth"
)

type contextKey string

const (
	userIDKey   contextKey = "userID"
	verifiedKey contextKey = "emailVerified"
)

// CodeEmailUnverified marks the 403 sent to signed-in users who haven't
// verified their email yet; the SPA sends them to the verification screen.
const CodeEmailUnverified = "email_unverified"

// SessionCookieName is the HttpOnly cookie that carries the JWT.
const SessionCookieName = "fronko_session"

// UserID returns the authenticated user's ID. Only call it from handlers
// mounted behind JWTMiddleware.
func UserID(ctx context.Context) int64 {
	id, _ := ctx.Value(userIDKey).(int64)
	return id
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":"` + msg + `"}`))
}

func writeErrorCode(w http.ResponseWriter, status int, msg, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":"` + msg + `","code":"` + code + `"}`))
}

// ErrSessionUserNotFound is what a SessionChecker returns for a deleted account.
var ErrSessionUserNotFound = errors.New("user not found")

// SessionChecker looks up the live state behind a token: its current session
// version (bumped to revoke every session) and whether the email is verified.
type SessionChecker interface {
	GetSessionState(ctx context.Context, userID int64) (sessionVersion int, verified bool, err error)
}

// SessionCheckerFunc adapts a function to SessionChecker.
type SessionCheckerFunc func(ctx context.Context, userID int64) (int, bool, error)

func (f SessionCheckerFunc) GetSessionState(ctx context.Context, userID int64) (int, bool, error) {
	return f(ctx, userID)
}

// EmailVerified reports whether the signed-in user has verified their email.
func EmailVerified(ctx context.Context) bool {
	v, _ := ctx.Value(verifiedKey).(bool)
	return v
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

// JWTMiddleware authenticates the session cookie. Besides the signature and
// expiry, it checks the token's session version against the database, so a
// password change or reset signs out every older session.
func JWTMiddleware(authService *auth.Service, sessions SessionChecker) func(http.Handler) http.Handler {
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

			current, verified, err := sessions.GetSessionState(r.Context(), userID)
			if err != nil {
				if errors.Is(err, ErrSessionUserNotFound) {
					writeError(w, http.StatusUnauthorized, "account no longer exists")
					return
				}
				log.Printf("session lookup: %v", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if version != current {
				writeError(w, http.StatusUnauthorized, "session expired, please sign in again")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = context.WithValue(ctx, verifiedKey, verified)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
