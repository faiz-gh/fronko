package leads

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/events"
)

// Store runs the SQL for leads.
type Store struct {
	db  *pgxpool.Pool
	bus *events.Bus
}

// NewStore returns the leads store. Created events go to bus, which may be
// nil when nothing listens (tools and tests).
func NewStore(db *pgxpool.Pool, bus *events.Bus) *Store {
	return &Store{db: db, bus: bus}
}

// leadColumns lists the columns scanLead expects, for leads l joined to their
// assignee as u.
const leadColumns = `l.lead_id, l.profile_id, l.name, l.email,
	COALESCE(l.phone_country_code, ''), COALESCE(l.phone_number, ''),
	COALESCE(l.notes, ''), COALESCE(l.source, ''), l.created_at, l.assigned_user_id, u.username`

func scanLead(rows pgx.Rows) (*Lead, error) {
	var l Lead
	var assigneeID *int64
	var assignee *string
	err := rows.Scan(&l.ID, &l.ProfileID, &l.Name, &l.Email, &l.PhoneCountryCode, &l.PhoneNumber, &l.Notes, &l.Source, &l.CreatedAt,
		&assigneeID, &assignee)
	if assigneeID != nil && assignee != nil {
		l.AssignedUser = &auth.UserRef{ID: *assigneeID, Username: *assignee}
	}
	return &l, err
}

// CreateLead stores a lead against whoever holds the card right now, so it
// stays theirs if the card is later reassigned, and publishes Created in the
// same transaction. It returns database.ErrNotFound if the profile does not
// exist or its organisation is suspended.
func (r *Store) CreateLead(ctx context.Context, lead *Lead) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		query := `
			WITH card AS (
				SELECT p.profile_id, p.org_id, p.assigned_user_id
				FROM profiles p JOIN organizations o ON o.org_id = p.org_id
				WHERE p.profile_id = $1 AND o.suspended_at IS NULL
			)
			INSERT INTO leads (profile_id, name, email, phone_country_code, phone_number, notes, source, assigned_user_id)
			SELECT profile_id, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, NULLIF($7, ''), assigned_user_id FROM card
			RETURNING lead_id, created_at, (SELECT org_id FROM card), COALESCE(assigned_user_id, 0)`
		evt := Created{ProfileID: lead.ProfileID}
		err := tx.QueryRow(ctx, query,
			lead.ProfileID, lead.Name, lead.Email, lead.PhoneCountryCode, lead.PhoneNumber, lead.Notes, lead.Source,
		).Scan(
			&lead.ID, &lead.CreatedAt, &evt.OrgID, &evt.AssignedUserID,
		)
		if err != nil {
			return database.MapError(err)
		}
		evt.LeadID = lead.ID
		return r.bus.Publish(ctx, tx, evt)
	})
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

// ListLeads returns one page of the leads the scope can see, newest first,
// plus how many match the filter in total. Admins see every lead on the
// organisation's cards; members only the leads that arrived while they held
// the card, and team leads also their teammates' leads. Leads outside the scope are never included, whatever the filter says.
func (r *Store) ListLeads(ctx context.Context, scope auth.Scope, f LeadFilter) ([]*Lead, int64, error) {
	var since *time.Time
	if !f.Since.IsZero() {
		since = &f.Since
	}
	search := ""
	if f.Search != "" {
		search = database.LikePattern(f.Search)
	}

	where := `
		FROM leads l
		JOIN profiles p ON p.profile_id = l.profile_id
		LEFT JOIN users u ON u.user_id = l.assigned_user_id
		WHERE p.org_id = $1
		  AND ` + auth.VisibleTo("l.assigned_user_id", 2) + `
		  AND ($3::bigint = 0 OR l.profile_id = $3)
		  AND ($4::text = '' OR l.name ILIKE $4 OR l.email ILIKE $4 OR l.phone_number ILIKE $4 OR l.notes ILIKE $4)
		  AND ($5::timestamptz IS NULL OR l.created_at >= $5)
		  AND ($6::bigint = 0 OR l.assigned_user_id = $6)
		  AND (NOT $7::bool OR l.assigned_user_id IS NULL)
		  AND ($8::bigint = 0 OR l.assigned_user_id IN (SELECT tm.user_id FROM team_members tm WHERE tm.team_id = $8))`
	args := []any{scope.OrgID, scope.MemberID(), f.ProfileID, search, since, f.UserID, f.Unassigned, f.TeamID}

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

	leads := []*Lead{}
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

// DeleteLead removes a lead from one of the organisation's cards. It returns
// database.ErrNotFound if there is no such lead in the organisation. Pending
// lead sync for it finds nothing and stops.
func (r *Store) DeleteLead(ctx context.Context, leadID, orgID int64) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM leads l USING profiles p
		WHERE l.lead_id = $1 AND p.profile_id = l.profile_id AND p.org_id = $2`, leadID, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return database.ErrNotFound
	}
	return nil
}

// SyncDetails is a lead with what lead sync sends alongside it.
type SyncDetails struct {
	Lead
	OrgID     int64
	OrgName   string
	OrgHandle string
	CardSlug  string
	// CardName is the name on the card.
	CardName string
	// OwnerEmail is the card holder's email, when there is a holder.
	OwnerEmail string
}

// SyncDetails loads a lead for lead sync. It returns database.ErrNotFound if
// the lead was deleted.
func (r *Store) SyncDetails(ctx context.Context, leadID int64) (*SyncDetails, error) {
	var d SyncDetails
	var assigneeID *int64
	var assignee, ownerEmail *string
	err := r.db.QueryRow(ctx, `
		SELECT l.lead_id, l.profile_id, l.name, l.email,
			COALESCE(l.phone_country_code, ''), COALESCE(l.phone_number, ''),
			COALESCE(l.notes, ''), COALESCE(l.source, ''), l.created_at, l.assigned_user_id, u.username, u.email,
			o.org_id, o.name, o.handle, p.slug, COALESCE(p.data->>'name', '')
		FROM leads l
		JOIN profiles p ON p.profile_id = l.profile_id
		JOIN organizations o ON o.org_id = p.org_id
		LEFT JOIN users u ON u.user_id = l.assigned_user_id
		WHERE l.lead_id = $1`, leadID,
	).Scan(&d.ID, &d.ProfileID, &d.Name, &d.Email, &d.PhoneCountryCode, &d.PhoneNumber, &d.Notes, &d.Source, &d.CreatedAt,
		&assigneeID, &assignee, &ownerEmail, &d.OrgID, &d.OrgName, &d.OrgHandle, &d.CardSlug, &d.CardName)
	if err != nil {
		return nil, database.MapError(err)
	}
	if assigneeID != nil && assignee != nil {
		d.AssignedUser = &auth.UserRef{ID: *assigneeID, Username: *assignee}
	}
	if ownerEmail != nil {
		d.OwnerEmail = *ownerEmail
	}
	return &d, nil
}
