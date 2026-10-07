//go:build integration

package integrationtest_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/cards"
	"github.com/faiz-gh/fronko/backend/internal/integrations"
	"github.com/faiz-gh/fronko/backend/internal/integrations/providers/webhook"
	"github.com/faiz-gh/fronko/backend/internal/leads"
	"github.com/faiz-gh/fronko/backend/internal/orgs"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/events"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
	"github.com/faiz-gh/fronko/backend/internal/platform/secrets"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// hookReceiver records webhook deliveries and answers with a settable status.
type hookReceiver struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []webhook.Envelope
	status atomic.Int32
}

func newHookReceiver(t *testing.T) *hookReceiver {
	h := &hookReceiver{}
	h.status.Store(http.StatusOK)
	h.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env webhook.Envelope
		_ = json.NewDecoder(r.Body).Decode(&env)
		h.mu.Lock()
		h.bodies = append(h.bodies, env)
		h.mu.Unlock()
		w.WriteHeader(int(h.status.Load()))
	}))
	t.Cleanup(h.Close)
	return h
}

func (h *hookReceiver) received() []webhook.Envelope {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]webhook.Envelope(nil), h.bodies...)
}

// fakeOAuth is an OAuth lead sync provider whose authorisation server and
// API are an httptest server.
type fakeOAuth struct {
	srv       *httptest.Server
	mu        sync.Mutex
	exchanges []url.Values
	refreshes int
	pushes    []string // Authorization headers seen by the API
}

func newFakeOAuth(t *testing.T) *fakeOAuth {
	f := &fakeOAuth{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			form := r.PostForm
			id, secret, _ := r.BasicAuth()
			form.Set("basic_id", id)
			form.Set("basic_secret", secret)
			w.Header().Set("Content-Type", "application/json")
			switch form.Get("grant_type") {
			case "authorization_code":
				if form.Get("code") != "good-code" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = io.WriteString(w, `{"error":"invalid_grant","error_description":"bad code"}`)
					return
				}
				f.exchanges = append(f.exchanges, form)
				_, _ = io.WriteString(w, `{"access_token":"access-1","refresh_token":"refresh-1","token_type":"Bearer","expires_in":-10}`)
			case "refresh_token":
				f.refreshes++
				fmt.Fprintf(w, `{"access_token":"access-%d","refresh_token":"refresh-%d","token_type":"Bearer","expires_in":3600}`,
					f.refreshes+1, f.refreshes+1)
			}
		case "/api/leads":
			f.pushes = append(f.pushes, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusCreated)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeOAuth) Manifest() integrations.Manifest {
	return integrations.Manifest{
		ID: "fake-crm", Name: "Fake CRM", Category: integrations.CategoryLeadSync, Description: "test",
		Scopes: []integrations.Scope{integrations.ScopeOrg}, Auth: integrations.AuthOAuth2, Status: integrations.Available,
		Fields: []integrations.Field{
			{Key: "client_id", Label: "Client ID", Type: integrations.FieldText, Required: true},
			{Key: "client_secret", Label: "Client secret", Type: integrations.FieldSecret, Required: true},
		},
	}
}

func (f *fakeOAuth) Validate(context.Context, integrations.Settings) error { return nil }

func (f *fakeOAuth) OAuth(integrations.Settings) integrations.OAuthSpec {
	return integrations.OAuthSpec{AuthURL: f.srv.URL + "/authorize", TokenURL: f.srv.URL + "/token", Scopes: []string{"crm.write"}, PKCE: true}
}

func (f *fakeOAuth) PushLead(ctx context.Context, call *integrations.Call, lead *integrations.Lead) (integrations.Result, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, f.srv.URL+"/api/leads", strings.NewReader("{}"))
	resp, err := call.HTTP.Do(req)
	if err != nil {
		return integrations.Result{}, err
	}
	_ = resp.Body.Close()
	return integrations.Result{Summary: "pushed"}, nil
}

