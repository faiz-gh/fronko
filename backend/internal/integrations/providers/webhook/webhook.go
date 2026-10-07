// Package webhook sends each lead to a URL as JSON: the generic lead sync
// that works with any tool able to receive a webhook (Zapier, Make, n8n, or
// your own code).
//
// Each delivery is a POST with a versioned envelope:
//
//	{"event": "lead.created", "version": 1, "test": false, "data": { …integrations.Lead… }}
//
// and these headers:
//
//	X-Fronko-Event:     lead.created
//	X-Fronko-Delivery:  an id that stays the same across retries of one delivery
//	X-Fronko-Signature: t=<unix seconds>,v1=<hex HMAC-SHA256 of "<t>.<body>">
//
// The signature is only sent when a signing secret is set. Receivers should
// recompute it, compare in constant time and reject old timestamps.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/faiz-gh/fronko/backend/internal/integrations"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
	"github.com/faiz-gh/fronko/backend/internal/platform/netguard"
)

// PayloadVersion is the envelope's version. Bump it only for changes that
// would break receivers.
const PayloadVersion = 1

const (
	EventLeadCreated = "lead.created"

	headerEvent     = "X-Fronko-Event"
	headerDelivery  = "X-Fronko-Delivery"
	headerSignature = "X-Fronko-Signature"
	userAgent       = "Fronko-Webhooks/1"

	// excerptLen caps the response body kept for the activity log.
	excerptLen = 300
)

// Envelope is the JSON body of every delivery.
type Envelope struct {
	Event   string             `json:"event"`
	Version int                `json:"version"`
	Test    bool               `json:"test"`
	Data    *integrations.Lead `json:"data"`
}

// Provider is the webhook provider. Presets for particular tools reuse it
// with their own manifest.
type Provider struct {
	M integrations.Manifest
	// Now is the clock for signatures (tests).
	Now func() time.Time
}

// New returns the generic webhook provider.
func New() *Provider {
	return &Provider{M: integrations.Manifest{
		ID:       "webhook",
		Name:     "Webhook",
		Category: integrations.CategoryLeadSync,
		Description: "Send each new lead as JSON to any URL: your own code, or an automation tool " +
			"such as Zapier, Make or n8n.",
		Scopes:   []integrations.Scope{integrations.ScopeOrg, integrations.ScopeUser},
		Auth:     integrations.AuthNone,
		Status:   integrations.Available,
		Multiple: true,
		Fields: []integrations.Field{
			{
				Key: "url", Label: "Payload URL", Type: integrations.FieldURL, Required: true,
				Placeholder: "https://example.com/hooks/fronko",
				Help:        "Fronko POSTs each lead here as JSON. It must use https.",
			},
			{
				Key: "signing_secret", Label: "Signing secret", Type: integrations.FieldSecret, Generate: true,
				Help: "Optional. When set, each request carries an X-Fronko-Signature header so the receiver can " +
					"check it came from Fronko. Copy it before saving: it isn't shown again.",
			},
		},
		SetupSteps: []string{
			"Create an endpoint that accepts POST requests with a JSON body, or a \"Catch hook\" trigger in your automation tool.",
			"Paste its URL below, optionally with a signing secret, and save.",
			"Send a test lead and check it arrives. Your endpoint should answer with any 2xx status.",
		},
		DocsURL:  "https://github.com/faiz-gh/fronko/blob/master/docs/integrations/webhook-zapier.md",
		Requires: []integrations.Requirement{integrations.RequiresSecretsKey},
		Keywords: []string{"zapier", "make", "n8n", "http", "api", "json"},
	}}
}

func (p *Provider) Manifest() integrations.Manifest { return p.M }

// Validate needs nothing beyond the field checks: the URL field already
// rejects private addresses and plain http.
func (p *Provider) Validate(context.Context, integrations.Settings) error { return nil }

// PushLead delivers one lead. The delivery id is the same on every retry, so
// receivers can drop duplicates.
func (p *Provider) PushLead(ctx context.Context, call *integrations.Call, lead *integrations.Lead) (integrations.Result, error) {
	delivery := fmt.Sprintf("lead-%d-%d", lead.ID, call.Connection.ID)
	test := lead.ID == 0
	if test {
		delivery = "test-" + strings.ToLower(rand.Text()[:12])
	}
	return p.deliver(ctx, call, Envelope{Event: EventLeadCreated, Version: PayloadVersion, Test: test, Data: lead}, delivery)
}

