package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

// ----------------------------------------------------------------------------
// Platform admins
// ----------------------------------------------------------------------------

const adminColumns = `admin_id, email, password_hash, session_version, last_login_at, created_at`

func scanAdmin(row pgx.Row) (*models.PlatformAdmin, error) {
	var a models.PlatformAdmin
	if err := row.Scan(&a.ID, &a.Email, &a.PasswordHash, &a.SessionVersion, &a.LastLoginAt, &a.CreatedAt); err != nil {
		return nil, mapError(err)
	}
	return &a, nil
}

// CreatePlatformAdmin adds an account that can sign in to the admin panel.
// It returns ErrConflict if the email is taken.
func (r *Repository) CreatePlatformAdmin(ctx context.Context, a *models.PlatformAdmin) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO platform_admins (email, password_hash) VALUES ($1, $2)
		RETURNING admin_id, session_version, created_at`, a.Email, a.PasswordHash,
	).Scan(&a.ID, &a.SessionVersion, &a.CreatedAt)
	return mapError(err)
}

// SetPlatformAdminPassword replaces an admin's password and signs out their sessions.
func (r *Repository) SetPlatformAdminPassword(ctx context.Context, email, hash string) error {
	return r.execOne(ctx, `
		UPDATE platform_admins SET password_hash = $2, session_version = session_version + 1
		WHERE LOWER(email) = LOWER($1)`, email, hash)
}

func (r *Repository) GetPlatformAdminByEmail(ctx context.Context, email string) (*models.PlatformAdmin, error) {
	return scanAdmin(r.db.QueryRow(ctx, `SELECT `+adminColumns+` FROM platform_admins WHERE LOWER(email) = LOWER($1)`, email))
}

func (r *Repository) GetPlatformAdmin(ctx context.Context, adminID int64) (*models.PlatformAdmin, error) {
	return scanAdmin(r.db.QueryRow(ctx, `SELECT `+adminColumns+` FROM platform_admins WHERE admin_id = $1`, adminID))
}

// TouchAdminLogin records an admin's successful sign-in.
func (r *Repository) TouchAdminLogin(ctx context.Context, adminID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE platform_admins SET last_login_at = now() WHERE admin_id = $1`, adminID)
	return mapError(err)
}

// ----------------------------------------------------------------------------
// Organisation usage
// ----------------------------------------------------------------------------

// orgUsageQuery is every organisation's usage, as aggregates only. The admin
// panel and the daily snapshots both read it, so their numbers always agree.
// It deliberately never selects card data, leads, file names or member details:
// the owner's email is the only personal data in it.
const orgUsageQuery = `
	SELECT o.org_id, o.name, o.handle, o.created_at, ow.email AS owner_email, o.suspended_at, o.suspended_reason,
	       u.user_count, u.admin_count, u.member_count, u.suspended_user_count,
	       c.card_count, l.lead_count, f.file_count, f.storage_used_bytes,
	       s.user_id IS NOT NULL AS storage_connected,
	       s.verified_at IS NOT NULL AS storage_verified,
	       s.provider AS storage_provider,
	       o.default_quota_bytes, u.last_active_at,
	       (SELECT COUNT(*) FROM teams t WHERE t.org_id = o.org_id) AS team_count,
	       o.logo_file IS NOT NULL AS logo_set, o.logo_policy,
	       COALESCE(o.signature->>'locked_template', '') <> '' AS signature_locked,
	       fp.files_by_purpose
	FROM organizations o
	LEFT JOIN users ow ON ow.org_id = o.org_id AND ow.role = 'owner'
	LEFT JOIN user_storage s ON s.user_id = ow.user_id
	CROSS JOIN LATERAL (
		SELECT COUNT(*) AS user_count,
		       COUNT(*) FILTER (WHERE role = 'admin') AS admin_count,
		       COUNT(*) FILTER (WHERE role = 'member') AS member_count,
		       COUNT(*) FILTER (WHERE suspended_at IS NOT NULL) AS suspended_user_count,
		       MAX(last_login_at) AS last_active_at
		FROM users WHERE org_id = o.org_id) u
	CROSS JOIN LATERAL (SELECT COUNT(*) AS card_count FROM profiles WHERE org_id = o.org_id) c
	CROSS JOIN LATERAL (
		SELECT COUNT(*) AS lead_count FROM leads ld
		JOIN profiles p ON p.profile_id = ld.profile_id WHERE p.org_id = o.org_id) l
	CROSS JOIN LATERAL (
		SELECT COUNT(*) AS file_count, COALESCE(SUM(size_bytes), 0)::bigint AS storage_used_bytes
		FROM files WHERE org_id = o.org_id) f
	CROSS JOIN LATERAL (
		SELECT COALESCE(jsonb_object_agg(purpose, n), '{}') AS files_by_purpose
		FROM (SELECT purpose, COUNT(*) AS n FROM files WHERE org_id = o.org_id GROUP BY purpose) p) fp`

