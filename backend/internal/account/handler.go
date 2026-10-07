package account

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/orgs"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
	"github.com/faiz-gh/fronko/backend/internal/teams"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// AuthHandler signs people in and out, registers organisations, and runs the
// self-service account flows (email verification and change, passwords).
type AuthHandler struct {
	users       *users.Store
	orgs        *orgs.Store
	teams       *teams.Store
	codes       *users.Codes
	authService *auth.Service
	// cookieSecure sets the Secure flag on the session cookie.
	cookieSecure bool
	// dummyHash is compared against when a username doesn't exist, so login
	// takes the same time whether or not the account exists.
	dummyHash string
}

func NewAuthHandler(userStore *users.Store, orgStore *orgs.Store, teamStore *teams.Store, codes *users.Codes, authService *auth.Service, cookieSecure bool) *AuthHandler {
	dummy, err := authService.HashPassword("fronko-timing-equalizer")
	if err != nil {
		log.Fatalf("hashing dummy password: %v", err)
	}
	return &AuthHandler{
		users:        userStore,
		orgs:         orgStore,
		teams:        teamStore,
		codes:        codes,
		authService:  authService,
		cookieSecure: cookieSecure,
		dummyHash:    dummy,
	}
}

type AuthRequest struct {
	// Username also accepts an email address on login.
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	// Organization names the organisation a registration creates; it defaults to the username.
	Organization string `json:"organization"`
}

// AuthResponse deliberately omits the token: it's only ever sent as an HttpOnly cookie.
type AuthResponse struct {
	ID            int64   `json:"id"`
	Username      string  `json:"username"`
	Email         *string `json:"email"`
	EmailVerified bool    `json:"email_verified"`
	Role          string  `json:"role"`
	OrgName       string  `json:"org_name"`
	// OrgHandle is the organisation's part of every card link, /p/{org_handle}/{slug}.
	OrgHandle          string `json:"org_handle"`
	MustChangePassword bool   `json:"must_change_password"`
	// Teams the user is in, with their role in each.
	Teams []auth.TeamRef `json:"teams"`
}

// authResponse describes the signed-in user, including their organisation's name.
func (h *AuthHandler) authResponse(ctx context.Context, u *users.User) AuthResponse {
	res := AuthResponse{
		ID: u.ID, Username: u.Username, Email: u.Email, EmailVerified: u.EmailVerifiedAt != nil,
		Role: u.Role, MustChangePassword: u.MustChangePassword,
	}
	if org, err := h.orgs.GetOrganization(ctx, u.OrgID); err == nil {
		res.OrgName = org.Name
		res.OrgHandle = org.Handle
	} else {
		log.Printf("get organization %d: %v", u.OrgID, err)
	}
	res.Teams = []auth.TeamRef{}
	if teams, err := h.teams.ListUserTeams(ctx, u.ID); err == nil {
		res.Teams = teams
	} else {
		log.Printf("list teams of user %d: %v", u.ID, err)
	}
	return res
}

// managedEmail reports whether the signed-in user's email is set by their
// organisation, and if so writes the 403. Only the owner manages their own.
func managedEmail(w http.ResponseWriter, r *http.Request) bool {
	if auth.PrincipalFrom(r.Context()).IsOwner() {
		return false
	}
	httpx.WriteError(w, http.StatusForbidden, "your email is managed by your organisation; ask them to change it")
	return true
}

// signIn issues a session for the user's current session version.
func (h *AuthHandler) signIn(w http.ResponseWriter, u *users.User) bool {
	token, err := h.authService.GenerateJWT(u.ID, u.SessionVersion)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to generate token")
		return false
	}
	auth.SetSessionCookie(w, token, h.cookieSecure)
	return true
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)

	var user *users.User
	var err error
	if strings.Contains(req.Username, "@") {
		user, err = h.users.GetUserByEmail(r.Context(), req.Username)
	} else {
		user, err = h.users.GetUserByUsername(r.Context(), req.Username)
	}
	if err != nil {
		if !errors.Is(err, database.ErrNotFound) {
			log.Printf("login lookup: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}
		h.authService.CheckPasswordHash(req.Password, h.dummyHash)
		httpx.WriteError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if !h.authService.CheckPasswordHash(req.Password, user.PasswordHash) {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	// Only said after the password checks out, so it doesn't reveal accounts.
	if user.SuspendedAt != nil {
		httpx.WriteJSON(w, http.StatusForbidden, map[string]string{
			"error": "this account is suspended; contact your organisation",
			"code":  auth.CodeAccountSuspended,
		})
		return
	}
	org, err := h.orgs.GetOrganization(r.Context(), user.OrgID)
	if err != nil {
		log.Printf("login org lookup: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if org.SuspendedAt != nil {
		reason := ""
		if org.SuspendedReason != nil {
			reason = *org.SuspendedReason
		}
		httpx.WriteJSON(w, http.StatusForbidden, map[string]string{
			"error":  auth.OrgSuspendedMessage,
			"code":   auth.CodeOrgSuspended,
			"reason": reason,
		})
		return
	}

	if !h.signIn(w, user) {
		return
	}
	if err := h.users.TouchLastLogin(r.Context(), user.ID); err != nil {
		log.Printf("touch last login: %v", err)
	}
	// Accounts made by an organisation have no code waiting from registration,
	// so the first sign-in sends one (the resend cooldown still applies).
	if user.CreatedBy != nil && user.EmailVerifiedAt == nil && user.Email != nil {
		if _, err := h.codes.Issue(r.Context(), user, auth.PurposeVerifyEmail, false); err != nil {
			log.Printf("issue verification code: %v", err)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, h.authResponse(r.Context(), user))
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
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
	orgName := strings.TrimSpace(req.Organization)
	if orgName == "" {
		orgName = req.Username
	}
	if !orgs.ValidName(orgName) {
		httpx.WriteError(w, http.StatusBadRequest, "organisation name must be 1-80 characters")
		return
	}

	hash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := &users.User{
		Username:     req.Username,
		Email:        &email,
		PasswordHash: hash,
	}

	// The unique indexes on LOWER(username) and LOWER(email) are the source of truth for duplicates.
	if err := h.orgs.CreateOrgWithOwner(r.Context(), orgName, user); err != nil {
		if errors.Is(err, database.ErrConflict) {
			if users.IsEmailConflict(err) {
				httpx.WriteError(w, http.StatusConflict, "an account with this email already exists")
			} else {
				httpx.WriteError(w, http.StatusConflict, "username already taken")
			}
			return
		}
		log.Printf("create user: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// The account exists either way; if this fails the user can resend.
	if _, err := h.codes.Issue(r.Context(), user, auth.PurposeVerifyEmail, true); err != nil {
		log.Printf("issue verification code: %v", err)
	}

	if !h.signIn(w, user) {
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, h.authResponse(r.Context(), user))
}

// Public: POST /auth/logout. Stateless JWTs can't be revoked server-side, so
// this just drops the cookie; the token itself still expires after SessionTTL.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// Protected (allowed while unverified): GET /api/me/user. The SPA can't read
// the HttpOnly cookie, so it asks who is signed in.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.authResponse(r.Context(), user))
}

// currentUser loads the signed-in user, writing an error response if it can't.
func (h *AuthHandler) currentUser(w http.ResponseWriter, r *http.Request) (*users.User, bool) {
	user, err := h.users.GetUserByID(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			// Deleted between the middleware's check and now.
			auth.ClearSessionCookie(w, h.cookieSecure)
			httpx.WriteError(w, http.StatusUnauthorized, "account no longer exists")
			return nil, false
		}
		log.Printf("get current user: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return nil, false
	}
	return user, true
}