func (p *Provider) deliver(ctx context.Context, call *integrations.Call, env Envelope, delivery string) (integrations.Result, error) {
	target, err := netguard.CheckURL(call.Settings.String("url"), call.Settings.AllowPrivate)
	if err != nil {
		return integrations.Result{}, jobs.Permanent(fmt.Errorf("the payload URL isn't allowed: %w", err))
	}
	body, err := json.Marshal(env)
	if err != nil {
		return integrations.Result{}, jobs.Permanent(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(body))
	if err != nil {
		return integrations.Result{}, jobs.Permanent(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set(headerEvent, env.Event)
	req.Header.Set(headerDelivery, delivery)
	if secret := call.Settings.Secrets["signing_secret"]; secret != "" {
		req.Header.Set(headerSignature, Sign(secret, p.now(), body))
	}

	started := time.Now()
	resp, err := call.HTTP.Do(req)
	if err != nil {
		if errors.Is(err, netguard.ErrPrivate) {
			return integrations.Result{}, jobs.Permanent(errors.New("the payload URL points at a private or local address"))
		}
		return integrations.Result{}, integrations.WithDetail(fmt.Errorf("couldn't reach %s", target.Host), map[string]any{"error": err.Error()})
	}
	defer func() { _ = resp.Body.Close() }()
	excerpt := readExcerpt(resp.Body)
	detail := map[string]any{
		"status":      resp.StatusCode,
		"delivery":    delivery,
		"duration_ms": time.Since(started).Milliseconds(),
	}
	if excerpt != "" {
		detail["response"] = excerpt
	}

	switch code := resp.StatusCode; {
	case code >= 200 && code < 300:
		return integrations.Result{Summary: fmt.Sprintf("Delivered to %s (%d)", target.Host, code), Detail: detail}, nil
	case code >= 300 && code < 400:
		detail["location"] = resp.Header.Get("Location")
		return integrations.Result{}, integrations.WithDetail(jobs.Permanent(
			fmt.Errorf("the receiver at %s answered with a redirect (%d); use the final URL instead", target.Host, code)), detail)
	case code == http.StatusRequestTimeout || code == http.StatusTooManyRequests || code >= 500:
		return integrations.Result{}, integrations.WithDetail(fmt.Errorf("the receiver at %s answered %d", target.Host, code), detail)
	default:
		// Other 4xx: the receiver rejected it, and sending it again won't help.
		return integrations.Result{}, integrations.WithDetail(jobs.Permanent(
			fmt.Errorf("the receiver at %s rejected the lead (%d)", target.Host, code)), detail)
	}
}

func (p *Provider) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

// Sign returns the X-Fronko-Signature value for body sent at t.
func Sign(secret string, t time.Time, body []byte) string {
	ts := strconv.FormatInt(t.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	return "t=" + ts + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

// Verify checks a signature header against body, rejecting signatures older
// than tolerance. Receivers written in Go can use it as is.
func Verify(secret, header string, body []byte, tolerance time.Duration, now time.Time) bool {
	var ts, sig string
	for part := range strings.SplitSeq(header, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch k {
		case "t":
			ts = v
		case "v1":
			sig = v
		}
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || sig == "" {
		return false
	}
	at := time.Unix(sec, 0)
	if now.Sub(at) > tolerance || at.Sub(now) > tolerance {
		return false
	}
	want := Sign(secret, at, body)
	return hmac.Equal([]byte(want), []byte("t="+ts+",v1="+sig))
}

// readExcerpt keeps the start of a response body for troubleshooting.
func readExcerpt(r io.Reader) string {
	buf, _ := io.ReadAll(io.LimitReader(r, excerptLen+1))
	s := strings.TrimSpace(strings.ToValidUTF8(string(buf), ""))
	if len(s) > excerptLen {
		s = s[:excerptLen]
		for !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
		s += "…"
	}
	return s
}
