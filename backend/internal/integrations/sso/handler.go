package sso

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/crewjam/saml"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/integrations"
	"github.com/faiz-gh/fronko/backend/internal/orgs"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// The sign-in flow (SP-initiated):
//
//  1. The browser opens /auth/sso/{handle}. Fronko sends it to the
//     organisation's identity provider with a SAML request, and sets a
//     short-lived signed cookie naming the request.
//  2. The identity provider posts the signed response to the connection's
//     ACS URL. Fronko checks the signature, the audience, the timing and
//     that it answers the request in the cookie (so nobody can sign someone
//     else's browser into their own account), then finds or creates the
//     person and starts a session.
const (
	stateCookie = "fronko_sso"
	stateTTL    = 10 * time.Minute
	loginPath   = "/login"
	homePath    = "/dashboard"
)

// Handler serves single sign-on and organisations' domains.
type Handler struct {
	svc          *integrations.Service
	store        *Store
	users        *users.Store
	orgs         *orgs.Store
	auth         *auth.Service
	codes        *users.Codes
	cache        *metadataCache
	client       *http.Client
	resolver     TXTResolver
	publicURL    string
	apiURL       string
	cookieSecure bool
	stateKey     []byte
}

// Options configure the handler.
type Options struct {
	// PublicURL is where people reach the site: sign-ins end there.
	PublicURL string
	// APIURL is where the API is reached (the ACS URL and entity ID are
	// built on it); empty means PublicURL. The sign-in cookie and the
	// session cookie are set on this address.
	APIURL       string
	CookieSecure bool
	// StateKey signs the sign-in cookie; derive it from a server secret.
	StateKey []byte
	// HTTP fetches identity providers' metadata (SSRF-guarded).
	HTTP     *http.Client
	Resolver TXTResolver
}

func NewHandler(svc *integrations.Service, store *Store, userStore *users.Store, orgStore *orgs.Store,
	authService *auth.Service, codes *users.Codes, opts Options) *Handler {
	h := &Handler{svc: svc, store: store, users: userStore, orgs: orgStore, auth: authService, codes: codes,
		cache: newMetadataCache(opts.HTTP), client: opts.HTTP, resolver: opts.Resolver,
		publicURL: opts.PublicURL, apiURL: opts.APIURL, cookieSecure: opts.CookieSecure, stateKey: opts.StateKey}
	if h.apiURL == "" {
		h.apiURL = h.publicURL
	}
	return h
}

// signInState is what the cookie remembers between the request and the response.
type signInState struct {
	ConnectionID int64  `json:"c"`
	RequestID    string `json:"r"`
	ReturnTo     string `json:"t"`
	Expires      int64  `json:"e"`
}

func (h *Handler) sign(st signInState) string {
	payload, _ := json.Marshal(st)
	body := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, h.stateKey)
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (h *Handler) parse(raw string) (signInState, bool) {
	var st signInState
	body, sig, ok := strings.Cut(raw, ".")
	if !ok {
		return st, false
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return st, false
	}
	mac := hmac.New(sha256.New, h.stateKey)
	mac.Write([]byte(body))
	if !hmac.Equal(got, mac.Sum(nil)) {
		return st, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || json.Unmarshal(payload, &st) != nil || time.Now().Unix() > st.Expires {
		return st, false
	}
	return st, true
}

// setState stores the sign-in cookie. The identity provider's response is
// a cross-site POST, so in production (HTTPS) the cookie must be
// SameSite=None; over plain http (local development) browsers refuse that,
// and Lax works because a local identity provider is on the same site.
func (h *Handler) setState(w http.ResponseWriter, value string, maxAge int) {
	c := &http.Cookie{Name: stateCookie, Value: value, Path: "/auth/saml/", MaxAge: maxAge,
		HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteLaxMode}
	if h.cookieSecure {
		c.SameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, c)
}

// fail sends the browser back to the site's sign-in page with a message.
// The site's address is spelled out, since these requests are served on
// the API's address, which may be another domain.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, h.publicURL+loginPath+"?"+url.Values{"sso_error": {msg}}.Encode(), http.StatusSeeOther)
}

// safeReturn keeps return_to to a path on this site.
func safeReturn(raw string) string {
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "/\\") ||
		strings.ContainsAny(raw, "\r\n") || len(raw) > 500 {
		return homePath
	}
	return raw
}

