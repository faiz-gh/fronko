package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
)

const (
	maxTeamNameLen        = 60
	maxTeamDescriptionLen = 280
	maxTeamMembers        = 1000
	maxTeamsPerUser       = 100
)

var teamColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// TeamHandler lets admins organise people into teams, and everyone see the teams they're in.
type TeamHandler struct {
	repo *repository.Repository
}

func NewTeamHandler(repo *repository.Repository) *TeamHandler {
	return &TeamHandler{repo: repo}
}

// TeamDetail is a team with its people.
type TeamDetail struct {
	*models.Team
	Members []models.TeamMember `json:"members"`
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

// validRoles checks every membership has a known team role, defaulting empty ones to member.
func validRoles(ms []models.TeamMembership) bool {
	for i := range ms {
		switch ms[i].Role {
		case "":
			ms[i].Role = models.TeamRoleMember
		case models.TeamRoleMember, models.TeamRoleLead:
		default:
			return false
		}
	}
	return true
}

func teamIDFromPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid team ID")
		return 0, false
	}
	return id, true
}

// Protected: GET /api/org/teams. Admins see every team; everyone else the teams they're in.
func (h *TeamHandler) List(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFrom(r.Context())
	var memberOf int64
	if !p.IsAdmin() {
		memberOf = p.UserID
	}
	teams, err := h.repo.ListTeams(r.Context(), p.OrgID, memberOf)
	if err != nil {
		internalError("list teams", err).write(w)
		return
	}
	writeJSON(w, http.StatusOK, teams)
}

// Protected: GET /api/org/teams/{id}. A team and its people, for admins and the team's own members.
func (h *TeamHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := teamIDFromPath(w, r)
	if !ok {
		return
	}
	p := middleware.PrincipalFrom(r.Context())
	if !p.IsAdmin() && !p.InTeam(id) {
		writeError(w, http.StatusNotFound, "team not found")
		return
	}
	h.writeTeam(w, r, http.StatusOK, id)
}

func (h *TeamHandler) writeTeam(w http.ResponseWriter, r *http.Request, status int, teamID int64) {
	team, err := h.repo.GetTeam(r.Context(), middleware.PrincipalFrom(r.Context()).OrgID, teamID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "team not found")
			return
		}
		internalError("get team", err).write(w)
		return
	}
	members, err := h.repo.ListTeamMembers(r.Context(), teamID)
	if err != nil {
		internalError("list team members", err).write(w)
		return
	}
	writeJSON(w, status, TeamDetail{Team: team, Members: members})
}

// Protected (admins): POST /api/org/teams {"name", "description", "color", "members"}.
func (h *TeamHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		teamRequest
		Members []models.TeamMembership `json:"members"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if len(req.Members) > maxTeamMembers || !validRoles(req.Members) {
		writeError(w, http.StatusBadRequest, "invalid members")
		return
	}
	p := middleware.PrincipalFrom(r.Context())
	team := &models.Team{OrgID: p.OrgID, Name: req.Name, Description: req.Description, Color: req.Color}
	if err := h.repo.CreateTeam(r.Context(), team); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			writeError(w, http.StatusConflict, "there's already a team with this name")
			return
		}
		internalError("create team", err).write(w)
		return
	}
	if len(req.Members) > 0 {
		if err := h.repo.ReplaceTeamMembers(r.Context(), p.OrgID, team.ID, req.Members); err != nil {
			internalError("set team members", err).write(w)
			return
		}
	}
	h.writeTeam(w, r, http.StatusCreated, team.ID)
}

// Protected (admins): PATCH /api/org/teams/{id} {"name", "description", "color"}.
func (h *TeamHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := teamIDFromPath(w, r)
	if !ok {
		return
	}
	var req teamRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := middleware.PrincipalFrom(r.Context())
	team := &models.Team{ID: id, OrgID: p.OrgID, Name: req.Name, Description: req.Description, Color: req.Color}
	if err := h.repo.UpdateTeam(r.Context(), team); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			writeError(w, http.StatusNotFound, "team not found")
		case errors.Is(err, repository.ErrConflict):
			writeError(w, http.StatusConflict, "there's already a team with this name")
		default:
			internalError("update team", err).write(w)
		}
		return
	}
	h.writeTeam(w, r, http.StatusOK, id)
}

// Protected (admins): DELETE /api/org/teams/{id}. The team's files move to
// the organisation's files so cards using them keep working.
func (h *TeamHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := teamIDFromPath(w, r)
	if !ok {
		return
	}
	if err := h.repo.DeleteTeam(r.Context(), middleware.PrincipalFrom(r.Context()).OrgID, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "team not found")
			return
		}
		internalError("delete team", err).write(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Protected (admins): PUT /api/org/teams/{id}/members {"members": [{"user_id", "role"}]}.
// Replaces who is in the team and their roles.
func (h *TeamHandler) SetMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := teamIDFromPath(w, r)
	if !ok {
		return
	}
	var req struct {
		Members []models.TeamMembership `json:"members"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Members) > maxTeamMembers || !validRoles(req.Members) {
		writeError(w, http.StatusBadRequest, "invalid members")
		return
	}
	p := middleware.PrincipalFrom(r.Context())
	if _, err := h.repo.GetTeam(r.Context(), p.OrgID, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "team not found")
			return
		}
		internalError("get team", err).write(w)
		return
	}
	if err := h.repo.ReplaceTeamMembers(r.Context(), p.OrgID, id, req.Members); err != nil {
		internalError("set team members", err).write(w)
		return
	}
	h.writeTeam(w, r, http.StatusOK, id)
}

// Protected (admins): PUT /api/org/users/{id}/teams {"teams": [{"team_id", "role"}]}.
// Replaces the teams a user is in and their role in each. Anyone in the
// organisation can be put in teams, admins and the owner included.
func (h *TeamHandler) SetUserTeams(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromPath(w, r)
	if !ok {
		return
	}
	var req struct {
		Teams []models.TeamMembership `json:"teams"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Teams) > maxTeamsPerUser || !validRoles(req.Teams) {
		writeError(w, http.StatusBadRequest, "invalid teams")
		return
	}
	p := middleware.PrincipalFrom(r.Context())
	if _, err := h.repo.GetUserInOrg(r.Context(), userID, p.OrgID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		internalError("get org user", err).write(w)
		return
	}
	if err := h.repo.ReplaceUserTeams(r.Context(), p.OrgID, userID, req.Teams); err != nil {
		internalError("set user teams", err).write(w)
		return
	}
	user, err := h.repo.GetOrgUser(r.Context(), userID, p.OrgID)
	if err != nil {
		internalError("get org user", err).write(w)
		return
	}
	writeJSON(w, http.StatusOK, user)
}
