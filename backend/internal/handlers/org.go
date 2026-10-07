package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/mail"
	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
)

// maxQuotaBytes keeps quotas to a sane range (1 TB).
const maxQuotaBytes = 1 << 40

// OrgHandler lets an organisation's owner and admins manage it and its users.
type OrgHandler struct {
	repo *repository.Repository
	auth *AuthHandler // for hashing passwords and sending codes
}

func NewOrgHandler(repo *repository.Repository, authHandler *AuthHandler) *OrgHandler {
	return &OrgHandler{repo: repo, auth: authHandler}
}

// optionalQuota tells a quota that wasn't sent apart from null (unlimited).
type optionalQuota struct {
	Set   bool
	Value *int64
}

func (q *optionalQuota) UnmarshalJSON(b []byte) error {
	q.Set = true
	if string(b) == "null" {
		q.Value = nil
		return nil
	}
	var v int64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	q.Value = &v
	return nil
}

func (q optionalQuota) valid() bool {
	return q.Value == nil || (*q.Value >= 0 && *q.Value <= maxQuotaBytes)
}

const errQuotaRange = "storage limit must be between 0 and 1 TB, or unlimited"

// Protected (admins): GET /api/org
func (h *OrgHandler) Get(w http.ResponseWriter, r *http.Request) {
	org, err := h.repo.GetOrganization(r.Context(), middleware.PrincipalFrom(r.Context()).OrgID)
	if err != nil {
		log.Printf("get organization: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, org)
}

// Protected (owner): PUT /api/org {"name", "default_quota_bytes"}
func (h *OrgHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name              string        `json:"name"`
		DefaultQuotaBytes optionalQuota `json:"default_quota_bytes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !validOrgName(req.Name) {
		writeError(w, http.StatusBadRequest, "organisation name must be 1-80 characters")
		return
	}
	if !req.DefaultQuotaBytes.valid() {
		writeError(w, http.StatusBadRequest, errQuotaRange)
		return
	}
	org := &models.Organization{
		ID:                middleware.PrincipalFrom(r.Context()).OrgID,
		Name:              req.Name,
		DefaultQuotaBytes: req.DefaultQuotaBytes.Value,
	}
	if err := h.repo.UpdateOrganization(r.Context(), org); err != nil {
		log.Printf("update organization: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, org)
}

// Protected (admins): PUT /api/org/handle {"handle"}. Changes the
// organisation's part of every card link. Links already printed on QR codes
// or written to NFC cards stop working, so the dashboard warns first.
func (h *OrgHandler) UpdateHandle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Handle string `json:"handle"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	handle := strings.ToLower(strings.TrimSpace(req.Handle))
	if !validHandle(handle) {
		writeError(w, http.StatusBadRequest, "handle must be 3-32 characters: lowercase letters, numbers and hyphens")
		return
	}
	orgID := middleware.PrincipalFrom(r.Context()).OrgID
	if err := h.repo.SetOrgHandle(r.Context(), orgID, handle); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			writeError(w, http.StatusConflict, "another organisation already uses that handle")
			return
		}
		log.Printf("set org handle: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.Get(w, r)
}

// Protected (admins): GET /api/org/users
func (h *OrgHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.repo.ListOrgUsers(r.Context(), middleware.PrincipalFrom(r.Context()).OrgID)
	if err != nil {
		log.Printf("list org users: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

type createUserRequest struct {
	Username   string        `json:"username"`
	Email      string        `json:"email"`
	Password   string        `json:"password"`
	Role       string        `json:"role"`
	QuotaBytes optionalQuota `json:"quota_bytes"`
	// Teams to put the new user in, with their role in each.
	Teams []models.TeamMembership `json:"teams"`
}

// Protected (admins): POST /api/org/users. Creates a user with a temporary
// password they must replace, and emails them their username and that
// password. They verify their email on first sign-in. Only the owner can
// create admins.
func (h *OrgHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFrom(r.Context())
	var req createUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !usernamePattern.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, "username must be 3-32 characters: letters, numbers, '.', '_' or '-'")
		return
	}
	email, ok := normalizeEmail(req.Email)
	if !ok {
		writeError(w, http.StatusBadRequest, "enter a valid email address")
		return
	}
	if !validPassword(w, req.Password) {
		return
	}
	switch req.Role {
	case "":
		req.Role = models.RoleMember
	case models.RoleMember:
	case models.RoleAdmin:
		if !p.IsOwner() {
			writeError(w, http.StatusForbidden, "only the owner can add admins")
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "role must be member or admin")
		return
	}
	if !req.QuotaBytes.valid() {
		writeError(w, http.StatusBadRequest, errQuotaRange)
		return
	}
	if len(req.Teams) > maxTeamsPerUser || !validRoles(req.Teams) {
		writeError(w, http.StatusBadRequest, "invalid teams")
		return
	}

	org, err := h.repo.GetOrganization(r.Context(), p.OrgID)
	if err != nil {
		log.Printf("get organization: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	quota := org.DefaultQuotaBytes
	if req.QuotaBytes.Set {
		quota = req.QuotaBytes.Value
	}

	hash, err := h.auth.authService.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	creator := p.UserID
	user := &models.User{
		OrgID:              p.OrgID,
		Role:               req.Role,
		Username:           req.Username,
		Email:              &email,
		PasswordHash:       hash,
		MustChangePassword: true,
		StorageQuotaBytes:  quota,
		CreatedBy:          &creator,
	}
	if err := h.repo.CreateUser(r.Context(), user, req.Teams...); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			if repository.IsEmailConflict(err) {
				writeError(w, http.StatusConflict, "an account with this email already exists")
			} else {
				writeError(w, http.StatusConflict, "username already taken")
			}
			return
		}
		log.Printf("create org user: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	h.auth.sendAsync(email, mail.MemberInviteMessage(org.Name, user.Username, req.Password))
	h.writeUser(w, r, http.StatusCreated, user.ID)
}

// Protected (admins): GET /api/org/users/{id}
func (h *OrgHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDFromPath(w, r)
	if !ok {
		return
	}
	h.writeUser(w, r, http.StatusOK, id)
}

func (h *OrgHandler) writeUser(w http.ResponseWriter, r *http.Request, status int, userID int64) {
	user, err := h.repo.GetOrgUser(r.Context(), userID, middleware.PrincipalFrom(r.Context()).OrgID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		log.Printf("get org user: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, status, user)
}

func userIDFromPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return 0, false
	}
	return id, true
}

// manageable loads a user the caller may manage, writing the error response
// if they can't: nobody manages the owner or themselves here, and only the
// owner manages admins.
func (h *OrgHandler) manageable(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	p := middleware.PrincipalFrom(r.Context())
	id, ok := userIDFromPath(w, r)
	if !ok {
		return nil, false
	}
	user, err := h.repo.GetUserInOrg(r.Context(), id, p.OrgID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return nil, false
		}
		log.Printf("get org user: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return nil, false
	}
	switch {
	case user.ID == p.UserID:
		writeError(w, http.StatusBadRequest, "manage your own account in Settings")
		return nil, false
	case user.Role == models.RoleOwner:
		writeError(w, http.StatusForbidden, "the owner's account can't be changed here")
		return nil, false
	case user.Role == models.RoleAdmin && !p.IsOwner():
		writeError(w, http.StatusForbidden, "only the owner can manage admins")
		return nil, false
	}
	return user, true
}

type updateUserRequest struct {
	Email      *string       `json:"email"`
	QuotaBytes optionalQuota `json:"quota_bytes"`
	Role       *string       `json:"role"`
	Suspended  *bool         `json:"suspended"`
}

// Protected (admins): PATCH /api/org/users/{id}. Every field is optional.
// The email can only be corrected while it's unverified; a new address gets
// a fresh verification code. Changing roles is for the owner only, and
// suspending ends the user's sessions at once.
func (h *OrgHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var req updateUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, ok := h.manageable(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	// Validate everything before changing anything.
	var email string
	if req.Email != nil {
		if email, ok = normalizeEmail(*req.Email); !ok {
			writeError(w, http.StatusBadRequest, "enter a valid email address")
			return
		}
		if user.Email != nil && *user.Email == email {
			req.Email = nil
		} else if user.EmailVerifiedAt != nil {
			writeError(w, http.StatusConflict, "this user has verified their email, so it can't be changed")
			return
		}
	}
	if !req.QuotaBytes.valid() {
		writeError(w, http.StatusBadRequest, errQuotaRange)
		return
	}
	if req.Role != nil {
		if !middleware.PrincipalFrom(ctx).IsOwner() {
			writeError(w, http.StatusForbidden, "only the owner can change roles")
			return
		}
		if *req.Role != models.RoleMember && *req.Role != models.RoleAdmin {
			writeError(w, http.StatusBadRequest, "role must be member or admin")
			return
		}
	}

	fail := func(what string, err error) {
		log.Printf("%s: %v", what, err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
	if req.Email != nil {
		if err := h.repo.SetUserEmail(ctx, user.ID, email); err != nil {
			if errors.Is(err, repository.ErrConflict) {
				writeError(w, http.StatusConflict, "an account with this email already exists")
				return
			}
			fail("set user email", err)
			return
		}
		user.Email = &email
		// Any code sent to the old address is replaced, so it stops working.
		if _, err := h.auth.issueCode(ctx, user, auth.PurposeVerifyEmail, true); err != nil {
			log.Printf("issue verification code: %v", err)
		}
	}
	if req.QuotaBytes.Set {
		if err := h.repo.SetUserQuota(ctx, user.ID, req.QuotaBytes.Value); err != nil {
			fail("set user quota", err)
			return
		}
	}
	if req.Role != nil && *req.Role != user.Role {
		if err := h.repo.SetUserRole(ctx, user.ID, *req.Role); err != nil {
			fail("set user role", err)
			return
		}
	}
	if req.Suspended != nil && *req.Suspended != (user.SuspendedAt != nil) {
		if err := h.repo.SetUserSuspended(ctx, user.ID, *req.Suspended); err != nil {
			fail("set user suspended", err)
			return
		}
	}
	h.writeUser(w, r, http.StatusOK, user.ID)
}

// Protected (admins): POST /api/org/users/{id}/password {"password"}. Gives
// the user a new temporary password, signs them out everywhere, and makes
// them choose their own when they next sign in.
func (h *OrgHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user, ok := h.manageable(w, r)
	if !ok {
		return
	}
	if !validPassword(w, req.Password) {
		return
	}
	hash, err := h.auth.authService.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	if err := h.repo.SetTemporaryPassword(r.Context(), user.ID, hash); err != nil {
		log.Printf("set temporary password: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.writeUser(w, r, http.StatusOK, user.ID)
}

// Protected (admins): DELETE /api/org/users/{id}. Their cards go back to the
// organisation and their personal files move to the organisation's files,
// so cards using them keep working.
func (h *OrgHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	user, ok := h.manageable(w, r)
	if !ok {
		return
	}
	if err := h.repo.DeleteOrgUser(r.Context(), user.ID, user.OrgID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		log.Printf("delete org user: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
