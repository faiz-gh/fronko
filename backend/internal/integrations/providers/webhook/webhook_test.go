package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/faiz-gh/fronko/backend/internal/integrations"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
	"github.com/faiz-gh/fronko/backend/internal/platform/netguard"
)

var fixedNow = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

type received struct {
	header http.Header
	body   []byte
}

// receiver answers each request with the next status and records it.
func receiver(t *testing.T, statuses ...int) (*httptest.Server, *[]received) {
	t.Helper()
	var got []received
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, received{r.Header.Clone(), body})
		status := http.StatusOK
		if len(got) <= len(statuses) {
			status = statuses[len(got)-1]
		}
		if status == http.StatusFound {
			w.Header().Set("Location", "https://example.com/elsewhere")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"ok":false,"why":"just testing"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func callFor(url, secret string, allowPrivate bool) *integrations.Call {
	return &integrations.Call{
		Connection: &integrations.Connection{ID: 9},
		Settings: integrations.Settings{
			Values:       map[string]any{"url": url},
			Secrets:      map[string]string{"signing_secret": secret},
			AllowPrivate: allowPrivate,
		},
		HTTP: netguard.Client(netguard.Options{AllowPrivate: allowPrivate}),
	}
}

func lead() *integrations.Lead {
	return &integrations.Lead{ID: 42, Name: "Ada", Email: "ada@example.com", Card: integrations.LeadCard{ID: 1, Slug: "maya"}}
}

func TestDeliversASignedVersionedEnvelope(t *testing.T) {
	srv, got := receiver(t)
	p := New()
	p.Now = func() time.Time { return fixedNow }

	res, err := p.PushLead(context.Background(), callFor(srv.URL, "s3cret", true), lead())
	require.NoError(t, err)
	assert.Contains(t, res.Summary, "Delivered to")
	assert.Equal(t, 200, res.Detail["status"])

	require.Len(t, *got, 1)
	r := (*got)[0]
	assert.Equal(t, "application/json", r.header.Get("Content-Type"))
	assert.Equal(t, "lead.created", r.header.Get("X-Fronko-Event"))
	assert.Equal(t, "lead-42-9", r.header.Get("X-Fronko-Delivery"), "stable across retries")
	assert.True(t, Verify("s3cret", r.header.Get("X-Fronko-Signature"), r.body, 5*time.Minute, fixedNow))
	assert.False(t, Verify("wrong", r.header.Get("X-Fronko-Signature"), r.body, 5*time.Minute, fixedNow))
	assert.False(t, Verify("s3cret", r.header.Get("X-Fronko-Signature"), r.body, 5*time.Minute, fixedNow.Add(time.Hour)), "too old")
	assert.False(t, Verify("s3cret", r.header.Get("X-Fronko-Signature"), append(r.body, ' '), 5*time.Minute, fixedNow))

	var env struct {
		Event   string
		Version int
		Test    bool
		Data    struct {
			ID    int64
			Email string
			Card  struct{ Slug string }
		}
	}
	require.NoError(t, json.Unmarshal(r.body, &env))
	assert.Equal(t, "lead.created", env.Event)
	assert.Equal(t, 1, env.Version)
	assert.False(t, env.Test)
	assert.Equal(t, int64(42), env.Data.ID)
	assert.Equal(t, "maya", env.Data.Card.Slug)
}

func TestTestLeadsAreMarkedAndUnsignedWithoutASecret(t *testing.T) {
	srv, got := receiver(t)
	test := lead()
	test.ID = 0
	_, err := New().PushLead(context.Background(), callFor(srv.URL, "", true), test)
	require.NoError(t, err)
	r := (*got)[0]
	assert.Empty(t, r.header.Get("X-Fronko-Signature"))
	assert.Regexp(t, `^test-`, r.header.Get("X-Fronko-Delivery"))
	assert.Contains(t, string(r.body), `"test":true`)
}

func TestResponseStatusesDecideRetries(t *testing.T) {
	cases := []struct {
		status    int
		permanent bool
	}{
		{http.StatusInternalServerError, false},
		{http.StatusBadGateway, false},
		{http.StatusTooManyRequests, false},
		{http.StatusRequestTimeout, false},
		{http.StatusBadRequest, true},
		{http.StatusUnauthorized, true},
		{http.StatusNotFound, true},
		{http.StatusGone, true},
		{http.StatusFound, true},
	}
	for _, tc := range cases {
		srv, _ := receiver(t, tc.status)
		_, err := New().PushLead(context.Background(), callFor(srv.URL, "", true), lead())
		require.Error(t, err, tc.status)
		assert.Equal(t, tc.permanent, jobs.IsPermanent(err), "status %d", tc.status)
	}
}

func TestRefusesPrivateAddressesOutsideDevelopment(t *testing.T) {
	srv, got := receiver(t)
	_, err := New().PushLead(context.Background(), callFor(srv.URL, "", false), lead())
	require.Error(t, err)
	assert.True(t, jobs.IsPermanent(err))
	assert.Empty(t, *got)
}

func TestUnreachableReceiversAreRetried(t *testing.T) {
	srv, _ := receiver(t)
	url := srv.URL
	srv.Close()
	_, err := New().PushLead(context.Background(), callFor(url, "", true), lead())
	require.Error(t, err)
	assert.False(t, jobs.IsPermanent(err))
}

func TestReadExcerptKeepsValidUTF8(t *testing.T) {
	long := strings.Repeat("é", excerptLen)
	s := readExcerpt(strings.NewReader(long))
	assert.LessOrEqual(t, len(s), excerptLen+len("…"))
}

func TestManifestRegisters(t *testing.T) {
	integrations.NewRegistry().Register(New())
}
