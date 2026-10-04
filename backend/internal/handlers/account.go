package handlers

import (
	"context"
	"errors"
	"log"
	"math"
	"net/http"
	netmail "net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/mail"
	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
)

// mailSendTimeout bounds one background send, including the SMTP handshake.
const mailSendTimeout = 30 * time.Second

var codePattern = regexp.MustCompile(`^[0-9]{6}$`)

// errInvalidCode is deliberately vague: it doesn't say whether the code was
// wrong, expired, used up, or never issued.
const errInvalidCode = "invalid or expired code"

// normalizeEmail accepts a bare address (no display name) and lower-cases it.
func normalizeEmail(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > 254 {
		return "", false
	}
	addr, err := netmail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@")+1:], ".") {
		return "", false
	}
	return email, true
}

// validPassword writes a 400 and returns false if the password is out of range.
// bcrypt ignores everything past 72 bytes, so longer passwords are rejected outright.
func validPassword(w http.ResponseWriter, password string) bool {
	if len(password) < 8 || len(password) > 72 {
		writeError(w, http.StatusBadRequest, "password must be 8-72 characters")
		return false
	}
	return true
}

// sendAsync delivers mail in the background so a slow SMTP server doesn't hold
// up the request, and so responses take the same time whether or not mail goes out.
func (h *AuthHandler) sendAsync(to string, m mail.Message) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), mailSendTimeout)
		defer cancel()
		if err := h.mailer.Send(ctx, to, m); err != nil {
			log.Printf("sending %q: %v", m.Subject, err)
		}
	}()
}

// issueCode emails the user a fresh code for purpose, replacing any older one.
// Unless force is set, it refuses while the previous code is inside the resend
// cooldown and returns how long is left.
func (h *AuthHandler) issueCode(ctx context.Context, user *models.User, purpose string, force bool) (time.Duration, error) {
	if user.Email == nil {
		return 0, errors.New("user has no email")
	}
	if !force {
		prev, err := h.repo.GetEmailCode(ctx, user.ID, purpose)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return 0, err
		}
		if prev != nil {
			if wait := auth.CodeResendCooldown - time.Since(prev.CreatedAt); wait > 0 {
				return wait, nil
			}
		}
	}

	code, err := auth.NewCode()
	if err != nil {
		return 0, err
	}
	err = h.repo.UpsertEmailCode(ctx, &models.EmailCode{
		UserID:    user.ID,
		Purpose:   purpose,
		CodeHash:  h.authService.HashCode(user.ID, purpose, code),
		ExpiresAt: time.Now().Add(auth.CodeTTL),
	})
	if err != nil {
		return 0, err
	}

	msg := mail.VerifyEmailMessage(code, auth.CodeTTL)
	if purpose == auth.PurposeResetPassword {
		msg = mail.ResetPasswordMessage(code, auth.CodeTTL)
	}
	h.sendAsync(*user.Email, msg)
	return 0, nil
}

