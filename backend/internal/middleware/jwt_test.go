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

// serve runs h behind JWTMiddleware for a user whose live state is state.
func serve(t *testing.T, state models.SessionState, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	svc := auth.NewService("test-secret-test-secret")
	token, err := svc.GenerateJWT(7, state.Version)
	require.NoError(t, err)
	sessions := SessionCheckerFunc(func(context.Context, int64) (models.SessionState, error) { return state, nil })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	JWTMiddleware(svc, sessions)(h).ServeHTTP(rec, req)
	return rec
}

var ok = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

func TestJWTMiddlewareSetsPrincipal(t *testing.T) {
	var got Principal
	rec := serve(t, models.SessionState{Verified: true, OrgID: 3, Role: models.RoleAdmin},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = PrincipalFrom(r.Context()) }))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, Principal{UserID: 7, OrgID: 3, Role: models.RoleAdmin}, got)
	assert.True(t, got.IsAdmin())
	assert.False(t, got.IsOwner())
}

func TestJWTMiddlewareRejectsSuspended(t *testing.T) {
	rec := serve(t, models.SessionState{Verified: true, Suspended: true, Role: models.RoleMember}, ok)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), CodeAccountSuspended)
}

func TestRequirePasswordSet(t *testing.T) {
	rec := serve(t, models.SessionState{Verified: true, MustChangePassword: true}, RequirePasswordSet(ok))
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), CodePasswordChangeRequired)

	rec = serve(t, models.SessionState{Verified: true}, RequirePasswordSet(ok))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRoleGates(t *testing.T) {
	cases := []struct {
		role         string
		admin, owner int
	}{
		{models.RoleOwner, http.StatusNoContent, http.StatusNoContent},
		{models.RoleAdmin, http.StatusNoContent, http.StatusForbidden},
		{models.RoleMember, http.StatusForbidden, http.StatusForbidden},
	}
	for _, c := range cases {
		t.Run(c.role, func(t *testing.T) {
			state := models.SessionState{Verified: true, Role: c.role}
			assert.Equal(t, c.admin, serve(t, state, RequireAdmin(ok)).Code)
			assert.Equal(t, c.owner, serve(t, state, RequireOwner(ok)).Code)
		})
	}
}
