package files

import (
	"context"

	"github.com/faiz-gh/fronko/backend/internal/platform/database"
)

func (r *Store) GetStorageSettings(ctx context.Context, userID int64) (*StorageSettings, error) {
	query := `
		SELECT user_id, provider, endpoint, region, bucket, path_style,
		       access_key_id_enc, secret_access_key_enc, access_key_hint,
		       verified_at, created_at, updated_at
		FROM user_storage WHERE user_id = $1`
	var s StorageSettings
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&s.UserID, &s.Provider, &s.Endpoint, &s.Region, &s.Bucket, &s.PathStyle,
		&s.AccessKeyIDEnc, &s.SecretAccessKeyEnc, &s.AccessKeyHint,
		&s.VerifiedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, database.MapError(err)
	}
	return &s, nil
}

// GetOrgStorageSettings returns the bucket an organisation uses: the one its owner connected.
func (r *Store) GetOrgStorageSettings(ctx context.Context, orgID int64) (*StorageSettings, error) {
	var ownerID int64
	err := r.db.QueryRow(ctx, `SELECT user_id FROM users WHERE org_id = $1 AND role = 'owner'`, orgID).Scan(&ownerID)
	if err != nil {
		return nil, database.MapError(err)
	}
	return r.GetStorageSettings(ctx, ownerID)
}

// UpsertStorageSettings creates or replaces the user's bucket configuration.
func (r *Store) UpsertStorageSettings(ctx context.Context, s *StorageSettings) error {
	query := `
		INSERT INTO user_storage (user_id, provider, endpoint, region, bucket, path_style,
		                          access_key_id_enc, secret_access_key_enc, access_key_hint, verified_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id) DO UPDATE SET
			provider = EXCLUDED.provider, endpoint = EXCLUDED.endpoint, region = EXCLUDED.region,
			bucket = EXCLUDED.bucket, path_style = EXCLUDED.path_style,
			access_key_id_enc = EXCLUDED.access_key_id_enc,
			secret_access_key_enc = EXCLUDED.secret_access_key_enc,
			access_key_hint = EXCLUDED.access_key_hint, verified_at = EXCLUDED.verified_at,
			updated_at = now()
		RETURNING created_at, updated_at`
	err := r.db.QueryRow(ctx, query, s.UserID, s.Provider, s.Endpoint, s.Region, s.Bucket, s.PathStyle,
		s.AccessKeyIDEnc, s.SecretAccessKeyEnc, s.AccessKeyHint, s.VerifiedAt,
	).Scan(&s.CreatedAt, &s.UpdatedAt)
	return database.MapError(err)
}

// DeleteStorageSettings removes the user's keys; database.ErrNotFound if none were saved.
func (r *Store) DeleteStorageSettings(ctx context.Context, userID int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM user_storage WHERE user_id = $1`, userID)
	if err != nil {
		return database.MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrNotFound
	}
	return nil
}
