// Package calendar puts a booking page on cards: a "Book a meeting" button
// that opens the card holder's booking page, or the organisation's default
// one for cards whose holder hasn't connected their own.
//
// Each provider is a link the person pastes, checked against the
// provider's own addresses. Some booking pages can be opened with the
// visitor's name and email filled in; the card does that when the visitor
// has already given them in the lead form.
package calendar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/integrations"
)

// Provider is one kind of booking page.
type Provider struct {
	m integrations.Manifest
	// hosts the link must be on (or a subdomain of); empty allows any.
	hosts []string
	// prefill maps the page's query parameters to what the visitor gave.
	prefill map[string]string
}

func (p *Provider) Manifest() integrations.Manifest { return p.m }

func (p *Provider) Validate(_ context.Context, cfg integrations.Settings) error {
	raw := cfg.String("url")
	if raw == "" || len(p.hosts) == 0 {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return integrations.NewFieldError("url", "enter your booking page's link")
	}
	if !onHosts(u.Hostname(), p.hosts) {
		return integrations.NewFieldError("url", fmt.Sprintf("this isn't a %s link; it should be on %s", p.m.Name, p.hosts[0]))
	}
	return nil
}

func onHosts(host string, hosts []string) bool {
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	for _, h := range hosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

// Booking is the button the card shows.
func (p *Provider) Booking(cfg integrations.Settings) *integrations.Booking {
	link := cfg.String("url")
	if link == "" {
		return nil
	}
	return &integrations.Booking{Provider: p.m.ID, Name: p.m.Name, URL: link, Prefill: p.prefill}
}

// Test opens the booking page and checks it's there.
func (p *Provider) Test(ctx context.Context, call *integrations.Call) (integrations.Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, call.Settings.String("url"), nil)
	if err != nil {
		return integrations.Result{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Fronko link check)")
	resp, err := call.HTTP.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return integrations.Result{}, fmt.Errorf("couldn't open the booking page: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	detail := map[string]any{"status": resp.StatusCode}
	if resp.StatusCode >= 400 {
		return integrations.Result{}, integrations.WithDetail(
			fmt.Errorf("the booking page answered %d; check the link", resp.StatusCode), detail)
	}
	return integrations.Result{Summary: "The booking page opens", Detail: detail}, nil
}

type spec struct {
	id, name, description, placeholder string
	hosts                              []string
	prefill                            map[string]string
	steps                              []string
	keywords                           []string
}

func newProvider(s spec) *Provider {
	help := "The page visitors book a meeting on."
	if len(s.prefill) > 0 {
		help += " Visitors who already gave their name and email on the card won't have to type them again."
	}
	return &Provider{
		m: integrations.Manifest{
			ID:          s.id,
			Name:        s.name,
			Category:    integrations.CategoryCalendar,
			Description: s.description,
			Scopes:      []integrations.Scope{integrations.ScopeUser, integrations.ScopeOrg},
			Auth:        integrations.AuthLink,
			Status:      integrations.Available,
			Fields: []integrations.Field{{
				Key: "url", Label: "Booking page link", Type: integrations.FieldURL, Required: true,
				Placeholder: s.placeholder, Help: help,
			}},
			SetupSteps: append(s.steps,
				"Paste the link below and save. The card's “Book a meeting” button opens it from then on."),
			DocsURL:  "https://github.com/faiz-gh/fronko/blob/master/docs/integrations/calendar.md",
			Keywords: append(s.keywords, "booking", "meeting", "calendar", "scheduling"),
		},
		hosts:   s.hosts,
		prefill: s.prefill,
	}
}

// All returns the booking providers in catalog order.
func All() []integrations.Provider {
	return []integrations.Provider{
		newProvider(spec{
			id: "calendly", name: "Calendly",
			description: "Show your Calendly page on your card, with the visitor's name and email filled in.",
			placeholder: "https://calendly.com/your-name/30min",
			hosts:       []string{"calendly.com"},
			prefill:     map[string]string{"name": "name", "email": "email"},
			steps:       []string{"In Calendly, open the event type visitors should book and click Copy link."},
		}),
		newProvider(spec{
			id: "chili-piper", name: "Chili Piper",
			description: "Let visitors book time with you through Chili Piper.",
			placeholder: "https://your-company.chilipiper.com/me/your-name",
			hosts:       []string{"chilipiper.com"},
			steps:       []string{"In Chili Piper, open your personal or team meeting link and copy it."},
		}),
		newProvider(spec{
			id: "microsoft-bookings", name: "Microsoft Bookings",
			description: "Show your Microsoft Bookings page on your card.",
			placeholder: "https://outlook.office.com/book/YourPage@your-company.com/",
			hosts:       []string{"outlook.office.com", "outlook.office365.com", "outlook.live.com", "bookings.cloud.microsoft"},
			steps:       []string{"In Microsoft Bookings, open your booking page or personal booking page and copy its link."},
			keywords:    []string{"outlook", "microsoft 365"},
		}),
		newProvider(spec{
			id: "hubspot-meetings", name: "HubSpot Meetings",
			description: "Show your HubSpot meetings link on your card, with the visitor's details filled in.",
			placeholder: "https://meetings.hubspot.com/your-name",
			hosts:       []string{"meetings.hubspot.com", "meetings-eu1.hubspot.com"},
			prefill:     map[string]string{"firstName": "first_name", "lastName": "last_name", "email": "email"},
			steps:       []string{"In HubSpot, go to Sales → Meetings, and copy the link of your scheduling page."},
			keywords:    []string{"hubspot"},
		}),
		newProvider(spec{
			id: "google-calendar", name: "Google Calendar",
			description: "Show your Google Calendar appointment schedule on your card.",
			placeholder: "https://calendar.app.google/…",
			hosts:       []string{"calendar.app.google", "calendar.google.com"},
			steps:       []string{"In Google Calendar, open your appointment schedule, click Share, and copy the booking page link."},
			keywords:    []string{"google", "appointment"},
		}),
		newProvider(spec{
			id: "booking-link", name: "Other booking link",
			description: "Any other booking page, such as Cal.com, Zoho Bookings, SavvyCal or TidyCal.",
			placeholder: "https://cal.com/your-name",
			steps:       []string{"Copy the link of your booking page from your scheduling tool."},
			keywords:    []string{"cal.com", "zoho", "savvycal", "tidycal"},
		}),
	}
}
