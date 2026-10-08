package account

import (
	"context"
	"log"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/app"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
)

// Routes registers sign-in, registration and the signed-in user's own
// account endpoints.
func (h *AuthHandler) Routes(r *app.Routes) {
	limit := r.AuthLimit
	r.Public("POST /auth/login", limit(h.Login))
	r.Public("POST /auth/register", limit(h.Register))
	r.Public("POST /auth/password/forgot", limit(h.ForgotPassword))
	r.Public("POST /auth/password/reset", limit(h.ResetPassword))
	r.Public("POST /auth/logout", h.Logout)

	// Usable before the email is verified, so the user can finish verifying.
	r.SignedIn("GET /api/me/user", h.Me)
	r.SignedIn("PUT /api/me/email", limit(h.SetEmail))
	r.SignedIn("POST /api/me/email/verify", limit(h.VerifyEmail))
	r.SignedIn("POST /api/me/email/resend", limit(h.ResendVerification))

	// Choosing a new password must work while the organisation's temporary
	// password is still in place.
	r.Verified("PUT /api/me/password", limit(h.ChangePassword))

	// Deleting the account, or first handing the organisation to someone
	// else. Works with a temporary password or single sign-on only, so
	// nobody is stuck with an account they can't delete.
	r.Verified("GET /api/me/export", limit(h.Export))
	r.Verified("GET /api/me/account/deletion", h.DeletionInfo)
	r.Verified("POST /api/me/account/deletion/code", limit(h.SendDeletionCode))
	r.Verified("POST /api/me/ownership/transfer", limit(h.TransferOwnership))
	r.Verified("DELETE /api/me/account", limit(h.DeleteAccount))

	r.User("POST /api/me/email/change", limit(h.RequestEmailChange))
	r.User("POST /api/me/email/change/confirm", limit(h.ConfirmEmailChange))
}

// Tasks deletes expired one-time codes every hour; a used code is deleted
// straight away, but one that was never used would otherwise stay.
func (h *AuthHandler) Tasks() []jobs.Task {
	return []jobs.Task{{
		Name:  "expired email codes",
		Every: time.Hour,
		Run: func(ctx context.Context) error {
			n, err := h.users.PurgeExpiredCodes(ctx)
			if n > 0 {
				log.Printf("deleted %d expired email codes", n)
			}
			return err
		},
	}}
}
