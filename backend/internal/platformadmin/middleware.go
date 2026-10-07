package platformadmin

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/faiz-gh/fronko/backend/internal/auth"
)

// AdminCookieName is the HttpOnly cookie that carries a platform admin's JWT.
// It is separate from the user session cookie, so one browser can hold both.
const AdminCookieName = "fronko_admin"

type contextKey string

const adminKey contextKey = "platformAdmin"

// ErrAdminNotFound is what an AdminLookup returns for a deleted admin.
var ErrAdminNotFound = errors.New("admin not found")

// AdminLookup loads a platform admin by ID.
type AdminLookup func(ctx context.Context, adminID int64) (*PlatformAdmin, error)

// AdminFrom returns the signed-in platform admin. Only call it from handlers
// mounted behind AdminMiddleware.
func AdminFrom(ctx context.Context) *PlatformAdmin {
	a, _ := ctx.Value(adminKey).(*PlatformAdmin)
	return a
}

// AdminMiddleware authenticates a platform admin's session cookie and checks
// its session version against the database. User session cookies are never
// accepted here.
func AdminMiddleware(authService *auth.Service, lookup AdminLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(AdminCookieName)
			if err != nil || cookie.Value == "" {
				writeError(w, http.StatusUnauthorized, "not signed in")
				return
			}
			adminID, version, err := authService.ValidateAdminJWT(cookie.Value)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "session expired, please sign in again")
				return
			}
			admin, err := lookup(r.Context(), adminID)
			if err != nil {
				if errors.Is(err, ErrAdminNotFound) {
					writeError(w, http.StatusUnauthorized, "account no longer exists")
					return
				}
				log.Printf("admin session lookup: %v", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if version != admin.SessionVersion {
				writeError(w, http.StatusUnauthorized, "session expired, please sign in again")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), adminKey, admin)))
		})
	}
}

// writeError matches the user session middleware's error body byte for byte.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