const orgUsageColumns = `org_id, name, handle, created_at, owner_email, suspended_at, suspended_reason,
	user_count, admin_count, member_count, suspended_user_count,
	card_count, lead_count, file_count, storage_used_bytes,
	storage_connected, storage_verified, storage_provider, default_quota_bytes, last_active_at, team_count,
	logo_set, logo_policy, signature_locked, files_by_purpose`

func scanOrgUsage(row pgx.Row) (*models.OrgUsage, error) {
	var o models.OrgUsage
	var byPurpose []byte
	err := row.Scan(&o.ID, &o.Name, &o.Handle, &o.CreatedAt, &o.OwnerEmail, &o.SuspendedAt, &o.SuspendedReason,
		&o.UserCount, &o.AdminCount, &o.MemberCount, &o.SuspendedUserCount,
		&o.CardCount, &o.LeadCount, &o.FileCount, &o.StorageUsedBytes,
		&o.StorageConnected, &o.StorageVerified, &o.StorageProvider, &o.DefaultQuotaBytes, &o.LastActiveAt,
		&o.TeamCount, &o.LogoSet, &o.LogoPolicy, &o.SignatureLocked, &byPurpose)
	if err != nil {
		return nil, mapError(err)
	}
	o.FilesByPurpose = map[string]int64{}
	if err := json.Unmarshal(byPurpose, &o.FilesByPurpose); err != nil {
		return nil, fmt.Errorf("decode files by purpose: %w", err)
	}
	return &o, nil
}

// OrgUsageFilter narrows and orders the organisation list.
type OrgUsageFilter struct {
	Search string // case-insensitive substring of the name or owner email
	Status string // "active", "suspended" or "" for both
	Sort   string // a key of orgUsageSorts; "" is newest first
	Limit  int
	Offset int
}

// orgUsageSorts maps the sort keys the API accepts to ORDER BY clauses.
var orgUsageSorts = map[string]string{
	"":            "created_at DESC",
	"newest":      "created_at DESC",
	"oldest":      "created_at ASC",
	"name":        "LOWER(name) ASC",
	"users":       "user_count DESC",
	"teams":       "team_count DESC",
	"cards":       "card_count DESC",
	"leads":       "lead_count DESC",
	"storage":     "storage_used_bytes DESC",
	"last_active": "last_active_at DESC NULLS LAST",
}

// ValidOrgUsageSort reports whether the API accepts this sort key.
func ValidOrgUsageSort(key string) bool {
	_, ok := orgUsageSorts[key]
	return ok
}

// ListOrgUsage returns one page of organisations with their usage, and the total matching.
func (r *Repository) ListOrgUsage(ctx context.Context, f OrgUsageFilter) ([]*models.OrgUsage, int64, error) {
	order, ok := orgUsageSorts[f.Sort]
	if !ok {
		return nil, 0, fmt.Errorf("unknown sort %q", f.Sort)
	}
	where := `
		WHERE ($1::text = '' OR name ILIKE $1 ESCAPE '\' OR owner_email ILIKE $1 ESCAPE '\')
		  AND ($2::text = '' OR ($2 = 'suspended') = (suspended_at IS NOT NULL))`
	search := ""
	if s := strings.TrimSpace(f.Search); s != "" {
		search = likePattern(s)
	}
	from := ` FROM (` + orgUsageQuery + `) usage` + where

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)`+from, search, f.Status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, `SELECT `+orgUsageColumns+from+`
		ORDER BY `+order+`, org_id DESC LIMIT $3 OFFSET $4`, search, f.Status, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	orgs := []*models.OrgUsage{}
	for rows.Next() {
		o, err := scanOrgUsage(rows)
		if err != nil {
			return nil, 0, err
		}
		orgs = append(orgs, o)
	}
	return orgs, total, rows.Err()
}

// GetOrgUsage returns one organisation's usage, or ErrNotFound.
func (r *Repository) GetOrgUsage(ctx context.Context, orgID int64) (*models.OrgUsage, error) {
	return scanOrgUsage(r.db.QueryRow(ctx,
		`SELECT `+orgUsageColumns+` FROM (`+orgUsageQuery+`) usage WHERE org_id = $1`, orgID))
}

// PlatformSummary totals usage across every organisation.
func (r *Repository) PlatformSummary(ctx context.Context) (*models.PlatformSummary, error) {
	var s models.PlatformSummary
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE suspended_at IS NOT NULL),
		       COUNT(*) FILTER (WHERE created_at > now() - interval '30 days'),
		       COALESCE(SUM(user_count), 0)::bigint,
		       COALESCE(SUM(card_count), 0)::bigint,
		       COALESCE(SUM(lead_count), 0)::bigint,
		       COALESCE(SUM(file_count), 0)::bigint,
		       COUNT(*) FILTER (WHERE storage_connected),
		       COALESCE(SUM(storage_used_bytes), 0)::bigint,
		       COUNT(*) FILTER (WHERE last_active_at > now() - interval '30 days'),
		       (SELECT COUNT(*) FROM feedback WHERE status = 'new'),
		       COALESCE(SUM(team_count), 0)::bigint,
		       COUNT(*) FILTER (WHERE team_count > 0),
		       COUNT(*) FILTER (WHERE logo_set)
		FROM (`+orgUsageQuery+`) usage`,
	).Scan(&s.OrgCount, &s.SuspendedOrgCount, &s.NewOrgs30d, &s.UserCount, &s.CardCount, &s.LeadCount,
		&s.FileCount, &s.OrgsWithStorage, &s.StorageUsedBytes, &s.ActiveOrgs30d, &s.NewFeedback,
		&s.TeamCount, &s.OrgsWithTeams, &s.OrgsWithLogo)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ----------------------------------------------------------------------------
