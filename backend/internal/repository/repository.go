package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("record not found")
	ErrConflict = errors.New("record already exists")
)

// mapError translates driver errors into repository sentinel errors.
func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			// Keep the driver error too, so callers can tell which constraint hit.
			return fmt.Errorf("%w: %w", ErrConflict, err)
		case "23503": // foreign_key_violation
			return ErrNotFound
		}
	}
	return err
}

type Repository struct {
	db *pgxpool.Pool
}

// querier is what both the pool and a transaction offer, so helpers can run in either.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Scope limits a query to what a signed-in user may see: everything in their
// organisation for owners and admins, or only their own things for members.
type Scope struct {
	OrgID  int64
	UserID int64
	Admin  bool
}

// memberID is the user id to filter by for members, or 0 for admins (no filter).
func (s Scope) memberID() int64 {
	if s.Admin {
		return 0
	}
	return s.UserID
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ----------------------------------------------------------------------------
// User Methods
// ----------------------------------------------------------------------------

// CreateUser inserts a user into an existing organisation (user.OrgID).
// Registering a new organisation goes through CreateOrgWithOwner instead.
func (r *Repository) CreateUser(ctx context.Context, user *models.User) error {
	return createUser(ctx, r.db, user)
}

func createUser(ctx context.Context, q querier, user *models.User) error {
	query := `
		INSERT INTO users (org_id, role, username, email, password_hash, must_change_password, storage_quota_bytes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING user_id, session_version, created_at, updated_at`
	err := q.QueryRow(ctx, query, user.OrgID, user.Role, user.Username, user.Email, user.PasswordHash,
		user.MustChangePassword, user.StorageQuotaBytes, user.CreatedBy).Scan(
		&user.ID, &user.SessionVersion, &user.CreatedAt, &user.UpdatedAt,
	)
	return mapError(err)
}

// CreateOrgWithOwner registers a new organisation named orgName with user as
// its owner, in one transaction. It fills in user.OrgID and user.Role.
func (r *Repository) CreateOrgWithOwner(ctx context.Context, orgName string, user *models.User) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO organizations (name) VALUES ($1) RETURNING org_id`, orgName).Scan(&user.OrgID); err != nil {
			return mapError(err)
		}
		user.Role = models.RoleOwner
		return createUser(ctx, tx, user)
	})
}

// IsEmailConflict reports whether a CreateUser/SetUserEmail error came from
// the email unique index rather than the username one.
func IsEmailConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == "users_email_lower_idx"
}

// userColumns lists the columns scanUser expects, optionally table-qualified.
func userColumns(p string) string {
	cols := []string{"user_id", "org_id", "role", "username", "password_hash", "email", "email_verified_at",
		"must_change_password", "storage_quota_bytes", "suspended_at", "created_by", "last_login_at",
		"session_version", "created_at", "updated_at"}
	return p + strings.Join(cols, ", "+p)
}

// userDest returns scan destinations matching userColumns.
func userDest(u *models.User) []any {
	return []any{&u.ID, &u.OrgID, &u.Role, &u.Username, &u.PasswordHash, &u.Email, &u.EmailVerifiedAt,
		&u.MustChangePassword, &u.StorageQuotaBytes, &u.SuspendedAt, &u.CreatedBy, &u.LastLoginAt,
		&u.SessionVersion, &u.CreatedAt, &u.UpdatedAt}
}

func (r *Repository) getUser(ctx context.Context, where string, args ...any) (*models.User, error) {
	var user models.User
	err := r.db.QueryRow(ctx, `SELECT `+userColumns("")+` FROM users WHERE `+where, args...).Scan(userDest(&user)...)
	if err != nil {
		return nil, mapError(err)
	}
	return &user, nil
}

// GetUserByUsername matches case-insensitively, consistent with users_username_lower_idx.
func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return r.getUser(ctx, `LOWER(username) = LOWER($1)`, username)
}

// GetUserByEmail matches case-insensitively, consistent with users_email_lower_idx.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.getUser(ctx, `LOWER(email) = LOWER($1)`, email)
}

func (r *Repository) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	return r.getUser(ctx, `user_id = $1`, id)
}

// GetUserInOrg returns the user only if they belong to orgID; otherwise ErrNotFound.
func (r *Repository) GetUserInOrg(ctx context.Context, userID, orgID int64) (*models.User, error) {
	return r.getUser(ctx, `user_id = $1 AND org_id = $2`, userID, orgID)
}

// GetSessionState is the per-request check behind the JWT middleware.
func (r *Repository) GetSessionState(ctx context.Context, userID int64) (models.SessionState, error) {
	var s models.SessionState
	err := r.db.QueryRow(ctx, `
		SELECT u.session_version, u.email_verified_at IS NOT NULL, u.org_id, u.role,
		       u.suspended_at IS NOT NULL, u.must_change_password,
		       o.suspended_at IS NOT NULL, COALESCE(o.suspended_reason, '')
		FROM users u JOIN organizations o ON o.org_id = u.org_id
		WHERE u.user_id = $1`, userID,
	).Scan(&s.Version, &s.Verified, &s.OrgID, &s.Role, &s.Suspended, &s.MustChangePassword,
		&s.OrgSuspended, &s.OrgSuspendedReason)
	return s, mapError(err)
}

// TouchLastLogin records a successful sign-in.
func (r *Repository) TouchLastLogin(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE user_id = $1`, userID)
	return mapError(err)
}

