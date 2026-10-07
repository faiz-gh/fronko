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
// Team leads also see the cards and leads of the people in the teams they lead.
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

// visibleTo is a condition on a user id column: true when the cards and leads
// that user holds are visible to the user in parameter $n (memberID). That's
// everyone for admins ($n = 0); otherwise themselves and the members of the
// teams they lead.
func visibleTo(col string, n int) string {
	return fmt.Sprintf(`($%[1]d::bigint = 0 OR %[2]s = $%[1]d OR %[2]s IN (
		SELECT tm.user_id FROM team_members tm
		JOIN team_members ld ON ld.team_id = tm.team_id AND ld.role = 'lead'
		WHERE ld.user_id = $%[1]d))`, n, col)
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ----------------------------------------------------------------------------
// User Methods
// ----------------------------------------------------------------------------

// CreateUser inserts a user into an existing organisation (user.OrgID).
// Registering a new organisation goes through CreateOrgWithOwner instead.
// Any teams given (outside the org are ignored) are joined in the same transaction.
func (r *Repository) CreateUser(ctx context.Context, user *models.User, teams ...models.TeamMembership) error {
	if len(teams) == 0 {
		return createUser(ctx, r.db, user)
	}
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := createUser(ctx, tx, user); err != nil {
			return err
		}
		return replaceUserTeams(ctx, tx, user.OrgID, user.ID, teams)
	})
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
// its owner, in one transaction. It fills in user.OrgID and user.Role. The
// organisation's link handle is made from its name (or the username), with a
// number added when that one is taken.
func (r *Repository) CreateOrgWithOwner(ctx context.Context, orgName string, user *models.User) error {
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
				return mapError(err)
			}
			user.Role = models.RoleOwner
			return createUser(ctx, tx, user)
		})
		if !isConstraint(err, "organizations_handle_lower_idx") {
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
func freeHandle(ctx context.Context, q querier, base string) (string, error) {
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

// isConstraint reports whether err is a unique violation of the named index.
func isConstraint(err error, name string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == name
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
	var teams []byte
	err := r.db.QueryRow(ctx, `
		SELECT u.session_version, u.email_verified_at IS NOT NULL, u.org_id, u.role,
		       u.suspended_at IS NOT NULL, u.must_change_password,
		       o.suspended_at IS NOT NULL, COALESCE(o.suspended_reason, ''), `+userTeamsJSON("u.user_id")+`
		FROM users u JOIN organizations o ON o.org_id = u.org_id
		WHERE u.user_id = $1`, userID,
	).Scan(&s.Version, &s.Verified, &s.OrgID, &s.Role, &s.Suspended, &s.MustChangePassword,
		&s.OrgSuspended, &s.OrgSuspendedReason, &teams)
	if err != nil {
		return s, mapError(err)
	}
	s.Teams, err = decodeTeamRefs(teams)
	return s, err
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

// CreateProfile saves a new card and records which library files it uses.
func (r *Repository) CreateProfile(ctx context.Context, profile *models.Profile) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		query := `
			INSERT INTO profiles (org_id, user_id, assigned_user_id, slug, data)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING profile_id, created_at, updated_at`
		err := tx.QueryRow(ctx, query, profile.OrgID, profile.UserID, profile.AssignedUserID, profile.Slug, profile.Data).Scan(
			&profile.ID, &profile.CreatedAt, &profile.UpdatedAt,
		)
		if err != nil {
			return mapError(err)
		}
		return syncFileRefs(ctx, tx, profile.ID, profile.OrgID, profile.Data)
	})
}

