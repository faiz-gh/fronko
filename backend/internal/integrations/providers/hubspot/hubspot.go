// Package hubspot creates or updates a HubSpot contact for every lead, using
// the organisation's own HubSpot app (OAuth).
//
// The field mapping is fixed:
//
//	lead name   → firstname (first word) and lastname (the rest)
//	lead email  → email (the contact is matched on it, so repeats update it)
//	lead phone  → phone
//	everything else (card, source, message) → a note on the contact
package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/faiz-gh/fronko/backend/internal/integrations"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
)

// HubSpot's endpoints.
const (
	defaultAuthURL  = "https://app.hubspot.com/oauth/authorize"
	defaultTokenURL = "https://api.hubapi.com/oauth/v1/token"
	defaultAPIBase  = "https://api.hubapi.com"
)

// Scopes the app must have.
var Scopes = []string{"oauth", "crm.objects.contacts.read", "crm.objects.contacts.write"}

// noteToContact is HubSpot's association type for a note on a contact.
const noteToContact = 202

// Provider is the HubSpot provider. The URLs are HubSpot's; tests point
// them at a fake.
type Provider struct {
	AuthURL  string
	TokenURL string
	APIBase  string
}

func New() *Provider {
	return &Provider{AuthURL: defaultAuthURL, TokenURL: defaultTokenURL, APIBase: defaultAPIBase}
}

func (p *Provider) Manifest() integrations.Manifest {
	return integrations.Manifest{
		ID:       "hubspot",
		Name:     "HubSpot",
		Category: integrations.CategoryLeadSync,
		Description: "Create or update a HubSpot contact for every lead, matched on email, with a note saying " +
			"which card it came from.",
		Scopes:   []integrations.Scope{integrations.ScopeOrg},
		Auth:     integrations.AuthOAuth2,
		Status:   integrations.Available,
		Multiple: false,
		Fields: []integrations.Field{
			{Key: "client_id", Label: "Client ID", Type: integrations.FieldText, Required: true,
				Placeholder: "e.g. 1a2b3c4d-…", Help: "From your HubSpot app's Auth settings."},
			{Key: "client_secret", Label: "Client secret", Type: integrations.FieldSecret, Required: true,
				Help: "From the same page. It's stored encrypted and never shown again."},
			{Key: "add_note", Label: "Add a note to the contact", Type: integrations.FieldBool, Default: true,
				Help: "Records the card, how the visitor found it and their message on the contact's timeline."},
		},
		SetupSteps: []string{
			"In your HubSpot developer account, create an app (Apps → Create app).",
			"Under Auth, add the redirect URL shown below, and the scopes oauth, crm.objects.contacts.read and crm.objects.contacts.write.",
			"Copy the app's client ID and client secret here, and save.",
			"Click Authorise with HubSpot and choose the HubSpot account that should receive leads.",
			"Send a test lead: a contact called Test Lead (test.lead@example.com) appears in HubSpot. You can delete it.",
		},
		DocsURL:  "https://github.com/faiz-gh/fronko/blob/master/docs/integrations/hubspot.md",
		Requires: []integrations.Requirement{integrations.RequiresPublicURL, integrations.RequiresSecretsKey},
		Keywords: []string{"crm", "contacts", "marketing"},
	}
}

func (p *Provider) Validate(_ context.Context, cfg integrations.Settings) error {
	if id := cfg.String("client_id"); id != "" && strings.ContainsAny(id, " /?#") {
		return integrations.NewFieldError("client_id", "the client ID doesn't look right")
	}
	return nil
}

func (p *Provider) OAuth(integrations.Settings) integrations.OAuthSpec {
	return integrations.OAuthSpec{
		AuthURL:   p.AuthURL,
		TokenURL:  p.TokenURL,
		Scopes:    Scopes,
		AuthStyle: oauth2.AuthStyleInParams,
	}
}

type upsertRequest struct {
	Inputs []upsertInput `json:"inputs"`
}

type upsertInput struct {
	IDProperty string            `json:"idProperty"`
	ID         string            `json:"id"`
	Properties map[string]string `json:"properties"`
}

type upsertResponse struct {
	Results []struct {
		ID  string `json:"id"`
		New bool   `json:"new"`
	} `json:"results"`
}

// Properties are the contact properties a lead sets.
func Properties(lead *integrations.Lead) map[string]string {
	first, last, _ := strings.Cut(strings.TrimSpace(lead.Name), " ")
	props := map[string]string{
		"email":     lead.Email,
		"firstname": first,
		"lastname":  strings.TrimSpace(last),
	}
	if lead.Phone != "" {
		props["phone"] = lead.Phone
	}
	return props
}

