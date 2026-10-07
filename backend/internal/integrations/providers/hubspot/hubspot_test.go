package hubspot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/faiz-gh/fronko/backend/internal/integrations"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
)

// fakeHubSpot answers like HubSpot's contacts and notes APIs.
type fakeHubSpot struct {
	upserts    []upsertRequest
	notes      []map[string]any
	auth       []string
	upsertCode int
	noteCode   int
	isNew      bool
}

func (f *fakeHubSpot) serve(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/crm/v3/objects/contacts/batch/upsert":
			var req upsertRequest
			require.NoError(t, json.Unmarshal(body, &req))
			f.upserts = append(f.upserts, req)
			if f.upsertCode != 0 {
				w.WriteHeader(f.upsertCode)
				_, _ = w.Write([]byte(`{"status":"error","message":"Property values were not valid","category":"VALIDATION_ERROR"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"COMPLETE","results":[{"id":"501","new":` + map[bool]string{true: "true", false: "false"}[f.isNew] + `}]}`))
		case "/crm/v3/objects/notes":
			var note map[string]any
			require.NoError(t, json.Unmarshal(body, &note))
			f.notes = append(f.notes, note)
			if f.noteCode != 0 {
				w.WriteHeader(f.noteCode)
				_, _ = w.Write([]byte(`{"message":"missing scope"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"9001"}`))
		case "/oauth/v1/token":
			form, _ := url.ParseQuery(string(body))
			assert.Equal(t, "the-id", form.Get("client_id"), "HubSpot wants the client in the body")
			_, _ = w.Write([]byte(`{"access_token":"fresh","refresh_token":"r2","expires_in":1800,"token_type":"bearer"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func lead() *integrations.Lead {
	return &integrations.Lead{
		ID: 7, CreatedAt: time.Date(2026, 10, 7, 9, 30, 0, 0, time.UTC),
		Name: "Ada King Lovelace", Email: "ada@example.com", Phone: "+441234567890",
		Notes: "Loved the talk.\nCall me <soon>.", Source: "nfc",
		Card:  integrations.LeadCard{ID: 1, Slug: "maya", Name: "Maya Chen", URL: "https://cards.example.com/p/lumen/maya"},
		Owner: &integrations.LeadOwner{ID: 2, Username: "maya"},
	}
}

func setup(t *testing.T, f *fakeHubSpot, values map[string]any) (*Provider, *integrations.Call) {
	srv := f.serve(t)
	p := &Provider{AuthURL: srv.URL + "/oauth/authorize", TokenURL: srv.URL + "/oauth/v1/token", APIBase: srv.URL}
	return p, &integrations.Call{
		Connection: &integrations.Connection{ID: 3},
		Settings:   integrations.Settings{Values: values},
		HTTP:       srv.Client(),
	}
}

func TestPushLeadUpsertsByEmailAndAddsANote(t *testing.T) {
	f := &fakeHubSpot{isNew: true}
	p, call := setup(t, f, nil)

	res, err := p.PushLead(context.Background(), call, lead())
	require.NoError(t, err)
	assert.Equal(t, "Created contact ada@example.com", res.Summary)
	assert.Equal(t, "501", res.Detail["contact_id"])

	require.Len(t, f.upserts, 1)
	in := f.upserts[0].Inputs[0]
	assert.Equal(t, "email", in.IDProperty)
	assert.Equal(t, "ada@example.com", in.ID)
	assert.Equal(t, map[string]string{"email": "ada@example.com", "firstname": "Ada", "lastname": "King Lovelace",
		"phone": "+441234567890"}, in.Properties)

	require.Len(t, f.notes, 1, "add_note defaults to on")
	body := f.notes[0]["properties"].(map[string]any)["hs_note_body"].(string)
	assert.Contains(t, body, "tapped the NFC card")
	assert.Contains(t, body, "Call me &lt;soon&gt;.", "the visitor's message is escaped")
	assert.Contains(t, body, "<br>", "line breaks are kept")
	assoc := f.notes[0]["associations"].([]any)[0].(map[string]any)
	assert.Equal(t, "501", assoc["to"].(map[string]any)["id"])
}

func TestPushLeadUpdatesAndCanSkipTheNote(t *testing.T) {
	f := &fakeHubSpot{}
	p, call := setup(t, f, map[string]any{"add_note": false})
	res, err := p.PushLead(context.Background(), call, lead())
	require.NoError(t, err)
	assert.Equal(t, "Updated contact ada@example.com", res.Summary)
	assert.Empty(t, f.notes)
}

func TestAFailedNoteDoesntFailTheLead(t *testing.T) {
	f := &fakeHubSpot{noteCode: http.StatusForbidden}
	p, call := setup(t, f, nil)
	res, err := p.PushLead(context.Background(), call, lead())
	require.NoError(t, err, "retrying would upsert the contact again")
	assert.Contains(t, res.Summary, "the note couldn't be added")
}

func TestPushLeadErrors(t *testing.T) {
	for _, tc := range []struct {
		code      int
		permanent bool
		message   string
	}{
		{http.StatusBadRequest, true, "Property values were not valid"},
		{http.StatusUnauthorized, true, "authorise the connection again"},
		{http.StatusForbidden, true, "missing a scope"},
		{http.StatusTooManyRequests, false, "answered 429"},
		{http.StatusBadGateway, false, "answered 502"},
	} {
		f := &fakeHubSpot{upsertCode: tc.code}
		p, call := setup(t, f, nil)
		_, err := p.PushLead(context.Background(), call, lead())
		require.Error(t, err, tc.code)
		assert.Equal(t, tc.permanent, jobs.IsPermanent(err), tc.code)
		assert.Contains(t, err.Error(), tc.message, tc.code)
	}
}

func TestOAuthSpecRefreshesWithTheClientInTheBody(t *testing.T) {
	f := &fakeHubSpot{isNew: true}
	p, _ := setup(t, f, nil)
	spec := p.OAuth(integrations.Settings{})
	assert.Equal(t, oauth2.AuthStyleInParams, spec.AuthStyle)
	assert.ElementsMatch(t, []string{"oauth", "crm.objects.contacts.read", "crm.objects.contacts.write"}, spec.Scopes)

	cfg := &oauth2.Config{ClientID: "the-id", ClientSecret: "s", Endpoint: oauth2.Endpoint{TokenURL: spec.TokenURL, AuthStyle: spec.AuthStyle}}
	expired := &oauth2.Token{AccessToken: "old", RefreshToken: "r1", Expiry: time.Now().Add(-time.Minute)}
	client := cfg.Client(context.Background(), expired)
	_, err := p.PushLead(context.Background(), &integrations.Call{Connection: &integrations.Connection{ID: 1},
		Settings: integrations.Settings{Values: map[string]any{"add_note": false}}, HTTP: client}, lead())
	require.NoError(t, err)
	assert.Equal(t, "Bearer fresh", f.auth[len(f.auth)-1])
}

func TestValidateRejectsAMangledClientID(t *testing.T) {
	err := New().Validate(context.Background(), integrations.Settings{Values: map[string]any{"client_id": "abc def"}})
	var fe *integrations.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "client_id", fe.Field)
	assert.NoError(t, New().Validate(context.Background(), integrations.Settings{Values: map[string]any{"client_id": "1a2b-3c"}}))
	assert.True(t, strings.HasPrefix(New().Manifest().DocsURL, "https://"))
}
