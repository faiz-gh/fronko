package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func okHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func TestSameOrigin(t *testing.T) {
	h := SameOrigin([]string{"https://web.example/"}, http.HandlerFunc(okHandler))
	cases := []struct {
		name, method, origin string
		want                 int
	}{
		{"same origin POST", http.MethodPost, "http://app.example", http.StatusOK},
		{"cross origin POST", http.MethodPost, "http://evil.example", http.StatusForbidden},
		{"cross origin DELETE", http.MethodDelete, "https://evil.example", http.StatusForbidden},
		{"no Origin (non-browser client)", http.MethodPost, "", http.StatusOK},
		{"cross origin GET is allowed", http.MethodGet, "http://evil.example", http.StatusOK},
		{"allowed CORS origin POST", http.MethodPost, "https://web.example", http.StatusOK},
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

func TestCORS(t *testing.T) {
	h := CORS([]string{"https://web.example"}, http.HandlerFunc(okHandler))

	t.Run("preflight from allowed origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "http://api.example/api/me/profiles", nil)
		req.Header.Set("Origin", "https://web.example")
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, "https://web.example", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("unknown origin gets no CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://api.example/api/me/profiles", nil)
		req.Header.Set("Origin", "https://evil.example")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	})
}
