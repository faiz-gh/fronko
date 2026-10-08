package users

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
)

// Store runs the SQL for users, their email codes and the organisation's user list.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// CreateUser inserts a user into an existing organisation (user.OrgID).
// Registering a new organisation goes through orgs.Store.CreateOrgWithOwner,
// and adding someone with their teams through orgs.Store.CreateMember.
func (r *Store) CreateUser(ctx context.Context, user *User) error {
	return Insert(ctx, r.db, user)
}

// Insert adds user with q, which may be a transaction.
func Insert(ctx context.Context, q database.Querier, user *User) error {
	query := `
		INSERT INTO users (org_id, role, username, email, email_verified_at, password_hash, must_change_password,
			storage_quota_bytes, created_by, full_name, external_id, external_username, provisioned_by)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8, $9, $10, $11, $12, $13)
		RETURNING user_id, session_version, created_at, updated_at`
	err := q.QueryRow(ctx, query, user.OrgID, user.Role, user.Username, user.Email, user.EmailVerifiedAt, user.PasswordHash,
		user.MustChangePassword, user.StorageQuotaBytes, user.CreatedBy, user.FullName, user.ExternalID,
		user.ExternalUsername, user.ProvisionedBy).Scan(
		&user.ID, &user.SessionVersion, &user.CreatedAt, &user.UpdatedAt,
	)
	return database.MapError(err)
}

// IsEmailConflict reports whether a CreateUser/SetUserEmail error came from
// the email unique index rather than the username one.
func IsEmailConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == "users_email_lower_idx"
}

// userColumns lists the columns scanUser expects, optionally table-qualified.
func userColumns(p string) string {
	cols := []string{"user_id", "org_id", "role", "username", "COALESCE(" + p + "password_hash, '')", "email",
		"email_verified_at", "must_change_password", "storage_quota_bytes", "suspended_at", "created_by",
		"last_login_at", "session_version", "full_name", "external_id", "external_username", "provisioned_by",
		"created_at", "updated_at"}
	for i, c := range cols {
		if !strings.Contains(c, "(") {
			cols[i] = p + c
		}
	}
	return strings.Join(cols, ", ")
}

// userDest returns scan destinations matching userColumns.
func userDest(u *User) []any {
	return []any{&u.ID, &u.OrgID, &u.Role, &u.Username, &u.PasswordHash, &u.Email, &u.EmailVerifiedAt,
		&u.MustChangePassword, &u.StorageQuotaBytes, &u.SuspendedAt, &u.CreatedBy, &u.LastLoginAt,
		&u.SessionVersion, &u.FullName, &u.ExternalID, &u.ExternalUsername, &u.ProvisionedBy, &u.CreatedAt, &u.UpdatedAt}
}

func (r *Store) getUser(ctx context.Context, where string, args ...any) (*User, error) {
	var user User
	err := r.db.QueryRow(ctx, `SELECT `+userColumns("")+` FROM users WHERE `+where, args...).Scan(userDest(&user)...)
	if err != nil {
		return nil, database.MapError(err)
	}
	return &user, nil
}

// FreeUsername returns base, or base-2, base-3… for the first username no
// account uses. The insert can still race; callers retry on a conflict.
func (r *Store) FreeUsername(ctx context.Context, base string) (string, error) {
	for n := 1; n < 1000; n++ {
		name := base
		if n > 1 {
			name = fmt.Sprintf("%s-%d", base, n)
		}
		var taken bool
		if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE LOWER(username) = LOWER($1))`,
			name).Scan(&taken); err != nil {
			return "", err
		}
		if !taken {
			return name, nil
		}
	}
	return "", errors.New("no free username")
}

// GetUserByUsername matches case-insensitively, consistent with users_username_lower_idx.
func (r *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	return r.getUser(ctx, `LOWER(username) = LOWER($1)`, username)
}

// GetUserByEmail matches case-insensitively, consistent with users_email_lower_idx.
func (r *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return r.getUser(ctx, `LOWER(email) = LOWER($1)`, email)
}

func (r *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	return r.getUser(ctx, `user_id = $1`, id)
}

// RehashPassword replaces a password hash with a stronger one for the same
// password, unless it changed meanwhile. Sessions stay valid.
func (r *Store) RehashPassword(ctx context.Context, userID int64, oldHash, newHash string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET password_hash = $3 WHERE user_id = $1 AND password_hash = $2`,
		userID, oldHash, newHash)
	return err
}

