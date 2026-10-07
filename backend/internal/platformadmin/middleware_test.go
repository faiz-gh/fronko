package platformadmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/faiz-gh/fronko/backend/internal/auth"
)

func serveAdmin(t *testing.T, cookieName, token string, admin *PlatformAdmin) (*httptest.ResponseRecorder, *PlatformAdmin) {
	t.Helper()
	svc := auth.NewService("test-secret-test-secret")
	lookup := func(context.Context, int64) (*PlatformAdmin, error) {
		if admin == nil {
			return nil, ErrAdminNotFound
		}
		return admin, nil
	}
	var got *PlatformAdmin
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	rec := httptest.NewRecorder()
	AdminMiddleware(svc, lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = AdminFrom(r.Context())
	})).ServeHTTP(rec, req)
	return rec, got
}

func TestAdminMiddleware(t *testing.T) {
	svc := auth.NewService("test-secret-test-secret")
	adminToken, err := svc.GenerateAdminJWT(1, 2)
	require.NoError(t, err)
	userToken, err := svc.GenerateJWT(1, 2)
	require.NoError(t, err)
	admin := &PlatformAdmin{ID: 1, Email: "a@example.com", SessionVersion: 2}

	rec, got := serveAdmin(t, AdminCookieName, adminToken, admin)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, admin, got)

	rec, _ = serveAdmin(t, AdminCookieName, userToken, admin)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "user token in the admin cookie")

	rec, _ = serveAdmin(t, auth.SessionCookieName, adminToken, admin)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "admin token in the user cookie")

	rec, _ = serveAdmin(t, AdminCookieName, adminToken, &PlatformAdmin{ID: 1, SessionVersion: 3})
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "revoked session")

	rec, _ = serveAdmin(t, AdminCookieName, adminToken, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "deleted admin")
}
