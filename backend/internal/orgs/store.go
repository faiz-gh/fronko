package orgs

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/teams"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// Store runs the SQL for organisations.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// CreateMember inserts a user into an existing organisation (user.OrgID) and
// puts them in the given teams (any outside the organisation are ignored),
// in one transaction.
func (r *Store) CreateMember(ctx context.Context, user *users.User, memberships ...teams.TeamMembership) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := users.Insert(ctx, tx, user); err != nil {
			return err
		}
		if len(memberships) == 0 {
			return nil
		}
		return teams.ReplaceUserTeamsTx(ctx, tx, user.OrgID, user.ID, memberships)
	})
}

// CreateOrgWithOwner registers a new organisation named orgName with user as
// its owner, in one transaction. It fills in user.OrgID and user.Role. The
// organisation's link handle is made from its name (or the username), with a
// number added when that one is taken.
func (r *Store) CreateOrgWithOwner(ctx context.Context, orgName string, user *users.User) error {
	base := HandleFromName(orgName)
	if base == "" {
		base = HandleFromName(user.Username)
	}
	if base == "" {
		base = "org"
	}
	var err error
	// The free-handle check and the insert can race another registration; the
	// unique index catches that, and the next attempt picks another number.
	for attempt := 0; attempt < 5; attempt++ {
		err = pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
			handle, err := freeHandle(ctx, tx, base)
			if err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, `INSERT INTO organizations (name, handle) VALUES ($1, $2) RETURNING org_id`,
				orgName, handle).Scan(&user.OrgID); err != nil {
				return database.MapError(err)
			}
			user.Role = auth.RoleOwner
			return users.Insert(ctx, tx, user)
		})
		if !database.IsConstraint(err, "organizations_handle_lower_idx") {
			return err
		}
	}
	return err
}

const (
	MinHandleLen = 3
	MaxHandleLen = 32
)

// HandleFromName turns a name into a link handle: lowercase letters and
// digits, with single hyphens between words. It returns "" when too little
// of the name is usable.
func HandleFromName(name string) string {
	var b strings.Builder
	hyphen := false
	for _, c := range strings.ToLower(name) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			if hyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			hyphen = false
			b.WriteRune(c)
		} else {
			hyphen = true
		}
	}
	h := b.String()
	if len(h) > MaxHandleLen {
		h = strings.TrimRight(h[:MaxHandleLen], "-")
	}
	if len(h) < MinHandleLen {
		return ""
	}
	return h
}

// freeHandle returns base, or base-2, base-3… for the first one no
// organisation uses.
func freeHandle(ctx context.Context, q database.Querier, base string) (string, error) {
	for n := 1; ; n++ {
		handle := base
		if n > 1 {
			suffix := fmt.Sprintf("-%d", n)
			handle = strings.TrimRight(base[:min(len(base), MaxHandleLen-len(suffix))], "-") + suffix
		}
		var taken bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM organizations WHERE LOWER(handle) = LOWER($1))`,
			handle).Scan(&taken); err != nil {
			return "", err
		}
		if !taken {
			return handle, nil
		}
	}
}

func (r *Store) GetOrganization(ctx context.Context, orgID int64) (*Organization, error) {
	var o Organization
	err := r.db.QueryRow(ctx,
		`SELECT org_id, name, handle, default_quota_bytes, created_at, updated_at, suspended_at, suspended_reason
		FROM organizations WHERE org_id = $1`, orgID,
	).Scan(&o.ID, &o.Name, &o.Handle, &o.DefaultQuotaBytes, &o.CreatedAt, &o.UpdatedAt, &o.SuspendedAt, &o.SuspendedReason)
	if err != nil {
		return nil, database.MapError(err)
	}
	return &o, nil
}

// UpdateOrganization saves the name and default quota.
func (r *Store) UpdateOrganization(ctx context.Context, o *Organization) error {
	err := r.db.QueryRow(ctx, `
		UPDATE organizations SET name = $2, default_quota_bytes = $3, updated_at = now()
		WHERE org_id = $1 RETURNING handle, created_at, updated_at`, o.ID, o.Name, o.DefaultQuotaBytes,
	).Scan(&o.Handle, &o.CreatedAt, &o.UpdatedAt)
	return database.MapError(err)
}

// SetOrgHandle changes the organisation's link handle. Every card link
// changes with it, and the old handle is free for anyone to take.
// database.ErrConflict if another organisation uses it.
func (r *Store) SetOrgHandle(ctx context.Context, orgID int64, handle string) error {
	return database.ExecOne(ctx, r.db, `UPDATE organizations SET handle = $2, updated_at = now() WHERE org_id = $1`, orgID, handle)
}

// IsOrgSuspended reports whether the organisation is suspended.
func (r *Store) IsOrgSuspended(ctx context.Context, orgID int64) (bool, error) {
	var suspended bool
	err := r.db.QueryRow(ctx, `SELECT suspended_at IS NOT NULL FROM organizations WHERE org_id = $1`, orgID).Scan(&suspended)
	return suspended, database.MapError(err)
}
