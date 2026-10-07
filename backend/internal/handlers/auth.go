package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/mail"
	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)

type AuthHandler struct {
	repo        *repository.Repository
	authService *auth.Service
	mailer      mail.Sender
	// cookieSecure sets the Secure flag on the session cookie.
	cookieSecure bool
	// dummyHash is compared against when a username doesn't exist, so login
	// takes the same time whether or not the account exists.
	dummyHash string
}

func NewAuthHandler(repo *repository.Repository, authService *auth.Service, mailer mail.Sender, cookieSecure bool) *AuthHandler {
	dummy, err := authService.HashPassword("fronko-timing-equalizer")
	if err != nil {
		log.Fatalf("hashing dummy password: %v", err)
	}
	return &AuthHandler{
		repo:         repo,
		authService:  authService,
		mailer:       mailer,
		cookieSecure: cookieSecure,
		dummyHash:    dummy,
	}
}

const maxOrgNameLen = 80

// validOrgName checks an organisation name: 1-80 characters, no control
// characters (it appears in email subjects and bodies).
func validOrgName(name string) bool {
	n := utf8.RuneCountInString(name)
	return n > 0 && n <= maxOrgNameLen && !strings.ContainsFunc(name, unicode.IsControl)
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
	Teams []models.TeamRef `json:"teams"`
}

// authResponse describes the signed-in user, including their organisation's name.
func (h *AuthHandler) authResponse(ctx context.Context, u *models.User) AuthResponse {
	res := AuthResponse{
		ID: u.ID, Username: u.Username, Email: u.Email, EmailVerified: u.EmailVerifiedAt != nil,
		Role: u.Role, MustChangePassword: u.MustChangePassword,
	}
	if org, err := h.repo.GetOrganization(ctx, u.OrgID); err == nil {
		res.OrgName = org.Name
		res.OrgHandle = org.Handle
	} else {
		log.Printf("get organization %d: %v", u.OrgID, err)
	}
	res.Teams = []models.TeamRef{}
	if teams, err := h.repo.ListUserTeams(ctx, u.ID); err == nil {
		res.Teams = teams
	} else {
		log.Printf("list teams of user %d: %v", u.ID, err)
	}
	return res
}

// managedEmail reports whether the signed-in user's email is set by their
// organisation, and if so writes the 403. Only the owner manages their own.
func managedEmail(w http.ResponseWriter, r *http.Request) bool {
	if middleware.PrincipalFrom(r.Context()).IsOwner() {
		return false
	}
	writeError(w, http.StatusForbidden, "your email is managed by your organisation; ask them to change it")
	return true
}

// signIn issues a session for the user's current session version.
func (h *AuthHandler) signIn(w http.ResponseWriter, u *models.User) bool {
	token, err := h.authService.GenerateJWT(u.ID, u.SessionVersion)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return false
	}
	setSessionCookie(w, token, h.cookieSecure)
	return true
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)

	var user *models.User
	var err error
	if strings.Contains(req.Username, "@") {
		user, err = h.repo.GetUserByEmail(r.Context(), req.Username)
	} else {
		user, err = h.repo.GetUserByUsername(r.Context(), req.Username)
	}
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			log.Printf("login lookup: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		h.authService.CheckPasswordHash(req.Password, h.dummyHash)
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if !h.authService.CheckPasswordHash(req.Password, user.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	// Only said after the password checks out, so it doesn't reveal accounts.
	if user.SuspendedAt != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "this account is suspended; contact your organisation",
			"code":  middleware.CodeAccountSuspended,
		})
		return
	}
	org, err := h.repo.GetOrganization(r.Context(), user.OrgID)
	if err != nil {
		log.Printf("login org lookup: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if org.SuspendedAt != nil {
		reason := ""
		if org.SuspendedReason != nil {
			reason = *org.SuspendedReason
		}
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error":  middleware.OrgSuspendedMessage,
			"code":   middleware.CodeOrgSuspended,
			"reason": reason,
		})
		return
	}

	if !h.signIn(w, user) {
		return
	}
	if err := h.repo.TouchLastLogin(r.Context(), user.ID); err != nil {
		log.Printf("touch last login: %v", err)
	}
	// Accounts made by an organisation have no code waiting from registration,
	// so the first sign-in sends one (the resend cooldown still applies).
	if user.CreatedBy != nil && user.EmailVerifiedAt == nil && user.Email != nil {
		if _, err := h.issueCode(r.Context(), user, auth.PurposeVerifyEmail, false); err != nil {
			log.Printf("issue verification code: %v", err)
		}
	}
	writeJSON(w, http.StatusOK, h.authResponse(r.Context(), user))
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
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
	orgName := strings.TrimSpace(req.Organization)
	if orgName == "" {
		orgName = req.Username
	}
	if !validOrgName(orgName) {
		writeError(w, http.StatusBadRequest, "organisation name must be 1-80 characters")
		return
	}

	hash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := &models.User{
		Username:     req.Username,
		Email:        &email,
		PasswordHash: hash,
	}

	// The unique indexes on LOWER(username) and LOWER(email) are the source of truth for duplicates.
	if err := h.repo.CreateOrgWithOwner(r.Context(), orgName, user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			if repository.IsEmailConflict(err) {
				writeError(w, http.StatusConflict, "an account with this email already exists")
			} else {
				writeError(w, http.StatusConflict, "username already taken")
			}
			return
		}
		log.Printf("create user: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// The account exists either way; if this fails the user can resend.
	if _, err := h.issueCode(r.Context(), user, auth.PurposeVerifyEmail, true); err != nil {
		log.Printf("issue verification code: %v", err)
	}

	if !h.signIn(w, user) {
		return
	}
	writeJSON(w, http.StatusCreated, h.authResponse(r.Context(), user))
}

// Public: POST /auth/logout. Stateless JWTs can't be revoked server-side, so
// this just drops the cookie; the token itself still expires after SessionTTL.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// Protected (allowed while unverified): GET /api/me/user. The SPA can't read
// the HttpOnly cookie, so it asks who is signed in.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.authResponse(r.Context(), user))
}

// currentUser loads the signed-in user, writing an error response if it can't.
func (h *AuthHandler) currentUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	user, err := h.repo.GetUserByID(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Deleted between the middleware's check and now.
			clearSessionCookie(w, h.cookieSecure)
			writeError(w, http.StatusUnauthorized, "account no longer exists")
			return nil, false
		}
		log.Printf("get current user: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return nil, false
	}
	return user, true
}
