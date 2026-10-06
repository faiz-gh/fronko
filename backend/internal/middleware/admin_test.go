package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serveAdmin(t *testing.T, cookieName, token string, admin *models.PlatformAdmin) (*httptest.ResponseRecorder, *models.PlatformAdmin) {
	t.Helper()
	svc := auth.NewService("test-secret-test-secret")
	lookup := func(context.Context, int64) (*models.PlatformAdmin, error) {
		if admin == nil {
			return nil, ErrAdminNotFound
		}
		return admin, nil
	}
	var got *models.PlatformAdmin
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
	admin := &models.PlatformAdmin{ID: 1, Email: "a@example.com", SessionVersion: 2}

	rec, got := serveAdmin(t, AdminCookieName, adminToken, admin)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, admin, got)

	rec, _ = serveAdmin(t, AdminCookieName, userToken, admin)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "user token in the admin cookie")

	rec, _ = serveAdmin(t, SessionCookieName, adminToken, admin)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "admin token in the user cookie")

	rec, _ = serveAdmin(t, AdminCookieName, adminToken, &models.PlatformAdmin{ID: 1, SessionVersion: 3})
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "revoked session")

	rec, _ = serveAdmin(t, AdminCookieName, adminToken, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "deleted admin")
}
