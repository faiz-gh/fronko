// Package httpx holds the JSON request and response helpers every module's
// handlers share.
package httpx

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// MaxBodyBytes caps request bodies so a client can't stream unbounded JSON.
const MaxBodyBytes = 64 << 10

// WriteJSON sends v as a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encoding response: %v", err)
	}
}

// WriteError sends {"error": msg}. Messages are shown to people as they are.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

// DecodeJSON reads a size-capped JSON body into dst, writing a 400 if it can't.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

// PageParams reads the 1-based `page` and `page_size` query parameters,
// writing a 400 and returning ok=false if either is out of range.
func PageParams(w http.ResponseWriter, r *http.Request, defaultSize, maxSize int) (page, size int, ok bool) {
	parse := func(name string, fallback, max int) (int, bool) {
		raw := r.URL.Query().Get(name)
		if raw == "" {
			return fallback, true
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > max {
			WriteError(w, http.StatusBadRequest, "invalid "+name)
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

// PathID reads a positive integer path value, writing a 404 if it isn't one.
func PathID(w http.ResponseWriter, r *http.Request, name, what string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id < 1 {
		WriteError(w, http.StatusNotFound, what+" not found")
		return 0, false
	}
	return id, true
}

// Error is a response a helper decided on, for the caller to write (or, in
// bulk actions, to report per item).
type Error struct {
	Status int
	Msg    string
}

func (e *Error) Write(w http.ResponseWriter) { WriteError(w, e.Status, e.Msg) }

// Internal logs err with what was being done and returns a generic 500.
func Internal(what string, err error) *Error {
	log.Printf("%s: %v", what, err)
	return &Error{http.StatusInternalServerError, "internal error"}
}

// IDFromPath reads the positive integer {id} path value, writing a 400
// ("invalid <what> ID") if it isn't one.
func IDFromPath(w http.ResponseWriter, r *http.Request, what string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		WriteError(w, http.StatusBadRequest, "invalid "+what+" ID")
		return 0, false
	}
	return id, true
}