// Usage snapshots (trends)
// ----------------------------------------------------------------------------

// TakeUsageSnapshot records every organisation's usage, and the platform
// totals, for the given day. Running it again the same day overwrites that
// day's numbers, so the last run of a day is what the trend shows.
func (r *Repository) TakeUsageSnapshot(ctx context.Context, day time.Time) error {
	date := day.UTC().Format(time.DateOnly)
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO org_usage_snapshots (snapshot_date, org_id, user_count, card_count, lead_count,
				file_count, storage_used_bytes, storage_connected, team_count)
			SELECT $1::date, org_id, user_count, card_count, lead_count, file_count, storage_used_bytes, storage_connected,
			       team_count
			FROM (`+orgUsageQuery+`) usage
			ON CONFLICT (snapshot_date, org_id) DO UPDATE SET
				team_count = EXCLUDED.team_count,
				user_count = EXCLUDED.user_count, card_count = EXCLUDED.card_count,
				lead_count = EXCLUDED.lead_count, file_count = EXCLUDED.file_count,
				storage_used_bytes = EXCLUDED.storage_used_bytes, storage_connected = EXCLUDED.storage_connected`,
			date); err != nil {
			return fmt.Errorf("org snapshot: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO platform_usage_snapshots (snapshot_date, org_count, user_count, card_count, lead_count,
				file_count, orgs_with_storage, storage_used_bytes, new_orgs, feedback_count,
				team_count, orgs_with_teams, orgs_with_logo)
			SELECT $1::date, COUNT(*),
			       COALESCE(SUM(user_count), 0), COALESCE(SUM(card_count), 0), COALESCE(SUM(lead_count), 0),
			       COALESCE(SUM(file_count), 0), COUNT(*) FILTER (WHERE storage_connected),
			       COALESCE(SUM(storage_used_bytes), 0),
			       COUNT(*) FILTER (WHERE (created_at AT TIME ZONE 'UTC')::date = $1::date),
			       (SELECT COUNT(*) FROM feedback WHERE (created_at AT TIME ZONE 'UTC')::date = $1::date),
			       COALESCE(SUM(team_count), 0), COUNT(*) FILTER (WHERE team_count > 0), COUNT(*) FILTER (WHERE logo_set)
			FROM (`+orgUsageQuery+`) usage
			ON CONFLICT (snapshot_date) DO UPDATE SET
				org_count = EXCLUDED.org_count, user_count = EXCLUDED.user_count,
				card_count = EXCLUDED.card_count, lead_count = EXCLUDED.lead_count,
				file_count = EXCLUDED.file_count, orgs_with_storage = EXCLUDED.orgs_with_storage,
				storage_used_bytes = EXCLUDED.storage_used_bytes, new_orgs = EXCLUDED.new_orgs,
				feedback_count = EXCLUDED.feedback_count, team_count = EXCLUDED.team_count,
				orgs_with_teams = EXCLUDED.orgs_with_teams, orgs_with_logo = EXCLUDED.orgs_with_logo`,
			date); err != nil {
			return fmt.Errorf("platform snapshot: %w", err)
		}
		return nil
	})
}

