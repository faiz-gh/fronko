package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serve runs h behind JWTMiddleware for a user whose live state is state.
func serve(t *testing.T, state SessionState, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	svc := NewService("test-secret-test-secret")
	token, err := svc.GenerateJWT(7, state.Version)
	require.NoError(t, err)
	sessions := SessionCheckerFunc(func(context.Context, int64) (SessionState, error) { return state, nil })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	JWTMiddleware(svc, sessions)(h).ServeHTTP(rec, req)
	return rec
}

var ok = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

func TestJWTMiddlewareSetsPrincipal(t *testing.T) {
	var got Principal
	rec := serve(t, SessionState{Verified: true, OrgID: 3, Role: RoleAdmin},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = PrincipalFrom(r.Context()) }))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, Principal{UserID: 7, OrgID: 3, Role: RoleAdmin}, got)
	assert.True(t, got.IsAdmin())
	assert.False(t, got.IsOwner())
}

func TestJWTMiddlewareRejectsSuspended(t *testing.T) {
	rec := serve(t, SessionState{Verified: true, Suspended: true, Role: RoleMember}, ok)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), CodeAccountSuspended)
}

func TestRequirePasswordSet(t *testing.T) {
	rec := serve(t, SessionState{Verified: true, MustChangePassword: true}, RequirePasswordSet(ok))
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), CodePasswordChangeRequired)

	rec = serve(t, SessionState{Verified: true}, RequirePasswordSet(ok))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRoleGates(t *testing.T) {
	cases := []struct {
		role         string
		admin, owner int
	}{
		{RoleOwner, http.StatusNoContent, http.StatusNoContent},
		{RoleAdmin, http.StatusNoContent, http.StatusForbidden},
		{RoleMember, http.StatusForbidden, http.StatusForbidden},
	}
	for _, c := range cases {
		t.Run(c.role, func(t *testing.T) {
			state := SessionState{Verified: true, Role: c.role}
			assert.Equal(t, c.admin, serve(t, state, RequireAdmin(ok)).Code)
			assert.Equal(t, c.owner, serve(t, state, RequireOwner(ok)).Code)
		})
	}
}

func TestJWTMiddlewareRejectsSuspendedOrg(t *testing.T) {
	rec := serve(t, SessionState{Verified: true, OrgSuspended: true, OrgSuspendedReason: `unpaid "invoice"`}, ok)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, CodeOrgSuspended, body["code"])
	assert.Equal(t, `unpaid "invoice"`, body["reason"], "the reason is JSON-encoded")
}

func TestJWTMiddlewareRejectsAdminCookie(t *testing.T) {
	svc := NewService("test-secret-test-secret")
	token, err := svc.GenerateAdminJWT(7, 0)
	require.NoError(t, err)
	sessions := SessionCheckerFunc(func(context.Context, int64) (SessionState, error) {
		return SessionState{Verified: true}, nil
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	JWTMiddleware(svc, sessions)(ok).ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