// Public: GET /auth/sso/{handle}?return_to=. Starts signing in to the
// organisation through its identity provider.
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	org, err := h.store.orgByHandle(ctx, r.PathValue("handle"))
	if errors.Is(err, database.ErrNotFound) {
		h.fail(w, r, "No organisation uses that address. Check the spelling, or sign in with your password.")
		return
	}
	if err != nil {
		log.Printf("sso: org lookup: %v", err)
		h.fail(w, r, "Something went wrong. Try again.")
		return
	}
	if org.Suspended {
		h.fail(w, r, auth.OrgSuspendedMessage)
		return
	}
	link, err := h.svc.OrgConnection(ctx, org.ID, integrations.CategorySSO)
	if errors.Is(err, database.ErrNotFound) {
		h.fail(w, r, "Single sign-on isn't set up for this organisation. Sign in with your password.")
		return
	}
	if err != nil {
		log.Printf("sso: connection lookup: %v", err)
		h.fail(w, r, "Something went wrong. Try again.")
		return
	}
	sp, err := h.serviceProvider(ctx, link)
	if err != nil {
		h.svc.Record(ctx, link.Connection.ID, integrations.Event{Kind: integrations.ActivitySignIn,
			Outcome: integrations.OutcomeFailed, Summary: "Couldn't start a sign-in: " + err.Error()})
		h.fail(w, r, "Your identity provider can't be reached right now. Try again, or ask your admin to check the single sign-on settings.")
		return
	}

	binding, location := saml.HTTPRedirectBinding, sp.GetSSOBindingLocation(saml.HTTPRedirectBinding)
	if location == "" {
		binding, location = saml.HTTPPostBinding, sp.GetSSOBindingLocation(saml.HTTPPostBinding)
	}
	req, err := sp.MakeAuthenticationRequest(location, binding, saml.HTTPPostBinding)
	if err != nil {
		log.Printf("sso: make request: %v", err)
		h.fail(w, r, "Something went wrong. Try again.")
		return
	}
	h.setState(w, h.sign(signInState{
		ConnectionID: link.Connection.ID,
		RequestID:    req.ID,
		ReturnTo:     safeReturn(r.URL.Query().Get("return_to")),
		Expires:      time.Now().Add(stateTTL).Unix(),
	}), int(stateTTL.Seconds()))

	if binding == saml.HTTPPostBinding {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(append([]byte("<!doctype html><title>Signing in…</title>"), req.Post("")...))
		return
	}
	target, err := req.Redirect("", sp)
	if err != nil {
		log.Printf("sso: redirect: %v", err)
		h.fail(w, r, "Something went wrong. Try again.")
		return
	}
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (h *Handler) serviceProvider(ctx context.Context, link *integrations.Linked) (*saml.ServiceProvider, error) {
	if h.apiURL == "" {
		return nil, errors.New("PUBLIC_URL isn't set")
	}
	md, err := h.cache.get(ctx, link)
	if err != nil {
		return nil, err
	}
	return serviceProvider(link, h.apiURL, md, h.client)
}

