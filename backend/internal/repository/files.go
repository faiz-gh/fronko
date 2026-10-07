package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

// ----------------------------------------------------------------------------
// File Methods
// ----------------------------------------------------------------------------

// ErrQuotaExceeded means a personal upload would take the user past their storage limit.
var ErrQuotaExceeded = errors.New("storage quota exceeded")

// fileColumns lists the columns scanFile expects, for files f joined to their
// uploader as u and their team as t.
const fileColumns = `f.file_id, f.public_id, f.org_id, f.user_id, f.area, u.username, f.former_owner,
	f.team_id, t.name, t.color, f.bucket, f.object_key, f.thumb_key, f.kind, f.purpose, f.content_type,
	f.size_bytes, f.original_name, f.title, f.width, f.height, f.pages, f.created_at, f.updated_at,
	(SELECT COUNT(DISTINCT fr.profile_id) FROM file_refs fr WHERE fr.file_id = f.file_id)
	+ (SELECT CASE WHEN o.logo_file = f.public_id THEN 1 ELSE 0 END
	        + CASE WHEN o.signature->>'banner_file' = f.public_id THEN 1 ELSE 0 END
	   FROM organizations o WHERE o.org_id = f.org_id)`

const fileFrom = ` FROM files f LEFT JOIN users u ON u.user_id = f.user_id LEFT JOIN teams t ON t.team_id = f.team_id`

// fileVisible is true when the scope ($1 org, $2 user, $3 admin) may see file f:
// admins see everything in the org; everyone else sees their own personal
// files, the shared area, the files of teams they're in, and files granted
// to them or to one of their teams.
const fileVisible = `(f.org_id = $1 AND ($3::bool
	OR f.area = 'shared'
	OR (f.area = 'personal' AND f.user_id = $2)
	OR (f.area = 'team' AND f.team_id IN (SELECT m.team_id FROM team_members m WHERE m.user_id = $2))
	OR EXISTS (SELECT 1 FROM file_grants g WHERE g.file_id = f.file_id AND g.user_id = $2)
	OR EXISTS (SELECT 1 FROM file_team_grants tg JOIN team_members m ON m.team_id = tg.team_id
	           WHERE tg.file_id = f.file_id AND m.user_id = $2)))`

// fileEditable is true when the scope may change or delete file f: admins
// anything in the org, everyone else their own personal files, and team leads
// their team's files.
const fileEditable = `(f.org_id = $1 AND ($3::bool
	OR (f.area = 'personal' AND f.user_id = $2)
	OR (f.area = 'team' AND f.team_id IN (SELECT m.team_id FROM team_members m WHERE m.user_id = $2 AND m.role = 'lead'))))`

func (s Scope) fileArgs() []any { return []any{s.OrgID, s.UserID, s.Admin} }