// SetUserEmail replaces the address and marks it unverified.
func (r *Repository) SetUserEmail(ctx context.Context, userID int64, email string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET email = $2, email_verified_at = NULL, updated_at = now() WHERE user_id = $1`, userID, email)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ChangeUserEmail switches a verified account to a new address that has just
// been confirmed with a code, so it's stored as verified.
func (r *Repository) ChangeUserEmail(ctx context.Context, userID int64, email string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET email = $2, email_verified_at = now(), updated_at = now() WHERE user_id = $1`, userID, email)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) MarkEmailVerified(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET email_verified_at = now(), updated_at = now() WHERE user_id = $1`, userID)
	return mapError(err)
}

// UpdatePassword sets a password the user chose and bumps the session
// version, which signs out every existing session. It also clears
// must_change_password. It returns the new version.
func (r *Repository) UpdatePassword(ctx context.Context, userID int64, hash string) (int, error) {
	var version int
	err := r.db.QueryRow(ctx, `
		UPDATE users SET password_hash = $2, must_change_password = false,
			session_version = session_version + 1, updated_at = now()
		WHERE user_id = $1
		RETURNING session_version`, userID, hash).Scan(&version)
	return version, mapError(err)
}

// ----------------------------------------------------------------------------
// Email Code Methods
// ----------------------------------------------------------------------------

// UpsertEmailCode replaces any live code for the same user and purpose.
func (r *Repository) UpsertEmailCode(ctx context.Context, c *models.EmailCode) error {
	query := `
		INSERT INTO email_codes (user_id, purpose, email, code_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, purpose) DO UPDATE
		SET email = EXCLUDED.email, code_hash = EXCLUDED.code_hash, attempts = 0,
			expires_at = EXCLUDED.expires_at, created_at = now()
		RETURNING attempts, created_at`
	err := r.db.QueryRow(ctx, query, c.UserID, c.Purpose, c.Email, c.CodeHash, c.ExpiresAt).Scan(&c.Attempts, &c.CreatedAt)
	return mapError(err)
}

func (r *Repository) GetEmailCode(ctx context.Context, userID int64, purpose string) (*models.EmailCode, error) {
	c := models.EmailCode{UserID: userID, Purpose: purpose}
	err := r.db.QueryRow(ctx,
		`SELECT email, code_hash, attempts, expires_at, created_at FROM email_codes WHERE user_id = $1 AND purpose = $2`,
		userID, purpose,
	).Scan(&c.Email, &c.CodeHash, &c.Attempts, &c.ExpiresAt, &c.CreatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &c, nil
}

// UseEmailCodeAttempt spends one guess on a live code and returns it for
// checking. Counting the attempt before the comparison keeps concurrent
// guesses from exceeding maxAttempts. ErrNotFound means there's no usable code:
// none was issued, it expired, or its attempts ran out.
func (r *Repository) UseEmailCodeAttempt(ctx context.Context, userID int64, purpose string, maxAttempts int) (*models.EmailCode, error) {
	c := models.EmailCode{UserID: userID, Purpose: purpose}
	err := r.db.QueryRow(ctx, `
		UPDATE email_codes SET attempts = attempts + 1
		WHERE user_id = $1 AND purpose = $2 AND attempts < $3 AND expires_at > now()
		RETURNING email, code_hash, attempts, expires_at, created_at`,
		userID, purpose, maxAttempts,
	).Scan(&c.Email, &c.CodeHash, &c.Attempts, &c.ExpiresAt, &c.CreatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &c, nil
}

func (r *Repository) DeleteEmailCode(ctx context.Context, userID int64, purpose string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM email_codes WHERE user_id = $1 AND purpose = $2`, userID, purpose)
	return mapError(err)
}

