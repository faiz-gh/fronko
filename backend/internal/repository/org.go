package repository

import (
	"context"

	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

// ----------------------------------------------------------------------------
// Organisation Methods
// ----------------------------------------------------------------------------

func (r *Repository) GetOrganization(ctx context.Context, orgID int64) (*models.Organization, error) {
	var o models.Organization
	err := r.db.QueryRow(ctx,
		`SELECT org_id, name, default_quota_bytes, created_at, updated_at FROM organizations WHERE org_id = $1`, orgID,
	).Scan(&o.ID, &o.Name, &o.DefaultQuotaBytes, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &o, nil
}

// UpdateOrganization saves the name and default quota.
func (r *Repository) UpdateOrganization(ctx context.Context, o *models.Organization) error {
	err := r.db.QueryRow(ctx, `
		UPDATE organizations SET name = $2, default_quota_bytes = $3, updated_at = now()
		WHERE org_id = $1 RETURNING created_at, updated_at`, o.ID, o.Name, o.DefaultQuotaBytes,
	).Scan(&o.CreatedAt, &o.UpdatedAt)
	return mapError(err)
}

// orgUserQuery reads users with what they hold: cards assigned to them,
// leads that arrived while they held a card, and the size of their personal files.
func orgUserQuery(where string) string {
	return `SELECT ` + userColumns("u.") + `,
	       (SELECT COUNT(*) FROM profiles p WHERE p.assigned_user_id = u.user_id),
	       (SELECT COUNT(*) FROM leads l WHERE l.assigned_user_id = u.user_id),
	       (SELECT COALESCE(SUM(f.size_bytes), 0) FROM files f WHERE f.user_id = u.user_id AND f.area = 'personal')
	FROM users u WHERE ` + where
}

func scanOrgUser(row pgx.Row) (*models.OrgUser, error) {
	var u models.OrgUser
	err := row.Scan(append(userDest(&u.User), &u.CardCount, &u.LeadCount, &u.UsedBytes)...)
	if err != nil {
		return nil, mapError(err)
	}
	return &u, nil
}

// ListOrgUsers returns everyone in the organisation: owner first, then admins,
// then members, each group by username.
func (r *Repository) ListOrgUsers(ctx context.Context, orgID int64) ([]*models.OrgUser, error) {
	rows, err := r.db.Query(ctx, orgUserQuery(`u.org_id = $1`)+`
		ORDER BY CASE u.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, LOWER(u.username)`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []*models.OrgUser{}
	for rows.Next() {
		u, err := scanOrgUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetOrgUser returns one user of orgID with their totals; otherwise ErrNotFound.
func (r *Repository) GetOrgUser(ctx context.Context, userID, orgID int64) (*models.OrgUser, error) {
	return scanOrgUser(r.db.QueryRow(ctx, orgUserQuery(`u.user_id = $1 AND u.org_id = $2`), userID, orgID))
}

// SetUserQuota sets a user's storage limit; nil means unlimited.
func (r *Repository) SetUserQuota(ctx context.Context, userID int64, quota *int64) error {
	return r.execOne(ctx, `UPDATE users SET storage_quota_bytes = $2, updated_at = now() WHERE user_id = $1`, userID, quota)
}

// SetUserRole switches a user between admin and member. The owner's role
// never changes here.
func (r *Repository) SetUserRole(ctx context.Context, userID int64, role string) error {
	return r.execOne(ctx,
		`UPDATE users SET role = $2, updated_at = now() WHERE user_id = $1 AND role <> 'owner'`, userID, role)
}

// SetUserSuspended suspends or restores a user. Suspending also bumps the
// session version, so every session they have ends at once.
func (r *Repository) SetUserSuspended(ctx context.Context, userID int64, suspended bool) error {
	if suspended {
		return r.execOne(ctx, `
			UPDATE users SET suspended_at = COALESCE(suspended_at, now()),
				session_version = session_version + 1, updated_at = now()
			WHERE user_id = $1 AND role <> 'owner'`, userID)
	}
	return r.execOne(ctx, `UPDATE users SET suspended_at = NULL, updated_at = now() WHERE user_id = $1`, userID)
}

// SetTemporaryPassword gives a user a password chosen by their organisation:
// it signs out their sessions and makes them pick a new one when they next sign in.
func (r *Repository) SetTemporaryPassword(ctx context.Context, userID int64, hash string) error {
	return r.execOne(ctx, `
		UPDATE users SET password_hash = $2, must_change_password = true,
			session_version = session_version + 1, updated_at = now()
		WHERE user_id = $1`, userID, hash)
}

// DeleteOrgUser removes a user from their organisation without losing work:
// their personal files move into the organisation's area (tagged with their
// username) and anything else they uploaded or created passes to the owner.
// Cards they held become unassigned and their leads stay on the cards as the
// organisation's (both via ON DELETE SET NULL). The owner can't be deleted.
func (r *Repository) DeleteOrgUser(ctx context.Context, userID, orgID int64) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var username string
		var ownerID int64
		err := tx.QueryRow(ctx, `
			SELECT u.username, o.user_id FROM users u
			JOIN users o ON o.org_id = u.org_id AND o.role = 'owner'
			WHERE u.user_id = $1 AND u.org_id = $2 AND u.role <> 'owner'
			FOR UPDATE OF u`, userID, orgID).Scan(&username, &ownerID)
		if err != nil {
			return mapError(err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE files SET
				former_owner = CASE WHEN area = 'personal' THEN $3 ELSE former_owner END,
				area = CASE WHEN area = 'personal' THEN 'org' ELSE area END,
				user_id = $2
			WHERE user_id = $1`, userID, ownerID, username); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE profiles SET user_id = $2 WHERE user_id = $1`, userID, ownerID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM users WHERE user_id = $1`, userID)
		return mapError(err)
	})
}

// execOne runs an update that must touch exactly one row; otherwise ErrNotFound.
func (r *Repository) execOne(ctx context.Context, sql string, args ...any) error {
	tag, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
