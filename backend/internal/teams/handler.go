package teams

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

const (
	maxTeamNameLen        = 60
	maxTeamDescriptionLen = 280
	maxTeamMembers        = 1000
	MaxTeamsPerUser       = 100
)

var teamColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// TeamHandler lets admins organise people into teams, and everyone see the teams they're in.
type TeamHandler struct {
	store *Store
	users *users.Store
}

func NewTeamHandler(store *Store, userStore *users.Store) *TeamHandler {
	return &TeamHandler{store: store, users: userStore}
}

// TeamDetail is a team with its people.
type TeamDetail struct {
	*Team
	Members []TeamMember `json:"members"`
}

type teamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

// validate trims the request and returns a message if it's not a valid team.
func (t *teamRequest) validate() string {
	t.Name = strings.TrimSpace(t.Name)
	t.Description = strings.TrimSpace(t.Description)
	t.Color = strings.TrimSpace(t.Color)
	switch {
	case t.Name == "" || utf8.RuneCountInString(t.Name) > maxTeamNameLen:
		return "team name must be 1-60 characters"
	case strings.ContainsFunc(t.Name, unicode.IsControl):
		return "team name can't contain line breaks or control characters"
	case utf8.RuneCountInString(t.Description) > maxTeamDescriptionLen:
		return "description must be at most 280 characters"
	case t.Color != "" && !teamColorPattern.MatchString(t.Color):
		return "colour must look like #1a2b3c"
	}
	t.Color = strings.ToLower(t.Color)
	return ""
}

// ValidRoles checks every membership has a known team role, defaulting empty ones to member.
func ValidRoles(ms []TeamMembership) bool {
	for i := range ms {
		switch ms[i].Role {
		case "":
			ms[i].Role = auth.TeamRoleMember
		case auth.TeamRoleMember, auth.TeamRoleLead:
		default:
			return false
		}
	}
	return true
}

// Protected: GET /api/org/teams. Admins see every team; everyone else the teams they're in.
func (h *TeamHandler) List(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var memberOf int64
	if !p.IsAdmin() {
		memberOf = p.UserID
	}
	teams, err := h.store.ListTeams(r.Context(), p.OrgID, memberOf)
	if err != nil {
		httpx.Internal("list teams", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, teams)
}

// Protected: GET /api/org/teams/{id}. A team and its people, for admins and the team's own members.
func (h *TeamHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDFromPath(w, r, "team")
	if !ok {
		return
	}
	p := auth.PrincipalFrom(r.Context())
	if !p.IsAdmin() && !p.InTeam(id) {
		httpx.WriteError(w, http.StatusNotFound, "team not found")
		return
	}
	h.writeTeam(w, r, http.StatusOK, id)
}

func (h *TeamHandler) writeTeam(w http.ResponseWriter, r *http.Request, status int, teamID int64) {
	team, err := h.store.GetTeam(r.Context(), auth.PrincipalFrom(r.Context()).OrgID, teamID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "team not found")
			return
		}
		httpx.Internal("get team", err).Write(w)
		return
	}
	members, err := h.store.ListTeamMembers(r.Context(), teamID)
	if err != nil {
		httpx.Internal("list team members", err).Write(w)
		return
	}
	httpx.WriteJSON(w, status, TeamDetail{Team: team, Members: members})
}

// Protected (admins): POST /api/org/teams {"name", "description", "color", "members"}.
func (h *TeamHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		teamRequest
		Members []TeamMembership `json:"members"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		httpx.WriteError(w, http.StatusBadRequest, msg)
		return
	}
	if len(req.Members) > maxTeamMembers || !ValidRoles(req.Members) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid members")
		return
	}
	p := auth.PrincipalFrom(r.Context())
	team := &Team{OrgID: p.OrgID, Name: req.Name, Description: req.Description, Color: req.Color}
	if err := h.store.CreateTeam(r.Context(), team); err != nil {
		if errors.Is(err, database.ErrConflict) {
			httpx.WriteError(w, http.StatusConflict, "there's already a team with this name")
			return
		}
		httpx.Internal("create team", err).Write(w)
		return
	}
	if len(req.Members) > 0 {
		if err := h.store.ReplaceTeamMembers(r.Context(), p.OrgID, team.ID, req.Members); err != nil {
			httpx.Internal("set team members", err).Write(w)
			return
		}
	}
	h.writeTeam(w, r, http.StatusCreated, team.ID)
}

// Protected (admins): PATCH /api/org/teams/{id} {"name", "description", "color"}.
func (h *TeamHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDFromPath(w, r, "team")
	if !ok {
		return
	}
	var req teamRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		httpx.WriteError(w, http.StatusBadRequest, msg)
		return
	}
	p := auth.PrincipalFrom(r.Context())
	team := &Team{ID: id, OrgID: p.OrgID, Name: req.Name, Description: req.Description, Color: req.Color}
	if err := h.store.UpdateTeam(r.Context(), team); err != nil {
		switch {
		case errors.Is(err, database.ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, "team not found")
		case errors.Is(err, database.ErrConflict):
			httpx.WriteError(w, http.StatusConflict, "there's already a team with this name")
		default:
			httpx.Internal("update team", err).Write(w)
		}
		return
	}
	h.writeTeam(w, r, http.StatusOK, id)
}

// Protected (admins): DELETE /api/org/teams/{id}. The team's files move to
// the organisation's files so cards using them keep working.
func (h *TeamHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDFromPath(w, r, "team")
	if !ok {
		return
	}
	if err := h.store.DeleteTeam(r.Context(), auth.PrincipalFrom(r.Context()).OrgID, id); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "team not found")
			return
		}
		httpx.Internal("delete team", err).Write(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Protected (admins): PUT /api/org/teams/{id}/members {"members": [{"user_id", "role"}]}.
// Replaces who is in the team and their roles.
func (h *TeamHandler) SetMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDFromPath(w, r, "team")
	if !ok {
		return
	}
	var req struct {
		Members []TeamMembership `json:"members"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if len(req.Members) > maxTeamMembers || !ValidRoles(req.Members) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid members")
		return
	}
	p := auth.PrincipalFrom(r.Context())
	if _, err := h.store.GetTeam(r.Context(), p.OrgID, id); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "team not found")
			return
		}
		httpx.Internal("get team", err).Write(w)
		return
	}
	if err := h.store.ReplaceTeamMembers(r.Context(), p.OrgID, id, req.Members); err != nil {
		httpx.Internal("set team members", err).Write(w)
		return
	}
	h.writeTeam(w, r, http.StatusOK, id)
}

// Protected (admins): PUT /api/org/users/{id}/teams {"teams": [{"team_id", "role"}]}.
// Replaces the teams a user is in and their role in each. Anyone in the
// organisation can be put in teams, admins and the owner included.
func (h *TeamHandler) SetUserTeams(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpx.IDFromPath(w, r, "user")
	if !ok {
		return
	}
	var req struct {
		Teams []TeamMembership `json:"teams"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if len(req.Teams) > MaxTeamsPerUser || !ValidRoles(req.Teams) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid teams")
		return
	}
	p := auth.PrincipalFrom(r.Context())
	if _, err := h.users.GetUserInOrg(r.Context(), userID, p.OrgID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		httpx.Internal("get org user", err).Write(w)
		return
	}
	if err := h.store.ReplaceUserTeams(r.Context(), p.OrgID, userID, req.Teams); err != nil {
		httpx.Internal("set user teams", err).Write(w)
		return
	}
	user, err := h.users.GetOrgUser(r.Context(), userID, p.OrgID)
	if err != nil {
		httpx.Internal("get org user", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}