// ----------------------------------------------------------------------------
// Profile Methods
// ----------------------------------------------------------------------------

func (r *Repository) CreateProfile(ctx context.Context, profile *models.Profile) error {
	query := `
		INSERT INTO profiles (org_id, user_id, assigned_user_id, slug, data)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING profile_id, created_at, updated_at`
	err := r.db.QueryRow(ctx, query, profile.OrgID, profile.UserID, profile.AssignedUserID, profile.Slug, profile.Data).Scan(
		&profile.ID, &profile.CreatedAt, &profile.UpdatedAt,
	)
	return mapError(err)
}

// UpdateProfile saves slug and data on a card the scope can edit (any card in
// the org for admins, an assigned card for members); otherwise ErrNotFound.
func (r *Repository) UpdateProfile(ctx context.Context, scope Scope, profile *models.Profile) error {
	query := `
		UPDATE profiles
		SET slug = $1, data = $2, updated_at = now()
		WHERE profile_id = $3 AND org_id = $4 AND ($5::bigint = 0 OR assigned_user_id = $5)
		RETURNING created_at, updated_at`
	err := r.db.QueryRow(ctx, query, profile.Slug, profile.Data, profile.ID, scope.OrgID, scope.memberID()).Scan(
		&profile.CreatedAt, &profile.UpdatedAt,
	)
	return mapError(err)
}