func scanFile(row pgx.Row) (*models.File, error) {
	var f models.File
	var owner, teamName, teamColor *string
	err := row.Scan(&f.ID, &f.PublicID, &f.OrgID, &f.UserID, &f.Area, &owner, &f.FormerOwner,
		&f.TeamID, &teamName, &teamColor, &f.Bucket, &f.ObjectKey, &f.ThumbKey, &f.Kind, &f.Purpose, &f.ContentType,
		&f.SizeBytes, &f.OriginalName, &f.Title, &f.Width, &f.Height, &f.Pages, &f.CreatedAt, &f.UpdatedAt,
		&f.UseCount)
	if err != nil {
		return nil, mapError(err)
	}
	if owner != nil {
		f.Owner = &models.UserRef{ID: f.UserID, Username: *owner}
	}
	if f.TeamID != nil && teamName != nil {
		f.Team = &models.TeamRef{ID: *f.TeamID, Name: *teamName, Color: deref(teamColor)}
	}
	f.HasThumb = f.ThumbKey != nil
	return &f, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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
			INSERT INTO files (public_id, org_id, user_id, area, team_id, bucket, object_key, thumb_key, kind, purpose,
			                   content_type, size_bytes, original_name, title, width, height, pages)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE(NULLIF($10, ''), 'other'), $11, $12, $13, $14, $15, $16, $17)
			RETURNING file_id, purpose, created_at, updated_at`
		err := tx.QueryRow(ctx, query, f.PublicID, f.OrgID, f.UserID, f.Area, f.TeamID, f.Bucket, f.ObjectKey, f.ThumbKey,
			f.Kind, f.Purpose, f.ContentType, f.SizeBytes, f.OriginalName, f.Title, f.Width, f.Height, f.Pages,
		).Scan(&f.ID, &f.Purpose, &f.CreatedAt, &f.UpdatedAt)
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

// File sort orders.
const (
	FileSortNewest = "newest"
	FileSortOldest = "oldest"
	FileSortName   = "name"
	FileSortSize   = "size"
)

var fileOrderBy = map[string]string{
	"":             `f.created_at DESC, f.file_id DESC`,
	FileSortNewest: `f.created_at DESC, f.file_id DESC`,
	FileSortOldest: `f.created_at ASC, f.file_id ASC`,
	FileSortName:   `LOWER(COALESCE(NULLIF(f.title, ''), f.original_name)) ASC, f.file_id DESC`,
	FileSortSize:   `f.size_bytes DESC, f.file_id DESC`,
}

// ValidFileSort reports whether s is a sort order ListFiles understands.
func ValidFileSort(s string) bool {
	_, ok := fileOrderBy[s]
	return ok
}

// FileFilter narrows a file listing. Zero values mean "no filter".
type FileFilter struct {
	Kind string // "image" or "pdf"
	Area string // "personal", "org", "shared" or "team"
	// TeamID keeps only the files in this team's area.
	TeamID int64
	// UserID keeps only files uploaded by (for personal files: belonging to) this user.
	UserID int64
	// GrantedTo keeps only files explicitly granted to this user, directly or through a team.
	GrantedTo int64
	Purposes  []string
	Search    string // case-insensitive substring of the title or file name
	Sort      string // one of the FileSort constants; newest by default
	Limit     int
	Offset    int
}

// fileWhere builds the FROM/WHERE for a listing; the purpose filter is left out
// when withPurpose is false, so per-purpose counts cover every purpose.
func fileWhere(scope Scope, ff FileFilter, withPurpose bool) (string, []any) {
	search := ""
	if ff.Search != "" {
		search = likePattern(ff.Search)
	}
	purposes := ff.Purposes
	if !withPurpose || purposes == nil {
		purposes = []string{}
	}
	where := fileFrom + ` WHERE ` + fileVisible + `
		AND ($4::text = '' OR f.kind = $4)
		AND ($5::text = '' OR f.area = $5)
		AND ($6::bigint = 0 OR f.user_id = $6)
		AND ($7::bigint = 0
			OR EXISTS (SELECT 1 FROM file_grants g2 WHERE g2.file_id = f.file_id AND g2.user_id = $7)
			OR EXISTS (SELECT 1 FROM file_team_grants tg2 JOIN team_members m2 ON m2.team_id = tg2.team_id
			           WHERE tg2.file_id = f.file_id AND m2.user_id = $7))
		AND ($8::bigint = 0 OR f.team_id = $8)
		AND (cardinality($9::text[]) = 0 OR f.purpose = ANY($9))
		AND ($10::text = '' OR f.title ILIKE $10 OR f.original_name ILIKE $10)`
	args := append(scope.fileArgs(), ff.Kind, ff.Area, ff.UserID, ff.GrantedTo, ff.TeamID, purposes, search)
	return where, args
}

// ListFiles returns one page of the files the scope can see and the total matching.
func (r *Repository) ListFiles(ctx context.Context, scope Scope, ff FileFilter) ([]*models.File, int64, error) {
	where, args := fileWhere(scope, ff, true)
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order, ok := fileOrderBy[ff.Sort]
	if !ok {
		order = fileOrderBy[""]
	}
	files, err := collectFiles(r.db.Query(ctx, `SELECT `+fileColumns+where+`
		ORDER BY `+order+` LIMIT $11 OFFSET $12`, append(args, ff.Limit, ff.Offset)...))
	if err != nil {
		return nil, 0, err
	}
	return files, total, nil
}

// CountFilesByPurpose counts the files matching the filter (ignoring its
// purposes) for each purpose. Purposes with no files are left out.
func (r *Repository) CountFilesByPurpose(ctx context.Context, scope Scope, ff FileFilter) (map[string]int64, error) {
	where, args := fileWhere(scope, ff, false)
	rows, err := r.db.Query(ctx, `SELECT f.purpose, COUNT(*)`+where+` GROUP BY f.purpose`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int64{}
	for rows.Next() {
		var p string
		var n int64
		if err := rows.Scan(&p, &n); err != nil {
			return nil, err
		}
		counts[p] = n
	}
	return counts, rows.Err()
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

// GetEditableFile returns a file the scope may change or delete; otherwise ErrNotFound.
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

// FilePatch is a change to a file's details. Nil fields are left alone. Area
// and TeamID move the file together; TeamID is only set for the team area.
type FilePatch struct {
	Title   *string
	Purpose *string
	Area    *string
	TeamID  *int64
}

// UpdateFile applies a patch to a file the scope may edit; otherwise
// ErrNotFound. The caller checks the scope may move the file where the patch
// says and that the team belongs to the organisation.
func (r *Repository) UpdateFile(ctx context.Context, scope Scope, publicID string, p FilePatch) (*models.File, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		UPDATE files f SET
			title = COALESCE($5::text, f.title),
			purpose = COALESCE($6::text, f.purpose),
			area = COALESCE($7::text, f.area),
			team_id = CASE WHEN $7::text IS NULL THEN f.team_id ELSE $8::bigint END,
			updated_at = now()
		WHERE `+fileEditable+` AND f.public_id = $4
		RETURNING f.file_id`,
		append(scope.fileArgs(), publicID, p.Title, p.Purpose, p.Area, p.TeamID)...).Scan(&id)
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

