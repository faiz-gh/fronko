package directory

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// Store runs the SQL behind SCIM: people and teams as an identity provider
// sees them. Creating and deleting people goes through the users and orgs
// stores, so it works exactly as it does from the dashboard.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

// person is a user as SCIM shows them.
type person struct {
	ID               int64
	Role             string
	Username         string
	Email            *string
	EmailVerified    bool
	FullName         *string
	ExternalID       *string
	ExternalUsername *string
	Suspended        bool
	Teams            []auth.TeamRef
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// userName is what the identity provider calls the person: the user name it
// gave, or else their email.
func (p *person) userName() string {
	if p.ExternalUsername != nil {
		return *p.ExternalUsername
	}
	if p.Email != nil {
		return *p.Email
	}
	return p.Username
}

var personQuery = `SELECT u.user_id, u.role, u.username, u.email, u.email_verified_at IS NOT NULL, u.full_name,
	u.external_id, u.external_username, u.suspended_at IS NOT NULL, ` + users.TeamsJSON("u.user_id") + `,
	u.created_at, u.updated_at FROM users u`

func scanPerson(row pgx.Row) (*person, error) {
	var p person
	var teams []byte
	err := row.Scan(&p.ID, &p.Role, &p.Username, &p.Email, &p.EmailVerified, &p.FullName,
		&p.ExternalID, &p.ExternalUsername, &p.Suspended, &teams, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, database.MapError(err)
	}
	if p.Teams, err = users.DecodeTeamRefs(teams); err != nil {
		return nil, err
	}
	return &p, nil
}

// userFilters are the SCIM attributes people can be looked up by.
var userFilters = map[string]string{
	// The identity provider's user name, or the email of someone it hasn't
	// named yet (an account made in Fronko before provisioning).
	"username":     `(lower(u.external_username) = lower($2) OR (u.external_username IS NULL AND lower(u.email) = lower($2)))`,
	"externalid":   `u.external_id = $2`,
	"emails":       `lower(u.email) = lower($2)`,
	"emails.value": `lower(u.email) = lower($2)`,
	"id":           `u.user_id::text = $2`,
}

// listPeople returns a page of the organisation's people, oldest first, and
// how many match in all.
func (s *Store) listPeople(ctx context.Context, orgID int64, f *filter, offset, limit int) ([]*person, int, error) {
	where, args := `u.org_id = $1`, []any{orgID}
	if f != nil {
		where += ` AND ` + userFilters[f.attr]
		args = append(args, f.value)
	}
	var total int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM users u WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, offset, limit)
	n := len(args)
	rows, err := s.db.Query(ctx, personQuery+` WHERE `+where+
		` ORDER BY u.user_id OFFSET $`+strconv.Itoa(n-1)+` LIMIT $`+strconv.Itoa(n), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*person{}
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func (s *Store) getPerson(ctx context.Context, orgID, id int64) (*person, error) {
	return scanPerson(s.db.QueryRow(ctx, personQuery+` WHERE u.org_id = $1 AND u.user_id = $2`, orgID, id))
}

// personChange is what an identity provider sets on someone.
type personChange struct {
	Email            string
	EmailVerified    bool
	FullName         *string
	ExternalID       *string
	ExternalUsername *string
}

// updatePerson saves what the identity provider knows about someone. A new
// email is verified only when the provider vouches for its domain.
func (s *Store) updatePerson(ctx context.Context, orgID, id int64, c personChange) error {
	return database.ExecOne(ctx, s.db, `
		UPDATE users SET
			email = $3,
			email_verified_at = CASE
				WHEN lower(email) IS NOT DISTINCT FROM lower($3) AND email_verified_at IS NOT NULL THEN email_verified_at
				WHEN $4 THEN now() END,
			full_name = $5, external_id = $6, external_username = $7, updated_at = now()
		WHERE org_id = $1 AND user_id = $2`,
		orgID, id, c.Email, c.EmailVerified, c.FullName, c.ExternalID, c.ExternalUsername)
}

// emailOrg returns the organisation of the account using email, or 0.
func (s *Store) emailOrg(ctx context.Context, email string) (int64, error) {
	var orgID int64
	err := s.db.QueryRow(ctx, `SELECT org_id FROM users WHERE lower(email) = lower($1)`, email).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return orgID, err
}

// group is a team as SCIM shows it.
type group struct {
	ID         int64
	Name       string
	ExternalID *string
	Members    []auth.UserRef
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

const groupSelect = `SELECT t.team_id, t.name, t.external_id, t.created_at, t.updated_at FROM teams t`

var groupFilters = map[string]string{
	"displayname": `lower(t.name) = lower($2)`,
	"externalid":  `t.external_id = $2`,
	"id":          `t.team_id::text = $2`,
}

func scanGroup(row pgx.Row) (*group, error) {
	var g group
	if err := row.Scan(&g.ID, &g.Name, &g.ExternalID, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return nil, database.MapError(err)
	}
	g.Members = []auth.UserRef{}
	return &g, nil
}

// listGroups returns a page of the organisation's teams, oldest first, and
// how many match in all; with members, each team's people too.
func (s *Store) listGroups(ctx context.Context, orgID int64, f *filter, offset, limit int, members bool) ([]*group, int, error) {
	where, args := `t.org_id = $1`, []any{orgID}
	if f != nil {
		where += ` AND ` + groupFilters[f.attr]
		args = append(args, f.value)
	}
	var total int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM teams t WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, offset, limit)
	n := len(args)
	rows, err := s.db.Query(ctx, groupSelect+` WHERE `+where+
		` ORDER BY t.team_id OFFSET $`+strconv.Itoa(n-1)+` LIMIT $`+strconv.Itoa(n), args...)
	if err != nil {
		return nil, 0, err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*group, error) { return scanGroup(row) })
	if err != nil || !members {
		return out, total, err
	}
	for _, g := range out {
		if g.Members, err = s.groupMembers(ctx, g.ID); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

func (s *Store) getGroup(ctx context.Context, orgID, id int64, members bool) (*group, error) {
	g, err := scanGroup(s.db.QueryRow(ctx, groupSelect+` WHERE t.org_id = $1 AND t.team_id = $2`, orgID, id))
	if err != nil || !members {
		return g, err
	}
	g.Members, err = s.groupMembers(ctx, id)
	return g, err
}

func (s *Store) groupMembers(ctx context.Context, teamID int64) ([]auth.UserRef, error) {
	rows, err := s.db.Query(ctx, `SELECT u.user_id, u.username FROM team_members m
		JOIN users u ON u.user_id = m.user_id WHERE m.team_id = $1 ORDER BY u.user_id`, teamID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[auth.UserRef])
}

// createGroup adds a team. database.ErrConflict means the name or external
// id is taken in the organisation.
func (s *Store) createGroup(ctx context.Context, orgID int64, name string, externalID *string) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `INSERT INTO teams (org_id, name, external_id) VALUES ($1, $2, $3) RETURNING team_id`,
		orgID, name, externalID).Scan(&id)
	return id, database.MapError(err)
}

func (s *Store) updateGroup(ctx context.Context, orgID, id int64, name string, externalID *string) error {
	return database.ExecOne(ctx, s.db, `UPDATE teams SET name = $3, external_id = $4, updated_at = now()
		WHERE org_id = $1 AND team_id = $2`, orgID, id, name, externalID)
}

// addMembers puts people in a team as members; people already in it keep
// their role, and people outside the organisation are ignored.
func (s *Store) addMembers(ctx context.Context, q database.Querier, orgID, teamID int64, userIDs []int64) error {
	_, err := q.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id)
		SELECT $1, u.user_id FROM users u WHERE u.org_id = $2 AND u.user_id = ANY($3)
		ON CONFLICT (team_id, user_id) DO NOTHING`, teamID, orgID, nonNil(userIDs))
	return database.MapError(err)
}

func (s *Store) removeMembers(ctx context.Context, q database.Querier, teamID int64, userIDs []int64) error {
	_, err := q.Exec(ctx, `DELETE FROM team_members WHERE team_id = $1 AND user_id = ANY($2)`, teamID, nonNil(userIDs))
	return err
}

// setMembers makes the team's people exactly userIDs, keeping the role of
// those who stay.
func (s *Store) setMembers(ctx context.Context, orgID, teamID int64, userIDs []int64) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM team_members WHERE team_id = $1 AND NOT (user_id = ANY($2))`,
			teamID, nonNil(userIDs)); err != nil {
			return err
		}
		return s.addMembers(ctx, tx, orgID, teamID, userIDs)
	})
}

func nonNil(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return ids
}