// GetOrgOwner returns the organisation's owner.
func (r *Store) GetOrgOwner(ctx context.Context, orgID int64) (*User, error) {
	return r.getUser(ctx, `org_id = $1 AND role = 'owner'`, orgID)
}

// GetUserInOrg returns the user only if they belong to orgID; otherwise database.ErrNotFound.
func (r *Store) GetUserInOrg(ctx context.Context, userID, orgID int64) (*User, error) {
	return r.getUser(ctx, `user_id = $1 AND org_id = $2`, userID, orgID)
}

// GetSessionState is the per-request check behind the JWT middleware.
func (r *Store) GetSessionState(ctx context.Context, userID int64) (auth.SessionState, error) {
	var s auth.SessionState
	var teams []byte
	err := r.db.QueryRow(ctx, `
		SELECT u.session_version, u.email_verified_at IS NOT NULL, u.org_id, u.role,
		       u.suspended_at IS NOT NULL, u.must_change_password,
		       o.suspended_at IS NOT NULL, COALESCE(o.suspended_reason, ''), `+TeamsJSON("u.user_id")+`
		FROM users u JOIN organizations o ON o.org_id = u.org_id
		WHERE u.user_id = $1`, userID,
	).Scan(&s.Version, &s.Verified, &s.OrgID, &s.Role, &s.Suspended, &s.MustChangePassword,
		&s.OrgSuspended, &s.OrgSuspendedReason, &teams)
	if err != nil {
		return s, database.MapError(err)
	}
	s.Teams, err = DecodeTeamRefs(teams)
	return s, err
}

// TouchLastLogin records a successful sign-in.
func (r *Store) TouchLastLogin(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE user_id = $1`, userID)
	return database.MapError(err)
}

// SetUserEmail replaces the address and marks it unverified.
func (r *Store) SetUserEmail(ctx context.Context, userID int64, email string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET email = $2, email_verified_at = NULL, updated_at = now() WHERE user_id = $1`, userID, email)
	if err != nil {
		return database.MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrNotFound
	}
	return nil
}

// ChangeUserEmail switches a verified account to a new address that has just
// been confirmed with a code, so it's stored as verified.
func (r *Store) ChangeUserEmail(ctx context.Context, userID int64, email string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET email = $2, email_verified_at = now(), updated_at = now() WHERE user_id = $1`, userID, email)
	if err != nil {
		return database.MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrNotFound
	}
	return nil
}

func (r *Store) MarkEmailVerified(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET email_verified_at = now(), updated_at = now() WHERE user_id = $1`, userID)
	return database.MapError(err)
}

// UpdatePassword sets a password the user chose and bumps the session
// version, which signs out every existing session. It also clears
// must_change_password. It returns the new version.
func (r *Store) UpdatePassword(ctx context.Context, userID int64, hash string) (int, error) {
	var version int
	err := r.db.QueryRow(ctx, `
		UPDATE users SET password_hash = $2, must_change_password = false,
			session_version = session_version + 1, updated_at = now()
		WHERE user_id = $1
		RETURNING session_version`, userID, hash).Scan(&version)
	return version, database.MapError(err)
}

// UpsertEmailCode replaces any live code for the same user and purpose.
func (r *Store) UpsertEmailCode(ctx context.Context, c *EmailCode) error {
	query := `
		INSERT INTO email_codes (user_id, purpose, email, code_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, purpose) DO UPDATE
		SET email = EXCLUDED.email, code_hash = EXCLUDED.code_hash, attempts = 0,
			expires_at = EXCLUDED.expires_at, created_at = now()
		RETURNING attempts, created_at`
	err := r.db.QueryRow(ctx, query, c.UserID, c.Purpose, c.Email, c.CodeHash, c.ExpiresAt).Scan(&c.Attempts, &c.CreatedAt)
	return database.MapError(err)
}

