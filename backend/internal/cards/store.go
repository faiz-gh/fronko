package cards

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/faiz-gh/fronko/backend/internal/analytics"
	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/files"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
)

// Store runs the SQL for cards (profiles).
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// CreateProfile saves a new card and records which library files it uses.
func (r *Store) CreateProfile(ctx context.Context, profile *Profile) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		query := `
			INSERT INTO profiles (org_id, user_id, assigned_user_id, slug, data)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING profile_id, created_at, updated_at`
		err := tx.QueryRow(ctx, query, profile.OrgID, profile.UserID, profile.AssignedUserID, profile.Slug, profile.Data).Scan(
			&profile.ID, &profile.CreatedAt, &profile.UpdatedAt,
		)
		if err != nil {
			return database.MapError(err)
		}
		return files.SyncRefs(ctx, tx, profile.ID, profile.OrgID, profile.Data)
	})
}

// UpdateProfile saves slug and data on a card the scope can edit (any card in
// the org for admins, an assigned card for members, and their teammates'
// cards for team leads); otherwise database.ErrNotFound. It also records which library
// files the card now uses.
func (r *Store) UpdateProfile(ctx context.Context, scope auth.Scope, profile *Profile) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		query := `
			UPDATE profiles
			SET slug = $1, data = $2, updated_at = now()
			WHERE profile_id = $3 AND org_id = $4 AND ` + auth.VisibleTo("assigned_user_id", 5) + `
			RETURNING created_at, updated_at`
		err := tx.QueryRow(ctx, query, profile.Slug, profile.Data, profile.ID, scope.OrgID, scope.MemberID()).Scan(
			&profile.CreatedAt, &profile.UpdatedAt,
		)
		if err != nil {
			return database.MapError(err)
		}
		return files.SyncRefs(ctx, tx, profile.ID, scope.OrgID, profile.Data)
	})
}

// DeleteProfile deletes a card in orgID; otherwise database.ErrNotFound.
func (r *Store) DeleteProfile(ctx context.Context, profileID, orgID int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM profiles WHERE profile_id = $1 AND org_id = $2`, profileID, orgID)
	if err != nil {
		return database.MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrNotFound
	}
	return nil
}

// SetProfileAssignee hands a card in orgID to userID, or back to the
// organisation when userID is nil. The caller checks userID is in the org.
func (r *Store) SetProfileAssignee(ctx context.Context, profileID, orgID int64, userID *int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE profiles SET assigned_user_id = $3, updated_at = now() WHERE profile_id = $1 AND org_id = $2`,
		profileID, orgID, userID)
	if err != nil {
		return database.MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrNotFound
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
	        WHERE l.profile_id = p.profile_id AND ` + auth.VisibleTo("l.assigned_user_id", 2) + `)
	FROM profiles p
	LEFT JOIN users au ON au.user_id = p.assigned_user_id`

func scanProfile(row pgx.Row) (*Profile, error) {
	var p Profile
	var assignee *string
	err := row.Scan(&p.ID, &p.OrgID, &p.UserID, &p.AssignedUserID, &assignee, &p.Slug, &p.Data,
		&p.CreatedAt, &p.UpdatedAt, &p.LeadCount)
	if err != nil {
		return nil, database.MapError(err)
	}
	if p.AssignedUserID != nil && assignee != nil {
		p.AssignedUser = &auth.UserRef{ID: *p.AssignedUserID, Username: *assignee}
	}
	return &p, nil
}

// GetProfileByPath finds a card by its link, /p/{handle}/{slug}: the
// organisation's handle and the card's slug, both case-insensitive.
func (r *Store) GetProfileByPath(ctx context.Context, handle, slug string) (*Profile, error) {
	query := `SELECT p.profile_id, p.org_id, p.user_id, p.assigned_user_id, p.slug, p.data, p.created_at, p.updated_at,
		       o.handle, o.suspended_at IS NOT NULL
		FROM profiles p JOIN organizations o ON o.org_id = p.org_id
		WHERE LOWER(o.handle) = LOWER($1) AND LOWER(p.slug) = LOWER($2)`
	var p Profile
	err := r.db.QueryRow(ctx, query, handle, slug).Scan(
		&p.ID, &p.OrgID, &p.UserID, &p.AssignedUserID, &p.Slug, &p.Data, &p.CreatedAt, &p.UpdatedAt, &p.OrgHandle, &p.OrgSuspended,
	)
	if err != nil {
		return nil, database.MapError(err)
	}
	return &p, nil
}

// GetProfile returns a card the scope can see (any card in the org for
// admins, an assigned card for members, teammates' cards for team leads);
// otherwise database.ErrNotFound.
func (r *Store) GetProfile(ctx context.Context, scope auth.Scope, profileID int64) (*Profile, error) {
	return scanProfile(r.db.QueryRow(ctx, profileSelect+`
		WHERE p.org_id = $1 AND `+auth.VisibleTo("p.assigned_user_id", 2)+` AND p.profile_id = $3`,
		scope.OrgID, scope.MemberID(), profileID))
}

// ListProfiles returns the cards the scope can see, newest first, each with
// its lead count.
func (r *Store) ListProfiles(ctx context.Context, scope auth.Scope) ([]*Profile, error) {
	rows, err := r.db.Query(ctx, profileSelect+`
		WHERE p.org_id = $1 AND `+auth.VisibleTo("p.assigned_user_id", 2)+`
		ORDER BY p.created_at DESC`, scope.OrgID, scope.MemberID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles := []*Profile{}
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// ResolveCard is the analytics.CardResolver for public beacons.
func (r *Store) ResolveCard(ctx context.Context, handle, slug string) (analytics.Card, error) {
	p, err := r.GetProfileByPath(ctx, handle, slug)
	if err != nil {
		return analytics.Card{}, err
	}
	return analytics.Card{ID: p.ID, OrgID: p.OrgID, OrgSuspended: p.OrgSuspended}, nil
}
