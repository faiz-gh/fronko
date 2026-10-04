package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/repository"
)

// maxBodyBytes caps request bodies so a client can't stream unbounded JSON.
const maxBodyBytes = 64 << 10

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encoding response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

// pageParams reads the 1-based `page` and `page_size` query parameters,
// writing a 400 and returning ok=false if either is out of range.
func pageParams(w http.ResponseWriter, r *http.Request, defaultSize, maxSize int) (page, size int, ok bool) {
	parse := func(name string, fallback, max int) (int, bool) {
		raw := r.URL.Query().Get(name)
		if raw == "" {
			return fallback, true
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > max {
			writeError(w, http.StatusBadRequest, "invalid "+name)
			return 0, false
		}
		return n, true
	}
	if page, ok = parse("page", 1, 1_000_000); !ok {
		return 0, 0, false
	}
	if size, ok = parse("page_size", defaultSize, maxSize); !ok {
		return 0, 0, false
	}
	return page, size, true
}

// scopeOf is what the signed-in user may see: their whole organisation for
// owners and admins, only their own things for members.
func scopeOf(r *http.Request) repository.Scope {
	p := middleware.PrincipalFrom(r.Context())
	return repository.Scope{OrgID: p.OrgID, UserID: p.UserID, Admin: p.IsAdmin()}
}
