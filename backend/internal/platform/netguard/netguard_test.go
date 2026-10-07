package netguard

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPublic(t *testing.T) {
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "2606:4700::1111"} {
		assert.True(t, IsPublic(netip.MustParseAddr(s)), s)
	}
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1",
		"0.0.0.0", "::1", "fe80::1", "fd00::1", "::ffff:127.0.0.1", "64:ff9b::a00:1", "224.0.0.1"} {
		assert.False(t, IsPublic(netip.MustParseAddr(s)), s)
	}
}

func TestCheckURL(t *testing.T) {
	u, err := CheckURL(" https://hooks.example.com/catch/1?x=y ", false)
	require.NoError(t, err)
	assert.Equal(t, "https://hooks.example.com/catch/1?x=y", u.String())

	for _, bad := range []string{"hooks.example.com", "http://hooks.example.com", "ftp://example.com", "https://user:pw@example.com"} {
		_, err := CheckURL(bad, false)
		assert.ErrorIs(t, err, ErrInvalidURL, bad)
	}
	for _, private := range []string{"https://localhost/x", "https://api.internal", "https://printer.local", "https://10.0.0.5", "https://[::1]:8443"} {
		_, err := CheckURL(private, false)
		assert.ErrorIs(t, err, ErrPrivate, private)
	}
	_, err = CheckURL("http://localhost:9000/hook", true)
	assert.NoError(t, err, "allowed in development")
}

func TestClientRefusesPrivateAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com/", http.StatusFound)
	}))
	defer srv.Close()

	// The URL check can be dodged by a name that resolves to loopback; the dialer must refuse it.
	_, err := Client(Options{}).Get(srv.URL)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPrivate), "got %v", err)

	resp, err := Client(Options{AllowPrivate: true}).Get(srv.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusFound, resp.StatusCode, "redirects are not followed")
}