func (r *Store) GetEmailCode(ctx context.Context, userID int64, purpose string) (*EmailCode, error) {
	c := EmailCode{UserID: userID, Purpose: purpose}
	err := r.db.QueryRow(ctx,
		`SELECT email, code_hash, attempts, expires_at, created_at FROM email_codes WHERE user_id = $1 AND purpose = $2`,
		userID, purpose,
	).Scan(&c.Email, &c.CodeHash, &c.Attempts, &c.ExpiresAt, &c.CreatedAt)
	if err != nil {
		return nil, database.MapError(err)
	}
	return &c, nil
}

// UseEmailCodeAttempt spends one guess on a live code and returns it for
// checking. Counting the attempt before the comparison keeps concurrent
// guesses from exceeding maxAttempts. database.ErrNotFound means there's no usable code:
// none was issued, it expired, or its attempts ran out.
func (r *Store) UseEmailCodeAttempt(ctx context.Context, userID int64, purpose string, maxAttempts int) (*EmailCode, error) {
	c := EmailCode{UserID: userID, Purpose: purpose}
	err := r.db.QueryRow(ctx, `
		UPDATE email_codes SET attempts = attempts + 1
		WHERE user_id = $1 AND purpose = $2 AND attempts < $3 AND expires_at > now()
		RETURNING email, code_hash, attempts, expires_at, created_at`,
		userID, purpose, maxAttempts,
	).Scan(&c.Email, &c.CodeHash, &c.Attempts, &c.ExpiresAt, &c.CreatedAt)
	if err != nil {
		return nil, database.MapError(err)
	}
	return &c, nil
}