// PushLead upserts the contact by email, then adds the note.
func (p *Provider) PushLead(ctx context.Context, call *integrations.Call, lead *integrations.Lead) (integrations.Result, error) {
	body := upsertRequest{Inputs: []upsertInput{{IDProperty: "email", ID: lead.Email, Properties: Properties(lead)}}}
	var out upsertResponse
	status, err := p.do(ctx, call.HTTP, "/crm/v3/objects/contacts/batch/upsert", body, &out)
	if err != nil {
		return integrations.Result{}, err
	}
	if len(out.Results) == 0 {
		return integrations.Result{}, integrations.WithDetail(errors.New("HubSpot didn't return the contact"), map[string]any{"status": status})
	}
	contact := out.Results[0]
	verb := "Updated"
	if contact.New {
		verb = "Created"
	}
	res := integrations.Result{
		Summary: fmt.Sprintf("%s contact %s", verb, lead.Email),
		Detail:  map[string]any{"status": status, "contact_id": contact.ID},
	}
	if v, set := call.Settings.Values["add_note"]; !set || v == true {
		if _, err := p.do(ctx, call.HTTP, "/crm/v3/objects/notes", noteFor(lead, contact.ID), nil); err != nil {
			// The contact is what matters; a missing note isn't worth a retry
			// that would upsert the contact again.
			res.Summary += " (the note couldn't be added)"
			res.Detail["note_error"] = err.Error()
		} else {
			res.Detail["note"] = "added"
		}
	}
	return res, nil
}

func noteFor(lead *integrations.Lead, contactID string) map[string]any {
	var b strings.Builder
	b.WriteString("<p><strong>New lead from Fronko</strong></p><p>")
	fmt.Fprintf(&b, "Card: %s", htmlEscape(lead.Card.Name))
	if lead.Card.URL != "" {
		fmt.Fprintf(&b, ` (<a href="%s">%s</a>)`, htmlEscape(lead.Card.URL), htmlEscape(lead.Card.URL))
	}
	if via := map[string]string{"nfc": "tapped the NFC card", "qr": "scanned the QR code", "link": "opened the link"}[lead.Source]; via != "" {
		fmt.Fprintf(&b, "<br>The visitor %s.", via)
	}
	if lead.Owner != nil {
		fmt.Fprintf(&b, "<br>Card holder: %s", htmlEscape(lead.Owner.Username))
	}
	b.WriteString("</p>")
	if notes := strings.TrimSpace(lead.Notes); notes != "" {
		b.WriteString("<p>" + strings.ReplaceAll(htmlEscape(notes), "\n", "<br>") + "</p>")
	}
	return map[string]any{
		"properties": map[string]string{
			"hs_timestamp": lead.CreatedAt.UTC().Format(time.RFC3339),
			"hs_note_body": b.String(),
		},
		"associations": []map[string]any{{
			"to":    map[string]string{"id": contactID},
			"types": []map[string]any{{"associationCategory": "HUBSPOT_DEFINED", "associationTypeId": noteToContact}},
		}},
	}
}

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func htmlEscape(s string) string { return htmlEscaper.Replace(s) }

// apiError is the body of HubSpot's error responses.
type apiError struct {
	Message  string `json:"message"`
	Category string `json:"category"`
}

// do POSTs body as JSON and decodes the response into out (if not nil). It
// sorts failures into ones worth retrying and ones that aren't.
func (p *Provider) do(ctx context.Context, client *http.Client, path string, body, out any) (int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, jobs.Permanent(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.APIBase+path, bytes.NewReader(raw))
	if err != nil {
		return 0, jobs.Permanent(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		if jobs.IsPermanent(err) {
			// The authorisation was revoked; drop the request URL from the message.
			var ue *url.Error
			if errors.As(err, &ue) {
				err = ue.Err
			}
			return 0, err
		}
		return 0, integrations.WithDetail(errors.New("couldn't reach HubSpot"), map[string]any{"error": err.Error()})
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out != nil {
			if err := json.Unmarshal(data, out); err != nil {
				return resp.StatusCode, fmt.Errorf("reading HubSpot's answer: %w", err)
			}
		}
		return resp.StatusCode, nil
	}

	var e apiError
	_ = json.Unmarshal(data, &e)
	detail := map[string]any{"status": resp.StatusCode}
	if e.Category != "" {
		detail["category"] = e.Category
	}
	msg := strings.TrimSpace(e.Message)
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	if msg != "" {
		detail["response"] = msg
	}
	switch code := resp.StatusCode; {
	case code == http.StatusUnauthorized:
		return code, integrations.WithDetail(jobs.Permanent(errors.New("HubSpot refused the authorisation; authorise the connection again")), detail)
	case code == http.StatusForbidden:
		return code, integrations.WithDetail(jobs.Permanent(errors.New(
			"the HubSpot app is missing a scope; add crm.objects.contacts.write and authorise again")), detail)
	case code == http.StatusTooManyRequests || code >= 500:
		return code, integrations.WithDetail(fmt.Errorf("HubSpot answered %d", code), detail)
	default:
		if msg == "" {
			msg = fmt.Sprintf("HubSpot rejected the request (%d)", code)
		} else {
			msg = "HubSpot rejected the request: " + msg
		}
		return code, integrations.WithDetail(jobs.Permanent(errors.New(msg)), detail)
	}
}
