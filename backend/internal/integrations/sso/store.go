package sso

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/faiz-gh/fronko/backend/internal/platform/database"
)

// Store runs the SQL for single sign-on: organisations' email domains, and
// finding an organisation to sign in to.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

// Domain is an email domain an organisation claims.
type Domain struct {
	ID     int64  `json:"id"`
	Domain string `json:"domain"`
	// Token goes in the TXT record that proves the organisation owns it.
	Token      string     `json:"-"`
	VerifiedAt *time.Time `json:"verified_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

const domainColumns = `domain_id, domain, verification_token, verified_at, created_at`

func scanDomain(row pgx.Row) (*Domain, error) {
	var d Domain
	if err := row.Scan(&d.ID, &d.Domain, &d.Token, &d.VerifiedAt, &d.CreatedAt); err != nil {
		return nil, database.MapError(err)
	}
	return &d, nil
}

func (s *Store) listDomains(ctx context.Context, orgID int64) ([]*Domain, error) {
	rows, err := s.db.Query(ctx, `SELECT `+domainColumns+` FROM org_domains WHERE org_id = $1 ORDER BY domain`, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Domain, error) { return scanDomain(row) })
}

func (s *Store) getDomain(ctx context.Context, orgID, id int64) (*Domain, error) {
	return scanDomain(s.db.QueryRow(ctx, `SELECT `+domainColumns+` FROM org_domains WHERE org_id = $1 AND domain_id = $2`, orgID, id))
}

func (s *Store) countDomains(ctx context.Context, orgID int64) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM org_domains WHERE org_id = $1`, orgID).Scan(&n)
	return n, err
}

// addDomain claims a domain. database.ErrConflict means the organisation
// already has it.
func (s *Store) addDomain(ctx context.Context, orgID int64, domain, token string, createdBy int64) (*Domain, error) {
	return scanDomain(s.db.QueryRow(ctx, `
		INSERT INTO org_domains (org_id, domain, verification_token, created_by) VALUES ($1, $2, $3, $4)
		RETURNING `+domainColumns, orgID, domain, token, createdBy))
}

// markVerified records that the organisation proved it owns the domain.
// database.ErrConflict means another organisation verified it first.
func (s *Store) markVerified(ctx context.Context, orgID, id int64) error {
	return database.ExecOne(ctx, s.db, `UPDATE org_domains SET verified_at = now()
		WHERE org_id = $1 AND domain_id = $2`, orgID, id)
}

func (s *Store) deleteDomain(ctx context.Context, orgID, id int64) error {
	return database.ExecOne(ctx, s.db, `DELETE FROM org_domains WHERE org_id = $1 AND domain_id = $2`, orgID, id)
}

// domainVerified reports whether the organisation has verified domain.
func (s *Store) domainVerified(ctx context.Context, orgID int64, domain string) (bool, error) {
	var ok bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM org_domains
		WHERE org_id = $1 AND domain = lower($2) AND verified_at IS NOT NULL)`, orgID, domain).Scan(&ok)
	return ok, err
}

// orgRef is an organisation someone is signing in to.
type orgRef struct {
	ID        int64
	Handle    string
	Suspended bool
}

func (s *Store) orgByHandle(ctx context.Context, handle string) (*orgRef, error) {
	var o orgRef
	err := s.db.QueryRow(ctx, `SELECT org_id, handle, suspended_at IS NOT NULL FROM organizations
		WHERE lower(handle) = lower($1)`, handle).Scan(&o.ID, &o.Handle, &o.Suspended)
	return &o, database.MapError(err)
}

func (s *Store) orgByID(ctx context.Context, id int64) (*orgRef, error) {
	var o orgRef
	err := s.db.QueryRow(ctx, `SELECT org_id, handle, suspended_at IS NOT NULL FROM organizations
		WHERE org_id = $1`, id).Scan(&o.ID, &o.Handle, &o.Suspended)
	return &o, database.MapError(err)
}

// orgByDomain returns the organisation that verified domain.
func (s *Store) orgByDomain(ctx context.Context, domain string) (*orgRef, error) {
	var o orgRef
	err := s.db.QueryRow(ctx, `SELECT o.org_id, o.handle, o.suspended_at IS NOT NULL
		FROM org_domains d JOIN organizations o ON o.org_id = d.org_id
		WHERE d.domain = lower($1) AND d.verified_at IS NOT NULL`, domain).Scan(&o.ID, &o.Handle, &o.Suspended)
	return &o, database.MapError(err)
}
