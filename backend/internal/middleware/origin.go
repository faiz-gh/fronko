package middleware

import (
	"net/http"
	"net/url"
)

// SameOrigin rejects state-changing requests whose Origin header names a
// different host. It's defence in depth for cookie auth on top of SameSite=Lax:
// browsers always send Origin on cross-site POST/PUT/DELETE, while non-browser
// clients (curl, scripts) usually omit it and are let through.
func SameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host {
				writeError(w, http.StatusForbidden, "cross-origin request rejected")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
