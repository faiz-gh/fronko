package web

import (
	"net/http"
	"strings"
)

// CORS lets browsers on the listed origins call the API with the session cookie.
// It's only needed when the frontend talks to the backend directly; behind the
// frontend's nginx proxy every request is same-origin and the list can be empty.
func CORS(allowed []string, next http.Handler) http.Handler {
	origins := originSet(allowed)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || !origins[origin] {
			next.ServeHTTP(w, r)
			return
		}

		h := w.Header()
		h.Add("Vary", "Origin")
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Credentials", "true")

		// Preflight: answer it here rather than letting the mux return 405.
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h.Add("Vary", "Access-Control-Request-Method")
			h.Add("Vary", "Access-Control-Request-Headers")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
			h.Set("Access-Control-Allow-Headers", "Content-Type")
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// originSet normalises origins so "https://app.example.com/" matches the
// browser's "https://app.example.com".
func originSet(list []string) map[string]bool {
	set := make(map[string]bool, len(list))
	for _, o := range list {
		if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
			set[o] = true
		}
	}
	return set
}