func (r *Store) DeleteEmailCode(ctx context.Context, userID int64, purpose string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM email_codes WHERE user_id = $1 AND purpose = $2`, userID, purpose)
	return database.MapError(err)
}

// orgUserQuery reads users with what they hold: cards assigned to them,
// leads that arrived while they held a card, the size of their personal
// files, and the teams they're in.
func orgUserQuery(where string) string {
	return `SELECT ` + userColumns("u.") + `,
	       (SELECT COUNT(*) FROM profiles p WHERE p.assigned_user_id = u.user_id),
	       (SELECT COUNT(*) FROM leads l WHERE l.assigned_user_id = u.user_id),
	       (SELECT COALESCE(SUM(f.size_bytes), 0) FROM files f WHERE f.user_id = u.user_id AND f.area = 'personal'),
	       ` + TeamsJSON("u.user_id") + `
	FROM users u WHERE ` + where
}

func scanOrgUser(row pgx.Row) (*OrgUser, error) {
	var u OrgUser
	var teams []byte
	err := row.Scan(append(userDest(&u.User), &u.CardCount, &u.LeadCount, &u.UsedBytes, &teams)...)
	if err != nil {
		return nil, database.MapError(err)
	}
	if u.Teams, err = DecodeTeamRefs(teams); err != nil {
		return nil, err
	}
	return &u, nil
}

// ListOrgUsers returns everyone in the organisation: owner first, then admins,
// then members, each group by username.
func (r *Store) ListOrgUsers(ctx context.Context, orgID int64) ([]*OrgUser, error) {
	rows, err := r.db.Query(ctx, orgUserQuery(`u.org_id = $1`)+`
		ORDER BY CASE u.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, LOWER(u.username)`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []*OrgUser{}
	for rows.Next() {
		u, err := scanOrgUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetOrgUser returns one user of orgID with their totals; otherwise database.ErrNotFound.
func (r *Store) GetOrgUser(ctx context.Context, userID, orgID int64) (*OrgUser, error) {
	return scanOrgUser(r.db.QueryRow(ctx, orgUserQuery(`u.user_id = $1 AND u.org_id = $2`), userID, orgID))
}

// SetUserQuota sets a user's storage limit; nil means unlimited.
func (r *Store) SetUserQuota(ctx context.Context, userID int64, quota *int64) error {
	return database.ExecOne(ctx, r.db, `UPDATE users SET storage_quota_bytes = $2, updated_at = now() WHERE user_id = $1`, userID, quota)
}

// SetUserRole switches a user between admin and member. The owner's role
// never changes here.
func (r *Store) SetUserRole(ctx context.Context, userID int64, role string) error {
	return database.ExecOne(ctx, r.db,
		`UPDATE users SET role = $2, updated_at = now() WHERE user_id = $1 AND role <> 'owner'`, userID, role)
}

// SetUserSuspended suspends or restores a user. Suspending also bumps the
// session version, so every session they have ends at once.
func (r *Store) SetUserSuspended(ctx context.Context, userID int64, suspended bool) error {
	if suspended {
		return database.ExecOne(ctx, r.db, `
			UPDATE users SET suspended_at = COALESCE(suspended_at, now()),
				session_version = session_version + 1, updated_at = now()
			WHERE user_id = $1 AND role <> 'owner'`, userID)
	}
	return database.ExecOne(ctx, r.db, `UPDATE users SET suspended_at = NULL, updated_at = now() WHERE user_id = $1`, userID)
}

// SetTemporaryPassword gives a user a password chosen by their organisation:
// it signs out their sessions and makes them pick a new one when they next sign in.
func (r *Store) SetTemporaryPassword(ctx context.Context, userID int64, hash string) error {
	return database.ExecOne(ctx, r.db, `
		UPDATE users SET password_hash = $2, must_change_password = true,
			session_version = session_version + 1, updated_at = now()
		WHERE user_id = $1`, userID, hash)
}

// DeleteOrgUser removes a user from their organisation without losing work:
// their personal files move into the organisation's area (tagged with their
// username) and anything else they uploaded or created passes to the owner.
// Cards they held become unassigned and their leads stay on the cards as the
// organisation's (both via ON DELETE SET NULL). The owner can't be deleted.
func (r *Store) DeleteOrgUser(ctx context.Context, userID, orgID int64) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var username string
		var ownerID int64
		err := tx.QueryRow(ctx, `
			SELECT u.username, o.user_id FROM users u
			JOIN users o ON o.org_id = u.org_id AND o.role = 'owner'
			WHERE u.user_id = $1 AND u.org_id = $2 AND u.role <> 'owner'
			FOR UPDATE OF u`, userID, orgID).Scan(&username, &ownerID)
		if err != nil {
			return database.MapError(err)
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
		if err := forgetUser(ctx, tx, userID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM users WHERE user_id = $1`, userID)
		return database.MapError(err)
	})
}

// forgetUser erases what would outlive a deleted user and still identify
// them: the email on feedback they sent, and integration log lines about
// them (single sign-on records "Signed in <email>").
func forgetUser(ctx context.Context, tx pgx.Tx, userID int64) error {
	if _, err := tx.Exec(ctx, `UPDATE feedback SET sender_email = NULL WHERE user_id = $1`, userID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `DELETE FROM integration_activity WHERE user_id = $1 AND lead_id IS NULL`, userID)
	return err
}

// Rekey re-encrypts an organisation's bucket keys for a new owner, since
// they're bound to the owner who holds them (see files.StorageService).
type Rekey func(accessKeyIDEnc, secretAccessKeyEnc []byte) (newAccessKeyIDEnc, newSecretAccessKeyEnc []byte, err error)

