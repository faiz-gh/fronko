package orgs

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// maxQuotaBytes keeps quotas to a sane range (1 TB).
const maxQuotaBytes = 1 << 40

// OrgHandler lets an organisation's owner and admins manage it and its users.
type OrgHandler struct {
	store *Store
	users *users.Store
	codes *users.Codes
	auth  *auth.Service // for hashing passwords
}

func NewOrgHandler(store *Store, userStore *users.Store, codes *users.Codes, authService *auth.Service) *OrgHandler {
	return &OrgHandler{store: store, users: userStore, codes: codes, auth: authService}
}

// optionalQuota tells a quota that wasn't sent apart from null (unlimited).
type optionalQuota struct {
	Set   bool
	Value *int64
}

func (q *optionalQuota) UnmarshalJSON(b []byte) error {
	q.Set = true
	if string(b) == "null" {
		q.Value = nil
		return nil
	}
	var v int64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	q.Value = &v
	return nil
}

func (q optionalQuota) valid() bool {
	return q.Value == nil || (*q.Value >= 0 && *q.Value <= maxQuotaBytes)
}

const errQuotaRange = "storage limit must be between 0 and 1 TB, or unlimited"

// Protected (admins): GET /api/org
func (h *OrgHandler) Get(w http.ResponseWriter, r *http.Request) {
	org, err := h.store.GetOrganization(r.Context(), auth.PrincipalFrom(r.Context()).OrgID)
	if err != nil {
		log.Printf("get organization: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, org)
}

// Protected (owner): PUT /api/org {"name", "default_quota_bytes"}
func (h *OrgHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name              string        `json:"name"`
		DefaultQuotaBytes optionalQuota `json:"default_quota_bytes"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !ValidName(req.Name) {
		httpx.WriteError(w, http.StatusBadRequest, "organisation name must be 1-80 characters")
		return
	}
	if !req.DefaultQuotaBytes.valid() {
		httpx.WriteError(w, http.StatusBadRequest, errQuotaRange)
		return
	}
	org := &Organization{
		ID:                auth.PrincipalFrom(r.Context()).OrgID,
		Name:              req.Name,
		DefaultQuotaBytes: req.DefaultQuotaBytes.Value,
	}
	if err := h.store.UpdateOrganization(r.Context(), org); err != nil {
		log.Printf("update organization: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, org)
}

// Protected (admins): PUT /api/org/handle {"handle"}. Changes the
// organisation's part of every card link. Links already printed on QR codes
// or written to NFC cards stop working, so the dashboard warns first.
func (h *OrgHandler) UpdateHandle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Handle string `json:"handle"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	handle := strings.ToLower(strings.TrimSpace(req.Handle))
	if !validHandle(handle) {
		httpx.WriteError(w, http.StatusBadRequest, "handle must be 3-32 characters: lowercase letters, numbers and hyphens")
		return
	}
	orgID := auth.PrincipalFrom(r.Context()).OrgID
	if err := h.store.SetOrgHandle(r.Context(), orgID, handle); err != nil {
		if errors.Is(err, database.ErrConflict) {
			httpx.WriteError(w, http.StatusConflict, "another organisation already uses that handle")
			return
		}
		log.Printf("set org handle: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.Get(w, r)
}
