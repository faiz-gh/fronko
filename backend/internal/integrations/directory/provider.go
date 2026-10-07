package directory

import (
	"context"

	"github.com/faiz-gh/fronko/backend/internal/integrations"
)

// Provider is an identity provider that provisions people over SCIM. The
// SCIM API is the same for all of them; each has its own setup steps.
type Provider struct{ m integrations.Manifest }

func (p *Provider) Manifest() integrations.Manifest { return p.m }

// Validate has nothing to check: the connection has no settings, only a token.
func (p *Provider) Validate(context.Context, integrations.Settings) error { return nil }

// Endpoints are what the identity provider needs: the SCIM base URL. The
// token is generated separately.
func (p *Provider) Endpoints(_ *integrations.Connection, env integrations.Env) []integrations.Endpoint {
	if env.APIURL == "" {
		return nil
	}
	return []integrations.Endpoint{{
		Key: "tenant_url", Label: "Tenant URL (SCIM base URL)", Value: env.APIURL + BasePath,
		Help: "Where the identity provider sends people and groups.",
	}}
}

// Entra is Microsoft Entra ID (Azure AD) provisioning.
func Entra() *Provider {
	return &Provider{m: integrations.Manifest{
		ID:       "entra-scim",
		Name:     "Microsoft Entra ID",
		Category: integrations.CategoryDirectory,
		Description: "Add, update and remove people automatically from Microsoft Entra ID (Azure AD) with SCIM, " +
			"and keep teams in step with groups.",
		Scopes: []integrations.Scope{integrations.ScopeOrg},
		Auth:   integrations.AuthSCIMToken,
		Status: integrations.Available,
		Fields: []integrations.Field{},
		SetupSteps: []string{
			"Save this connection, then generate a secret token below and copy it.",
			"In the Microsoft Entra admin center, go to Enterprise applications → New application → Create your own application, and choose a non-gallery app.",
			"In the app, open Provisioning, set the mode to Automatic, and paste the Tenant URL and Secret token shown here. Click Test Connection, then Save.",
			"Under Mappings, keep Provision Microsoft Entra ID Users (userPrincipalName → userName, mail → emails[type eq \"work\"].value) and, for teams, Provision Microsoft Entra ID Groups.",
			"Assign the people and groups who should be in Fronko under Users and groups, then Start provisioning.",
		},
		DocsURL:  "https://github.com/faiz-gh/fronko/blob/master/docs/integrations/entra-scim.md",
		Requires: []integrations.Requirement{integrations.RequiresPublicURL},
		Keywords: []string{"azure", "azure ad", "scim", "microsoft", "provisioning", "users"},
	}}
}