// External: POST /auth/saml/{id}/acs. The identity provider's response.
func (h *Handler) ACS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := httpx.PathID(w, r, "id", "connection")
	if !ok {
		return
	}
	var state signInState
	if c, err := r.Cookie(stateCookie); err == nil {
		state, ok = h.parse(c.Value)
		if !ok || state.ConnectionID != id {
			state = signInState{}
		}
	}
	h.setState(w, "", -1)

	link, err := h.svc.ConnectionByID(ctx, id)
	if err != nil || link.Connection.Category != integrations.CategorySSO || link.Connection.Status != integrations.StatusActive {
		h.fail(w, r, "This single sign-on connection is switched off or no longer exists.")
		return
	}
	connID := link.Connection.ID
	reject := func(summary, msg string, detail map[string]any) {
		h.svc.Record(ctx, connID, integrations.Event{Kind: integrations.ActivitySignIn,
			Outcome: integrations.OutcomeFailed, Summary: summary, Detail: detail})
		h.fail(w, r, msg)
	}

	sp, err := h.serviceProvider(ctx, link)
	if err != nil {
		reject("Couldn't check a sign-in: "+err.Error(),
			"Your identity provider can't be reached right now. Try again.", nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		h.fail(w, r, "The sign-in response couldn't be read. Try again.")
		return
	}
	var ids []string
	if state.RequestID != "" {
		ids = []string{state.RequestID}
	}
	assertion, err := sp.ParseResponse(r, ids)
	if err != nil {
		reason := err.Error()
		var ire *saml.InvalidResponseError
		if errors.As(err, &ire) && ire.PrivateErr != nil {
			reason = ire.PrivateErr.Error()
		}
		msg := "Sign-in failed. Try again, or ask your admin to check the single sign-on settings."
		if state.RequestID == "" && !sp.AllowIDPInitiated {
			msg = "Start signing in from Fronko's sign-in page (sign-in from your app dashboard isn't allowed), or your sign-in took too long. Try again."
		}
		reject("Rejected a sign-in response", msg, map[string]any{"reason": reason})
		return
	}

	who := readAssertion(assertion)
	if who.email == "" {
		reject("Rejected a sign-in: the identity provider sent no email address",
			"Your identity provider didn't send your email address. Ask your admin to check the single sign-on settings.",
			map[string]any{"name_id": who.nameID})
		return
	}
	user, created, msg, err := h.resolveUser(ctx, link, who)
	if err != nil {
		log.Printf("sso: resolve user: %v", err)
		h.fail(w, r, "Something went wrong. Try again.")
		return
	}
	if msg != "" {
		reject("Refused "+who.email+": "+msg, msg, nil)
		return
	}

	token, err := h.auth.GenerateJWT(user.ID, user.SessionVersion)
	if err != nil {
		log.Printf("sso: token: %v", err)
		h.fail(w, r, "Something went wrong. Try again.")
		return
	}
	auth.SetSessionCookie(w, token, h.cookieSecure)
	if err := h.users.TouchLastLogin(ctx, user.ID); err != nil {
		log.Printf("sso: touch last login: %v", err)
	}
	// An address single sign-on couldn't vouch for still needs a code.
	if user.EmailVerifiedAt == nil && user.Email != nil {
		if _, err := h.codes.Issue(ctx, user, auth.PurposeVerifyEmail, false); err != nil {
			log.Printf("sso: issue verification code: %v", err)
		}
	}
	summary := "Signed in " + who.email
	if created {
		summary = "Created an account for " + who.email + " and signed them in"
	}
	h.svc.Record(ctx, connID, integrations.Event{Kind: integrations.ActivitySignIn, Outcome: integrations.OutcomeSuccess,
		Summary: summary, UserID: user.ID})

	returnTo := homePath
	if state.ReturnTo != "" {
		returnTo = state.ReturnTo
	}
	http.Redirect(w, r, h.publicURL+returnTo, http.StatusSeeOther)
}

// identity is who the identity provider says signed in.
type identity struct {
	nameID string
	email  string
	name   string
}

// Attribute names identity providers commonly use, in order of preference.
var (
	emailAttrs = []string{"email", "mail", "emailaddress", "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
		"urn:oid:0.9.2342.19200300.100.1.3", "user.email"}
	displayNameAttrs = []string{"displayname", "http://schemas.microsoft.com/identity/claims/displayname", "urn:oid:2.16.840.1.113730.3.1.241", "fullname"}
	givenAttrs       = []string{"firstname", "givenname", "given_name", "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname", "urn:oid:2.5.4.42"}
	familyAttrs      = []string{"lastname", "surname", "sn", "familyname", "family_name", "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname", "urn:oid:2.5.4.4"}
)

func readAssertion(a *saml.Assertion) identity {
	values := map[string]string{}
	for _, st := range a.AttributeStatements {
		for _, attr := range st.Attributes {
			if len(attr.Values) == 0 {
				continue
			}
			v := strings.TrimSpace(attr.Values[0].Value)
			for _, key := range []string{attr.Name, attr.FriendlyName} {
				if k := strings.ToLower(key); k != "" && values[k] == "" {
					values[k] = v
				}
			}
		}
	}
	first := func(keys []string) string {
		for _, k := range keys {
			if v := values[strings.ToLower(k)]; v != "" {
				return v
			}
		}
		return ""
	}
	var id identity
	if a.Subject != nil && a.Subject.NameID != nil {
		id.nameID = strings.TrimSpace(a.Subject.NameID.Value)
	}
	if email, ok := users.NormalizeEmail(first(emailAttrs)); ok {
		id.email = email
	} else if email, ok := users.NormalizeEmail(id.nameID); ok {
		id.email = email
	}
	id.name = first(displayNameAttrs)
	if id.name == "" {
		id.name = strings.TrimSpace(first(givenAttrs) + " " + first(familyAttrs))
	}
	if len(id.name) > 200 || strings.ContainsAny(id.name, "\r\n") {
		id.name = ""
	}
	return id
}