// GetFileUsage lists where a file is used. Cards the scope can't see are only counted.
func (r *Repository) GetFileUsage(ctx context.Context, scope Scope, file *models.File) (*models.FileUsage, error) {
	usage := &models.FileUsage{Cards: []models.FileUse{}}
	rows, err := r.db.Query(ctx, `
		SELECT p.profile_id, p.slug, COALESCE(p.data->>'name', ''), fr.slot
		FROM file_refs fr JOIN profiles p ON p.profile_id = fr.profile_id
		WHERE fr.file_id = $1 AND p.org_id = $2 AND `+visibleTo("p.assigned_user_id", 3)+`
		ORDER BY LOWER(COALESCE(NULLIF(p.data->>'name', ''), p.slug)), fr.slot`,
		file.ID, scope.OrgID, scope.memberID())
	if err != nil {
		return nil, err
	}
	usage.Cards, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.FileUse, error) {
		var u models.FileUse
		err := row.Scan(&u.ProfileID, &u.Slug, &u.Name, &u.Slot)
		return u, err
	})
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRow(ctx, `
		SELECT (SELECT COUNT(DISTINCT fr.profile_id) FROM file_refs fr WHERE fr.file_id = $1)
		         - (SELECT COUNT(DISTINCT fr.profile_id) FROM file_refs fr JOIN profiles p ON p.profile_id = fr.profile_id
		            WHERE fr.file_id = $1 AND `+visibleTo("p.assigned_user_id", 3)+`),
		       COALESCE(o.logo_file = $4, false), COALESCE(o.signature->>'banner_file' = $4, false)
		FROM organizations o WHERE o.org_id = $2`,
		file.ID, scope.OrgID, scope.memberID(), file.PublicID,
	).Scan(&usage.HiddenCards, &usage.OrgLogo, &usage.SigBanner)
	return usage, mapError(err)
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