// UpdateProfile saves slug and data on a card the scope can edit (any card in
// the org for admins, an assigned card for members, and their teammates'
// cards for team leads); otherwise ErrNotFound. It also records which library
// files the card now uses.
func (r *Repository) UpdateProfile(ctx context.Context, scope Scope, profile *models.Profile) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		query := `
			UPDATE profiles
			SET slug = $1, data = $2, updated_at = now()
			WHERE profile_id = $3 AND org_id = $4 AND ` + visibleTo("assigned_user_id", 5) + `
			RETURNING created_at, updated_at`
		err := tx.QueryRow(ctx, query, profile.Slug, profile.Data, profile.ID, scope.OrgID, scope.memberID()).Scan(
			&profile.CreatedAt, &profile.UpdatedAt,
		)
		if err != nil {
			return mapError(err)
		}
		return syncFileRefs(ctx, tx, profile.ID, scope.OrgID, profile.Data)
	})
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
// sees: every lead for admins, otherwise their own leads and, for team leads,
// their teammates' ($2).
var profileSelect = `
	SELECT p.profile_id, p.org_id, p.user_id, p.assigned_user_id, au.username, p.slug, p.data,
	       p.created_at, p.updated_at,
	       (SELECT COUNT(*) FROM leads l
	        WHERE l.profile_id = p.profile_id AND ` + visibleTo("l.assigned_user_id", 2) + `)
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

// GetProfileByPath finds a card by its link, /p/{handle}/{slug}: the
// organisation's handle and the card's slug, both case-insensitive.
func (r *Repository) GetProfileByPath(ctx context.Context, handle, slug string) (*models.Profile, error) {
	query := `SELECT p.profile_id, p.org_id, p.user_id, p.assigned_user_id, p.slug, p.data, p.created_at, p.updated_at,
		       o.handle, o.suspended_at IS NOT NULL
		FROM profiles p JOIN organizations o ON o.org_id = p.org_id
		WHERE LOWER(o.handle) = LOWER($1) AND LOWER(p.slug) = LOWER($2)`
	var p models.Profile
	err := r.db.QueryRow(ctx, query, handle, slug).Scan(
		&p.ID, &p.OrgID, &p.UserID, &p.AssignedUserID, &p.Slug, &p.Data, &p.CreatedAt, &p.UpdatedAt, &p.OrgHandle, &p.OrgSuspended,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &p, nil
}

// GetProfile returns a card the scope can see (any card in the org for
// admins, an assigned card for members, teammates' cards for team leads);
// otherwise ErrNotFound.
func (r *Repository) GetProfile(ctx context.Context, scope Scope, profileID int64) (*models.Profile, error) {
	return scanProfile(r.db.QueryRow(ctx, profileSelect+`
		WHERE p.org_id = $1 AND `+visibleTo("p.assigned_user_id", 2)+` AND p.profile_id = $3`,
		scope.OrgID, scope.memberID(), profileID))
}

// ListProfiles returns the cards the scope can see, newest first, each with
// its lead count.
func (r *Repository) ListProfiles(ctx context.Context, scope Scope) ([]*models.Profile, error) {
	rows, err := r.db.Query(ctx, profileSelect+`
		WHERE p.org_id = $1 AND `+visibleTo("p.assigned_user_id", 2)+`
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
	COALESCE(l.notes, ''), COALESCE(l.source, ''), l.created_at, l.assigned_user_id, u.username`

func scanLead(rows pgx.Rows) (*models.Lead, error) {
	var l models.Lead
	var assigneeID *int64
	var assignee *string
	err := rows.Scan(&l.ID, &l.ProfileID, &l.Name, &l.Email, &l.PhoneCountryCode, &l.PhoneNumber, &l.Notes, &l.Source, &l.CreatedAt,
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
		INSERT INTO leads (profile_id, name, email, phone_country_code, phone_number, notes, source, assigned_user_id)
		SELECT p.profile_id, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, NULLIF($7, ''), p.assigned_user_id
		FROM profiles p JOIN organizations o ON o.org_id = p.org_id
		WHERE p.profile_id = $1 AND o.suspended_at IS NULL
		RETURNING lead_id, created_at`
	err := r.db.QueryRow(ctx, query,
		lead.ProfileID, lead.Name, lead.Email, lead.PhoneCountryCode, lead.PhoneNumber, lead.Notes, lead.Source,
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
	// TeamID keeps only leads that arrived while someone in this team held the card.
	TeamID int64
	Search string    // case-insensitive substring of name, email, phone number or notes
	Since  time.Time // received at or after this time
	Limit  int
	Offset int
}

// likePattern escapes LIKE wildcards so user input matches literally.
func likePattern(s string) string {
	s = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
	return "%" + s + "%"
}

// ListLeads returns one page of the leads the scope can see, newest first,
// plus how many match the filter in total. Admins see every lead on the
// organisation's cards; members only the leads that arrived while they held
// the card, and team leads also their teammates' leads. Leads outside the scope are never included, whatever the filter says.
func (r *Repository) ListLeads(ctx context.Context, scope Scope, f LeadFilter) ([]*models.Lead, int64, error) {
	var since *time.Time
	if !f.Since.IsZero() {
		since = &f.Since
	}
	search := ""
	if f.Search != "" {
		search = likePattern(f.Search)
	}

	where := `
		FROM leads l
		JOIN profiles p ON p.profile_id = l.profile_id
		LEFT JOIN users u ON u.user_id = l.assigned_user_id
		WHERE p.org_id = $1
		  AND ` + visibleTo("l.assigned_user_id", 2) + `
		  AND ($3::bigint = 0 OR l.profile_id = $3)
		  AND ($4::text = '' OR l.name ILIKE $4 OR l.email ILIKE $4 OR l.phone_number ILIKE $4 OR l.notes ILIKE $4)
		  AND ($5::timestamptz IS NULL OR l.created_at >= $5)
		  AND ($6::bigint = 0 OR l.assigned_user_id = $6)
		  AND (NOT $7::bool OR l.assigned_user_id IS NULL)
		  AND ($8::bigint = 0 OR l.assigned_user_id IN (SELECT tm.user_id FROM team_members tm WHERE tm.team_id = $8))`
	args := []any{scope.OrgID, scope.memberID(), f.ProfileID, search, since, f.UserID, f.Unassigned, f.TeamID}

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx,
		`SELECT `+leadColumns+where+`
		ORDER BY l.created_at DESC, l.lead_id DESC
		LIMIT $9 OFFSET $10`,
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