func TestIntegrationsIntegration(t *testing.T) {
	dbUrl := os.Getenv("TEST_DATABASE_URL")
	if dbUrl == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration tests")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, dbUrl)
	require.NoError(t, err)
	defer pool.Close()
	_, err = pool.Exec(ctx, "TRUNCATE TABLE organizations, users, profiles, leads, jobs, integration_connections, integration_activity RESTART IDENTITY CASCADE")
	require.NoError(t, err)
	repo := newStores(pool)

	// An organisation with an owner, a member holding a card, and another member.
	owner := &users.User{Username: "int-owner", PasswordHash: "hash"}
	require.NoError(t, repo.CreateOrgWithOwner(ctx, "Integrations Org", owner))
	newMember := func(name string) *users.User {
		u := &users.User{OrgID: owner.OrgID, Role: auth.RoleMember, Username: name, Email: ptr(name + "@example.com"), PasswordHash: "hash", CreatedBy: &owner.ID}
		require.NoError(t, repo.CreateUser(ctx, u))
		return u
	}
	rep, other := newMember("int-rep"), newMember("int-other")
	repCard := &cards.Profile{OrgID: owner.OrgID, UserID: owner.ID, AssignedUserID: &rep.ID, Slug: "rep-card", Data: json.RawMessage(`{"name":"Rep Card"}`)}
	require.NoError(t, repo.CreateProfile(ctx, repCard))

	admin := auth.Principal{UserID: owner.ID, OrgID: owner.OrgID, Role: auth.RoleOwner}
	member := auth.Principal{UserID: rep.ID, OrgID: owner.OrgID, Role: auth.RoleMember}
	outsider := auth.Principal{UserID: other.ID, OrgID: owner.OrgID, Role: auth.RoleMember}

	key := make([]byte, secrets.KeySize)
	_, _ = rand.Read(key)
	box, err := secrets.New(key)
	require.NoError(t, err)
	oauthFake := newFakeOAuth(t)
	registry := integrations.NewRegistry()
	registry.Register(webhook.New())
	registry.Register(oauthFake)

	bus := events.New()
	leadStore := leads.NewStore(pool, bus)
	svc := integrations.NewService(integrations.NewStore(pool), registry, leadStore, orgs.NewStore(pool), []byte("state-key"),
		integrations.Options{PublicURL: "https://fronko.test", Box: box, AllowPrivate: true})
	svc.Subscribe(bus)

	hook := newHookReceiver(t)
	createHook := func(t *testing.T, p auth.Principal, scope integrations.Scope, url string) *integrations.View {
		t.Helper()
		v, err := svc.Create(ctx, p, integrations.CreateInput{Provider: "webhook", Scope: scope,
			Values: map[string]any{"url": url}, Secrets: map[string]string{"signing_secret": "whsec-123"}})
		require.NoError(t, err)
		return v
	}
	// runJobs runs queued jobs until none are left that are due.
	runJobs := func(t *testing.T) {
		t.Helper()
		w := jobs.NewWorker(pool, svc.JobHandlers(), 1)
		w.PollInterval = 20 * time.Millisecond
		wctx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() { w.Run(wctx); close(done) }()
		require.Eventually(t, func() bool {
			var n int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE status IN ('queued','running') AND run_at <= now()`).Scan(&n))
			return n == 0
		}, 5*time.Second, 20*time.Millisecond)
		cancel()
		<-done
	}
	newLead := func(t *testing.T, email string) *leads.Lead {
		t.Helper()
		l := &leads.Lead{ProfileID: repCard.ID, Name: "Lead " + email, Email: email, PhoneCountryCode: "+91", PhoneNumber: "9876543210"}
		require.NoError(t, leadStore.CreateLead(ctx, l))
		return l
	}
	reset := func(t *testing.T) {
		t.Helper()
		_, err := pool.Exec(ctx, "TRUNCATE TABLE jobs, integration_connections, integration_activity RESTART IDENTITY CASCADE")
		require.NoError(t, err)
		hook.mu.Lock()
		hook.bodies = nil
		hook.mu.Unlock()
		hook.status.Store(http.StatusOK)
	}

	t.Run("connections belong to the organisation or to one person", func(t *testing.T) {
		reset(t)
		orgHook := createHook(t, admin, integrations.ScopeOrg, hook.URL)
		assert.Equal(t, integrations.StatusActive, orgHook.Status)
		assert.Equal(t, integrations.ScopeOrg, orgHook.Scope)
		assert.Equal(t, map[string]bool{"signing_secret": true}, orgHook.Secrets)
		assert.Empty(t, orgHook.Missing)
		assert.Equal(t, owner.ID, orgHook.CreatedBy.ID)

		_, err := svc.Create(ctx, member, integrations.CreateInput{Provider: "webhook", Scope: integrations.ScopeOrg,
			Values: map[string]any{"url": hook.URL}})
		assert.ErrorIs(t, err, integrations.ErrForbidden, "members can't add organisation connections")
		mine := createHook(t, member, integrations.ScopeUser, hook.URL)

		_, err = svc.Get(ctx, outsider, mine.ID)
		assert.ErrorIs(t, err, database.ErrNotFound, "someone else's personal connection")
		_, err = svc.Get(ctx, admin, mine.ID)
		assert.ErrorIs(t, err, database.ErrNotFound, "admins don't manage personal connections")
		_, err = svc.Get(ctx, member, orgHook.ID)
		assert.ErrorIs(t, err, database.ErrNotFound, "members don't manage the organisation's")

		_, err = svc.List(ctx, member, integrations.ScopeOrg, "")
		assert.ErrorIs(t, err, integrations.ErrForbidden)
		list, err := svc.List(ctx, member, "", "")
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, mine.ID, list[0].ID)
		list, err = svc.List(ctx, admin, "", "webhook")
		require.NoError(t, err)
		assert.Len(t, list, 1, "an admin's list holds the organisation's and their own")

		// Secrets are sealed, never stored or returned in the clear.
		var raw []byte
		require.NoError(t, pool.QueryRow(ctx, `SELECT secrets FROM integration_connections WHERE connection_id = $1`, orgHook.ID).Scan(&raw))
		assert.NotContains(t, string(raw), "whsec-123")
		body, _ := json.Marshal(orgHook)
		assert.NotContains(t, string(body), "whsec-123")
	})

	t.Run("incomplete connections stay pending until finished", func(t *testing.T) {
		reset(t)
		v, err := svc.Create(ctx, admin, integrations.CreateInput{Provider: "webhook", Name: "  Zapier  "})
		require.NoError(t, err)
		assert.Equal(t, integrations.StatusPending, v.Status)
		assert.Equal(t, "Zapier", v.Name)
		assert.Equal(t, []string{"Payload URL"}, v.Missing)
		_, err = svc.Test(ctx, admin, v.ID)
		assert.ErrorIs(t, err, integrations.ErrNotReady)

		_, err = svc.Update(ctx, admin, v.ID, integrations.UpdateInput{Values: map[string]any{"url": "ftp://nope"}})
		var fe *integrations.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "url", fe.Field)

		v, err = svc.Update(ctx, admin, v.ID, integrations.UpdateInput{Values: map[string]any{"url": hook.URL}})
		require.NoError(t, err)
		assert.Equal(t, integrations.StatusActive, v.Status)
		assert.Equal(t, map[string]bool{"signing_secret": false}, v.Secrets)
	})

	t.Run("leads go to the organisation's and the card holder's connections", func(t *testing.T) {
		reset(t)
		orgHook := createHook(t, admin, integrations.ScopeOrg, hook.URL+"/org")
		repHook := createHook(t, member, integrations.ScopeUser, hook.URL+"/rep")
		createHook(t, outsider, integrations.ScopeUser, hook.URL+"/other")
		paused := createHook(t, admin, integrations.ScopeOrg, hook.URL+"/paused")
		off := false
		_, err := svc.Update(ctx, admin, paused.ID, integrations.UpdateInput{Enabled: &off})
		require.NoError(t, err)

		lead := newLead(t, "sync@example.com")
		var queued []int64
		rows, err := pool.Query(ctx, `SELECT (payload->>'connection_id')::bigint FROM jobs WHERE kind = $1 ORDER BY 1`, integrations.JobPushLead)
		require.NoError(t, err)
		for rows.Next() {
			var id int64
			require.NoError(t, rows.Scan(&id))
			queued = append(queued, id)
		}
		assert.Equal(t, []int64{orgHook.ID, repHook.ID}, queued)

		runJobs(t)
		got := hook.received()
		require.Len(t, got, 2)
		for _, env := range got {
			assert.Equal(t, "lead.created", env.Event)
			assert.Equal(t, lead.ID, env.Data.ID)
			assert.Equal(t, "+919876543210", env.Data.Phone)
			assert.Equal(t, "rep-card", env.Data.Card.Slug)
			assert.Equal(t, "Rep Card", env.Data.Card.Name)
			assert.Equal(t, "https://fronko.test/p/"+env.Data.Organisation.Handle+"/rep-card", env.Data.Card.URL)
			require.NotNil(t, env.Data.Owner)
			assert.Equal(t, "int-rep", env.Data.Owner.Username)
			assert.Equal(t, "int-rep@example.com", env.Data.Owner.Email)
		}

		activity, err := svc.Activity(ctx, admin, orgHook.ID, 0, 10)
		require.NoError(t, err)
		require.NotEmpty(t, activity)
		assert.Equal(t, integrations.OutcomeSuccess, activity[0].Outcome)
		assert.Equal(t, &lead.ID, activity[0].LeadID)
		assert.Equal(t, 200, int(activity[0].Detail["status"].(float64)))
		v, err := svc.Get(ctx, admin, orgHook.ID)
		require.NoError(t, err)
		assert.NotNil(t, v.LastSyncedAt)
	})

	t.Run("failures retry, then stop the connection after three in a row", func(t *testing.T) {
		reset(t)
		h := createHook(t, admin, integrations.ScopeOrg, hook.URL)

		hook.status.Store(http.StatusServiceUnavailable)
		newLead(t, "retry@example.com")
		runJobs(t)
		var status string
		var attempts int
		require.NoError(t, pool.QueryRow(ctx, `SELECT status, attempts FROM jobs`).Scan(&status, &attempts))
		assert.Equal(t, "queued", status, "a 503 is retried later")
		assert.Equal(t, 1, attempts)
		activity, err := svc.Activity(ctx, admin, h.ID, 0, 1)
		require.NoError(t, err)
		assert.Equal(t, integrations.OutcomeRetrying, activity[0].Outcome)
		assert.Equal(t, 1, *activity[0].Attempt)

		_, err = pool.Exec(ctx, `DELETE FROM jobs`)
		require.NoError(t, err)
		hook.status.Store(http.StatusBadRequest)
		for i := range 3 {
			newLead(t, fmt.Sprintf("reject%d@example.com", i))
			runJobs(t)
		}
		v, err := svc.Get(ctx, admin, h.ID)
		require.NoError(t, err)
		assert.Equal(t, integrations.StatusError, v.Status)
		assert.Equal(t, 3, v.FailureCount)
		require.NotNil(t, v.LastError)
		assert.Contains(t, *v.LastError, "rejected the lead (400)")

		// A connection in error gets no new work...
		before := len(hook.received())
		newLead(t, "ignored@example.com")
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE status = 'queued'`).Scan(&n))
		assert.Zero(t, n)
		assert.Len(t, hook.received(), before)

		// ...until someone fixes its settings.
		hook.status.Store(http.StatusOK)
		v, err = svc.Update(ctx, admin, h.ID, integrations.UpdateInput{Values: map[string]any{"url": hook.URL + "/fixed"}})
		require.NoError(t, err)
		assert.Equal(t, integrations.StatusActive, v.Status)
		assert.Zero(t, v.FailureCount)
		assert.Nil(t, v.LastError)
	})

	t.Run("a test sends a sample lead and logs the outcome", func(t *testing.T) {
		reset(t)
		h := createHook(t, admin, integrations.ScopeOrg, hook.URL)
		res, err := svc.Test(ctx, admin, h.ID)
		require.NoError(t, err)
		assert.True(t, res.OK)
		got := hook.received()
		require.Len(t, got, 1)
		assert.True(t, got[0].Test)
		assert.Equal(t, "Integrations Org", got[0].Data.Organisation.Name)

		hook.status.Store(http.StatusUnauthorized)
		res, err = svc.Test(ctx, admin, h.ID)
		require.NoError(t, err)
		assert.False(t, res.OK)
		assert.Contains(t, res.Summary, "401")
		activity, err := svc.Activity(ctx, admin, h.ID, 0, 10)
		require.NoError(t, err)
		require.Len(t, activity, 3, "created, test passed, test failed")
		assert.Equal(t, integrations.OutcomeFailed, activity[0].Outcome)
		assert.Equal(t, "int-owner", activity[0].User.Username)
		page, err := svc.Activity(ctx, admin, h.ID, activity[1].ID, 10)
		require.NoError(t, err)
		require.Len(t, page, 1)
		assert.Equal(t, integrations.ActivitySetup, page[0].Kind)
	})

	t.Run("only one connection per owner unless the provider allows more", func(t *testing.T) {
		reset(t)
		in := integrations.CreateInput{Provider: "fake-crm", Values: map[string]any{"client_id": "cid"}, Secrets: map[string]string{"client_secret": "cs"}}
		_, err := svc.Create(ctx, admin, in)
		require.NoError(t, err)
		_, err = svc.Create(ctx, admin, in)
		assert.ErrorIs(t, err, integrations.ErrOnlyOne)
		createHook(t, admin, integrations.ScopeOrg, hook.URL)
		createHook(t, admin, integrations.ScopeOrg, hook.URL)
		_, err = svc.Create(ctx, admin, integrations.CreateInput{Provider: "nope"})
		assert.ErrorIs(t, err, integrations.ErrUnknownProvider)
	})

	t.Run("OAuth authorises with the organisation's own app and refreshes tokens", func(t *testing.T) {
		reset(t)
		v, err := svc.Create(ctx, admin, integrations.CreateInput{Provider: "fake-crm",
			Values: map[string]any{"client_id": "client-123"}, Secrets: map[string]string{"client_secret": "secret-456"}})
		require.NoError(t, err)
		assert.Equal(t, integrations.StatusPending, v.Status, "not authorised yet")
		assert.False(t, v.Authorized)

		authURL, cookie, err := svc.StartOAuth(ctx, admin, v.ID)
		require.NoError(t, err)
		u, err := url.Parse(authURL)
		require.NoError(t, err)
		q := u.Query()
		assert.Equal(t, oauthFake.srv.URL+"/authorize", u.Scheme+"://"+u.Host+u.Path)
		assert.Equal(t, "client-123", q.Get("client_id"))
		assert.Equal(t, "https://fronko.test/api/integrations/oauth/callback", q.Get("redirect_uri"))
		assert.Equal(t, "crm.write", q.Get("scope"))
		assert.Equal(t, "S256", q.Get("code_challenge_method"))
		state := q.Get("state")

		_, err = svc.FinishOAuth(ctx, outsider, state, "good-code", cookie)
		assert.ErrorIs(t, err, integrations.ErrOAuthState, "another user can't finish it")
		_, err = svc.FinishOAuth(ctx, admin, state, "good-code", "wrong-nonce")
		assert.ErrorIs(t, err, integrations.ErrOAuthState, "the browser that started it must finish it")
		_, err = svc.FinishOAuth(ctx, admin, state, "bad-code", cookie)
		assert.ErrorContains(t, err, "bad code")

		provider, err := svc.FinishOAuth(ctx, admin, state, "good-code", cookie)
		require.NoError(t, err)
		assert.Equal(t, "fake-crm", provider)
		require.Len(t, oauthFake.exchanges, 1)
		ex := oauthFake.exchanges[0]
		assert.NotEmpty(t, ex.Get("code_verifier"), "PKCE verifier from the cookie")
		assert.Equal(t, "https://fronko.test/api/integrations/oauth/callback", ex.Get("redirect_uri"))
		assert.Contains(t, []string{ex.Get("client_id"), ex.Get("basic_id")}, "client-123")

		v, err = svc.Get(ctx, admin, v.ID)
		require.NoError(t, err)
		assert.True(t, v.Authorized)
		assert.Equal(t, integrations.StatusActive, v.Status)

		// The first token was already expired: using it refreshes and saves the new one.
		res, err := svc.Test(ctx, admin, v.ID)
		require.NoError(t, err)
		require.True(t, res.OK, res.Summary)
		res, err = svc.Test(ctx, admin, v.ID)
		require.NoError(t, err)
		require.True(t, res.OK, res.Summary)
		assert.Equal(t, 1, oauthFake.refreshes, "the refreshed token was stored and reused")
		assert.Equal(t, []string{"Bearer access-2", "Bearer access-2"}, oauthFake.pushes)

		// New app credentials need a new authorisation.
		v, err = svc.Update(ctx, admin, v.ID, integrations.UpdateInput{Values: map[string]any{"client_id": "client-789"}})
		require.NoError(t, err)
		assert.False(t, v.Authorized)
		assert.Equal(t, integrations.StatusPending, v.Status)
	})

	t.Run("the browser's OAuth round trip through the handlers", func(t *testing.T) {
		reset(t)
		v, err := svc.Create(ctx, admin, integrations.CreateInput{Provider: "fake-crm",
			Values: map[string]any{"client_id": "client-123"}, Secrets: map[string]string{"client_secret": "secret-456"}})
		require.NoError(t, err)
		h := integrations.NewHandler(svc, true)
		as := func(r *http.Request, p auth.Principal) *http.Request {
			return r.WithContext(auth.WithPrincipal(r.Context(), p))
		}

		start := httptest.NewRequest(http.MethodGet, "/api/integrations/connections/x/oauth/start", nil)
		start.SetPathValue("id", fmt.Sprint(v.ID))
		w := httptest.NewRecorder()
		h.StartOAuth(w, as(start, admin))
		require.Equal(t, http.StatusFound, w.Code)
		loc, err := url.Parse(w.Header().Get("Location"))
		require.NoError(t, err)
		assert.Equal(t, oauthFake.srv.URL+"/authorize", loc.Scheme+"://"+loc.Host+loc.Path)
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		c := cookies[0]
		assert.Equal(t, "fronko_oauth", c.Name)
		assert.True(t, c.HttpOnly && c.Secure)
		assert.Equal(t, "/api/integrations/oauth", c.Path)

		callback := func(query string, withCookie bool) *url.URL {
			r := httptest.NewRequest(http.MethodGet, "/api/integrations/oauth/callback?"+query, nil)
			if withCookie {
				r.AddCookie(c)
			}
			w := httptest.NewRecorder()
			h.OAuthCallback(w, as(r, admin))
			require.Equal(t, http.StatusFound, w.Code)
			back, err := url.Parse(w.Header().Get("Location"))
			require.NoError(t, err)
			return back
		}
		state := url.QueryEscape(loc.Query().Get("state"))

		back := callback("state="+state+"&error=access_denied", true)
		assert.Equal(t, "/dashboard/integrations/fake-crm", back.Path)
		assert.Equal(t, "authorisation was cancelled", back.Query().Get("oauth_error"))

		back = callback("state="+state+"&code=good-code", false)
		assert.Contains(t, back.Query().Get("oauth_error"), "expired or wasn't started here", "no cookie, no token")

		back = callback("state="+state+"&code=good-code", true)
		assert.Equal(t, "https", back.Scheme)
		assert.Equal(t, "fronko.test", back.Host)
		assert.Equal(t, "/dashboard/integrations/fake-crm", back.Path)
		assert.Equal(t, "connected", back.Query().Get("oauth"))
		assert.Equal(t, fmt.Sprint(v.ID), back.Query().Get("connection"))
		v, err = svc.Get(ctx, admin, v.ID)
		require.NoError(t, err)
		assert.True(t, v.Authorized)

		back = callback("state=forged&code=good-code", true)
		assert.Equal(t, "/dashboard/integrations", back.Path)
		assert.NotEmpty(t, back.Query().Get("oauth_error"))
	})

	t.Run("deleting a connection removes its log", func(t *testing.T) {
		reset(t)
		h := createHook(t, admin, integrations.ScopeOrg, hook.URL)
		assert.ErrorIs(t, svc.Delete(ctx, member, h.ID), database.ErrNotFound)
		require.NoError(t, svc.Delete(ctx, admin, h.ID))
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM integration_activity`).Scan(&n))
		assert.Zero(t, n)
		_, err := svc.Get(ctx, admin, h.ID)
		assert.True(t, errors.Is(err, database.ErrNotFound))
	})
}
