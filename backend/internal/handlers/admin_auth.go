package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/mail"
	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
)

// AdminHandler serves the platform admin panel: its own sign-in, usage
// across organisations, suspensions, the feedback inbox and the audit log.
type AdminHandler struct {
	repo        *repository.Repository
	authService *auth.Service
	mailer      mail.Sender
	// replyTo is the Reply-To on emails to users (FEEDBACK_NOTIFY_EMAIL); may be empty.
	replyTo      string
	cookieSecure bool
	dummyHash    string
}

func NewAdminHandler(repo *repository.Repository, authService *auth.Service, mailer mail.Sender, replyTo string, cookieSecure bool) *AdminHandler {
	dummy, err := authService.HashPassword("fronko-admin-timing-equalizer")
	if err != nil {
		log.Fatalf("hashing dummy password: %v", err)
	}
	return &AdminHandler{
		repo: repo, authService: authService, mailer: mailer,
		replyTo: replyTo, cookieSecure: cookieSecure, dummyHash: dummy,
	}
}

// The admin cookie is Strict: the panel never needs it on a cross-site request.
func setAdminCookie(w http.ResponseWriter, token string, secure bool, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.AdminCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// Public: POST /auth/admin/login.
func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	admin, err := h.repo.GetPlatformAdminByEmail(r.Context(), strings.TrimSpace(req.Email))
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			log.Printf("admin login lookup: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		h.authService.CheckPasswordHash(req.Password, h.dummyHash)
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if !h.authService.CheckPasswordHash(req.Password, admin.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := h.authService.GenerateAdminJWT(admin.ID, admin.SessionVersion)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	setAdminCookie(w, token, h.cookieSecure, int(auth.AdminSessionTTL.Seconds()))
	if err := h.repo.TouchAdminLogin(r.Context(), admin.ID); err != nil {
		log.Printf("touch admin login: %v", err)
	}
	h.audit(r, admin, models.AuditAdminLogin, repository.AuditTarget{}, nil)
	writeJSON(w, http.StatusOK, admin)
}

// Public: POST /auth/admin/logout.
func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	setAdminCookie(w, "", h.cookieSecure, -1)
	w.WriteHeader(http.StatusNoContent)
}

// Admin: GET /api/admin/me.
func (h *AdminHandler) Me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, middleware.AdminFrom(r.Context()))
}

// audit records what an admin did. A failure is logged, not shown: the action itself already happened.
func (h *AdminHandler) audit(r *http.Request, admin *models.PlatformAdmin, action string, target repository.AuditTarget, detail map[string]any) {
	if err := h.repo.AddAudit(r.Context(), admin, action, target, detail); err != nil {
		log.Printf("audit %s: %v", action, err)
	}
}
