package calendar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/faiz-gh/fronko/backend/internal/integrations"
)

func byID(t *testing.T, id string) *Provider {
	t.Helper()
	for _, p := range All() {
		if p.Manifest().ID == id {
			return p.(*Provider)
		}
	}
	t.Fatalf("no provider %q", id)
	return nil
}

func settings(url string) integrations.Settings {
	return integrations.Settings{Values: map[string]any{"url": url}}
}

func TestLinksMustBeOnTheProvidersOwnSite(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		provider, url string
		ok            bool
	}{
		{"calendly", "https://calendly.com/ada/30min", true},
		{"calendly", "https://www.calendly.com/ada", true},
		{"calendly", "https://calendly.com.evil.example/ada", false},
		{"calendly", "https://cal.com/ada", false},
		{"chili-piper", "https://acme.chilipiper.com/me/ada", true},
		{"microsoft-bookings", "https://outlook.office.com/book/Sales@acme.com/", true},
		{"hubspot-meetings", "https://meetings-eu1.hubspot.com/ada", true},
		{"google-calendar", "https://calendar.app.google/abc123", true},
		{"google-calendar", "https://calendly.com/ada", false},
		{"booking-link", "https://cal.com/ada", true},
	} {
		err := byID(t, tc.provider).Validate(ctx, settings(tc.url))
		if tc.ok {
			assert.NoError(t, err, "%s %s", tc.provider, tc.url)
		} else {
			var fe *integrations.FieldError
			assert.ErrorAs(t, err, &fe, "%s %s", tc.provider, tc.url)
		}
	}
}

func TestBookingCarriesTheLinkAndPrefill(t *testing.T) {
	b := byID(t, "calendly").Booking(settings("https://calendly.com/ada/30min"))
	require.NotNil(t, b)
	assert.Equal(t, "calendly", b.Provider)
	assert.Equal(t, "Calendly", b.Name)
	assert.Equal(t, "https://calendly.com/ada/30min", b.URL)
	assert.Equal(t, map[string]string{"name": "name", "email": "email"}, b.Prefill)

	assert.Nil(t, byID(t, "booking-link").Booking(settings("")))
	assert.Empty(t, byID(t, "booking-link").Booking(settings("https://cal.com/ada")).Prefill)
}

func TestTestOpensThePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gone" {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	p := byID(t, "booking-link")
	call := func(path string) *integrations.Call {
		return &integrations.Call{Settings: settings(srv.URL + path), HTTP: srv.Client()}
	}
	res, err := p.Test(context.Background(), call("/ada"))
	require.NoError(t, err)
	assert.Equal(t, "The booking page opens", res.Summary)
	_, err = p.Test(context.Background(), call("/gone"))
	assert.ErrorContains(t, err, "answered 404")
}
