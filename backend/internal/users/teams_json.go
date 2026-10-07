package users

import (
	"encoding/json"
	"fmt"

	"github.com/faiz-gh/fronko/backend/internal/auth"
)

// TeamsJSON selects, as a JSON array, the teams of the user in column
// col with their role in each, by team name.
func TeamsJSON(col string) string {
	return fmt.Sprintf(`COALESCE((SELECT json_agg(json_build_object('id', t.team_id, 'name', t.name, 'color', t.color, 'role', tm.role)
		ORDER BY LOWER(t.name)) FROM team_members tm JOIN teams t ON t.team_id = tm.team_id WHERE tm.user_id = %s), '[]')`, col)
}

func DecodeTeamRefs(raw []byte) ([]auth.TeamRef, error) {
	teams := []auth.TeamRef{}
	if len(raw) == 0 {
		return teams, nil
	}
	if err := json.Unmarshal(raw, &teams); err != nil {
		return nil, fmt.Errorf("decode teams: %w", err)
	}
	return teams, nil
}
