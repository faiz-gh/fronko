package orgs

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
	"github.com/faiz-gh/fronko/backend/internal/platform/mail"
	"github.com/faiz-gh/fronko/backend/internal/teams"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// Protected (admins): GET /api/org/users
func (h *OrgHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.ListOrgUsers(r.Context(), auth.PrincipalFrom(r.Context()).OrgID)
	if err != nil {
		log.Printf("list org users: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, users)
}

type createUserRequest struct {
	Username   string        `json:"username"`
	Email      string        `json:"email"`
	Password   string        `json:"password"`
	Role       string        `json:"role"`
	QuotaBytes optionalQuota `json:"quota_bytes"`
	// Teams to put the new user in, with their role in each.
	Teams []teams.TeamMembership `json:"teams"`
}

// Protected (admins): POST /api/org/users. Creates a user with a temporary
// password they must replace, and emails them their username and that
// password. They verify their email on first sign-in. Only the owner can
// create admins.
func (h *OrgHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req createUserRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !users.ValidUsername(req.Username) {
		httpx.WriteError(w, http.StatusBadRequest, "username must be 3-32 characters: letters, numbers, '.', '_' or '-'")
		return
	}
	email, ok := users.NormalizeEmail(req.Email)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "enter a valid email address")
		return
	}
	if !users.ValidPassword(w, req.Password) {
		return
	}
	switch req.Role {
	case "":
		req.Role = auth.RoleMember
	case auth.RoleMember:
	case auth.RoleAdmin:
		if !p.IsOwner() {
			httpx.WriteError(w, http.StatusForbidden, "only the owner can add admins")
			return
		}
	default:
		httpx.WriteError(w, http.StatusBadRequest, "role must be member or admin")
		return
	}
	if !req.QuotaBytes.valid() {
		httpx.WriteError(w, http.StatusBadRequest, errQuotaRange)
		return
	}
	if len(req.Teams) > teams.MaxTeamsPerUser || !teams.ValidRoles(req.Teams) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid teams")
		return
	}

	org, err := h.store.GetOrganization(r.Context(), p.OrgID)
	if err != nil {
		log.Printf("get organization: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	quota := org.DefaultQuotaBytes
	if req.QuotaBytes.Set {
		quota = req.QuotaBytes.Value
	}

	hash, err := h.auth.HashPassword(req.Password)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	creator := p.UserID
	user := &users.User{
		OrgID:              p.OrgID,
		Role:               req.Role,
		Username:           req.Username,
		Email:              &email,
		PasswordHash:       hash,
		MustChangePassword: true,
		StorageQuotaBytes:  quota,
		CreatedBy:          &creator,
	}
	if err := h.store.CreateMember(r.Context(), user, req.Teams...); err != nil {
		if errors.Is(err, database.ErrConflict) {
			if users.IsEmailConflict(err) {
				httpx.WriteError(w, http.StatusConflict, "an account with this email already exists")
			} else {
				httpx.WriteError(w, http.StatusConflict, "username already taken")
			}
			return
		}
		log.Printf("create org user: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	h.codes.SendAsync(email, mail.MemberInviteMessage(org.Name, user.Username, req.Password))
	h.writeUser(w, r, http.StatusCreated, user.ID)
}

// Protected (admins): GET /api/org/users/{id}
func (h *OrgHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDFromPath(w, r, "user")
	if !ok {
		return
	}
	h.writeUser(w, r, http.StatusOK, id)
}

func (h *OrgHandler) writeUser(w http.ResponseWriter, r *http.Request, status int, userID int64) {
	user, err := h.users.GetOrgUser(r.Context(), userID, auth.PrincipalFrom(r.Context()).OrgID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		log.Printf("get org user: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, status, user)
}

// manageable loads a user the caller may manage, writing the error response
// if they can't: nobody manages the owner or themselves here, and only the
// owner manages admins.
func (h *OrgHandler) manageable(w http.ResponseWriter, r *http.Request) (*users.User, bool) {
	p := auth.PrincipalFrom(r.Context())
	id, ok := httpx.IDFromPath(w, r, "user")
	if !ok {
		return nil, false
	}
	user, err := h.users.GetUserInOrg(r.Context(), id, p.OrgID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "user not found")
			return nil, false
		}
		log.Printf("get org user: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return nil, false
	}
	switch {
	case user.ID == p.UserID:
		httpx.WriteError(w, http.StatusBadRequest, "manage your own account in Settings")
		return nil, false
	case user.Role == auth.RoleOwner:
		httpx.WriteError(w, http.StatusForbidden, "the owner's account can't be changed here")
		return nil, false
	case user.Role == auth.RoleAdmin && !p.IsOwner():
		httpx.WriteError(w, http.StatusForbidden, "only the owner can manage admins")
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
	if !httpx.DecodeJSON(w, r, &req) {
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
		if email, ok = users.NormalizeEmail(*req.Email); !ok {
			httpx.WriteError(w, http.StatusBadRequest, "enter a valid email address")
			return
		}
		if user.Email != nil && *user.Email == email {
			req.Email = nil
		} else if user.EmailVerifiedAt != nil {
			httpx.WriteError(w, http.StatusConflict, "this user has verified their email, so it can't be changed")
			return
		}
	}
	if !req.QuotaBytes.valid() {
		httpx.WriteError(w, http.StatusBadRequest, errQuotaRange)
		return
	}
	if req.Role != nil {
		if !auth.PrincipalFrom(ctx).IsOwner() {
			httpx.WriteError(w, http.StatusForbidden, "only the owner can change roles")
			return
		}
		if *req.Role != auth.RoleMember && *req.Role != auth.RoleAdmin {
			httpx.WriteError(w, http.StatusBadRequest, "role must be member or admin")
			return
		}
	}

	fail := func(what string, err error) {
		log.Printf("%s: %v", what, err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
	}
	if req.Email != nil {
		if err := h.users.SetUserEmail(ctx, user.ID, email); err != nil {
			if errors.Is(err, database.ErrConflict) {
				httpx.WriteError(w, http.StatusConflict, "an account with this email already exists")
				return
			}
			fail("set user email", err)
			return
		}
		user.Email = &email
		// Any code sent to the old address is replaced, so it stops working.
		if _, err := h.codes.Issue(ctx, user, auth.PurposeVerifyEmail, true); err != nil {
			log.Printf("issue verification code: %v", err)
		}
	}
	if req.QuotaBytes.Set {
		if err := h.users.SetUserQuota(ctx, user.ID, req.QuotaBytes.Value); err != nil {
			fail("set user quota", err)
			return
		}
	}
	if req.Role != nil && *req.Role != user.Role {
		if err := h.users.SetUserRole(ctx, user.ID, *req.Role); err != nil {
			fail("set user role", err)
			return
		}
	}
	if req.Suspended != nil && *req.Suspended != (user.SuspendedAt != nil) {
		if err := h.users.SetUserSuspended(ctx, user.ID, *req.Suspended); err != nil {
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
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	user, ok := h.manageable(w, r)
	if !ok {
		return
	}
	if !users.ValidPassword(w, req.Password) {
		return
	}
	hash, err := h.auth.HashPassword(req.Password)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	if err := h.users.SetTemporaryPassword(r.Context(), user.ID, hash); err != nil {
		log.Printf("set temporary password: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
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
	if err := h.users.DeleteOrgUser(r.Context(), user.ID, user.OrgID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		log.Printf("delete org user: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
