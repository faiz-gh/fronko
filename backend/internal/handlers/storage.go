package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
	"github.com/faiz-gh/fronko/backend/internal/secrets"
	"github.com/faiz-gh/fronko/backend/internal/storage"
)

const probeTimeout = 20 * time.Second

var (
	errStorageDisabled      = errors.New("file storage is not enabled on this server")
	errStorageNotConfigured = errors.New("storage not configured")

	// S3 bucket naming is the strictest of the providers we support.
	bucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
	regionPattern = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)
	providers     = map[string]bool{"r2": true, "b2": true, "s3": true, "minio": true, "other": true}
)

// StorageService owns users' bucket credentials: it encrypts them for the
// database and turns them back into a working client when needed.
type StorageService struct {
	repo     *repository.Repository
	box      *secrets.Box // nil when SECRETS_KEY is unset: storage is disabled
	newStore storage.Factory
	// private permits http and private-network endpoints (local MinIO only).
	private bool
}

func NewStorageService(repo *repository.Repository, box *secrets.Box, factory storage.Factory, allowPrivate bool) *StorageService {
	return &StorageService{repo: repo, box: box, newStore: factory, private: allowPrivate}
}

func (s *StorageService) Enabled() bool { return s.box != nil }

func (s *StorageService) allowPrivate() bool { return s.private }

// aad binds each ciphertext to its owner and field, so it can't be replayed onto another row.
func aad(userID int64, field string) []byte {
	return []byte(fmt.Sprintf("storage:%d:%s", userID, field))
}

func (s *StorageService) credentials(settings *models.StorageSettings) (accessKeyID, secretKey string, err error) {
	id, err := s.box.Open(settings.AccessKeyIDEnc, aad(settings.UserID, "access_key_id"))
	if err != nil {
		return "", "", err
	}
	secret, err := s.box.Open(settings.SecretAccessKeyEnc, aad(settings.UserID, "secret_access_key"))
	if err != nil {
		return "", "", err
	}
	return string(id), string(secret), nil
}

// StoreFor returns a client for the user's bucket. bucket overrides the
// configured one, e.g. for a file uploaded before the user switched buckets.
func (s *StorageService) StoreFor(ctx context.Context, userID int64, bucket string) (storage.Store, error) {
	if !s.Enabled() {
		return nil, errStorageDisabled
	}
	settings, err := s.repo.GetStorageSettings(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, errStorageNotConfigured
	}
	if err != nil {
		return nil, err
	}
	id, secret, err := s.credentials(settings)
	if err != nil {
		return nil, fmt.Errorf("decrypting storage credentials: %w", err)
	}
	if bucket == "" {
		bucket = settings.Bucket
	}
	return s.newStore(storage.Config{
		Endpoint: settings.Endpoint, Region: settings.Region, Bucket: bucket,
		AccessKeyID: id, SecretAccessKey: secret, PathStyle: settings.PathStyle,
	})
}

type StorageHandler struct {
	svc *StorageService
}

func NewStorageHandler(svc *StorageService) *StorageHandler {
	return &StorageHandler{svc: svc}
}

// storageView is what the settings page sees. It never includes key material.
type storageView struct {
	Enabled       bool       `json:"enabled"`
	Configured    bool       `json:"configured"`
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
	writeError(w, http.StatusServiceUnavailable, errStorageDisabled.Error())
	return true
}

func (h *StorageHandler) view(ctx context.Context, userID int64) (storageView, error) {
	v := storageView{Enabled: h.svc.Enabled()}
	if !v.Enabled {
		return v, nil
	}
	settings, err := h.svc.repo.GetStorageSettings(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	count, err := h.svc.repo.CountFilesForUser(ctx, userID)
	if err != nil {
		return v, err
	}
	v.Configured = true
	v.Provider, v.Endpoint, v.Region, v.Bucket = settings.Provider, settings.Endpoint, settings.Region, settings.Bucket
	v.PathStyle, v.AccessKeyHint, v.VerifiedAt, v.FileCount = settings.PathStyle, settings.AccessKeyHint, settings.VerifiedAt, count
	return v, nil
}

// Protected: GET /api/me/storage
func (h *StorageHandler) Get(w http.ResponseWriter, r *http.Request) {
	v, err := h.view(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		log.Printf("get storage: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, v)
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
		saved, err := h.svc.repo.GetStorageSettings(ctx, userID)
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

// Protected: POST /api/me/storage/test. Checks settings without saving them.
func (h *StorageHandler) Test(w http.ResponseWriter, r *http.Request) {
	if h.disabled(w) {
		return
	}
	userID := middleware.UserID(r.Context())
	var req storageRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	cfg, msg := h.resolve(r.Context(), userID, &req)
	if msg == "" {
		msg = h.probe(r.Context(), userID, cfg)
	}
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Protected: PUT /api/me/storage. Only saves settings that pass a live check.
func (h *StorageHandler) Put(w http.ResponseWriter, r *http.Request) {
	if h.disabled(w) {
		return
	}
	userID := middleware.UserID(r.Context())
	var req storageRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	cfg, msg := h.resolve(r.Context(), userID, &req)
	if msg == "" {
		msg = h.probe(r.Context(), userID, cfg)
	}
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	idEnc, err := h.svc.box.Seal([]byte(cfg.AccessKeyID), aad(userID, "access_key_id"))
	if err != nil {
		log.Printf("seal access key: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	secretEnc, err := h.svc.box.Seal([]byte(cfg.SecretAccessKey), aad(userID, "secret_access_key"))
	if err != nil {
		log.Printf("seal secret key: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	hint := cfg.AccessKeyID
	if len(hint) > 4 {
		hint = hint[len(hint)-4:]
	}
	now := time.Now()
	settings := &models.StorageSettings{
		UserID: userID, Provider: req.Provider, Endpoint: cfg.Endpoint, Region: cfg.Region, Bucket: cfg.Bucket,
		PathStyle: cfg.PathStyle, AccessKeyIDEnc: idEnc, SecretAccessKeyEnc: secretEnc, AccessKeyHint: hint,
		VerifiedAt: &now,
	}
	if err := h.svc.repo.UpsertStorageSettings(r.Context(), settings); err != nil {
		log.Printf("save storage: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save storage settings")
		return
	}

	v, err := h.view(r.Context(), userID)
	if err != nil {
		log.Printf("get storage: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// Protected: DELETE /api/me/storage. Forgets the keys; files already in the
// bucket stay there, but can't be shown until storage is connected again.
func (h *StorageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.disabled(w) {
		return
	}
	err := h.svc.repo.DeleteStorageSettings(r.Context(), middleware.UserID(r.Context()))
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		log.Printf("delete storage: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to disconnect storage")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