// DeleteProfile deletes a card in orgID; otherwise ErrNotFound.
func (r *Repository) DeleteProfile(ctx context.Context, profileID, orgID int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM profiles WHERE profile_id = $1 AND org_id = $2`, profileID, orgID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetProfileAssignee hands a card in orgID to userID, or back to the
// organisation when userID is nil. The caller checks userID is in the org.
func (r *Repository) SetProfileAssignee(ctx context.Context, profileID, orgID int64, userID *int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE profiles SET assigned_user_id = $3, updated_at = now() WHERE profile_id = $1 AND org_id = $2`,
		profileID, orgID, userID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// profileSelect reads a card with its assignee and the lead count the scope
// sees: every lead for admins, only their own leads for members ($2).
const profileSelect = `
	SELECT p.profile_id, p.org_id, p.user_id, p.assigned_user_id, au.username, p.slug, p.data,
	       p.created_at, p.updated_at,
	       (SELECT COUNT(*) FROM leads l
	        WHERE l.profile_id = p.profile_id AND ($2::bigint = 0 OR l.assigned_user_id = $2))
	FROM profiles p
	LEFT JOIN users au ON au.user_id = p.assigned_user_id`

func scanProfile(row pgx.Row) (*models.Profile, error) {
	var p models.Profile
	var assignee *string
	err := row.Scan(&p.ID, &p.OrgID, &p.UserID, &p.AssignedUserID, &assignee, &p.Slug, &p.Data,
		&p.CreatedAt, &p.UpdatedAt, &p.LeadCount)
	if err != nil {
		return nil, mapError(err)
	}
	if p.AssignedUserID != nil && assignee != nil {
		p.AssignedUser = &models.UserRef{ID: *p.AssignedUserID, Username: *assignee}
	}
	return &p, nil
}

func (r *Repository) GetProfileBySlug(ctx context.Context, slug string) (*models.Profile, error) {
	query := `SELECT p.profile_id, p.org_id, p.user_id, p.assigned_user_id, p.slug, p.data, p.created_at, p.updated_at,
		       o.suspended_at IS NOT NULL
		FROM profiles p JOIN organizations o ON o.org_id = p.org_id
		WHERE LOWER(p.slug) = LOWER($1)`
	var p models.Profile
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&p.ID, &p.OrgID, &p.UserID, &p.AssignedUserID, &p.Slug, &p.Data, &p.CreatedAt, &p.UpdatedAt, &p.OrgSuspended,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &p, nil
}

// GetProfile returns a card the scope can see (any card in the org for
// admins, an assigned card for members); otherwise ErrNotFound.
func (r *Repository) GetProfile(ctx context.Context, scope Scope, profileID int64) (*models.Profile, error) {
	return scanProfile(r.db.QueryRow(ctx, profileSelect+`
		WHERE p.org_id = $1 AND ($2::bigint = 0 OR p.assigned_user_id = $2) AND p.profile_id = $3`,
		scope.OrgID, scope.memberID(), profileID))
}

// ListProfiles returns the cards the scope can see, newest first, each with
// its lead count.
func (r *Repository) ListProfiles(ctx context.Context, scope Scope) ([]*models.Profile, error) {
	rows, err := r.db.Query(ctx, profileSelect+`
		WHERE p.org_id = $1 AND ($2::bigint = 0 OR p.assigned_user_id = $2)
		ORDER BY p.created_at DESC`, scope.OrgID, scope.memberID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles := []*models.Profile{}
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// ----------------------------------------------------------------------------
// Lead Methods
// ----------------------------------------------------------------------------

// leadColumns lists the columns scanLead expects, for leads l joined to their
// assignee as u.
const leadColumns = `l.lead_id, l.profile_id, l.name, l.email,
	COALESCE(l.phone_country_code, ''), COALESCE(l.phone_number, ''),
	COALESCE(l.notes, ''), l.created_at, l.assigned_user_id, u.username`

func scanLead(rows pgx.Rows) (*models.Lead, error) {
	var l models.Lead
	var assigneeID *int64
	var assignee *string
	err := rows.Scan(&l.ID, &l.ProfileID, &l.Name, &l.Email, &l.PhoneCountryCode, &l.PhoneNumber, &l.Notes, &l.CreatedAt,
		&assigneeID, &assignee)
	if assigneeID != nil && assignee != nil {
		l.AssignedUser = &models.UserRef{ID: *assigneeID, Username: *assignee}
	}
	return &l, err
}

// CreateLead stores a lead against whoever holds the card right now, so it
// stays theirs if the card is later reassigned. It returns ErrNotFound if
// the profile does not exist or its organisation is suspended.
func (r *Repository) CreateLead(ctx context.Context, lead *models.Lead) error {
	query := `
		INSERT INTO leads (profile_id, name, email, phone_country_code, phone_number, notes, assigned_user_id)
		SELECT p.profile_id, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, p.assigned_user_id
		FROM profiles p JOIN organizations o ON o.org_id = p.org_id
		WHERE p.profile_id = $1 AND o.suspended_at IS NULL
		RETURNING lead_id, created_at`
	err := r.db.QueryRow(ctx, query,
		lead.ProfileID, lead.Name, lead.Email, lead.PhoneCountryCode, lead.PhoneNumber, lead.Notes,
	).Scan(
		&lead.ID, &lead.CreatedAt,
	)
	return mapError(err)
}

// LeadFilter narrows a user's leads. Zero values mean "no filter".
type LeadFilter struct {
	ProfileID int64 // only this profile's leads
	// UserID keeps only leads that arrived while this user held the card.
	UserID int64
	// Unassigned keeps only leads that arrived while the organisation held the card.
	Unassigned bool
	Search     string    // case-insensitive substring of name, email, phone number or notes
	Since      time.Time // received at or after this time
	Limit      int
	Offset     int
}

// likePattern escapes LIKE wildcards so user input matches literally.
func likePattern(s string) string {
	s = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
	return "%" + s + "%"
}

// ListLeads returns one page of the leads the scope can see, newest first,
// plus how many match the filter in total. Admins see every lead on the
// organisation's cards; members only the leads that arrived while they held
// the card. Leads outside the scope are never included, whatever the filter says.
func (r *Repository) ListLeads(ctx context.Context, scope Scope, f LeadFilter) ([]*models.Lead, int64, error) {
	var since *time.Time
	if !f.Since.IsZero() {
		since = &f.Since
	}
	search := ""
	if f.Search != "" {
		search = likePattern(f.Search)
	}

	const where = `
		FROM leads l
		JOIN profiles p ON p.profile_id = l.profile_id
		LEFT JOIN users u ON u.user_id = l.assigned_user_id
		WHERE p.org_id = $1
		  AND ($2::bigint = 0 OR l.assigned_user_id = $2)
		  AND ($3::bigint = 0 OR l.profile_id = $3)
		  AND ($4::text = '' OR l.name ILIKE $4 OR l.email ILIKE $4 OR l.phone_number ILIKE $4 OR l.notes ILIKE $4)
		  AND ($5::timestamptz IS NULL OR l.created_at >= $5)
		  AND ($6::bigint = 0 OR l.assigned_user_id = $6)
		  AND (NOT $7::bool OR l.assigned_user_id IS NULL)`
	args := []any{scope.OrgID, scope.memberID(), f.ProfileID, search, since, f.UserID, f.Unassigned}

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx,
		`SELECT `+leadColumns+where+`
		ORDER BY l.created_at DESC, l.lead_id DESC
		LIMIT $8 OFFSET $9`,
		append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	leads := []*models.Lead{}
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			return nil, 0, err
		}
		leads = append(leads, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return leads, total, nil
}

// ----------------------------------------------------------------------------
// Storage Settings Methods
// ----------------------------------------------------------------------------

func (r *Repository) GetStorageSettings(ctx context.Context, userID int64) (*models.StorageSettings, error) {
	query := `
		SELECT user_id, provider, endpoint, region, bucket, path_style,
		       access_key_id_enc, secret_access_key_enc, access_key_hint,
		       verified_at, created_at, updated_at
		FROM user_storage WHERE user_id = $1`
	var s models.StorageSettings
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&s.UserID, &s.Provider, &s.Endpoint, &s.Region, &s.Bucket, &s.PathStyle,
		&s.AccessKeyIDEnc, &s.SecretAccessKeyEnc, &s.AccessKeyHint,
		&s.VerifiedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &s, nil
}

// GetOrgStorageSettings returns the bucket an organisation uses: the one its owner connected.
func (r *Repository) GetOrgStorageSettings(ctx context.Context, orgID int64) (*models.StorageSettings, error) {
	var ownerID int64
	err := r.db.QueryRow(ctx, `SELECT user_id FROM users WHERE org_id = $1 AND role = 'owner'`, orgID).Scan(&ownerID)
	if err != nil {
		return nil, mapError(err)
	}
	return r.GetStorageSettings(ctx, ownerID)
}

// UpsertStorageSettings creates or replaces the user's bucket configuration.
func (r *Repository) UpsertStorageSettings(ctx context.Context, s *models.StorageSettings) error {
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
	return mapError(err)
}

// DeleteStorageSettings removes the user's keys; ErrNotFound if none were saved.
func (r *Repository) DeleteStorageSettings(ctx context.Context, userID int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM user_storage WHERE user_id = $1`, userID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ----------------------------------------------------------------------------
// File Methods
// ----------------------------------------------------------------------------

// ErrQuotaExceeded means a personal upload would take the user past their storage limit.
var ErrQuotaExceeded = errors.New("storage quota exceeded")

// fileColumns lists the columns scanFile expects, for files f joined to their uploader as u.
const fileColumns = `f.file_id, f.public_id, f.org_id, f.user_id, f.area, u.username, f.former_owner,
	f.bucket, f.object_key, f.kind, f.content_type, f.size_bytes, f.original_name, f.title, f.created_at`

const fileFrom = ` FROM files f LEFT JOIN users u ON u.user_id = f.user_id`

// fileVisible is true when the scope ($1 org, $2 user, $3 admin) may see file f:
// admins see everything in the org; members see their own personal files, the
// shared area, and files granted to them.
const fileVisible = `(f.org_id = $1 AND ($3::bool
	OR f.area = 'shared'
	OR (f.area = 'personal' AND f.user_id = $2)
	OR EXISTS (SELECT 1 FROM file_grants g WHERE g.file_id = f.file_id AND g.user_id = $2)))`

// fileEditable is true when the scope may rename or delete file f: admins
// anything in the org, members only their own personal files.
const fileEditable = `(f.org_id = $1 AND ($3::bool OR (f.area = 'personal' AND f.user_id = $2)))`

func (s Scope) fileArgs() []any { return []any{s.OrgID, s.UserID, s.Admin} }

func scanFile(row pgx.Row) (*models.File, error) {
	var f models.File
	var owner *string
	err := row.Scan(&f.ID, &f.PublicID, &f.OrgID, &f.UserID, &f.Area, &owner, &f.FormerOwner,
		&f.Bucket, &f.ObjectKey, &f.Kind, &f.ContentType, &f.SizeBytes, &f.OriginalName, &f.Title, &f.CreatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	if owner != nil {
		f.Owner = &models.UserRef{ID: f.UserID, Username: *owner}
	}
	return &f, nil
}

func collectFiles(rows pgx.Rows, err error) ([]*models.File, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := []*models.File{}
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// CreateFile records an uploaded file. A personal file counts against its
// owner's quota: the owner's row is locked while the total is checked, so
// concurrent uploads can't overshoot, and ErrQuotaExceeded is returned if it
// wouldn't fit.
func (r *Repository) CreateFile(ctx context.Context, f *models.File) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if f.Area == models.AreaPersonal {
			var quota *int64
			if err := tx.QueryRow(ctx, `SELECT storage_quota_bytes FROM users WHERE user_id = $1 FOR UPDATE`, f.UserID).Scan(&quota); err != nil {
				return mapError(err)
			}
			if quota != nil {
				used, err := usedBytes(ctx, tx, f.UserID)
				if err != nil {
					return err
				}
				if used+f.SizeBytes > *quota {
					return ErrQuotaExceeded
				}
			}
		}
		query := `
			INSERT INTO files (public_id, org_id, user_id, area, bucket, object_key, kind, content_type, size_bytes, original_name, title)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING file_id, created_at`
		err := tx.QueryRow(ctx, query, f.PublicID, f.OrgID, f.UserID, f.Area, f.Bucket, f.ObjectKey, f.Kind, f.ContentType,
			f.SizeBytes, f.OriginalName, f.Title).Scan(&f.ID, &f.CreatedAt)
		return mapError(err)
	})
}

func usedBytes(ctx context.Context, q querier, userID int64) (int64, error) {
	var used int64
	err := q.QueryRow(ctx,
		`SELECT COALESCE(SUM(size_bytes), 0) FROM files WHERE user_id = $1 AND area = 'personal'`, userID).Scan(&used)
	return used, err
}

// UsedBytes is the total size of a user's personal files: what their quota limits.
func (r *Repository) UsedBytes(ctx context.Context, userID int64) (int64, error) {
	return usedBytes(ctx, r.db, userID)
}

// FileFilter narrows a file listing. Zero values mean "no filter".
type FileFilter struct {
	Kind string // "image" or "pdf"
	Area string // "personal", "org" or "shared"
	// UserID keeps only files uploaded by (for personal files: belonging to) this user.
	UserID int64
	// GrantedTo keeps only files explicitly granted to this user.
	GrantedTo int64
	Limit     int
	Offset    int
}

// ListFiles returns one page of the files the scope can see, newest first,
// and the total matching.
func (r *Repository) ListFiles(ctx context.Context, scope Scope, ff FileFilter) ([]*models.File, int64, error) {
	where := fileFrom + ` WHERE ` + fileVisible + `
		AND ($4::text = '' OR f.kind = $4)
		AND ($5::text = '' OR f.area = $5)
		AND ($6::bigint = 0 OR f.user_id = $6)
		AND ($7::bigint = 0 OR EXISTS (SELECT 1 FROM file_grants g2 WHERE g2.file_id = f.file_id AND g2.user_id = $7))`
	args := append(scope.fileArgs(), ff.Kind, ff.Area, ff.UserID, ff.GrantedTo)

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	files, err := collectFiles(r.db.Query(ctx, `SELECT `+fileColumns+where+`
		ORDER BY f.created_at DESC, f.file_id DESC LIMIT $8 OFFSET $9`, append(args, ff.Limit, ff.Offset)...))
	if err != nil {
		return nil, 0, err
	}
	return files, total, nil
}

// CountFilesForOrg is shown in settings so the owner knows what a bucket change affects.
func (r *Repository) CountFilesForOrg(ctx context.Context, orgID int64) (int64, error) {
	var n int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM files WHERE org_id = $1`, orgID).Scan(&n)
	return n, err
}

// GetFile returns a file the scope can see; otherwise ErrNotFound.
func (r *Repository) GetFile(ctx context.Context, scope Scope, publicID string) (*models.File, error) {
	return scanFile(r.db.QueryRow(ctx,
		`SELECT `+fileColumns+fileFrom+` WHERE `+fileVisible+` AND f.public_id = $4`,
		append(scope.fileArgs(), publicID)...))
}

// GetEditableFile returns a file the scope may rename or delete; otherwise ErrNotFound.
func (r *Repository) GetEditableFile(ctx context.Context, scope Scope, publicID string) (*models.File, error) {
	return scanFile(r.db.QueryRow(ctx,
		`SELECT `+fileColumns+fileFrom+` WHERE `+fileEditable+` AND f.public_id = $4`,
		append(scope.fileArgs(), publicID)...))
}

func (r *Repository) GetFileByPublicID(ctx context.Context, publicID string) (*models.File, error) {
	return scanFile(r.db.QueryRow(ctx, `SELECT `+fileColumns+fileFrom+` WHERE f.public_id = $1`, publicID))
}

// GetOrgFilesByPublicIDs returns those of the given files that belong to orgID, in no particular order.
func (r *Repository) GetOrgFilesByPublicIDs(ctx context.Context, orgID int64, publicIDs []string) ([]*models.File, error) {
	if len(publicIDs) == 0 {
		return []*models.File{}, nil
	}
	return collectFiles(r.db.Query(ctx,
		`SELECT `+fileColumns+fileFrom+` WHERE f.org_id = $1 AND f.public_id = ANY($2)`, orgID, publicIDs))
}

// CountVisibleFiles reports how many of the given files the scope can see.
func (r *Repository) CountVisibleFiles(ctx context.Context, scope Scope, publicIDs []string) (int, error) {
	if len(publicIDs) == 0 {
		return 0, nil
	}
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM files f WHERE `+fileVisible+` AND f.public_id = ANY($4)`,
		append(scope.fileArgs(), publicIDs)...).Scan(&n)
	return n, err
}

// UpdateFileTitle renames a file the scope may edit; otherwise ErrNotFound.
func (r *Repository) UpdateFileTitle(ctx context.Context, scope Scope, publicID, title string) (*models.File, error) {
	var id int64
	err := r.db.QueryRow(ctx, `UPDATE files f SET title = $5 WHERE `+fileEditable+` AND f.public_id = $4 RETURNING f.file_id`,
		append(scope.fileArgs(), publicID, title)...).Scan(&id)
	if err != nil {
		return nil, mapError(err)
	}
	return scanFile(r.db.QueryRow(ctx, `SELECT `+fileColumns+fileFrom+` WHERE f.file_id = $1`, id))
}

// DeleteFile removes the record of a file in orgID; otherwise ErrNotFound.
// The caller checks the user may delete it (GetEditableFile).
func (r *Repository) DeleteFile(ctx context.Context, fileID, orgID int64) error {
	// The organisation's logo or signature banner may point at the file;
	// forget it there too so no card or signature shows a broken image.
	var n int
	err := r.db.QueryRow(ctx, `
		WITH d AS (DELETE FROM files WHERE file_id = $1 AND org_id = $2 RETURNING public_id),
		o AS (
			UPDATE organizations SET
				logo_file = CASE WHEN logo_file IN (SELECT public_id FROM d) THEN NULL ELSE logo_file END,
				signature = CASE WHEN signature->>'banner_file' IN (SELECT public_id FROM d)
					THEN signature - 'banner_file' ELSE signature END
			WHERE org_id = $2 AND (logo_file IN (SELECT public_id FROM d)
				OR signature->>'banner_file' IN (SELECT public_id FROM d))
		)
		SELECT COUNT(*) FROM d`, fileID, orgID).Scan(&n)
	if err != nil {
		return mapError(err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListFileGrants returns the users a file has been granted to, by username.
func (r *Repository) ListFileGrants(ctx context.Context, fileID int64) ([]models.UserRef, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.user_id, u.username FROM file_grants g JOIN users u ON u.user_id = g.user_id
		WHERE g.file_id = $1 ORDER BY LOWER(u.username)`, fileID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.UserRef, error) {
		var u models.UserRef
		err := row.Scan(&u.ID, &u.Username)
		return u, err
	})
}

// ReplaceFileGrants sets exactly which users a file is granted to. Ids of
// users outside orgID are ignored.
func (r *Repository) ReplaceFileGrants(ctx context.Context, fileID, orgID, grantedBy int64, userIDs []int64) error {
	if userIDs == nil {
		userIDs = []int64{} // a nil slice is NULL, which would match nothing below
	}
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM file_grants WHERE file_id = $1 AND NOT (user_id = ANY($2))`, fileID, userIDs); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO file_grants (file_id, user_id, granted_by)
			SELECT $1, u.user_id, $3 FROM users u WHERE u.org_id = $4 AND u.user_id = ANY($2)
			ON CONFLICT DO NOTHING`, fileID, userIDs, grantedBy, orgID)
		return mapError(err)
	})
}
