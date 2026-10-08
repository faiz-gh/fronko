package integrations

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
)

const (
	defaultActivityPage = 25
	maxActivityPage     = 100
)

// CategoryInfo is a catalog section.
type CategoryInfo struct {
	ID          Category `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	// Single means one connection per owner across the category's providers.
	Single bool `json:"single"`
}

var categoryInfo = map[Category]CategoryInfo{
	CategoryLeadSync: {CategoryLeadSync, "Lead Sync",
		"Send every lead your cards collect to your CRM or automation tool, as soon as it arrives.", false},
	CategoryCalendar: {CategoryCalendar, "Calendar Booking",
		"Connect your booking pages, then choose which one each card shows.", false},
	CategoryDirectory: {CategoryDirectory, "Team Member Import",
		"Add, update and remove people in Fronko automatically from your identity provider.", true},
	CategorySSO: {CategorySSO, "SAML SSO",
		"Let people sign in with your company's identity provider.", true},
}

// CatalogEntry is a provider as the catalog shows it.
type CatalogEntry struct {
	Manifest
	// Unavailable says why it can't be connected here, or is empty.
	Unavailable string `json:"unavailable"`
	// Testable means connections have a Test (or Send test lead) action.
	Testable bool `json:"testable"`
	// Connections are the user's (and, for admins, the organisation's)
	// connections to it, for the status badges.
	Connections []ConnectionSummary `json:"connections"`
}

// ConnectionSummary is enough of a connection for a badge.
type ConnectionSummary struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Scope   Scope  `json:"scope"`
	Status  string `json:"status"`
	Enabled bool   `json:"enabled"`
}

// Catalog is the response of GET /api/integrations/catalog.
type Catalog struct {
	Categories []CategoryInfo  `json:"categories"`
	Providers  []*CatalogEntry `json:"providers"`
	// OAuthRedirectURL is what to register in an OAuth app, or "" when
	// PUBLIC_URL isn't set.
	OAuthRedirectURL string `json:"oauth_redirect_url"`
}

// Handler serves the integrations API.
type Handler struct {
	svc *Service
	// cookieSecure marks the OAuth cookie Secure.
	cookieSecure bool
}

func NewHandler(svc *Service, cookieSecure bool) *Handler {
	return &Handler{svc: svc, cookieSecure: cookieSecure}
}

// Protected: GET /api/integrations/catalog. Members only see the
// integrations people can connect for themselves.
func (h *Handler) Catalog(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	conns, err := h.svc.store.ListConnections(r.Context(), p.OrgID, ConnectionFilter{Org: p.IsAdmin(), UserID: p.UserID})
	if err != nil {
		httpx.Internal("list connections", err).Write(w)
		return
	}
	byProvider := map[string][]ConnectionSummary{}
	for _, c := range conns {
		byProvider[c.Provider] = append(byProvider[c.Provider], ConnectionSummary{
			ID: c.ID, Name: c.Name, Scope: c.Scope(), Status: c.Status, Enabled: c.Enabled,
		})
	}
	out := Catalog{Providers: []*CatalogEntry{}, OAuthRedirectURL: h.svc.OAuthRedirectURL()}
	seen := map[Category]bool{}
	for _, prov := range h.svc.registry.All() {
		m := prov.Manifest()
		if !p.IsAdmin() && !m.AllowsScope(ScopeUser) {
			continue
		}
		_, pushes := prov.(LeadPusher)
		_, tests := prov.(Tester)
		entry := &CatalogEntry{Manifest: m, Unavailable: h.svc.Unavailable(m), Testable: pushes || tests,
			Connections: byProvider[m.ID]}
		if entry.Connections == nil {
			entry.Connections = []ConnectionSummary{}
		}
		if entry.Fields == nil {
			entry.Fields = []Field{}
		}
		out.Providers = append(out.Providers, entry)
		seen[m.Category] = true
	}
	for _, c := range Categories {
		if seen[c] {
			out.Categories = append(out.Categories, categoryInfo[c])
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Protected: GET /api/integrations/connections?scope=org|me&provider=.
func (h *Handler) ListConnections(w http.ResponseWriter, r *http.Request) {
	var scope Scope
	switch r.URL.Query().Get("scope") {
	case "":
	case "org":
		scope = ScopeOrg
	case "me":
		scope = ScopeUser
	default:
		httpx.WriteError(w, http.StatusBadRequest, "scope must be org or me")
		return
	}
	views, err := h.svc.List(r.Context(), auth.PrincipalFrom(r.Context()), scope, r.URL.Query().Get("provider"))
	if err != nil {
		h.writeErr(w, "list connections", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, views)
}

// Protected: POST /api/integrations/connections.
func (h *Handler) CreateConnection(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	view, err := h.svc.Create(r.Context(), auth.PrincipalFrom(r.Context()), in)
	if err != nil {
		h.writeErr(w, "create connection", err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}

// Protected: GET /api/integrations/connections/{id}.
func (h *Handler) GetConnection(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "id", "connection")
	if !ok {
		return
	}
	view, err := h.svc.Get(r.Context(), auth.PrincipalFrom(r.Context()), id)
	if err != nil {
		h.writeErr(w, "get connection", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

// Protected: PATCH /api/integrations/connections/{id}.
func (h *Handler) UpdateConnection(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "id", "connection")
	if !ok {
		return
	}
	var in UpdateInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	view, err := h.svc.Update(r.Context(), auth.PrincipalFrom(r.Context()), id, in)
	if err != nil {
		h.writeErr(w, "update connection", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

// Protected: DELETE /api/integrations/connections/{id}.
func (h *Handler) DeleteConnection(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "id", "connection")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), auth.PrincipalFrom(r.Context()), id); err != nil {
		h.writeErr(w, "delete connection", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Protected: POST /api/integrations/connections/{id}/test. A failed test is
// a 200 with ok: false; errors are for tests that couldn't run.
func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "id", "connection")
	if !ok {
		return
	}
	res, err := h.svc.Test(r.Context(), auth.PrincipalFrom(r.Context()), id)
	if err != nil {
		h.writeErr(w, "test connection", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// Protected: POST /api/integrations/connections/{id}/tokens. Generates the
// token a scim_token provider calls Fronko with, replacing any earlier one.
// The response is the only time the token is shown.
func (h *Handler) RotateToken(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "id", "connection")
	if !ok {
		return
	}
	token, view, err := h.svc.RotateToken(r.Context(), auth.PrincipalFrom(r.Context()), id)
	if err != nil {
		h.writeErr(w, "rotate token", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"token": token, "connection": view})
}

// Protected: GET /api/integrations/connections/{id}/activity?before=&limit=.
func (h *Handler) ListActivity(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "id", "connection")
	if !ok {
		return
	}
	q := r.URL.Query()
	before, err := strconv.ParseInt(q.Get("before"), 10, 64)
	if q.Get("before") != "" && (err != nil || before < 1) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid before")
		return
	}
	limit := defaultActivityPage
	if raw := q.Get("limit"); raw != "" {
		if limit, err = strconv.Atoi(raw); err != nil || limit < 1 || limit > maxActivityPage {
			httpx.WriteError(w, http.StatusBadRequest, "invalid limit")
			return
		}
	}
	items, err := h.svc.Activity(r.Context(), auth.PrincipalFrom(r.Context()), id, before, limit)
	if err != nil {
		h.writeErr(w, "list activity", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

// Protected: GET /api/integrations/connections/{id}/oauth/start. The browser
// navigates here, so failures redirect back to the page with a message.
func (h *Handler) StartOAuth(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "id", "connection")
	if !ok {
		return
	}
	p := auth.PrincipalFrom(r.Context())
	authURL, cookie, err := h.svc.StartOAuth(r.Context(), p, id)
	if err != nil {
		provider := ""
		if c, getErr := h.svc.store.GetConnection(r.Context(), p.OrgID, id); getErr == nil {
			provider = c.Provider
		}
		h.redirectBack(w, r, provider, id, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthCookieName,
		Value:    cookie,
		Path:     "/api/integrations/oauth",
		MaxAge:   int(oauthStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		// Lax, so it comes back on the provider's top-level redirect.
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Protected: GET /api/integrations/oauth/callback?state=&code= (or &error=).
func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cookie := ""
	if c, err := r.Cookie(oauthCookieName); err == nil {
		cookie = c.Value
	}
	http.SetCookie(w, &http.Cookie{Name: oauthCookieName, Path: "/api/integrations/oauth", MaxAge: -1, HttpOnly: true,
		Secure: h.cookieSecure, SameSite: http.SameSiteLaxMode})

	p := auth.PrincipalFrom(r.Context())
	st, stateErr := h.svc.parseState(q.Get("state"))
	if denied := q.Get("error"); denied != "" && stateErr == nil {
		provider := ""
		if c, err := h.svc.store.GetConnection(r.Context(), p.OrgID, st.ConnectionID); err == nil {
			provider = c.Provider
		}
		msg := "authorisation was cancelled"
		if d := q.Get("error_description"); d != "" && len(d) < 300 {
			msg = "the provider said: " + d
		}
		h.redirectBack(w, r, provider, st.ConnectionID, userErr(msg))
		return
	}
	provider, err := h.svc.FinishOAuth(r.Context(), p, q.Get("state"), q.Get("code"), cookie)
	h.redirectBack(w, r, provider, st.ConnectionID, err)
}

// redirectBack sends the browser to the integration's page (or the
// catalog), saying how the OAuth step went.
func (h *Handler) redirectBack(w http.ResponseWriter, r *http.Request, provider string, connectionID int64, err error) {
	target := "/dashboard/integrations"
	if provider != "" {
		target += "/" + url.PathEscape(provider)
	}
	q := url.Values{}
	if connectionID > 0 {
		q.Set("connection", strconv.FormatInt(connectionID, 10))
	}
	if err != nil {
		if !isUserError(err) {
			logf("oauth: %v", err)
			err = errors.New("something went wrong while connecting; try again")
		}
		q.Set("oauth_error", err.Error())
	} else {
		q.Set("oauth", "connected")
	}
	http.Redirect(w, r, h.svc.opts.PublicURL+target+"?"+q.Encode(), http.StatusFound)
}

// isUserError reports whether err is safe and useful to show as is.
func isUserError(err error) bool {
	var fe *FieldError
	var ue *userError
	return errors.As(err, &fe) || errors.As(err, &ue) || errors.Is(err, database.ErrNotFound) ||
		errors.Is(err, ErrOAuthState) || errors.Is(err, ErrUnknownProvider) || errors.Is(err, ErrForbidden) ||
		errors.Is(err, ErrOnlyOne) || errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNotReady) ||
		errors.Is(err, ErrNoSecretsKey)
}

func (h *Handler) writeErr(w http.ResponseWriter, what string, err error) {
	var fe *FieldError
	var ue *userError
	switch {
	case errors.As(err, &fe):
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": fe.Message, "field": fe.Field})
	case errors.As(err, &ue):
		httpx.WriteError(w, http.StatusBadRequest, ue.msg)
	case errors.Is(err, database.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "connection not found")
	case errors.Is(err, ErrUnknownProvider):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrForbidden):
		httpx.WriteError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrOnlyOne):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrUnavailable), errors.Is(err, ErrNotReady), errors.Is(err, ErrNoSecretsKey):
		httpx.WriteError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		httpx.Internal(what, err).Write(w)
	}
}

// activityRetention is how long the activity log is kept.
const activityRetention = 90 * 24 * time.Hour
