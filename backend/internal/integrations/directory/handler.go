package directory

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/integrations"
	"github.com/faiz-gh/fronko/backend/internal/orgs"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/mail"
	"github.com/faiz-gh/fronko/backend/internal/teams"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// BasePath is where the SCIM API is served.
const BasePath = "/scim/v2"

// Identity is what provisioning needs to know about an organisation's
// single sign-on (the sso package).
type Identity interface {
	// SSOSignInURL is where the organisation's people sign in with single
	// sign-on, or "" when it has none. People added while it's set get no
	// password.
	SSOSignInURL(ctx context.Context, orgID int64) (string, error)
	// VerifiedEmail reports whether email is on one of the organisation's
	// verified domains; only then is a provisioned address trusted as verified.
	VerifiedEmail(ctx context.Context, orgID int64, email string) (bool, error)
}

// Handler serves the SCIM API to an organisation's identity provider,
// authenticated by the token of its directory connection.
type Handler struct {
	svc       *integrations.Service
	store     *Store
	users     *users.Store
	orgs      *orgs.Store
	teams     *teams.Store
	auth      *auth.Service
	codes     *users.Codes
	identity  Identity
	publicURL string
}

func NewHandler(svc *integrations.Service, store *Store, userStore *users.Store, orgStore *orgs.Store,
	teamStore *teams.Store, authService *auth.Service, codes *users.Codes, identity Identity, publicURL string) *Handler {
	return &Handler{svc: svc, store: store, users: userStore, orgs: orgStore, teams: teamStore,
		auth: authService, codes: codes, identity: identity, publicURL: publicURL}
}

// call is one authenticated SCIM request.
type call struct {
	conn  *integrations.Connection
	orgID int64
	base  string
}

type handlerFunc func(w http.ResponseWriter, r *http.Request, c *call) *scimError

// authed checks the bearer token and serves the request for its organisation.
func (h *Handler) authed(next handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="fronko-scim"`)
			writeSCIMError(w, &scimError{Status: http.StatusUnauthorized, Detail: "a bearer token is required"})
			return
		}
		conn, err := h.svc.Authenticate(r.Context(), strings.TrimSpace(token), integrations.CategoryDirectory)
		if errors.Is(err, integrations.ErrBadToken) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="fronko-scim", error="invalid_token"`)
			writeSCIMError(w, &scimError{Status: http.StatusUnauthorized, Detail: "the token is invalid, revoked, or its connection is paused"})
			return
		}
		if err != nil {
			log.Printf("scim: authenticate: %v", err)
			writeSCIMError(w, &scimError{Status: http.StatusInternalServerError, Detail: "internal error"})
			return
		}
		if suspended, err := h.orgs.IsOrgSuspended(r.Context(), conn.OrgID); err != nil || suspended {
			writeSCIMError(w, &scimError{Status: http.StatusForbidden, Detail: "this organisation is suspended"})
			return
		}
		base := h.publicURL
		if base == "" {
			scheme := "https"
			if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
				scheme = "http"
			}
			base = scheme + "://" + r.Host
		}
		c := &call{conn: conn, orgID: conn.OrgID, base: base + BasePath}
		if e := next(w, r, c); e != nil {
			if e.Status >= 500 {
				e.Detail = "internal error"
			}
			if r.Method != http.MethodGet && e.Status < 500 {
				h.svc.Record(r.Context(), conn.ID, integrations.Event{Kind: integrations.ActivityProvision,
					Outcome: integrations.OutcomeFailed, Summary: "Refused a request: " + e.Detail,
					Detail: map[string]any{"status": e.Status, "request": r.Method + " " + r.URL.Path}})
			}
			writeSCIMError(w, e)
		}
	}
}

func internal(what string, err error) *scimError {
	log.Printf("scim: %s: %v", what, err)
	return &scimError{Status: http.StatusInternalServerError, Detail: "internal error"}
}

