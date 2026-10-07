// Package netguard makes outgoing HTTP requests to addresses that users
// typed in (storage endpoints, webhook URLs, OAuth token endpoints) without
// letting them reach the server's own network: loopback, private ranges,
// link-local metadata services and the like.
//
// Checking the URL alone isn't enough, since a public-looking name can
// resolve to a private address (or be re-pointed after the check), so
// Client also checks every address it actually dials.
package netguard

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// ErrPrivate is returned for a URL or connection that points at a private
// or local address.
var ErrPrivate = errors.New("the address is private or local")

// ErrInvalidURL is returned by CheckURL for a URL that isn't an absolute
// http(s) URL.
var ErrInvalidURL = errors.New("invalid URL")

var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), // benchmarking
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"), // NAT64 can reach private IPv4
}

// IsPublic reports whether addr is a public unicast address.
func IsPublic(addr netip.Addr) bool {
	addr = addr.Unmap()
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	for _, p := range nonPublicPrefixes {
		if p.Contains(addr) {
			return false
		}
	}
	return true
}

// CheckHost rejects host names that are obviously local (localhost, *.internal)
// and literal private addresses. Names that resolve to private addresses are
// caught by Client when it dials.
func CheckHost(host string) error {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") ||
		strings.HasSuffix(host, ".local") {
		return ErrPrivate
	}
	if addr, err := netip.ParseAddr(host); err == nil && !IsPublic(addr) {
		return ErrPrivate
	}
	return nil
}

// CheckURL parses a URL someone typed in for the server to call. It must be
// an absolute https URL without credentials, on a public host. With
// allowPrivate (local development) plain http and private hosts are allowed.
func CheckURL(raw string, allowPrivate bool) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return nil, fmt.Errorf("%w: enter a full URL starting with https://", ErrInvalidURL)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: credentials don't belong in the URL", ErrInvalidURL)
	}
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && allowPrivate:
	default:
		return nil, fmt.Errorf("%w: the URL must use https", ErrInvalidURL)
	}
	if !allowPrivate {
		if err := CheckHost(u.Hostname()); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// Options tune Client.
type Options struct {
	// AllowPrivate turns the address checks off (local development only).
	AllowPrivate bool
	// Timeout bounds a whole request; zero means 30 seconds.
	Timeout time.Duration
}

// Client returns an HTTP client that refuses to connect to non-public
// addresses, ignores proxy settings from the environment, and doesn't follow
// redirects (which could leave the checked host).
func Client(opts Options) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	if !opts.AllowPrivate {
		dialer.Control = func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return ErrPrivate
			}
			addr, err := netip.ParseAddr(host)
			if err != nil || !IsPublic(addr) {
				return ErrPrivate
			}
			return nil
		}
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			MaxIdleConnsPerHost:   4,
			IdleConnTimeout:       60 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
