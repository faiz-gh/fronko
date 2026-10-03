package handlers

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)

type AuthHandler struct {
	repo        *repository.Repository
	authService *auth.Service
	// cookieSecure sets the Secure flag on the session cookie.
	cookieSecure bool
	// dummyHash is compared against when a username doesn't exist, so login
	// takes the same time whether or not the account exists.
	dummyHash string
}

func NewAuthHandler(repo *repository.Repository, authService *auth.Service, cookieSecure bool) *AuthHandler {
	dummy, err := authService.HashPassword("fronko-timing-equalizer")
	if err != nil {
		log.Fatalf("hashing dummy password: %v", err)
	}
	return &AuthHandler{
		repo:         repo,
		authService:  authService,
		cookieSecure: cookieSecure,
		dummyHash:    dummy,
	}
}

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse deliberately omits the token: it's only ever sent as an HttpOnly cookie.
type AuthResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)

	user, err := h.repo.GetUserByUsername(r.Context(), req.Username)
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

	token, err := h.authService.GenerateJWT(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	setSessionCookie(w, token, h.cookieSecure)
	writeJSON(w, http.StatusOK, AuthResponse{ID: user.ID, Username: user.Username})
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
	// bcrypt ignores everything past 72 bytes, so reject longer passwords outright.
	if len(req.Password) < 8 || len(req.Password) > 72 {
		writeError(w, http.StatusBadRequest, "password must be 8-72 characters")
		return
	}

	hash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := &models.User{
		Username:     req.Username,
		PasswordHash: hash,
	}

	// The unique index on LOWER(username) is the source of truth for duplicates.
	if err := h.repo.CreateUser(r.Context(), user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			writeError(w, http.StatusConflict, "username already taken")
			return
		}
		log.Printf("create user: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	token, err := h.authService.GenerateJWT(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	setSessionCookie(w, token, h.cookieSecure)
	writeJSON(w, http.StatusCreated, AuthResponse{ID: user.ID, Username: user.Username})
}

// Public: POST /auth/logout. Stateless JWTs can't be revoked server-side, so
// this just drops the cookie; the token itself still expires after SessionTTL.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// Protected: GET /api/me/user. The SPA can't read the HttpOnly cookie, so it
// asks who is signed in.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, err := h.repo.GetUserByID(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Valid token for a deleted account.
			clearSessionCookie(w, h.cookieSecure)
			writeError(w, http.StatusUnauthorized, "account no longer exists")
			return
		}
		log.Printf("get current user: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{ID: user.ID, Username: user.Username})
}