func (h *Handler) record(ctx context.Context, c *call, summary string) {
	h.svc.Record(ctx, c.conn.ID, integrations.Event{Kind: integrations.ActivityProvision, Outcome: integrations.OutcomeSuccess, Summary: summary})
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func (c *call) userOut(p *person) userOut {
	out := userOut{
		Schemas:  []string{schemaUser},
		ID:       strconv.FormatInt(p.ID, 10),
		UserName: p.userName(),
		Active:   !p.Suspended,
		Groups:   []ref{},
		Meta: meta{ResourceType: "User", Created: p.CreatedAt, LastModified: p.UpdatedAt,
			Location: fmt.Sprintf("%s/Users/%d", c.base, p.ID)},
	}
	if p.ExternalID != nil {
		out.ExternalID = *p.ExternalID
	}
	if p.FullName != nil {
		name := splitName(*p.FullName)
		out.Name = &name
		out.DisplayName = *p.FullName
	}
	if p.Email != nil {
		out.Emails = []emailAttr{{Value: *p.Email, Type: "work", Primary: true}}
	}
	for _, t := range p.Teams {
		out.Groups = append(out.Groups, ref{Value: strconv.FormatInt(t.ID, 10), Display: t.Name,
			Ref: fmt.Sprintf("%s/Groups/%d", c.base, t.ID)})
	}
	return out
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request, c *call) *scimError {
	f, e := parseFilter(r.URL.Query().Get("filter"), userFilters)
	if e != nil {
		return e
	}
	start, offset, limit, e := page(r)
	if e != nil {
		return e
	}
	people, total, err := h.store.listPeople(r.Context(), c.orgID, f, offset, limit)
	if err != nil {
		return internal("list users", err)
	}
	res := listResponse{Schemas: []string{schemaList}, TotalResults: total, StartIndex: start,
		ItemsPerPage: len(people), Resources: []any{}}
	for _, p := range people {
		res.Resources = append(res.Resources, c.userOut(p))
	}
	writeSCIM(w, http.StatusOK, res)
	return nil
}

func (h *Handler) loadPerson(ctx context.Context, r *http.Request, c *call) (*person, *scimError) {
	id, ok := pathID(r)
	if !ok {
		return nil, notFound("user")
	}
	p, err := h.store.getPerson(ctx, c.orgID, id)
	if errors.Is(err, database.ErrNotFound) {
		return nil, notFound("user")
	}
	if err != nil {
		return nil, internal("get user", err)
	}
	return p, nil
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request, c *call) *scimError {
	p, e := h.loadPerson(r.Context(), r, c)
	if e != nil {
		return e
	}
	writeSCIM(w, http.StatusOK, c.userOut(p))
	return nil
}

// draft is a person as an identity provider wants them, built from a POST,
// a PUT or a PATCH, before it's checked and saved.
type draft struct {
	userName   string
	externalID *string
	// displayName and the given and family names; the Set flags mark the
	// ones this request gave.
	displayName *string
	displaySet  bool
	given       string
	family      string
	nameSet     bool
	email       string
	active      bool
}

func draftOf(p *person) *draft {
	d := &draft{userName: p.userName(), externalID: p.ExternalID, displayName: p.FullName, active: !p.Suspended}
	if p.Email != nil {
		d.email = *p.Email
	}
	if p.FullName != nil {
		n := splitName(*p.FullName)
		d.given, d.family = n.GivenName, n.FamilyName
	}
	return d
}

func draftFrom(in userIn) *draft {
	d := &draft{userName: in.UserName, externalID: in.ExternalID, displayName: in.DisplayName,
		displaySet: in.DisplayName != nil, active: true}
	if in.Name != nil {
		d.nameSet = true
		d.given, d.family = in.Name.GivenName, in.Name.FamilyName
		if d.displayName == nil && in.Name.Formatted != "" {
			d.displayName, d.displaySet = &in.Name.Formatted, true
		}
	}
	d.email = primaryEmail(in.Emails)
	if in.Active != nil {
		d.active = bool(*in.Active)
	}
	return d
}

// fullName is the name to store: a display name the request gave, or else
// given and family names it gave, or else the name already stored.
func (d *draft) fullName() (*string, *scimError) {
	name := ""
	switch {
	case d.displaySet && d.displayName != nil && strings.TrimSpace(*d.displayName) != "":
		name = *d.displayName
	case d.nameSet:
		name = strings.TrimSpace(d.given + " " + d.family)
	case d.displayName != nil:
		name = *d.displayName
	}
	return optText(&name, maxNameLen, "the name")
}

// checked validates a draft and returns what to save.
func (h *Handler) checked(ctx context.Context, c *call, d *draft, selfID int64) (personChange, *scimError) {
	var ch personChange
	userName, e := cleanText(d.userName, 254, "userName")
	if e != nil {
		return ch, e
	}
	if userName == "" {
		return ch, badRequest("invalidValue", "userName is required")
	}
	ch.ExternalUsername = &userName
	if ch.ExternalID, e = optText(d.externalID, maxExternalIDLen, "externalId"); e != nil {
		return ch, e
	}
	if ch.FullName, e = d.fullName(); e != nil {
		return ch, e
	}
	raw := d.email
	if raw == "" && strings.Contains(userName, "@") {
		raw = userName
	}
	email, ok := users.NormalizeEmail(raw)
	if !ok {
		return ch, badRequest("invalidValue", "a valid work email is required (emails, or a userName that is an email address)")
	}
	ch.Email = email
	owner, err := h.store.emailOrg(ctx, email)
	if err != nil {
		return ch, internal("email lookup", err)
	}
	if owner != 0 && owner != c.orgID {
		return ch, conflict("%s already has a Fronko account in another organisation", email)
	}
	if ch.EmailVerified, err = h.identity.VerifiedEmail(ctx, c.orgID, email); err != nil {
		return ch, internal("check domain", err)
	}
	// The same person can't be provisioned twice.
	for _, f := range []*filter{{"username", userName}, {"emails", email}} {
		if f.value == "" {
			continue
		}
		found, _, err := h.store.listPeople(ctx, c.orgID, f, 0, 2)
		if err != nil {
			return ch, internal("find user", err)
		}
		for _, p := range found {
			if p.ID != selfID {
				return ch, conflict("a user with this %s already exists", map[string]string{"username": "userName", "emails": "email"}[f.attr])
			}
		}
	}
	if ch.ExternalID != nil {
		found, _, err := h.store.listPeople(ctx, c.orgID, &filter{"externalid", *ch.ExternalID}, 0, 2)
		if err != nil {
			return ch, internal("find user", err)
		}
		for _, p := range found {
			if p.ID != selfID {
				return ch, conflict("a user with this externalId already exists")
			}
		}
	}
	return ch, nil
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request, c *call) *scimError {
	var in userIn
	if e := decode(w, r, &in); e != nil {
		return e
	}
	ctx := r.Context()
	d := draftFrom(in)
	ch, e := h.checked(ctx, c, d, 0)
	if e != nil {
		return e
	}
	org, err := h.orgs.GetOrganization(ctx, c.orgID)
	if err != nil {
		return internal("get organisation", err)
	}
	ssoURL, err := h.identity.SSOSignInURL(ctx, c.orgID)
	if err != nil {
		return internal("sso lookup", err)
	}

	provisioned := users.ProvisionedSCIM
	u := &users.User{
		OrgID:             c.orgID,
		Role:              auth.RoleMember,
		Email:             &ch.Email,
		StorageQuotaBytes: org.DefaultQuotaBytes,
		FullName:          ch.FullName,
		ExternalID:        ch.ExternalID,
		ExternalUsername:  ch.ExternalUsername,
		ProvisionedBy:     &provisioned,
	}
	if c.conn.CreatedBy != nil {
		u.CreatedBy = &c.conn.CreatedBy.ID
	}
	if ch.EmailVerified {
		now := time.Now()
		u.EmailVerifiedAt = &now
	}
	// Without single sign-on, people sign in with a temporary password they
	// must replace, as when an admin adds them.
	tempPassword := ""
	if ssoURL == "" {
		tempPassword = rand.Text()[:16]
		if u.PasswordHash, err = h.auth.HashPassword(tempPassword); err != nil {
			return internal("hash password", err)
		}
		u.MustChangePassword = true
	}

	base := users.UsernameFromEmail(ch.Email)
	for attempt := 1; ; attempt++ {
		if u.Username, err = h.users.FreeUsername(ctx, base); err != nil {
			return internal("pick username", err)
		}
		err = h.orgs.CreateMember(ctx, u)
		if err == nil || !errors.Is(err, database.ErrConflict) || users.IsEmailConflict(err) || attempt == 3 {
			break
		}
	}
	switch {
	case users.IsEmailConflict(err):
		return conflict("a user with this email already exists")
	case errors.Is(err, database.ErrConflict):
		return conflict("a user with this userName or externalId already exists")
	case err != nil:
		return internal("create user", err)
	}
	if !d.active {
		if err := h.users.SetUserSuspended(ctx, u.ID, true); err != nil {
			return internal("suspend user", err)
		}
	}
	if tempPassword != "" {
		h.codes.SendAsync(ch.Email, mail.MemberInviteMessage(org.Name, u.Username, tempPassword))
	} else {
		h.codes.SendAsync(ch.Email, mail.MemberSSOInviteMessage(org.Name, u.Username, ssoURL))
	}
	h.record(ctx, c, "Added "+ch.Email)

	p, err := h.store.getPerson(ctx, c.orgID, u.ID)
	if err != nil {
		return internal("get user", err)
	}
	w.Header().Set("Location", c.userOut(p).Meta.Location)
	writeSCIM(w, http.StatusCreated, c.userOut(p))
	return nil
}

// save applies a draft to a person who already exists.
func (h *Handler) save(ctx context.Context, c *call, p *person, d *draft) *scimError {
	ch, e := h.checked(ctx, c, d, p.ID)
	if e != nil {
		return e
	}
	if p.Role == auth.RoleOwner && !d.active {
		return badRequest("mutability", "the organisation's owner can't be deactivated from the directory")
	}
	err := h.store.updatePerson(ctx, c.orgID, p.ID, ch)
	if errors.Is(err, database.ErrConflict) {
		return conflict("a user with this email or externalId already exists")
	}
	if err != nil {
		return internal("update user", err)
	}
	what := "Updated " + ch.Email
	if d.active == p.Suspended {
		if err := h.users.SetUserSuspended(ctx, p.ID, !d.active); err != nil {
			return internal("suspend user", err)
		}
		what = map[bool]string{true: "Reactivated ", false: "Deactivated "}[d.active] + ch.Email
	}
	h.record(ctx, c, what)
	return nil
}

func (h *Handler) replaceUser(w http.ResponseWriter, r *http.Request, c *call) *scimError {
	p, e := h.loadPerson(r.Context(), r, c)
	if e != nil {
		return e
	}
	var in userIn
	if e := decode(w, r, &in); e != nil {
		return e
	}
	d := draftFrom(in)
	if d.email == "" && p.Email != nil {
		d.email = *p.Email
	}
	if e := h.save(r.Context(), c, p, d); e != nil {
		return e
	}
	return h.getUser(w, r, c)
}

func (h *Handler) patchUser(w http.ResponseWriter, r *http.Request, c *call) *scimError {
	p, e := h.loadPerson(r.Context(), r, c)
	if e != nil {
		return e
	}
	var req patchRequest
	if e := decode(w, r, &req); e != nil {
		return e
	}
	d := draftOf(p)
	for _, raw := range req.Operations {
		ops, e := raw.expand()
		if e != nil {
			return e
		}
		for _, op := range ops {
			if e := applyUserOp(d, op); e != nil {
				return e
			}
		}
	}
	if e := h.save(r.Context(), c, p, d); e != nil {
		return e
	}
	return h.getUser(w, r, c)
}

// applyUserOp changes a draft by one PATCH operation. Attributes Fronko
// doesn't keep (job title, phone numbers, the enterprise extension…) are
// accepted and ignored, so default attribute mappings work.
func applyUserOp(d *draft, op patchOp) *scimError {
	kind, e := op.kind()
	if e != nil {
		return e
	}
	path := op.path()
	if kind == "remove" {
		switch path {
		case "externalid":
			d.externalID = nil
		case "displayname", "name.formatted":
			d.displayName, d.displaySet = nil, true
		case "name.givenname":
			d.given, d.nameSet = "", true
		case "name.familyname":
			d.family, d.nameSet = "", true
		}
		return nil
	}
	switch {
	case path == "active":
		v, e := op.boolean()
		if e != nil {
			return e
		}
		d.active = v
	case path == "username":
		v, e := op.str()
		if e != nil {
			return e
		}
		d.userName = v
	case path == "externalid":
		v, e := op.str()
		if e != nil {
			return e
		}
		d.externalID = &v
	case path == "displayname" || path == "name.formatted":
		v, e := op.str()
		if e != nil {
			return e
		}
		d.displayName, d.displaySet = &v, true
	case path == "name.givenname":
		v, e := op.str()
		if e != nil {
			return e
		}
		d.given, d.nameSet = v, true
	case path == "name.familyname":
		v, e := op.str()
		if e != nil {
			return e
		}
		d.family, d.nameSet = v, true
	case path == "emails":
		var emails []emailAttr
		if err := json.Unmarshal(op.Value, &emails); err != nil {
			return badRequest("invalidValue", "emails must be a list")
		}
		if v := primaryEmail(emails); v != "" {
			d.email = v
		}
	case strings.HasPrefix(path, "emails[") && strings.HasSuffix(path, "].value"):
		v, e := op.str()
		if e != nil {
			return e
		}
		d.email = v
	}
	return nil
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request, c *call) *scimError {
	p, e := h.loadPerson(r.Context(), r, c)
	if e != nil {
		return e
	}
	if p.Role == auth.RoleOwner {
		return badRequest("mutability", "the organisation's owner can't be deleted from the directory")
	}
	if err := h.users.DeleteOrgUser(r.Context(), p.ID, c.orgID); err != nil {
		return internal("delete user", err)
	}
	h.record(r.Context(), c, "Removed "+p.userName())
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ---------------------------------------------------------------------------
// Groups (teams)
// ---------------------------------------------------------------------------

func (c *call) groupOut(g *group, members bool) groupOut {
	out := groupOut{
		Schemas:     []string{schemaGroup},
		ID:          strconv.FormatInt(g.ID, 10),
		DisplayName: g.Name,
		Meta: meta{ResourceType: "Group", Created: g.CreatedAt, LastModified: g.UpdatedAt,
			Location: fmt.Sprintf("%s/Groups/%d", c.base, g.ID)},
	}
	if g.ExternalID != nil {
		out.ExternalID = *g.ExternalID
	}
	if members {
		out.Members = []ref{}
		for _, m := range g.Members {
			out.Members = append(out.Members, ref{Value: strconv.FormatInt(m.ID, 10), Display: m.Username,
				Ref: fmt.Sprintf("%s/Users/%d", c.base, m.ID)})
		}
	}
	return out
}

// wantMembers reports whether the client asked for members; identity
// providers leave them out of lookups with excludedAttributes=members.
func wantMembers(r *http.Request) bool {
	return !strings.Contains(strings.ToLower(r.URL.Query().Get("excludedAttributes")), "members")
}

func (h *Handler) listGroups(w http.ResponseWriter, r *http.Request, c *call) *scimError {
	f, e := parseFilter(r.URL.Query().Get("filter"), groupFilters)
	if e != nil {
		return e
	}
	start, offset, limit, e := page(r)
	if e != nil {
		return e
	}
	members := wantMembers(r)
	groups, total, err := h.store.listGroups(r.Context(), c.orgID, f, offset, limit, members)
	if err != nil {
		return internal("list groups", err)
	}
	res := listResponse{Schemas: []string{schemaList}, TotalResults: total, StartIndex: start,
		ItemsPerPage: len(groups), Resources: []any{}}
	for _, g := range groups {
		res.Resources = append(res.Resources, c.groupOut(g, members))
	}
	writeSCIM(w, http.StatusOK, res)
	return nil
}

func (h *Handler) loadGroup(ctx context.Context, r *http.Request, c *call, members bool) (*group, *scimError) {
	id, ok := pathID(r)
	if !ok {
		return nil, notFound("group")
	}
	g, err := h.store.getGroup(ctx, c.orgID, id, members)
	if errors.Is(err, database.ErrNotFound) {
		return nil, notFound("group")
	}
	if err != nil {
		return nil, internal("get group", err)
	}
	return g, nil
}

func (h *Handler) getGroup(w http.ResponseWriter, r *http.Request, c *call) *scimError {
	members := wantMembers(r)
	g, e := h.loadGroup(r.Context(), r, c, members)
	if e != nil {
		return e
	}
	writeSCIM(w, http.StatusOK, c.groupOut(g, members))
	return nil
}

func groupName(name string) (string, *scimError) {
	name, e := cleanText(name, 80, "displayName")
	if e != nil {
		return "", e
	}
	if name == "" {
		return "", badRequest("invalidValue", "displayName is required")
	}
	return name, nil
}

func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request, c *call) *scimError {
	var in groupIn
	if e := decode(w, r, &in); e != nil {
		return e
	}
	name, e := groupName(in.DisplayName)
	if e != nil {
		return e
	}
	externalID, e := optText(in.ExternalID, maxExternalIDLen, "externalId")
	if e != nil {
		return e
	}
	ctx := r.Context()
	id, err := h.store.createGroup(ctx, c.orgID, name, externalID)
	if errors.Is(err, database.ErrConflict) {
		return conflict("a group named %q already exists", name)
	}
	if err != nil {
		return internal("create group", err)
	}
	if len(in.Members) > 0 {
		if err := h.store.addMembers(ctx, h.store.db, c.orgID, id, refIDs(in.Members)); err != nil {
			return internal("add members", err)
		}
	}
	h.record(ctx, c, "Created team "+name)
	g, err := h.store.getGroup(ctx, c.orgID, id, true)
	if err != nil {
		return internal("get group", err)
	}
	out := c.groupOut(g, true)
	w.Header().Set("Location", out.Meta.Location)
	writeSCIM(w, http.StatusCreated, out)
	return nil
}

func (h *Handler) replaceGroup(w http.ResponseWriter, r *http.Request, c *call) *scimError {
	g, e := h.loadGroup(r.Context(), r, c, false)
	if e != nil {
		return e
	}
	var in groupIn
	if e := decode(w, r, &in); e != nil {
		return e
	}
	name, e := groupName(in.DisplayName)
	if e != nil {
		return e
	}
	externalID, e := optText(in.ExternalID, maxExternalIDLen, "externalId")
	if e != nil {
		return e
	}
	ctx := r.Context()
	if e := h.renameGroup(ctx, c, g.ID, name, externalID); e != nil {
		return e
	}
	if err := h.store.setMembers(ctx, c.orgID, g.ID, refIDs(in.Members)); err != nil {
		return internal("set members", err)
	}
	h.record(ctx, c, "Updated team "+name)
	return h.getGroup(w, r, c)
}

func (h *Handler) renameGroup(ctx context.Context, c *call, id int64, name string, externalID *string) *scimError {
	err := h.store.updateGroup(ctx, c.orgID, id, name, externalID)
	if errors.Is(err, database.ErrConflict) {
		return conflict("a group named %q already exists", name)
	}
	if err != nil {
		return internal("update group", err)
	}
	return nil
}

func (h *Handler) patchGroup(w http.ResponseWriter, r *http.Request, c *call) *scimError {
	g, e := h.loadGroup(r.Context(), r, c, false)
	if e != nil {
		return e
	}
	var req patchRequest
	if e := decode(w, r, &req); e != nil {
		return e
	}
	ctx := r.Context()
	name, externalID := g.Name, g.ExternalID
	var added, removed int
	for _, raw := range req.Operations {
		ops, e := raw.expand()
		if e != nil {
			return e
		}
		for _, op := range ops {
			kind, e := op.kind()
			if e != nil {
				return e
			}
			path := op.path()
			if id, ok := memberFilter(path); ok && kind == "remove" {
				n, _ := strconv.ParseInt(id, 10, 64)
				if err := h.store.removeMembers(ctx, h.store.db, g.ID, []int64{n}); err != nil {
					return internal("remove member", err)
				}
				removed++
				continue
			}
			switch path {
			case "displayname":
				if kind == "remove" {
					return badRequest("mutability", "displayName can't be removed")
				}
				v, e := op.str()
				if e != nil {
					return e
				}
				if name, e = groupName(v); e != nil {
					return e
				}
			case "externalid":
				if kind == "remove" {
					externalID = nil
					continue
				}
				v, e := op.str()
				if e != nil {
					return e
				}
				if externalID, e = optText(&v, maxExternalIDLen, "externalId"); e != nil {
					return e
				}
			case "members":
				var refs []ref
				if len(op.Value) > 0 {
					if err := json.Unmarshal(op.Value, &refs); err != nil {
						return badRequest("invalidValue", "members must be a list of {\"value\": id}")
					}
				}
				ids := refIDs(refs)
				var err error
				switch kind {
				case "add":
					err = h.store.addMembers(ctx, h.store.db, c.orgID, g.ID, ids)
					added += len(ids)
				case "replace":
					err = h.store.setMembers(ctx, c.orgID, g.ID, ids)
					added += len(ids)
				case "remove":
					if len(op.Value) == 0 {
						err = h.store.setMembers(ctx, c.orgID, g.ID, nil)
					} else {
						err = h.store.removeMembers(ctx, h.store.db, g.ID, ids)
					}
					removed += max(len(ids), 1)
				}
				if err != nil {
					return internal("change members", err)
				}
			}
		}
	}
	if name != g.Name || !sameText(externalID, g.ExternalID) {
		if e := h.renameGroup(ctx, c, g.ID, name, externalID); e != nil {
			return e
		}
	}
	summary := "Updated team " + name
	switch {
	case added > 0 && removed == 0:
		summary = fmt.Sprintf("Added %d %s to team %s", added, plural(added, "person", "people"), name)
	case removed > 0 && added == 0:
		summary = fmt.Sprintf("Removed %d %s from team %s", removed, plural(removed, "person", "people"), name)
	}
	h.record(ctx, c, summary)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func sameText(a, b *string) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func (h *Handler) deleteGroup(w http.ResponseWriter, r *http.Request, c *call) *scimError {
	g, e := h.loadGroup(r.Context(), r, c, false)
	if e != nil {
		return e
	}
	if err := h.teams.DeleteTeam(r.Context(), c.orgID, g.ID); err != nil && !errors.Is(err, database.ErrNotFound) {
		return internal("delete group", err)
	}
	h.record(r.Context(), c, "Deleted team "+g.Name)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
