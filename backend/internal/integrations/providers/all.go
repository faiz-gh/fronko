// Package providers registers every integration provider. Adding one is a
// new folder under providers/ (or a package beside the core for providers
// that serve requests, like sso and directory) and one line in All.
package providers

import (
	"github.com/faiz-gh/fronko/backend/internal/integrations"
	"github.com/faiz-gh/fronko/backend/internal/integrations/directory"
	"github.com/faiz-gh/fronko/backend/internal/integrations/providers/calendar"
	"github.com/faiz-gh/fronko/backend/internal/integrations/providers/hubspot"
	"github.com/faiz-gh/fronko/backend/internal/integrations/providers/webhook"
	"github.com/faiz-gh/fronko/backend/internal/integrations/sso"
)

// All registers every provider with r, in catalog order within each category.
func All(r *integrations.Registry) {
	// Lead Sync
	r.Register(webhook.New())
	r.Register(webhook.Zapier())
	r.Register(webhook.Make())
	r.Register(webhook.N8n())
	r.Register(hubspot.New())

	// Calendar Booking
	for _, p := range calendar.All() {
		r.Register(p)
	}

	// Team Member Import
	r.Register(directory.Entra())

	// SAML SSO
	for _, p := range sso.All() {
		r.Register(p)
	}

	for _, p := range soon() {
		r.Register(p)
	}
}
