package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func okHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func request(h http.HandlerFunc, remoteAddr, realIP string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = remoteAddr
	if realIP != "" {
		req.Header.Set("X-Real-IP", realIP)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestRateLimiter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("allows burst then rejects with Retry-After", func(t *testing.T) {
		h := NewRateLimiter(ctx, rate.Every(15*time.Second), 3, false).Limit(okHandler)
		for i := range 3 {
			assert.Equal(t, http.StatusOK, request(h, "10.0.0.1:1234", "").Code, "request %d", i)
		}
		rec := request(h, "10.0.0.1:1234", "")
		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.NotEmpty(t, rec.Header().Get("Retry-After"))
		assert.Contains(t, rec.Body.String(), "too many requests")
	})

	t.Run("clients are limited independently", func(t *testing.T) {
		h := NewRateLimiter(ctx, rate.Every(time.Minute), 1, false).Limit(okHandler)
		assert.Equal(t, http.StatusOK, request(h, "10.0.0.1:1", "").Code)
		assert.Equal(t, http.StatusTooManyRequests, request(h, "10.0.0.1:2", "").Code, "same IP, different port")
		assert.Equal(t, http.StatusOK, request(h, "10.0.0.2:1", "").Code)
	})

	t.Run("X-Real-IP ignored unless proxy is trusted", func(t *testing.T) {
		untrusted := NewRateLimiter(ctx, rate.Every(time.Minute), 1, false).Limit(okHandler)
		assert.Equal(t, http.StatusOK, request(untrusted, "172.18.0.5:1", "1.1.1.1").Code)
		// A spoofed header must not buy a fresh bucket.
		assert.Equal(t, http.StatusTooManyRequests, request(untrusted, "172.18.0.5:1", "2.2.2.2").Code)

		trusted := NewRateLimiter(ctx, rate.Every(time.Minute), 1, true).Limit(okHandler)
		assert.Equal(t, http.StatusOK, request(trusted, "172.18.0.5:1", "1.1.1.1").Code)
		assert.Equal(t, http.StatusOK, request(trusted, "172.18.0.5:1", "2.2.2.2").Code, "distinct real clients behind the proxy")
		assert.Equal(t, http.StatusTooManyRequests, request(trusted, "172.18.0.5:1", "1.1.1.1").Code)
	})
}

func TestSameOrigin(t *testing.T) {
	h := SameOrigin(http.HandlerFunc(okHandler))
	cases := []struct {
		name, method, origin string
		want                 int
	}{
		{"same origin POST", http.MethodPost, "http://app.example", http.StatusOK},
		{"cross origin POST", http.MethodPost, "http://evil.example", http.StatusForbidden},
		{"cross origin DELETE", http.MethodDelete, "https://evil.example", http.StatusForbidden},
		{"no Origin (non-browser client)", http.MethodPost, "", http.StatusOK},
		{"cross origin GET is allowed", http.MethodGet, "http://evil.example", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://app.example/api/me/profiles", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			assert.Equal(t, tc.want, rec.Code)
		})
	}
}
