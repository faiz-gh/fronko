//go:build integration

package integrationtest_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/faiz-gh/fronko/backend/internal/app"
	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/integrations"
	"github.com/faiz-gh/fronko/backend/internal/integrations/directory"
	"github.com/faiz-gh/fronko/backend/internal/integrations/providers"
	"github.com/faiz-gh/fronko/backend/internal/integrations/sso"
	"github.com/faiz-gh/fronko/backend/internal/leads"
	"github.com/faiz-gh/fronko/backend/internal/orgs"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/mail"
	"github.com/faiz-gh/fronko/backend/internal/platform/secrets"
	"github.com/faiz-gh/fronko/backend/internal/teams"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// TestIdentityIntegration covers booking pages, SCIM provisioning and the
// single sign-on policy against a real database.
func TestIdentityIntegration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration tests")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()
	_, err = pool.Exec(ctx, "TRUNCATE TABLE organizations, users, jobs, integration_connections RESTART IDENTITY CASCADE")
	require.NoError(t, err)
	repo := newStores(pool)

	owner := &users.User{Username: "id-owner", Email: ptr("owner@acme.example"), PasswordHash: "hash"}
	require.NoError(t, repo.CreateOrgWithOwner(ctx, "Acme", owner))
	rep := &users.User{OrgID: owner.OrgID, Role: auth.RoleMember, Username: "id-rep", Email: ptr("rep@acme.example"),
		PasswordHash: "hash", CreatedBy: &owner.ID}
	require.NoError(t, repo.CreateUser(ctx, rep))
	admin := auth.Principal{UserID: owner.ID, OrgID: owner.OrgID, Role: auth.RoleOwner}
	member := auth.Principal{UserID: rep.ID, OrgID: owner.OrgID, Role: auth.RoleMember}

	key := make([]byte, secrets.KeySize)
	_, _ = rand.Read(key)
	box, err := secrets.New(key)
	require.NoError(t, err)
	registry := integrations.NewRegistry()
	providers.All(registry)
	orgStore := orgs.NewStore(pool)
	svc := integrations.NewService(integrations.NewStore(pool), registry, leads.NewStore(pool, nil), orgStore, []byte("k"),
		integrations.Options{PublicURL: "https://fronko.test", Box: box, AllowPrivate: true})

	t.Run("one booking page per person, with the organisation's as the default", func(t *testing.T) {
		org, err := svc.Create(ctx, admin, integrations.CreateInput{Provider: "booking-link", Scope: integrations.ScopeOrg,
			Values: map[string]any{"url": "https://cal.com/acme"}})
		require.NoError(t, err)
		assert.Equal(t, integrations.StatusActive, org.Status)
		_, err = svc.Create(ctx, member, integrations.CreateInput{Provider: "calendly", Scope: integrations.ScopeUser,
			Values: map[string]any{"url": "https://calendly.com/rep/30min"}})
		require.NoError(t, err)
		_, err = svc.Create(ctx, member, integrations.CreateInput{Provider: "google-calendar", Scope: integrations.ScopeUser,
			Values: map[string]any{"url": "https://calendar.app.google/x"}})
		assert.ErrorIs(t, err, integrations.ErrOnlyOne)
		assert.ErrorContains(t, err, "booking page")

		bookings, err := svc.Bookings(ctx, owner.OrgID)
		require.NoError(t, err)
		assert.Equal(t, "https://calendly.com/rep/30min", bookings.For(rep.ID).URL)
		assert.Equal(t, "https://cal.com/acme", bookings.For(owner.ID).URL, "no page of their own: the default")
		assert.Equal(t, "https://cal.com/acme", bookings.For(0).URL)

		_, err = svc.Update(ctx, admin, org.ID, integrations.UpdateInput{Enabled: ptr(false)})
		require.NoError(t, err)
		bookings, err = svc.Bookings(ctx, owner.OrgID)
		require.NoError(t, err)
		assert.Nil(t, bookings.For(0), "a paused page isn't shown")
	})

	// SCIM: an Entra connection, its token, and the API behind it.
	conn, err := svc.Create(ctx, admin, integrations.CreateInput{Provider: "entra-scim"})
	require.NoError(t, err)
	assert.Equal(t, integrations.StatusPending, conn.Status, "no token yet")
	assert.Equal(t, "https://fronko.test/scim/v2", conn.Endpoints[0].Value)
	_, err = svc.Create(ctx, admin, integrations.CreateInput{Provider: "entra-scim"})
	assert.ErrorIs(t, err, integrations.ErrOnlyOne)

	first, view, err := svc.RotateToken(ctx, admin, conn.ID)
	require.NoError(t, err)
	assert.Equal(t, integrations.StatusActive, view.Status)
	require.NotNil(t, view.Token)
	assert.Equal(t, first[len(first)-4:], view.Token.Hint)
	token, _, err := svc.RotateToken(ctx, admin, conn.ID)
	require.NoError(t, err)
	_, err = svc.Authenticate(ctx, first, integrations.CategoryDirectory)
	assert.ErrorIs(t, err, integrations.ErrBadToken, "rotating revokes the old token")
	_, err = svc.Authenticate(ctx, token, integrations.CategorySSO)
	assert.ErrorIs(t, err, integrations.ErrBadToken, "a SCIM token is for SCIM only")

	ssoStore := sso.NewStore(pool)
	policy := sso.NewPolicy(svc, ssoStore, "https://fronko.test")
	userStore := users.NewStore(pool)
	authSvc := auth.NewService("0123456789abcdef")
	sent := &recordingMailer{}
	scim := directory.NewHandler(svc, directory.NewStore(pool), userStore, orgStore, teams.NewStore(pool), authSvc,
		users.NewCodes(userStore, authSvc, sent), policy, "https://fronko.test")
	routes := app.NewRoutes(ctx, func(h http.Handler) http.Handler { return h }, false, 1, 1000)
	scim.Routes(routes)
	srv := httptest.NewServer(routes.Handler(func(h http.Handler) http.Handler { return h }, func(h http.Handler) http.Handler { return h }))
	defer srv.Close()

	call := func(t *testing.T, method, path, body string) (int, map[string]any) {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+"/scim/v2"+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/scim+json")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var out map[string]any
		_ = json.Unmarshal(raw, &out)
		return resp.StatusCode, out
	}

	t.Run("SCIM needs the token", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/scim/v2/Users")
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		code, _ := call(t, "GET", "/ServiceProviderConfig", "")
		assert.Equal(t, http.StatusOK, code)
	})

	var adaID string
	t.Run("Entra's provisioning sequence", func(t *testing.T) {
		// Entra's "Test Connection": a lookup of a user that doesn't exist.
		code, list := call(t, "GET", `/Users?filter=userName+eq+%22not-there%40acme.example%22`, "")
		assert.Equal(t, http.StatusOK, code)
		assert.EqualValues(t, 0, list["totalResults"])

		code, created := call(t, "POST", "/Users", `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],
			"externalId":"e-1","userName":"ada@acme.example","active":true,"displayName":"Ada Lovelace",
			"emails":[{"primary":true,"type":"work","value":"ada@acme.example"}],
			"name":{"formatted":"Ada Lovelace","familyName":"Lovelace","givenName":"Ada"}}`)
		require.Equal(t, http.StatusCreated, code, created)
		adaID = created["id"].(string)
		assert.Equal(t, "ada@acme.example", created["userName"])
		assert.Equal(t, true, created["active"])

		ada, err := userStore.GetUserByEmail(ctx, "ada@acme.example")
		require.NoError(t, err)
		assert.Equal(t, "ada", ada.Username)
		assert.Equal(t, auth.RoleMember, ada.Role)
		assert.Equal(t, users.ProvisionedSCIM, *ada.ProvisionedBy)
		assert.True(t, ada.MustChangePassword, "no single sign-on: a temporary password")
		assert.Nil(t, ada.EmailVerifiedAt, "the domain isn't verified")
		assert.Eventually(t, func() bool { return strings.Contains(sent.subjects(), "Acme added you to Fronko") },
			2*time.Second, 10*time.Millisecond, "the welcome email is sent")

		code, again := call(t, "POST", "/Users", `{"userName":"ada@acme.example","emails":[{"value":"ada@acme.example"}]}`)
		assert.Equal(t, http.StatusConflict, code)
		assert.Equal(t, "uniqueness", again["scimType"])

		code, found := call(t, "GET", `/Users?filter=userName+eq+%22ADA%40acme.example%22`, "")
		assert.Equal(t, http.StatusOK, code)
		assert.EqualValues(t, 1, found["totalResults"])

		// An account made in Fronko is found by its email, so it's linked rather than duplicated.
		code, existing := call(t, "GET", `/Users?filter=userName+eq+%22rep%40acme.example%22`, "")
		assert.Equal(t, http.StatusOK, code)
		assert.EqualValues(t, 1, existing["totalResults"])

		code, patched := call(t, "PATCH", "/Users/"+adaID, `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations":[{"op":"Replace","path":"active","value":"False"},{"op":"Replace","path":"name.familyName","value":"King"}]}`)
		require.Equal(t, http.StatusOK, code, patched)
		assert.Equal(t, false, patched["active"])
		assert.Equal(t, "Ada King", patched["displayName"])
		ada, err = userStore.GetUserByID(ctx, ada.ID)
		require.NoError(t, err)
		assert.NotNil(t, ada.SuspendedAt, "deactivated people are suspended")

		code, _ = call(t, "PATCH", "/Users/"+adaID, `{"Operations":[{"op":"Replace","path":"active","value":true}]}`)
		assert.Equal(t, http.StatusOK, code)

		ownerID := fmt.Sprint(owner.ID)
		code, refused := call(t, "PATCH", "/Users/"+ownerID, `{"Operations":[{"op":"Replace","path":"active","value":false}]}`)
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, "mutability", refused["scimType"])
	})

	t.Run("groups are teams", func(t *testing.T) {
		code, g := call(t, "POST", "/Groups", fmt.Sprintf(`{"displayName":"Engineering","externalId":"g-1","members":[{"value":"%s"}]}`, adaID))
		require.Equal(t, http.StatusCreated, code, g)
		gid := g["id"].(string)
		assert.Len(t, g["members"], 1)

		code, _ = call(t, "PATCH", "/Groups/"+gid, fmt.Sprintf(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations":[{"op":"Add","path":"members","value":[{"value":"%d"}]},
			{"op":"Remove","path":"members[value eq \"%s\"]"}]}`, rep.ID, adaID))
		assert.Equal(t, http.StatusNoContent, code)
		code, g = call(t, "GET", "/Groups/"+gid, "")
		require.Equal(t, http.StatusOK, code)
		members := g["members"].([]any)
		require.Len(t, members, 1)
		assert.Equal(t, fmt.Sprint(rep.ID), members[0].(map[string]any)["value"])

		code, found := call(t, "GET", `/Groups?filter=displayName+eq+%22engineering%22&excludedAttributes=members`, "")
		assert.Equal(t, http.StatusOK, code)
		assert.EqualValues(t, 1, found["totalResults"])
		assert.Nil(t, found["Resources"].([]any)[0].(map[string]any)["members"])

		code, _ = call(t, "DELETE", "/Groups/"+gid, "")
		assert.Equal(t, http.StatusNoContent, code)
	})

	t.Run("deleting a user", func(t *testing.T) {
		code, _ := call(t, "DELETE", "/Users/"+adaID, "")
		assert.Equal(t, http.StatusNoContent, code)
		_, err := userStore.GetUserByEmail(ctx, "ada@acme.example")
		assert.ErrorIs(t, err, database.ErrNotFound)
	})

	t.Run("single sign-on policy", func(t *testing.T) {
		_, blocked, err := policy.PasswordBlocked(ctx, rep)
		require.NoError(t, err)
		assert.False(t, blocked, "no single sign-on yet")

		samlConn, err := svc.Create(ctx, admin, integrations.CreateInput{Provider: "saml",
			Values: map[string]any{"metadata_url": "https://idp.acme.example/metadata", "enforce": true}})
		require.NoError(t, err)
		assert.Equal(t, integrations.StatusActive, samlConn.Status)
		assert.Len(t, samlConn.Endpoints, 4)
		assert.False(t, samlConn.Secrets["sp_key"], "internal keys aren't listed")

		path, blocked, err := policy.PasswordBlocked(ctx, rep)
		require.NoError(t, err)
		assert.True(t, blocked)
		org, _ := orgStore.GetOrganization(ctx, owner.OrgID)
		assert.Equal(t, "/auth/sso/"+org.Handle, path)
		_, blocked, err = policy.PasswordBlocked(ctx, owner)
		require.NoError(t, err)
		assert.False(t, blocked, "the owner keeps their password")

		_, err = svc.Update(ctx, admin, samlConn.ID, integrations.UpdateInput{Values: map[string]any{"enforce": false}})
		require.NoError(t, err)
		_, blocked, _ = policy.PasswordBlocked(ctx, rep)
		assert.False(t, blocked)
		ssoOnly := &users.User{OrgID: owner.OrgID, Role: auth.RoleMember, Username: "id-sso", Email: ptr("sso@acme.example")}
		require.NoError(t, repo.CreateUser(ctx, ssoOnly))
		got, err := userStore.GetUserByID(ctx, ssoOnly.ID)
		require.NoError(t, err)
		assert.False(t, got.HasPassword())
		_, blocked, _ = policy.PasswordBlocked(ctx, got)
		assert.True(t, blocked, "no password: single sign-on only")

		url, err := policy.SSOSignInURL(ctx, owner.OrgID)
		require.NoError(t, err)
		assert.Equal(t, "https://fronko.test/login/sso/"+org.Handle, url)

		_, err = pool.Exec(ctx, `INSERT INTO org_domains (org_id, domain, verification_token, verified_at) VALUES ($1, 'acme.example', 't', now())`, owner.OrgID)
		require.NoError(t, err)
		ok, err := policy.VerifiedEmail(ctx, owner.OrgID, "new@acme.example")
		require.NoError(t, err)
		assert.True(t, ok)

		// With single sign-on on, SCIM adds people without a password.
		code, created := call(t, "POST", "/Users", `{"userName":"grace@acme.example","emails":[{"value":"grace@acme.example","primary":true}]}`)
		require.Equal(t, http.StatusCreated, code, created)
		grace, err := userStore.GetUserByEmail(ctx, "grace@acme.example")
		require.NoError(t, err)
		assert.False(t, grace.HasPassword())
		assert.False(t, grace.MustChangePassword)
		assert.NotNil(t, grace.EmailVerifiedAt, "the domain is verified")
	})
}

// recordingMailer keeps the subjects of the emails it's asked to send.
type recordingMailer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (m *recordingMailer) Send(_ context.Context, _ string, msg mail.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buf.WriteString(msg.Subject + "\n")
	return nil
}

func (m *recordingMailer) subjects() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.String()
}