// ListFileTeamGrants returns the teams a file has been granted to, by name.
func (r *Repository) ListFileTeamGrants(ctx context.Context, fileID int64) ([]models.TeamRef, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.team_id, t.name, t.color FROM file_team_grants g JOIN teams t ON t.team_id = g.team_id
		WHERE g.file_id = $1 ORDER BY LOWER(t.name)`, fileID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.TeamRef, error) {
		var t models.TeamRef
		err := row.Scan(&t.ID, &t.Name, &t.Color)
		return t, err
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

// ReplaceFileTeamGrants sets exactly which teams a file is granted to. Ids of
// teams outside orgID are ignored.
func (r *Repository) ReplaceFileTeamGrants(ctx context.Context, fileID, orgID, grantedBy int64, teamIDs []int64) error {
	if teamIDs == nil {
		teamIDs = []int64{}
	}
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM file_team_grants WHERE file_id = $1 AND NOT (team_id = ANY($2))`, fileID, teamIDs); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO file_team_grants (file_id, team_id, granted_by)
			SELECT $1, t.team_id, $3 FROM teams t WHERE t.org_id = $4 AND t.team_id = ANY($2)
			ON CONFLICT DO NOTHING`, fileID, teamIDs, grantedBy, orgID)
		return mapError(err)
	})
}

// ----------------------------------------------------------------------------
// Card file references
// ----------------------------------------------------------------------------

// Card slots that hold library files.
const (
	SlotAvatar   = "avatar"
	SlotCover    = "cover"
	SlotDocument = "document"
	SlotGallery  = "gallery"
)

// MaxCardFiles bounds how many files one card can reference: a photo, a
// cover, 10 brochures and a few galleries.
const MaxCardFiles = 64

// FileRef is one file a card uses, and where on the card.
type FileRef struct {
	PublicID string
	Slot     string
}

var publicIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)

// CardFileRefs pulls the library files a card's data points at. The backend
// otherwise treats card data as opaque; these keys are the exception, so a
// public card only reveals files it uses and the library knows what's in use.
func CardFileRefs(data []byte) []FileRef {
	var card struct {
		AvatarFile string `json:"avatar_file"`
		CoverFile  string `json:"cover_file"`
		Documents  []struct {
			File string `json:"file"`
		} `json:"documents"`
		Blocks []struct {
			Images []struct {
				File string `json:"file"`
			} `json:"images"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(data, &card); err != nil {
		return nil
	}
	seen := map[FileRef]bool{}
	var refs []FileRef
	add := func(id, slot string) {
		ref := FileRef{id, slot}
		if publicIDPattern.MatchString(id) && !seen[ref] && len(refs) < MaxCardFiles {
			seen[ref] = true
			refs = append(refs, ref)
		}
	}
	add(card.AvatarFile, SlotAvatar)
	add(card.CoverFile, SlotCover)
	for _, d := range card.Documents {
		add(d.File, SlotDocument)
	}
	for _, b := range card.Blocks {
		for _, img := range b.Images {
			add(img.File, SlotGallery)
		}
	}
	return refs
}

// syncFileRefs records exactly which of the organisation's files a card uses.
func syncFileRefs(ctx context.Context, q querier, profileID, orgID int64, data []byte) error {
	refs := CardFileRefs(data)
	ids, slots := make([]string, len(refs)), make([]string, len(refs))
	for i, ref := range refs {
		ids[i], slots[i] = ref.PublicID, ref.Slot
	}
	if _, err := q.Exec(ctx, `DELETE FROM file_refs WHERE profile_id = $1`, profileID); err != nil {
		return err
	}
	if len(refs) == 0 {
		return nil
	}
	_, err := q.Exec(ctx, `
		INSERT INTO file_refs (file_id, profile_id, slot)
		SELECT f.file_id, $1, r.slot
		FROM unnest($2::text[], $3::text[]) AS r(public_id, slot)
		JOIN files f ON f.public_id = r.public_id AND f.org_id = $4
		ON CONFLICT DO NOTHING`, profileID, ids, slots, orgID)
	return mapError(err)
}

// userTeamsJSON selects, as a JSON array, the teams of the user in column
// col with their role in each, by team name.
func userTeamsJSON(col string) string {
	return fmt.Sprintf(`COALESCE((SELECT json_agg(json_build_object('id', t.team_id, 'name', t.name, 'color', t.color, 'role', tm.role)
		ORDER BY LOWER(t.name)) FROM team_members tm JOIN teams t ON t.team_id = tm.team_id WHERE tm.user_id = %s), '[]')`, col)
}

func decodeTeamRefs(raw []byte) ([]models.TeamRef, error) {
	teams := []models.TeamRef{}
	if len(raw) == 0 {
		return teams, nil
	}
	if err := json.Unmarshal(raw, &teams); err != nil {
		return nil, fmt.Errorf("decode teams: %w", err)
	}
	return teams, nil
}