// consumeCode checks a code, spending one attempt. A correct code is deleted
// so it can't be used twice.
func (h *AuthHandler) consumeCode(ctx context.Context, userID int64, purpose, code string) (bool, error) {
	code = strings.TrimSpace(code)
	if !codePattern.MatchString(code) {
		return false, nil
	}
	stored, err := h.repo.UseEmailCodeAttempt(ctx, userID, purpose, auth.CodeMaxAttempts)
	if errors.Is(err, repository.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !h.authService.CheckCode(userID, purpose, code, stored.CodeHash) {
		return false, nil
	}
	return true, h.repo.DeleteEmailCode(ctx, userID, purpose)
}

func writeRetryAfter(w http.ResponseWriter, wait time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
	writeError(w, http.StatusTooManyRequests, "please wait before requesting another code")
}

// ----------------------------------------------------------------------------
// Password reset (public)
// ----------------------------------------------------------------------------

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// Public: POST /auth/password/forgot. Always answers 204 so it can't be used
// to find out which emails have accounts. Only verified addresses get a code.
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	email, ok := normalizeEmail(req.Email)
	if !ok {
		writeError(w, http.StatusBadRequest, "enter a valid email address")
		return
	}

	user, err := h.repo.GetUserByEmail(r.Context(), email)
	switch {
	case errors.Is(err, repository.ErrNotFound):
	case err != nil:
		log.Printf("forgot password lookup: %v", err)
	case user.EmailVerifiedAt != nil:
		// Inside the cooldown we quietly skip sending; the earlier code still works.
		if _, err := h.issueCode(r.Context(), user, auth.PurposeResetPassword, false); err != nil {
			log.Printf("issue reset code: %v", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

type ResetPasswordRequest struct {
	Email    string `json:"email"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

// Public: POST /auth/password/reset. Sets a new password with an emailed code
// and signs out every existing session. The client then signs in normally.
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !validPassword(w, req.Password) {
		return
	}
	email, ok := normalizeEmail(req.Email)
	if !ok {
		writeError(w, http.StatusBadRequest, errInvalidCode)
		return
	}

	user, err := h.repo.GetUserByEmail(r.Context(), email)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			log.Printf("reset password lookup: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeError(w, http.StatusBadRequest, errInvalidCode)
		return
	}

	valid, err := h.consumeCode(r.Context(), user.ID, auth.PurposeResetPassword, req.Code)
	if err != nil {
		log.Printf("check reset code: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !valid {
		writeError(w, http.StatusBadRequest, errInvalidCode)
		return
	}

	hash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	if _, err := h.repo.UpdatePassword(r.Context(), user.ID, hash); err != nil {
		log.Printf("reset password: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----------------------------------------------------------------------------
// Email verification (signed in, allowed while unverified)
// ----------------------------------------------------------------------------

type SetEmailRequest struct {
	Email string `json:"email"`
}

// Protected: PUT /api/me/email. Adds an email to an older account or fixes a
// typo before verification, and sends a code to the new address. A verified
// email can't be changed here.
func (h *AuthHandler) SetEmail(w http.ResponseWriter, r *http.Request) {
	var req SetEmailRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	email, ok := normalizeEmail(req.Email)
	if !ok {
		writeError(w, http.StatusBadRequest, "enter a valid email address")
		return
	}

	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if user.EmailVerifiedAt != nil {
		writeError(w, http.StatusConflict, "your email is already verified")
		return
	}

	changed := user.Email == nil || *user.Email != email
	if changed {
		if err := h.repo.SetUserEmail(r.Context(), user.ID, email); err != nil {
			if errors.Is(err, repository.ErrConflict) {
				writeError(w, http.StatusConflict, "an account with this email already exists")
				return
			}
			log.Printf("set email: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		user.Email = &email
	}

	// A new address always gets a new code (the old one went elsewhere);
	// re-submitting the same address respects the cooldown.
	if _, err := h.issueCode(r.Context(), user, auth.PurposeVerifyEmail, changed); err != nil {
		log.Printf("issue verification code: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to send the code")
		return
	}
	writeJSON(w, http.StatusOK, authResponse(user))
}

type VerifyEmailRequest struct {
	Code string `json:"code"`
}

// Protected: POST /api/me/email/verify.
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if user.EmailVerifiedAt != nil {
		writeJSON(w, http.StatusOK, authResponse(user))
		return
	}
	if user.Email == nil {
		writeError(w, http.StatusBadRequest, "add an email address first")
		return
	}

	valid, err := h.consumeCode(r.Context(), user.ID, auth.PurposeVerifyEmail, req.Code)
	if err != nil {
		log.Printf("check verification code: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !valid {
		writeError(w, http.StatusBadRequest, errInvalidCode)
		return
	}

	if err := h.repo.MarkEmailVerified(r.Context(), user.ID); err != nil {
		log.Printf("mark email verified: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	now := time.Now()
	user.EmailVerifiedAt = &now
	writeJSON(w, http.StatusOK, authResponse(user))
}

// Protected: POST /api/me/email/resend. Answers 429 with Retry-After inside the cooldown.
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if user.EmailVerifiedAt != nil {
		writeError(w, http.StatusConflict, "your email is already verified")
		return
	}
	if user.Email == nil {
		writeError(w, http.StatusBadRequest, "add an email address first")
		return
	}

	wait, err := h.issueCode(r.Context(), user, auth.PurposeVerifyEmail, false)
	if err != nil {
		log.Printf("resend verification code: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to send the code")
		return
	}
	if wait > 0 {
		writeRetryAfter(w, wait)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----------------------------------------------------------------------------
// Change password (signed in and verified)
// ----------------------------------------------------------------------------

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Protected: PUT /api/me/password. Signs out every other session; this one
// gets a fresh cookie for the new session version.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req ChangePasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if !h.authService.CheckPasswordHash(req.CurrentPassword, user.PasswordHash) {
		writeError(w, http.StatusBadRequest, "current password is incorrect")
		return
	}
	if !validPassword(w, req.NewPassword) {
		return
	}
	if req.NewPassword == req.CurrentPassword {
		writeError(w, http.StatusBadRequest, "choose a password different from your current one")
		return
	}

	hash, err := h.authService.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	version, err := h.repo.UpdatePassword(r.Context(), middleware.UserID(r.Context()), hash)
	if err != nil {
		log.Printf("change password: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user.SessionVersion = version
	if !h.signIn(w, user) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
