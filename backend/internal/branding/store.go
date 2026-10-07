package branding

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/faiz-gh/fronko/backend/internal/platform/database"
)

// Store runs the SQL for organisation branding.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// GetOrgBranding returns the organisation's logo and signature settings.
func (r *Store) GetOrgBranding(ctx context.Context, orgID int64) (*OrgBranding, error) {
	var b OrgBranding
	var sig []byte
	err := r.db.QueryRow(ctx,
		`SELECT name, logo_file, logo_policy, signature FROM organizations WHERE org_id = $1`, orgID,
	).Scan(&b.Name, &b.LogoFile, &b.LogoPolicy, &sig)
	if err != nil {
		return nil, database.MapError(err)
	}
	if err := json.Unmarshal(sig, &b.Signature); err != nil {
		return nil, err
	}
	return &b, nil
}

// UpdateOrgBranding saves the logo, logo policy and signature settings. The name is left alone.
func (r *Store) UpdateOrgBranding(ctx context.Context, orgID int64, b *OrgBranding) error {
	sig, err := json.Marshal(b.Signature)
	if err != nil {
		return err
	}
	err = r.db.QueryRow(ctx, `
		UPDATE organizations SET logo_file = $2, logo_policy = $3, signature = $4, updated_at = now()
		WHERE org_id = $1 RETURNING name`, orgID, b.LogoFile, b.LogoPolicy, sig,
	).Scan(&b.Name)
	return database.MapError(err)
}