// PlatformTrend returns the platform totals for the last `days` days, oldest first.
func (r *Repository) PlatformTrend(ctx context.Context, days int) ([]models.UsagePoint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT to_char(snapshot_date, 'YYYY-MM-DD'), org_count, user_count, card_count, lead_count,
		       file_count, orgs_with_storage, storage_used_bytes, new_orgs, feedback_count,
		       team_count, orgs_with_teams, orgs_with_logo
		FROM platform_usage_snapshots
		WHERE snapshot_date > (now() AT TIME ZONE 'UTC')::date - $1::int
		ORDER BY snapshot_date`, days)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.UsagePoint, error) {
		var p models.UsagePoint
		err := row.Scan(&p.Date, &p.OrgCount, &p.UserCount, &p.CardCount, &p.LeadCount,
			&p.FileCount, &p.OrgsWithStorage, &p.StorageUsedBytes, &p.NewOrgs, &p.FeedbackCount,
			&p.TeamCount, &p.OrgsWithTeams, &p.OrgsWithLogo)
		return p, err
	})
}

// OrgTrend returns one organisation's usage for the last `days` days, oldest first.
func (r *Repository) OrgTrend(ctx context.Context, orgID int64, days int) ([]models.UsagePoint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT to_char(snapshot_date, 'YYYY-MM-DD'), user_count, card_count, lead_count, file_count, storage_used_bytes,
		       team_count
		FROM org_usage_snapshots
		WHERE org_id = $1 AND snapshot_date > (now() AT TIME ZONE 'UTC')::date - $2::int
		ORDER BY snapshot_date`, orgID, days)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.UsagePoint, error) {
		var p models.UsagePoint
		err := row.Scan(&p.Date, &p.UserCount, &p.CardCount, &p.LeadCount, &p.FileCount, &p.StorageUsedBytes, &p.TeamCount)
		return p, err
	})
}

// ----------------------------------------------------------------------------
// Suspending organisations
// ----------------------------------------------------------------------------

// SuspendOrg suspends an organisation: nobody in it can sign in and its public
// cards go offline. Every member's session version is bumped in the same
// transaction, so their open sessions end at once.
func (r *Repository) SuspendOrg(ctx context.Context, orgID int64, reason string) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE organizations SET suspended_at = COALESCE(suspended_at, now()), suspended_reason = $2, updated_at = now()
			WHERE org_id = $1`, orgID, reason)
		if err != nil {
			return mapError(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		_, err = tx.Exec(ctx, `UPDATE users SET session_version = session_version + 1 WHERE org_id = $1`, orgID)
		return err
	})
}

// ReinstateOrg lifts an organisation's suspension.
func (r *Repository) ReinstateOrg(ctx context.Context, orgID int64) error {
	return r.execOne(ctx, `
		UPDATE organizations SET suspended_at = NULL, suspended_reason = NULL, updated_at = now()
		WHERE org_id = $1`, orgID)
}

// IsOrgSuspended reports whether the organisation is suspended.
func (r *Repository) IsOrgSuspended(ctx context.Context, orgID int64) (bool, error) {
	var suspended bool
	err := r.db.QueryRow(ctx, `SELECT suspended_at IS NOT NULL FROM organizations WHERE org_id = $1`, orgID).Scan(&suspended)
	return suspended, mapError(err)
}

// ----------------------------------------------------------------------------
// Audit log
// ----------------------------------------------------------------------------

// AuditTarget is what an audited action was done to; the zero value means nothing in particular.
type AuditTarget struct {
	Type string
	ID   int64
}

// AddAudit records something a platform admin did.
func (r *Repository) AddAudit(ctx context.Context, admin *models.PlatformAdmin, action string, target AuditTarget, detail map[string]any) error {
	if detail == nil {
		detail = map[string]any{}
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	var targetType *string
	var targetID *int64
	if target.Type != "" {
		targetType, targetID = &target.Type, &target.ID
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO admin_audit_log (admin_id, admin_email, action, target_type, target_id, detail)
		VALUES ($1, $2, $3, $4, $5, $6)`, admin.ID, admin.Email, action, targetType, targetID, raw)
	return mapError(err)
}

// ListAudit returns one page of the audit log, newest first, and its total size.
func (r *Repository) ListAudit(ctx context.Context, limit, offset int) ([]*models.AuditEntry, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM admin_audit_log`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT log_id, admin_id, admin_email, action, target_type, target_id, detail, created_at
		FROM admin_audit_log ORDER BY created_at DESC, log_id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	entries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*models.AuditEntry, error) {
		var e models.AuditEntry
		err := row.Scan(&e.ID, &e.AdminID, &e.AdminEmail, &e.Action, &e.TargetType, &e.TargetID, &e.Detail, &e.CreatedAt)
		return &e, err
	})
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}
