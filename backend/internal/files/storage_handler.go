package files

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
	"github.com/faiz-gh/fronko/backend/internal/platform/storage"
)

type StorageHandler struct {
	svc *StorageService
}

func NewStorageHandler(svc *StorageService) *StorageHandler {
	return &StorageHandler{svc: svc}
}

// storageView is what the settings page sees. It never includes key material,
// and only the owner gets the bucket details.
type storageView struct {
	Enabled    bool `json:"enabled"`
	Configured bool `json:"configured"`
	// The signed-in user's personal files against their limit (nil: unlimited).
	UsedBytes     int64      `json:"used_bytes"`
	QuotaBytes    *int64     `json:"quota_bytes"`
	Provider      string     `json:"provider,omitempty"`
	Endpoint      string     `json:"endpoint,omitempty"`
	Region        string     `json:"region,omitempty"`
	Bucket        string     `json:"bucket,omitempty"`
	PathStyle     bool       `json:"path_style"`
	AccessKeyHint string     `json:"access_key_hint,omitempty"`
	VerifiedAt    *time.Time `json:"verified_at,omitempty"`
	FileCount     int64      `json:"file_count"`
}

type storageRequest struct {
	Provider        string `json:"provider"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	PathStyle       bool   `json:"path_style"`
	AccessKeyID     string `json:"access_key_id"`     // optional on update: keeps the saved key
	SecretAccessKey string `json:"secret_access_key"` // optional on update: keeps the saved secret
}

func (h *StorageHandler) disabled(w http.ResponseWriter) bool {
	if h.svc.Enabled() {
		return false
	}
	httpx.WriteError(w, http.StatusServiceUnavailable, errStorageDisabled.Error())
	return true
}

func (h *StorageHandler) view(ctx context.Context, p auth.Principal) (storageView, error) {
	v := storageView{Enabled: h.svc.Enabled()}
	if !v.Enabled {
		return v, nil
	}
	user, err := h.svc.users.GetUserByID(ctx, p.UserID)
	if err != nil {
		return v, err
	}
	v.QuotaBytes = user.StorageQuotaBytes
	if v.UsedBytes, err = h.svc.store.UsedBytes(ctx, p.UserID); err != nil {
		return v, err
	}
	settings, err := h.svc.store.GetOrgStorageSettings(ctx, p.OrgID)
	if errors.Is(err, database.ErrNotFound) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	v.Configured = true
	if !p.IsOwner() {
		return v, nil
	}
	count, err := h.svc.store.CountFilesForOrg(ctx, p.OrgID)
	if err != nil {
		return v, err
	}
	v.Provider, v.Endpoint, v.Region, v.Bucket = settings.Provider, settings.Endpoint, settings.Region, settings.Bucket
	v.PathStyle, v.AccessKeyHint, v.VerifiedAt, v.FileCount = settings.PathStyle, settings.AccessKeyHint, settings.VerifiedAt, count
	return v, nil
}

// Protected: GET /api/me/storage. Everyone learns whether uploads work and
// their own usage; only the owner sees the bucket settings.
func (h *StorageHandler) Get(w http.ResponseWriter, r *http.Request) {
	v, err := h.view(r.Context(), auth.PrincipalFrom(r.Context()))
	if err != nil {
		log.Printf("get storage: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

// resolve validates the request and fills omitted keys from the saved settings.
// It returns a user-facing message on failure.
func (h *StorageHandler) resolve(ctx context.Context, userID int64, req *storageRequest) (storage.Config, string) {
	req.Provider = strings.TrimSpace(req.Provider)
	req.Bucket = strings.TrimSpace(req.Bucket)
	req.Region = strings.TrimSpace(strings.ToLower(req.Region))
	req.AccessKeyID = strings.TrimSpace(req.AccessKeyID)
	req.SecretAccessKey = strings.TrimSpace(req.SecretAccessKey)

	if !providers[req.Provider] {
		return storage.Config{}, "choose a storage provider"
	}
	endpoint, err := storage.ValidateEndpoint(req.Endpoint, h.svc.allowPrivate())
	if err != nil {
		return storage.Config{}, storage.Describe(err)
	}
	req.Endpoint = endpoint
	if !bucketPattern.MatchString(req.Bucket) {
		return storage.Config{}, "bucket names are 3-63 characters: lowercase letters, numbers, dots and hyphens"
	}
	if req.Region == "" {
		req.Region = "auto"
		if req.Provider != "r2" {
			req.Region = "us-east-1"
		}
	}
	if !regionPattern.MatchString(req.Region) {
		return storage.Config{}, "region should look like us-east-1 or auto"
	}
	if len(req.AccessKeyID) > 256 || len(req.SecretAccessKey) > 256 {
		return storage.Config{}, "keys are too long"
	}

	// Keys are write-only in the UI, so an edit that leaves them blank keeps the saved ones.
	if req.AccessKeyID == "" || req.SecretAccessKey == "" {
		saved, err := h.svc.store.GetStorageSettings(ctx, userID)
		if err != nil {
			return storage.Config{}, "enter your access key ID and secret access key"
		}
		id, secret, err := h.svc.credentials(saved)
		if err != nil {
			log.Printf("decrypt storage credentials for user %d: %v", userID, err)
			return storage.Config{}, "your saved keys can't be read; please enter them again"
		}
		if req.AccessKeyID == "" {
			req.AccessKeyID = id
		}
		if req.SecretAccessKey == "" {
			req.SecretAccessKey = secret
		}
	}

	return storage.Config{
		Endpoint: req.Endpoint, Region: req.Region, Bucket: req.Bucket,
		AccessKeyID: req.AccessKeyID, SecretAccessKey: req.SecretAccessKey, PathStyle: req.PathStyle,
	}, ""
}

// probe connects with cfg, returning a user-facing message on failure.
func (h *StorageHandler) probe(ctx context.Context, userID int64, cfg storage.Config) string {
	store, err := h.svc.newStore(cfg)
	if err == nil {
		ctx, cancel := context.WithTimeout(ctx, probeTimeout)
		defer cancel()
		err = store.Probe(ctx)
	}
	if err != nil {
		log.Printf("storage probe for user %d: %v", userID, err)
		return "couldn't use this bucket: " + storage.Describe(err)
	}
	return ""
}

// Protected (owner): POST /api/me/storage/test. Checks settings without saving them.
func (h *StorageHandler) Test(w http.ResponseWriter, r *http.Request) {
	if h.disabled(w) {
		return
	}
	userID := auth.UserID(r.Context())
	var req storageRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	cfg, msg := h.resolve(r.Context(), userID, &req)
	if msg == "" {
		msg = h.probe(r.Context(), userID, cfg)
	}
	if msg != "" {
		httpx.WriteError(w, http.StatusBadRequest, msg)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Protected (owner): PUT /api/me/storage. Only saves settings that pass a live check.
func (h *StorageHandler) Put(w http.ResponseWriter, r *http.Request) {
	if h.disabled(w) {
		return
	}
	userID := auth.UserID(r.Context())
	var req storageRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	cfg, msg := h.resolve(r.Context(), userID, &req)
	if msg == "" {
		msg = h.probe(r.Context(), userID, cfg)
	}
	if msg != "" {
		httpx.WriteError(w, http.StatusBadRequest, msg)
		return
	}

	idEnc, err := h.svc.box.Seal([]byte(cfg.AccessKeyID), aad(userID, "access_key_id"))
	if err != nil {
		log.Printf("seal access key: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	secretEnc, err := h.svc.box.Seal([]byte(cfg.SecretAccessKey), aad(userID, "secret_access_key"))
	if err != nil {
		log.Printf("seal secret key: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	hint := cfg.AccessKeyID
	if len(hint) > 4 {
		hint = hint[len(hint)-4:]
	}
	now := time.Now()
	settings := &StorageSettings{
		UserID: userID, Provider: req.Provider, Endpoint: cfg.Endpoint, Region: cfg.Region, Bucket: cfg.Bucket,
		PathStyle: cfg.PathStyle, AccessKeyIDEnc: idEnc, SecretAccessKeyEnc: secretEnc, AccessKeyHint: hint,
		VerifiedAt: &now,
	}
	if err := h.svc.store.UpsertStorageSettings(r.Context(), settings); err != nil {
		log.Printf("save storage: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to save storage settings")
		return
	}

	v, err := h.view(r.Context(), auth.PrincipalFrom(r.Context()))
	if err != nil {
		log.Printf("get storage: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

// Protected (owner): DELETE /api/me/storage. Forgets the keys; files already in the
// bucket stay there, but can't be shown until storage is connected again.
// The keys are stored against the owner, who is the only one allowed here.
func (h *StorageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.disabled(w) {
		return
	}
	err := h.svc.store.DeleteStorageSettings(r.Context(), auth.UserID(r.Context()))
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		log.Printf("delete storage: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to disconnect storage")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
