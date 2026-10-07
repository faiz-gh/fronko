package files

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/secrets"
	"github.com/faiz-gh/fronko/backend/internal/platform/storage"
	"github.com/faiz-gh/fronko/backend/internal/users"
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
	store    *Store
	users    *users.Store
	box      *secrets.Box // nil when SECRETS_KEY is unset: storage is disabled
	newStore storage.Factory
	// private permits http and private-network endpoints (local MinIO only).
	private bool
}

func NewStorageService(store *Store, userStore *users.Store, box *secrets.Box, factory storage.Factory, allowPrivate bool) *StorageService {
	return &StorageService{store: store, users: userStore, box: box, newStore: factory, private: allowPrivate}
}

func (s *StorageService) Enabled() bool { return s.box != nil }

func (s *StorageService) allowPrivate() bool { return s.private }

// aad binds each ciphertext to its owner and field, so it can't be replayed onto another row.
func aad(userID int64, field string) []byte {
	return []byte(fmt.Sprintf("storage:%d:%s", userID, field))
}

func (s *StorageService) credentials(settings *StorageSettings) (accessKeyID, secretKey string, err error) {
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

// StoreFor returns a client for the organisation's bucket (the one its owner
// connected). bucket overrides the configured one, e.g. for a file uploaded
// before the owner switched buckets.
func (s *StorageService) StoreFor(ctx context.Context, orgID int64, bucket string) (storage.Store, error) {
	if !s.Enabled() {
		return nil, errStorageDisabled
	}
	settings, err := s.store.GetOrgStorageSettings(ctx, orgID)
	if errors.Is(err, database.ErrNotFound) {
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
