package repository

import (
	"context"

	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

// ----------------------------------------------------------------------------
// Team Methods
// ----------------------------------------------------------------------------

const teamSelect = `
	SELECT t.team_id, t.org_id, t.name, t.description, t.color,
	       (SELECT COUNT(*) FROM team_members m WHERE m.team_id = t.team_id),
	       (SELECT COUNT(*) FROM team_members m WHERE m.team_id = t.team_id AND m.role = 'lead'),
	       (SELECT COUNT(*) FROM files f WHERE f.team_id = t.team_id),
	       t.created_at, t.updated_at
	FROM teams t`

func scanTeam(row pgx.Row) (*models.Team, error) {
	var t models.Team
	err := row.Scan(&t.ID, &t.OrgID, &t.Name, &t.Description, &t.Color,
		&t.MemberCount, &t.LeadCount, &t.FileCount, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &t, nil
}

// ListTeams returns the organisation's teams by name: all of them, or when
// memberOf is set only the teams that user is in.
func (r *Repository) ListTeams(ctx context.Context, orgID, memberOf int64) ([]*models.Team, error) {
	rows, err := r.db.Query(ctx, teamSelect+`
		WHERE t.org_id = $1 AND ($2::bigint = 0
			OR EXISTS (SELECT 1 FROM team_members m WHERE m.team_id = t.team_id AND m.user_id = $2))
		ORDER BY LOWER(t.name)`, orgID, memberOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	teams := []*models.Team{}
	for rows.Next() {
		t, err := scanTeam(rows)
		if err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

// GetTeam returns a team in orgID; otherwise ErrNotFound.
func (r *Repository) GetTeam(ctx context.Context, orgID, teamID int64) (*models.Team, error) {
	return scanTeam(r.db.QueryRow(ctx, teamSelect+` WHERE t.org_id = $1 AND t.team_id = $2`, orgID, teamID))
}

// CreateTeam adds a team. ErrConflict means the name is taken in the organisation.
func (r *Repository) CreateTeam(ctx context.Context, t *models.Team) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO teams (org_id, name, description, color) VALUES ($1, $2, $3, $4)
		RETURNING team_id, created_at, updated_at`, t.OrgID, t.Name, t.Description, t.Color,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	return mapError(err)
}

// UpdateTeam saves a team's name, description and colour.
func (r *Repository) UpdateTeam(ctx context.Context, t *models.Team) error {
	return r.execOne(ctx, `
		UPDATE teams SET name = $3, description = $4, color = $5, updated_at = now()
		WHERE team_id = $1 AND org_id = $2`, t.ID, t.OrgID, t.Name, t.Description, t.Color)
}

// DeleteTeam removes a team. Its files move to the organisation's files so
// cards using them keep working; memberships and grants go with the team.
func (r *Repository) DeleteTeam(ctx context.Context, orgID, teamID int64) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE files SET area = 'org', team_id = NULL, updated_at = now()
			WHERE team_id = $1 AND org_id = $2`, teamID, orgID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM teams WHERE team_id = $1 AND org_id = $2`, teamID, orgID)
		if err != nil {
			return mapError(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// ListTeamMembers returns a team's people: leads first, then by username.
func (r *Repository) ListTeamMembers(ctx context.Context, teamID int64) ([]models.TeamMember, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.user_id, u.username, u.email, u.role, m.role, m.added_at
		FROM team_members m JOIN users u ON u.user_id = m.user_id
		WHERE m.team_id = $1
		ORDER BY CASE m.role WHEN 'lead' THEN 0 ELSE 1 END, LOWER(u.username)`, teamID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.TeamMember, error) {
		var m models.TeamMember
		err := row.Scan(&m.ID, &m.Username, &m.Email, &m.OrgRole, &m.Role, &m.AddedAt)
		return m, err
	})
}

// ReplaceTeamMembers sets exactly who is in a team and their roles. Users
// outside orgID are ignored. Members who stay keep their original added_at.
func (r *Repository) ReplaceTeamMembers(ctx context.Context, orgID, teamID int64, members []models.TeamMembership) error {
	userIDs, roles := splitMemberships(members, func(m models.TeamMembership) int64 { return m.UserID })
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM team_members WHERE team_id = $1 AND NOT (user_id = ANY($2))`, teamID, userIDs); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO team_members (team_id, user_id, role)
			SELECT $1, u.user_id, m.role
			FROM unnest($2::bigint[], $3::text[]) AS m(user_id, role)
			JOIN users u ON u.user_id = m.user_id AND u.org_id = $4
			ON CONFLICT (team_id, user_id) DO UPDATE SET role = EXCLUDED.role`, teamID, userIDs, roles, orgID)
		return mapError(err)
	})
}

// ReplaceUserTeams sets exactly which teams a user is in and their role in
// each. Teams outside orgID are ignored.
func (r *Repository) ReplaceUserTeams(ctx context.Context, orgID, userID int64, teams []models.TeamMembership) error {
	return replaceUserTeams(ctx, r.db, orgID, userID, teams)
}

func replaceUserTeams(ctx context.Context, q querier, orgID, userID int64, teams []models.TeamMembership) error {
	teamIDs, roles := splitMemberships(teams, func(m models.TeamMembership) int64 { return m.TeamID })
	if _, err := q.Exec(ctx, `DELETE FROM team_members WHERE user_id = $1 AND NOT (team_id = ANY($2))`, userID, teamIDs); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, role)
		SELECT t.team_id, $1, m.role
		FROM unnest($2::bigint[], $3::text[]) AS m(team_id, role)
		JOIN teams t ON t.team_id = m.team_id AND t.org_id = $4
		ON CONFLICT (team_id, user_id) DO UPDATE SET role = EXCLUDED.role`, userID, teamIDs, roles, orgID)
	return mapError(err)
}

// splitMemberships turns memberships into parallel id and role arrays for
// unnest, never nil (a nil slice is NULL, which would match nothing).
func splitMemberships(ms []models.TeamMembership, id func(models.TeamMembership) int64) ([]int64, []string) {
	ids, roles := make([]int64, 0, len(ms)), make([]string, 0, len(ms))
	for _, m := range ms {
		ids = append(ids, id(m))
		roles = append(roles, m.Role)
	}
	return ids, roles
}

// CountTeams is the number of teams in an organisation.
func (r *Repository) CountTeams(ctx context.Context, orgID int64) (int64, error) {
	var n int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM teams WHERE org_id = $1`, orgID).Scan(&n)
	return n, err
}

// ListUserTeams returns the teams a user is in, with their role in each, by name.
func (r *Repository) ListUserTeams(ctx context.Context, userID int64) ([]models.TeamRef, error) {
	var raw []byte
	if err := r.db.QueryRow(ctx, `SELECT `+userTeamsJSON("$1::bigint"), userID).Scan(&raw); err != nil {
		return nil, err
	}
	return decodeTeamRefs(raw)
}
