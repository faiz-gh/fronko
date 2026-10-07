// Package sso is SAML 2.0 single sign-on: Fronko as the service provider
// for each organisation's identity provider (Okta, Microsoft Entra ID or any
// SAML 2.0 provider), with people found by their email address and, when
// the connection allows, created on their first sign-in.
//
// It also manages organisations' email domains, which route sign-ins by
// email to the right organisation once verified, and the "require single
// sign-on" policy the account module enforces on passwords.
package sso

import "github.com/faiz-gh/fronko/backend/internal/app"

// Routes registers sign-in and the domains API.
func (h *Handler) Routes(r *app.Routes) {
	r.Public("GET /auth/sso/{handle}", h.Start)
	r.Public("POST /auth/sso/discover", r.AuthLimit(h.Discover))
	r.Public("GET /auth/saml/{id}/metadata", h.Metadata)
	// Posted by the identity provider from its own origin.
	r.External("POST /auth/saml/{id}/acs", r.AuthLimit(h.ACS))

	r.Admin("GET /api/org/domains", h.ListDomains)
	r.Admin("POST /api/org/domains", h.AddDomain)
	r.Admin("POST /api/org/domains/{id}/verify", r.AuthLimit(h.VerifyDomain))
	r.Admin("DELETE /api/org/domains/{id}", h.DeleteDomain)
}
