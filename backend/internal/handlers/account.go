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

// issueCode emails the user a fresh code for purpose at their current
// address, replacing any older one. See issueCodeTo.
func (h *AuthHandler) issueCode(ctx context.Context, user *models.User, purpose string, force bool) (time.Duration, error) {
	if user.Email == nil {
		return 0, errors.New("user has no email")
	}
	return h.issueCodeTo(ctx, user.ID, *user.Email, purpose, force)
}

// issueCodeTo emails a fresh code for purpose to the given address, replacing
// any older one. Unless force is set, it refuses while the previous code to the
// same address is inside the resend cooldown and returns how long is left.
// A change_email code remembers the address it was sent to.
func (h *AuthHandler) issueCodeTo(ctx context.Context, userID int64, to, purpose string, force bool) (time.Duration, error) {
	if !force {
		prev, err := h.repo.GetEmailCode(ctx, userID, purpose)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return 0, err
		}
		if prev != nil && (prev.Email == nil || *prev.Email == to) {
			if wait := auth.CodeResendCooldown - time.Since(prev.CreatedAt); wait > 0 {
				return wait, nil
			}
		}
	}

	code, err := auth.NewCode()
	if err != nil {
		return 0, err
	}
	stored := &models.EmailCode{
		UserID:    userID,
		Purpose:   purpose,
		CodeHash:  h.authService.HashCode(userID, purpose, code),
		ExpiresAt: time.Now().Add(auth.CodeTTL),
	}
	if purpose == auth.PurposeChangeEmail {
		stored.Email = &to
	}
	if err := h.repo.UpsertEmailCode(ctx, stored); err != nil {
		return 0, err
	}

	var msg mail.Message
	switch purpose {
	case auth.PurposeResetPassword:
		msg = mail.ResetPasswordMessage(code, auth.CodeTTL)
	case auth.PurposeChangeEmail:
		msg = mail.ChangeEmailMessage(code, auth.CodeTTL)
	default:
		msg = mail.VerifyEmailMessage(code, auth.CodeTTL)
	}
	h.sendAsync(to, msg)
	return 0, nil
}

// consumeCode checks a code, spending one attempt. A correct code is deleted
// so it can't be used twice.
func (h *AuthHandler) consumeCode(ctx context.Context, userID int64, purpose, code string) (bool, error) {
	stored, err := h.consumeCodeRow(ctx, userID, purpose, code)
	return stored != nil, err
}

// consumeCodeRow is consumeCode, returning the matched code (nil if invalid).
func (h *AuthHandler) consumeCodeRow(ctx context.Context, userID int64, purpose, code string) (*models.EmailCode, error) {
	code = strings.TrimSpace(code)
	if !codePattern.MatchString(code) {
		return nil, nil
	}
	stored, err := h.repo.UseEmailCodeAttempt(ctx, userID, purpose, auth.CodeMaxAttempts)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !h.authService.CheckCode(userID, purpose, code, stored.CodeHash) {
		return nil, nil
	}
	return stored, h.repo.DeleteEmailCode(ctx, userID, purpose)
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
	if managedEmail(w, r) {
		return
	}
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
	writeJSON(w, http.StatusOK, h.authResponse(r.Context(), user))
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
		writeJSON(w, http.StatusOK, h.authResponse(r.Context(), user))
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
	writeJSON(w, http.StatusOK, h.authResponse(r.Context(), user))
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
// Change a verified email (signed in and verified)
// ----------------------------------------------------------------------------

type ChangeEmailRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Protected: POST /api/me/email/change. Checks the password and sends a code
// to the new address. The current email stays in force until the code is
// confirmed. Re-requesting the same address inside the cooldown answers 429.
func (h *AuthHandler) RequestEmailChange(w http.ResponseWriter, r *http.Request) {
	if managedEmail(w, r) {
		return
	}
	var req ChangeEmailRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if !h.authService.CheckPasswordHash(req.Password, user.PasswordHash) {
		writeError(w, http.StatusBadRequest, "password is incorrect")
		return
	}
	email, ok := normalizeEmail(req.Email)
	if !ok {
		writeError(w, http.StatusBadRequest, "enter a valid email address")
		return
	}
	if user.Email != nil && *user.Email == email {
		writeError(w, http.StatusBadRequest, "that's already your email")
		return
	}

	// Checked again by the unique index when the change is confirmed.
	switch _, err := h.repo.GetUserByEmail(r.Context(), email); {
	case err == nil:
		writeError(w, http.StatusConflict, "an account with this email already exists")
		return
	case !errors.Is(err, repository.ErrNotFound):
		log.Printf("email change lookup: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	wait, err := h.issueCodeTo(r.Context(), user.ID, email, auth.PurposeChangeEmail, false)
	if err != nil {
		log.Printf("issue email change code: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to send the code")
		return
	}
	if wait > 0 {
		writeRetryAfter(w, wait)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type ConfirmEmailChangeRequest struct {
	Code string `json:"code"`
}

// Protected: POST /api/me/email/change/confirm. Switches to the address the
// code was sent to (already verified by the code itself) and notifies the old one.
func (h *AuthHandler) ConfirmEmailChange(w http.ResponseWriter, r *http.Request) {
	if managedEmail(w, r) {
		return
	}
	var req ConfirmEmailChangeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	stored, err := h.consumeCodeRow(r.Context(), user.ID, auth.PurposeChangeEmail, req.Code)
	if err != nil {
		log.Printf("check email change code: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if stored == nil || stored.Email == nil {
		writeError(w, http.StatusBadRequest, errInvalidCode)
		return
	}

	newEmail := *stored.Email
	if err := h.repo.ChangeUserEmail(r.Context(), user.ID, newEmail); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			writeError(w, http.StatusConflict, "an account with this email already exists")
			return
		}
		log.Printf("change email: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if user.Email != nil && *user.Email != newEmail {
		h.sendAsync(*user.Email, mail.EmailChangedNotice(newEmail))
	}
	now := time.Now()
	user.Email, user.EmailVerifiedAt = &newEmail, &now
	writeJSON(w, http.StatusOK, h.authResponse(r.Context(), user))
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
