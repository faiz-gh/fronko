package repository

import (
	"context"
	"errors"
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
			return ErrConflict
		case "23503": // foreign_key_violation
			return ErrNotFound
		}
	}
	return err
}

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ----------------------------------------------------------------------------
// User Methods
// ----------------------------------------------------------------------------

func (r *Repository) CreateUser(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (username, password_hash)
		VALUES ($1, $2)
		RETURNING user_id, created_at, updated_at`
	err := r.db.QueryRow(ctx, query, user.Username, user.PasswordHash).Scan(
		&user.ID, &user.CreatedAt, &user.UpdatedAt,
	)
	return mapError(err)
}

// GetUserByUsername matches case-insensitively, consistent with users_username_lower_idx.
func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `SELECT user_id, username, password_hash, created_at, updated_at FROM users WHERE LOWER(username) = LOWER($1)`
	var user models.User
	err := r.db.QueryRow(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &user, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	query := `SELECT user_id, username, password_hash, created_at, updated_at FROM users WHERE user_id = $1`
	var user models.User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &user, nil
}

// ----------------------------------------------------------------------------
// Profile Methods
// ----------------------------------------------------------------------------

func (r *Repository) CreateProfile(ctx context.Context, profile *models.Profile) error {
	query := `
		INSERT INTO profiles (user_id, slug, data)
		VALUES ($1, $2, $3)
		RETURNING profile_id, created_at, updated_at`
	err := r.db.QueryRow(ctx, query, profile.UserID, profile.Slug, profile.Data).Scan(
		&profile.ID, &profile.CreatedAt, &profile.UpdatedAt,
	)
	return mapError(err)
}

// UpdateProfile only updates a profile owned by profile.UserID; otherwise ErrNotFound.
func (r *Repository) UpdateProfile(ctx context.Context, profile *models.Profile) error {
	query := `
		UPDATE profiles
		SET slug = $1, data = $2, updated_at = now()
		WHERE profile_id = $3 AND user_id = $4
		RETURNING created_at, updated_at`
	err := r.db.QueryRow(ctx, query, profile.Slug, profile.Data, profile.ID, profile.UserID).Scan(
		&profile.CreatedAt, &profile.UpdatedAt,
	)
	return mapError(err)
}

// DeleteProfile deletes a profile owned by userID; otherwise ErrNotFound.
func (r *Repository) DeleteProfile(ctx context.Context, profileID, userID int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM profiles WHERE profile_id = $1 AND user_id = $2`, profileID, userID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetProfileBySlug(ctx context.Context, slug string) (*models.Profile, error) {
	query := `SELECT profile_id, user_id, slug, data, created_at, updated_at FROM profiles WHERE LOWER(slug) = LOWER($1)`
	var p models.Profile
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&p.ID, &p.UserID, &p.Slug, &p.Data, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &p, nil
}

// GetProfileForUser returns the profile only if it belongs to userID; otherwise ErrNotFound.
func (r *Repository) GetProfileForUser(ctx context.Context, profileID, userID int64) (*models.Profile, error) {
	query := `SELECT profile_id, user_id, slug, data, created_at, updated_at FROM profiles WHERE profile_id = $1 AND user_id = $2`
	var p models.Profile
	err := r.db.QueryRow(ctx, query, profileID, userID).Scan(
		&p.ID, &p.UserID, &p.Slug, &p.Data, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &p, nil
}

// GetProfilesByUserID returns the user's profiles, newest first, each with its lead count.
func (r *Repository) GetProfilesByUserID(ctx context.Context, userID int64) ([]*models.Profile, error) {
	query := `
		SELECT p.profile_id, p.user_id, p.slug, p.data, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM leads l WHERE l.profile_id = p.profile_id)
		FROM profiles p
		WHERE p.user_id = $1
		ORDER BY p.created_at DESC`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles := []*models.Profile{}
	for rows.Next() {
		var p models.Profile
		if err := rows.Scan(&p.ID, &p.UserID, &p.Slug, &p.Data, &p.CreatedAt, &p.UpdatedAt, &p.LeadCount); err != nil {
			return nil, err
		}
		profiles = append(profiles, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return profiles, nil
}

// ----------------------------------------------------------------------------
// Lead Methods
// ----------------------------------------------------------------------------

// leadColumns lists the columns scanLead expects, optionally table-qualified.
func leadColumns(p string) string {
	return p + "lead_id, " + p + "profile_id, " + p + "name, " + p + "email, " +
		"COALESCE(" + p + "phone_country_code, ''), COALESCE(" + p + "phone_number, ''), " +
		"COALESCE(" + p + "notes, ''), " + p + "created_at"
}

func scanLead(rows pgx.Rows) (*models.Lead, error) {
	var l models.Lead
	err := rows.Scan(&l.ID, &l.ProfileID, &l.Name, &l.Email, &l.PhoneCountryCode, &l.PhoneNumber, &l.Notes, &l.CreatedAt)
	return &l, err
}

// CreateLead returns ErrNotFound if the profile does not exist.
func (r *Repository) CreateLead(ctx context.Context, lead *models.Lead) error {
	query := `
		INSERT INTO leads (profile_id, name, email, phone_country_code, phone_number, notes)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6)
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
	ProfileID int64     // only this profile's leads
	Search    string    // case-insensitive substring of name, email, phone number or notes
	Since     time.Time // received at or after this time
	Limit     int
	Offset    int
}

// likePattern escapes LIKE wildcards so user input matches literally.
func likePattern(s string) string {
	s = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
	return "%" + s + "%"
}

// ListLeadsForUser returns one page of the leads across all of userID's
// profiles, newest first, plus how many leads match the filter in total.
// Leads on other users' profiles are never included, even when ProfileID names one.
func (r *Repository) ListLeadsForUser(ctx context.Context, userID int64, f LeadFilter) ([]*models.Lead, int64, error) {
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
		WHERE p.user_id = $1
		  AND ($2::bigint = 0 OR l.profile_id = $2)
		  AND ($3::text = '' OR l.name ILIKE $3 OR l.email ILIKE $3 OR l.phone_number ILIKE $3 OR l.notes ILIKE $3)
		  AND ($4::timestamptz IS NULL OR l.created_at >= $4)`
	args := []any{userID, f.ProfileID, search, since}

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx,
		`SELECT `+leadColumns("l.")+where+`
		ORDER BY l.created_at DESC, l.lead_id DESC
		LIMIT $5 OFFSET $6`,
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

func (r *Repository) GetLeadsByProfileID(ctx context.Context, profileID int64) ([]*models.Lead, error) {
	query := `SELECT ` + leadColumns("") + ` FROM leads WHERE profile_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	leads := []*models.Lead{}
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			return nil, err
		}
		leads = append(leads, l)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return leads, nil
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

const fileColumns = `file_id, public_id, user_id, bucket, object_key, kind, content_type,
	size_bytes, original_name, title, created_at`

func scanFile(row pgx.Row) (*models.File, error) {
	var f models.File
	err := row.Scan(&f.ID, &f.PublicID, &f.UserID, &f.Bucket, &f.ObjectKey, &f.Kind, &f.ContentType,
		&f.SizeBytes, &f.OriginalName, &f.Title, &f.CreatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &f, nil
}

func (r *Repository) CreateFile(ctx context.Context, f *models.File) error {
	query := `
		INSERT INTO files (public_id, user_id, bucket, object_key, kind, content_type, size_bytes, original_name, title)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING file_id, created_at`
	err := r.db.QueryRow(ctx, query, f.PublicID, f.UserID, f.Bucket, f.ObjectKey, f.Kind, f.ContentType,
		f.SizeBytes, f.OriginalName, f.Title).Scan(&f.ID, &f.CreatedAt)
	return mapError(err)
}

// ListFilesForUser returns one page of the user's files, newest first, and the
// total matching. An empty kind means all kinds.
func (r *Repository) ListFilesForUser(ctx context.Context, userID int64, kind string, limit, offset int) ([]*models.File, int64, error) {
	const where = ` FROM files WHERE user_id = $1 AND ($2::text = '' OR kind = $2)`

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)`+where, userID, kind).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `SELECT `+fileColumns+where+`
		ORDER BY created_at DESC, file_id DESC LIMIT $3 OFFSET $4`, userID, kind, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	files := []*models.File{}
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, 0, err
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return files, total, nil
}

// CountFilesForUser is shown in settings so users know what a bucket change affects.
func (r *Repository) CountFilesForUser(ctx context.Context, userID int64) (int64, error) {
	var n int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM files WHERE user_id = $1`, userID).Scan(&n)
	return n, err
}

// GetFileForUser returns the file only if userID owns it; otherwise ErrNotFound.
func (r *Repository) GetFileForUser(ctx context.Context, publicID string, userID int64) (*models.File, error) {
	return scanFile(r.db.QueryRow(ctx,
		`SELECT `+fileColumns+` FROM files WHERE public_id = $1 AND user_id = $2`, publicID, userID))
}

func (r *Repository) GetFileByPublicID(ctx context.Context, publicID string) (*models.File, error) {
	return scanFile(r.db.QueryRow(ctx, `SELECT `+fileColumns+` FROM files WHERE public_id = $1`, publicID))
}

// GetFilesByPublicIDs returns those of the given files that userID owns, in no particular order.
func (r *Repository) GetFilesByPublicIDs(ctx context.Context, userID int64, publicIDs []string) ([]*models.File, error) {
	files := []*models.File{}
	if len(publicIDs) == 0 {
		return files, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT `+fileColumns+` FROM files WHERE user_id = $1 AND public_id = ANY($2)`, userID, publicIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// UpdateFileTitle renames a file userID owns; otherwise ErrNotFound.
func (r *Repository) UpdateFileTitle(ctx context.Context, publicID string, userID int64, title string) (*models.File, error) {
	return scanFile(r.db.QueryRow(ctx,
		`UPDATE files SET title = $3 WHERE public_id = $1 AND user_id = $2 RETURNING `+fileColumns,
		publicID, userID, title))
}

// DeleteFile removes the row for a file userID owns; otherwise ErrNotFound.
func (r *Repository) DeleteFile(ctx context.Context, publicID string, userID int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM files WHERE public_id = $1 AND user_id = $2`, publicID, userID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
