package account

import "github.com/faiz-gh/fronko/backend/internal/app"

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

	r.User("POST /api/me/email/change", limit(h.RequestEmailChange))
	r.User("POST /api/me/email/change/confirm", limit(h.ConfirmEmailChange))
}