// resolveUser finds the person signing in, or creates them when the
// connection allows it. msg explains a refusal.
func (h *Handler) resolveUser(ctx context.Context, link *integrations.Linked, who identity) (u *users.User, created bool, msg string, err error) {
	orgID := link.Connection.OrgID
	org, err := h.store.orgByID(ctx, orgID)
	if err != nil {
		return nil, false, "", err
	}
	if org.Suspended {
		return nil, false, auth.OrgSuspendedMessage, nil
	}
	verified, err := h.store.domainVerified(ctx, orgID, domainOf(who.email))
	if err != nil {
		return nil, false, "", err
	}

	u, err = h.users.GetUserByEmail(ctx, who.email)
	switch {
	case err == nil && u.OrgID != orgID:
		return nil, false, "This email address belongs to a Fronko account in another organisation.", nil
	case err == nil:
		if u.SuspendedAt != nil {
			return nil, false, "This account is suspended; contact your organisation.", nil
		}
		if u.EmailVerifiedAt == nil && verified {
			if err := h.users.MarkEmailVerified(ctx, u.ID); err != nil {
				return nil, false, "", err
			}
			now := time.Now()
			u.EmailVerifiedAt = &now
		}
		return u, false, "", nil
	case !errors.Is(err, database.ErrNotFound):
		return nil, false, "", err
	}

	if v, set := link.Settings.Values["jit"]; set && v != true {
		return nil, false, "You don't have a Fronko account yet. Ask your organisation's admin to add you.", nil
	}
	o, err := h.orgs.GetOrganization(ctx, orgID)
	if err != nil {
		return nil, false, "", err
	}
	provisioned := users.ProvisionedSSO
	u = &users.User{
		OrgID:             orgID,
		Role:              auth.RoleMember,
		Email:             &who.email,
		StorageQuotaBytes: o.DefaultQuotaBytes,
		ProvisionedBy:     &provisioned,
	}
	if who.name != "" {
		u.FullName = &who.name
	}
	if link.Connection.CreatedBy != nil {
		u.CreatedBy = &link.Connection.CreatedBy.ID
	}
	if verified {
		now := time.Now()
		u.EmailVerifiedAt = &now
	}
	base := users.UsernameFromEmail(who.email)
	for attempt := 1; ; attempt++ {
		if u.Username, err = h.users.FreeUsername(ctx, base); err != nil {
			return nil, false, "", err
		}
		err = h.orgs.CreateMember(ctx, u)
		if err == nil || !errors.Is(err, database.ErrConflict) || users.IsEmailConflict(err) || attempt == 3 {
			break
		}
	}
	if err != nil {
		return nil, false, "", fmt.Errorf("creating %s: %w", who.email, err)
	}
	return u, true, "", nil
}

func domainOf(email string) string {
	return email[strings.LastIndexByte(email, '@')+1:]
}

// Public: GET /auth/saml/{id}/metadata. Fronko's service provider metadata,
// for identity providers that read it from a URL.
func (h *Handler) Metadata(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "id", "connection")
	if !ok {
		return
	}
	link, err := h.svc.ConnectionByID(r.Context(), id)
	if err != nil || link.Connection.Category != integrations.CategorySSO || h.apiURL == "" {
		httpx.WriteError(w, http.StatusNotFound, "not found")
		return
	}
	sp, err := serviceProvider(link, h.apiURL, nil, h.client)
	if err != nil {
		httpx.Internal("sp metadata", err).Write(w)
		return
	}
	out, err := xml.MarshalIndent(sp.Metadata(), "", "  ")
	if err != nil {
		httpx.Internal("sp metadata", err).Write(w)
		return
	}
	w.Header().Set("Content-Type", "application/samlmetadata+xml")
	_, _ = w.Write(append([]byte(xml.Header), out...))
}

// Public: POST /auth/sso/discover {"identifier"}. Finds where to sign in
// with single sign-on, from an organisation handle or an email address on a
// verified domain.
func (h *Handler) Discover(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identifier string `json:"identifier"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	ident := strings.ToLower(strings.TrimSpace(req.Identifier))
	notFound := func() {
		httpx.WriteError(w, http.StatusNotFound,
			"we couldn't find single sign-on for that; check the spelling, or sign in with your password")
	}
	if ident == "" || len(ident) > 254 {
		notFound()
		return
	}
	var org *orgRef
	var err error
	if strings.Contains(ident, "@") {
		org, err = h.store.orgByDomain(r.Context(), domainOf(ident))
	} else {
		org, err = h.store.orgByHandle(r.Context(), strings.TrimPrefix(ident, "@"))
	}
	if errors.Is(err, database.ErrNotFound) {
		notFound()
		return
	}
	if err != nil {
		httpx.Internal("sso discover", err).Write(w)
		return
	}
	if _, err := h.svc.OrgConnection(r.Context(), org.ID, integrations.CategorySSO); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			notFound()
			return
		}
		httpx.Internal("sso discover", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"url": "/auth/sso/" + org.Handle, "handle": org.Handle})
}
