package account

import (
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
	"github.com/faiz-gh/fronko/backend/internal/platform/mail"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// errInvalidCode is deliberately vague: it doesn't say whether the code was
// wrong, expired, used up, or never issued.
const errInvalidCode = "invalid or expired code"

func writeRetryAfter(w http.ResponseWriter, wait time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
	httpx.WriteError(w, http.StatusTooManyRequests, "please wait before requesting another code")
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// Public: POST /auth/password/forgot. Always answers 204 so it can't be used
// to find out which emails have accounts. Only verified addresses get a code.
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	email, ok := users.NormalizeEmail(req.Email)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "enter a valid email address")
		return
	}

	user, err := h.users.GetUserByEmail(r.Context(), email)
	switch {
	case errors.Is(err, database.ErrNotFound):
	case err != nil:
		log.Printf("forgot password lookup: %v", err)
	case user.EmailVerifiedAt != nil:
		// People who must use single sign-on get no code: a password
		// wouldn't let them in.
		if _, blocked, err := h.sso.PasswordBlocked(r.Context(), user); err != nil || blocked {
			if err != nil {
				log.Printf("forgot password sso policy: %v", err)
			}
			break
		}
		// Inside the cooldown we quietly skip sending; the earlier code still works.
		if _, err := h.codes.Issue(r.Context(), user, auth.PurposeResetPassword, false); err != nil {
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
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if !users.ValidPassword(w, req.Password) {
		return
	}
	email, ok := users.NormalizeEmail(req.Email)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, errInvalidCode)
		return
	}

	user, err := h.users.GetUserByEmail(r.Context(), email)
	if err != nil {
		if !errors.Is(err, database.ErrNotFound) {
			log.Printf("reset password lookup: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, errInvalidCode)
		return
	}

	if path, blocked, err := h.sso.PasswordBlocked(r.Context(), user); err != nil || blocked {
		if err != nil {
			log.Printf("reset password sso policy: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeSSORequired(w, path)
		return
	}
	valid, err := h.codes.Consume(r.Context(), user.ID, auth.PurposeResetPassword, req.Code)
	if err != nil {
		log.Printf("check reset code: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !valid {
		httpx.WriteError(w, http.StatusBadRequest, errInvalidCode)
		return
	}

	hash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	if _, err := h.users.UpdatePassword(r.Context(), user.ID, hash); err != nil {
		log.Printf("reset password: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

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
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	email, ok := users.NormalizeEmail(req.Email)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "enter a valid email address")
		return
	}

	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if user.EmailVerifiedAt != nil {
		httpx.WriteError(w, http.StatusConflict, "your email is already verified")
		return
	}

	changed := user.Email == nil || *user.Email != email
	if changed {
		if err := h.users.SetUserEmail(r.Context(), user.ID, email); err != nil {
			if errors.Is(err, database.ErrConflict) {
				httpx.WriteError(w, http.StatusConflict, "an account with this email already exists")
				return
			}
			log.Printf("set email: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}
		user.Email = &email
	}

	// A new address always gets a new code (the old one went elsewhere);
	// re-submitting the same address respects the cooldown.
	if _, err := h.codes.Issue(r.Context(), user, auth.PurposeVerifyEmail, changed); err != nil {
		log.Printf("issue verification code: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to send the code")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.authResponse(r.Context(), user))
}

type VerifyEmailRequest struct {
	Code string `json:"code"`
}

// Protected: POST /api/me/email/verify.
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if user.EmailVerifiedAt != nil {
		httpx.WriteJSON(w, http.StatusOK, h.authResponse(r.Context(), user))
		return
	}
	if user.Email == nil {
		httpx.WriteError(w, http.StatusBadRequest, "add an email address first")
		return
	}

	valid, err := h.codes.Consume(r.Context(), user.ID, auth.PurposeVerifyEmail, req.Code)
	if err != nil {
		log.Printf("check verification code: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !valid {
		httpx.WriteError(w, http.StatusBadRequest, errInvalidCode)
		return
	}

	if err := h.users.MarkEmailVerified(r.Context(), user.ID); err != nil {
		log.Printf("mark email verified: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	now := time.Now()
	user.EmailVerifiedAt = &now
	httpx.WriteJSON(w, http.StatusOK, h.authResponse(r.Context(), user))
}

// Protected: POST /api/me/email/resend. Answers 429 with Retry-After inside the cooldown.
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if user.EmailVerifiedAt != nil {
		httpx.WriteError(w, http.StatusConflict, "your email is already verified")
		return
	}
	if user.Email == nil {
		httpx.WriteError(w, http.StatusBadRequest, "add an email address first")
		return
	}

	wait, err := h.codes.Issue(r.Context(), user, auth.PurposeVerifyEmail, false)
	if err != nil {
		log.Printf("resend verification code: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to send the code")
		return
	}
	if wait > 0 {
		writeRetryAfter(w, wait)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

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
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if !h.authService.CheckPasswordHash(req.Password, user.PasswordHash) {
		httpx.WriteError(w, http.StatusBadRequest, "password is incorrect")
		return
	}
	email, ok := users.NormalizeEmail(req.Email)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "enter a valid email address")
		return
	}
	if user.Email != nil && *user.Email == email {
		httpx.WriteError(w, http.StatusBadRequest, "that's already your email")
		return
	}

	// Checked again by the unique index when the change is confirmed.
	switch _, err := h.users.GetUserByEmail(r.Context(), email); {
	case err == nil:
		httpx.WriteError(w, http.StatusConflict, "an account with this email already exists")
		return
	case !errors.Is(err, database.ErrNotFound):
		log.Printf("email change lookup: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	wait, err := h.codes.IssueTo(r.Context(), user.ID, email, auth.PurposeChangeEmail, false)
	if err != nil {
		log.Printf("issue email change code: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to send the code")
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
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	stored, err := h.codes.ConsumeRow(r.Context(), user.ID, auth.PurposeChangeEmail, req.Code)
	if err != nil {
		log.Printf("check email change code: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if stored == nil || stored.Email == nil {
		httpx.WriteError(w, http.StatusBadRequest, errInvalidCode)
		return
	}

	newEmail := *stored.Email
	if err := h.users.ChangeUserEmail(r.Context(), user.ID, newEmail); err != nil {
		if errors.Is(err, database.ErrConflict) {
			httpx.WriteError(w, http.StatusConflict, "an account with this email already exists")
			return
		}
		log.Printf("change email: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if user.Email != nil && *user.Email != newEmail {
		h.codes.SendAsync(*user.Email, mail.EmailChangedNotice(newEmail))
	}
	now := time.Now()
	user.Email, user.EmailVerifiedAt = &newEmail, &now
	httpx.WriteJSON(w, http.StatusOK, h.authResponse(r.Context(), user))
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Protected: PUT /api/me/password. Signs out every other session; this one
// gets a fresh cookie for the new session version.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req ChangePasswordRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if !h.authService.CheckPasswordHash(req.CurrentPassword, user.PasswordHash) {
		httpx.WriteError(w, http.StatusBadRequest, "current password is incorrect")
		return
	}
	if !users.ValidPassword(w, req.NewPassword) {
		return
	}
	if req.NewPassword == req.CurrentPassword {
		httpx.WriteError(w, http.StatusBadRequest, "choose a password different from your current one")
		return
	}

	hash, err := h.authService.HashPassword(req.NewPassword)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	version, err := h.users.UpdatePassword(r.Context(), auth.UserID(r.Context()), hash)
	if err != nil {
		log.Printf("change password: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user.SessionVersion = version
	if !h.signIn(w, user) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
