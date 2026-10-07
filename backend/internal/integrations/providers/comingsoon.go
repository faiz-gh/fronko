package providers

import "github.com/faiz-gh/fronko/backend/internal/integrations"

// soon lists integrations that are planned but not built yet. They show in
// the catalog as "Coming soon" so people can see what's on the way. When one
// is built, delete its entry here and register the real provider in all.go.
func soon() []integrations.Provider {
	org := []integrations.Scope{integrations.ScopeOrg}
	both := []integrations.Scope{integrations.ScopeOrg, integrations.ScopeUser}
	user := []integrations.Scope{integrations.ScopeUser, integrations.ScopeOrg}
	leadSync := func(id, name, desc string, keywords ...string) integrations.Provider {
		return integrations.Placeholder{M: integrations.Manifest{ID: id, Name: name, Category: integrations.CategoryLeadSync,
			Description: desc, Scopes: org, Auth: integrations.AuthOAuth2, Keywords: append(keywords, "crm")}}
	}
	calendar := func(id, name, desc string) integrations.Provider {
		return integrations.Placeholder{M: integrations.Manifest{ID: id, Name: name, Category: integrations.CategoryCalendar,
			Description: desc, Scopes: user, Auth: integrations.AuthLink, Keywords: []string{"booking", "meeting"}}}
	}
	return []integrations.Provider{
		// Lead Sync
		leadSync("hubspot", "HubSpot", "Create or update a HubSpot contact for every lead, using your own HubSpot app."),
		leadSync("salesforce", "Salesforce", "Create a lead in Salesforce for every lead your cards collect."),
		leadSync("zoho-crm", "Zoho CRM", "Create a lead in Zoho CRM for every lead your cards collect."),
		leadSync("dynamics-365", "Microsoft Dynamics 365", "Create a lead in Dynamics 365 Sales for every lead.", "microsoft"),
		leadSync("pardot", "Salesforce Account Engagement", "Add every lead as a prospect in Account Engagement (Pardot).", "pardot", "marketing"),
		leadSync("pipedrive", "Pipedrive", "Create a person and a lead in Pipedrive for every lead."),
		leadSync("monday", "monday.com", "Add every lead as an item on a monday.com board."),
		leadSync("marketo", "Marketo", "Add every lead to Adobe Marketo Engage.", "adobe", "marketing"),
		integrations.Placeholder{M: integrations.Manifest{ID: "slack", Name: "Slack", Category: integrations.CategoryLeadSync,
			Description: "Post a message to a Slack channel when a lead arrives.", Scopes: both, Auth: integrations.AuthOAuth2,
			Keywords: []string{"chat", "notification"}}},
		integrations.Placeholder{M: integrations.Manifest{ID: "outlook", Name: "Microsoft Outlook", Category: integrations.CategoryLeadSync,
			Description: "Save every lead as a contact in Outlook.", Scopes: user, Auth: integrations.AuthOAuth2,
			Keywords: []string{"microsoft", "contacts", "email"}}},

		// Calendar Booking
		calendar("calendly", "Calendly", "Show your Calendly page on your card, with the visitor's name and email filled in."),
		calendar("chili-piper", "Chili Piper", "Let visitors book time with you through Chili Piper."),
		calendar("microsoft-bookings", "Microsoft Bookings", "Show your Microsoft Bookings page on your card."),
		calendar("hubspot-meetings", "HubSpot Meetings", "Show your HubSpot meetings link on your card."),
		calendar("google-calendar", "Google Calendar", "Show your Google Calendar appointment schedule on your card."),
		calendar("booking-link", "Other booking link", "Any other booking page, such as Cal.com, Zoho Bookings or SavvyCal."),

		// Team Member Import
		integrations.Placeholder{M: integrations.Manifest{ID: "entra-scim", Name: "Microsoft Entra ID", Category: integrations.CategoryDirectory,
			Description: "Provision and deprovision people from Microsoft Entra ID (Azure AD) with SCIM.", Scopes: org,
			Auth: integrations.AuthSCIMToken, Keywords: []string{"azure", "scim", "microsoft"}}},
		integrations.Placeholder{M: integrations.Manifest{ID: "okta-scim", Name: "Okta", Category: integrations.CategoryDirectory,
			Description: "Provision and deprovision people from Okta with SCIM.", Scopes: org,
			Auth: integrations.AuthSCIMToken, Keywords: []string{"scim"}}},
		integrations.Placeholder{M: integrations.Manifest{ID: "google-workspace", Name: "Google Workspace", Category: integrations.CategoryDirectory,
			Description: "Import people from your Google Workspace directory.", Scopes: org,
			Auth: integrations.AuthOAuth2, Keywords: []string{"google", "directory"}}},

		// SAML SSO
		integrations.Placeholder{M: integrations.Manifest{ID: "okta-saml", Name: "Okta", Category: integrations.CategorySSO,
			Description: "Sign in to Fronko with Okta.", Scopes: org, Auth: integrations.AuthSAML, Keywords: []string{"saml", "sso"}}},
		integrations.Placeholder{M: integrations.Manifest{ID: "entra-saml", Name: "Microsoft Entra ID", Category: integrations.CategorySSO,
			Description: "Sign in to Fronko with Microsoft Entra ID (Azure AD).", Scopes: org, Auth: integrations.AuthSAML,
			Keywords: []string{"azure", "saml", "sso", "microsoft"}}},
		integrations.Placeholder{M: integrations.Manifest{ID: "saml", Name: "Other SAML provider", Category: integrations.CategorySSO,
			Description: "Sign in to Fronko with any SAML 2.0 identity provider.", Scopes: org, Auth: integrations.AuthSAML,
			Keywords: []string{"saml", "sso"}}},
	}
}