// TransferOwnership makes toID the organisation's owner and fromID an admin,
// moving the organisation's bucket keys (re-encrypted by rekey) to the new
// owner. Both are signed out everywhere, since their roles changed.
// database.ErrNotFound unless fromID is the owner and toID an active,
// verified member of the same organisation.
func (r *Store) TransferOwnership(ctx context.Context, orgID, fromID, toID int64, rekey Rekey) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM users WHERE org_id = $1 AND (
				(user_id = $2 AND role = 'owner') OR
				(user_id = $3 AND role <> 'owner' AND suspended_at IS NULL AND email_verified_at IS NOT NULL))`,
			orgID, fromID, toID).Scan(&n); err != nil {
			return err
		}
		if n != 2 || fromID == toID {
			return database.ErrNotFound
		}
		// Only one owner at a time (users_one_owner_per_org_idx), so demote first.
		if _, err := tx.Exec(ctx, `
			UPDATE users SET role = 'admin', session_version = session_version + 1, updated_at = now()
			WHERE user_id = $1`, fromID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE users SET role = 'owner', session_version = session_version + 1, updated_at = now()
			WHERE user_id = $1`, toID); err != nil {
			return err
		}

		var idEnc, secretEnc []byte
		err := tx.QueryRow(ctx, `
			SELECT access_key_id_enc, secret_access_key_enc FROM user_storage WHERE user_id = $1 FOR UPDATE`,
			fromID).Scan(&idEnc, &secretEnc)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		newID, newSecret, err := rekey(idEnc, secretEnc)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM user_storage WHERE user_id = $1`, toID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE user_storage SET user_id = $2, access_key_id_enc = $3, secret_access_key_enc = $4, updated_at = now()
			WHERE user_id = $1`, fromID, toID, newID, newSecret)
		return err
	})
}

// DeletionSummary is what deleting an account would affect, so the
// dashboard can spell it out first.
type DeletionSummary struct {
	// What the user created or holds.
	CardsHeld     int64 `json:"cards_held"`
	CardsMade     int64 `json:"cards_made"`
	Files         int64 `json:"files"`
	PersonalFiles int64 `json:"personal_files"`
	Leads         int64 `json:"leads"`
	// The whole organisation, for an owner deleting it.
	OrgMembers int64 `json:"org_members"`
	OrgCards   int64 `json:"org_cards"`
	OrgLeads   int64 `json:"org_leads"`
	OrgFiles   int64 `json:"org_files"`
	OrgTeams   int64 `json:"org_teams"`
}

// GetDeletionSummary counts what the user has and what their organisation has.
func (r *Store) GetDeletionSummary(ctx context.Context, userID, orgID int64) (*DeletionSummary, error) {
	var d DeletionSummary
	err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM profiles WHERE assigned_user_id = $1),
			(SELECT COUNT(*) FROM profiles WHERE user_id = $1),
			(SELECT COUNT(*) FROM files WHERE user_id = $1),
			(SELECT COUNT(*) FROM files WHERE user_id = $1 AND area = 'personal'),
			(SELECT COUNT(*) FROM leads WHERE assigned_user_id = $1),
			(SELECT COUNT(*) FROM users WHERE org_id = $2),
			(SELECT COUNT(*) FROM profiles WHERE org_id = $2),
			(SELECT COUNT(*) FROM leads l JOIN profiles p ON p.profile_id = l.profile_id WHERE p.org_id = $2),
			(SELECT COUNT(*) FROM files WHERE org_id = $2),
			(SELECT COUNT(*) FROM teams WHERE org_id = $2)`, userID, orgID,
	).Scan(&d.CardsHeld, &d.CardsMade, &d.Files, &d.PersonalFiles, &d.Leads,
		&d.OrgMembers, &d.OrgCards, &d.OrgLeads, &d.OrgFiles, &d.OrgTeams)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ListTransferCandidates returns who the organisation's ownership could pass
// to: everyone else who is active and has a verified email, admins first.
func (r *Store) ListTransferCandidates(ctx context.Context, orgID, ownerID int64) ([]auth.UserRef, error) {
	rows, err := r.db.Query(ctx, `
		SELECT user_id, username FROM users
		WHERE org_id = $1 AND user_id <> $2 AND suspended_at IS NULL AND email_verified_at IS NOT NULL
		ORDER BY role = 'admin' DESC, LOWER(username)`, orgID, ownerID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (auth.UserRef, error) {
		var u auth.UserRef
		err := row.Scan(&u.ID, &u.Username)
		return u, err
	})
}

// PurgeExpiredCodes deletes one-time codes past their expiry.
func (r *Store) PurgeExpiredCodes(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM email_codes WHERE expires_at < now()`)
	return tag.RowsAffected(), err
}
