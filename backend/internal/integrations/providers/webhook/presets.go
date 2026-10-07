package webhook

import (
	"context"
	"net/url"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/integrations"
)

// Presets are the webhook provider with a manifest written for one tool:
// its own name, logo and setup steps, and a check that the URL is that
// tool's. They deliver exactly like the generic webhook.

// preset is a webhook preset; hosts, when set, limit the URL to the tool's
// own webhook addresses.
type preset struct {
	*Provider
	hosts []string
	what  string
}

func (p *preset) Validate(_ context.Context, cfg integrations.Settings) error {
	if len(p.hosts) == 0 {
		return nil
	}
	u, err := url.Parse(cfg.String("url"))
	if err != nil {
		return integrations.NewFieldError("url", "enter the webhook URL")
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range p.hosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return nil
		}
	}
	return integrations.NewFieldError("url", "this doesn't look like "+p.what+"; copy it from "+p.M.Name)
}

func urlField(placeholder, help string) integrations.Field {
	return integrations.Field{
		Key: "url", Label: "Webhook URL", Type: integrations.FieldURL, Required: true,
		Placeholder: placeholder, Help: help,
	}
}

const presetDocs = "https://github.com/faiz-gh/fronko/blob/master/docs/integrations/webhook-zapier.md"

// Zapier sends leads to a Zap's "Catch Hook" trigger.
func Zapier() integrations.Provider {
	return &preset{
		Provider: &Provider{M: integrations.Manifest{
			ID:          "zapier",
			Name:        "Zapier",
			Category:    integrations.CategoryLeadSync,
			Description: "Start a Zap with every new lead, and send it on to any of thousands of apps.",
			Scopes:      []integrations.Scope{integrations.ScopeOrg, integrations.ScopeUser},
			Auth:        integrations.AuthNone,
			Status:      integrations.Available,
			Multiple:    true,
			Fields: []integrations.Field{
				urlField("https://hooks.zapier.com/hooks/catch/…", "The address of your Zap's Catch Hook trigger."),
			},
			SetupSteps: []string{
				"In Zapier, create a Zap and choose Webhooks by Zapier → Catch Hook as the trigger.",
				"Copy the webhook URL Zapier shows and paste it below, then save.",
				"Send a test lead, then in Zapier click Test trigger: the lead's fields are under data.",
				"Add the actions you want, such as creating a contact in your CRM, and publish the Zap.",
			},
			DocsURL:  presetDocs,
			Keywords: []string{"webhook", "automation", "zap"},
		}},
		hosts: []string{"hooks.zapier.com"},
		what:  "a Zapier webhook URL (hooks.zapier.com)",
	}
}

// Make sends leads to a Make scenario's custom webhook.
func Make() integrations.Provider {
	return &preset{
		Provider: &Provider{M: integrations.Manifest{
			ID:          "make",
			Name:        "Make",
			Category:    integrations.CategoryLeadSync,
			Description: "Run a Make scenario with every new lead.",
			Scopes:      []integrations.Scope{integrations.ScopeOrg, integrations.ScopeUser},
			Auth:        integrations.AuthNone,
			Status:      integrations.Available,
			Multiple:    true,
			Fields: []integrations.Field{
				urlField("https://hook.eu1.make.com/…", "The address of your scenario's custom webhook."),
			},
			SetupSteps: []string{
				"In Make, create a scenario that starts with Webhooks → Custom webhook, and add a webhook.",
				"Copy its address and paste it below, then save.",
				"In Make, click Run once, then send a test lead here so Make learns the lead's fields.",
				"Add the modules you want and switch the scenario on.",
			},
			DocsURL:  presetDocs,
			Keywords: []string{"webhook", "automation", "integromat"},
		}},
		hosts: []string{"make.com", "integromat.com"},
		what:  "a Make webhook address (hook.….make.com)",
	}
}

// N8n sends leads to an n8n workflow's Webhook node, on n8n Cloud or a
// self-hosted n8n, so any URL is accepted. The signing secret can be
// checked in the workflow.
func N8n() integrations.Provider {
	return &preset{Provider: &Provider{M: integrations.Manifest{
		ID:          "n8n",
		Name:        "n8n",
		Category:    integrations.CategoryLeadSync,
		Description: "Start an n8n workflow with every new lead, on n8n Cloud or your own server.",
		Scopes:      []integrations.Scope{integrations.ScopeOrg, integrations.ScopeUser},
		Auth:        integrations.AuthNone,
		Status:      integrations.Available,
		Multiple:    true,
		Fields: []integrations.Field{
			urlField("https://your-team.app.n8n.cloud/webhook/…",
				"The Production URL of your workflow's Webhook node (HTTP method POST)."),
			{
				Key: "signing_secret", Label: "Signing secret", Type: integrations.FieldSecret, Generate: true,
				Help: "Optional. When set, each request carries an X-Fronko-Signature header your workflow can check. " +
					"Copy it before saving: it isn't shown again.",
			},
		},
		SetupSteps: []string{
			"In n8n, start a workflow with a Webhook node, set its HTTP method to POST, and copy its Production URL.",
			"Paste it below, then save.",
			"Activate the workflow, then send a test lead: the lead is in the body under data.",
		},
		DocsURL:  presetDocs,
		Requires: []integrations.Requirement{integrations.RequiresSecretsKey},
		Keywords: []string{"webhook", "automation", "workflow"},
	}}}
}
