package sso

import (
	"context"
	"crypto/rand"
	"errors"
	"log"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
)

// An organisation proves it owns an email domain with a DNS TXT record on
// the domain itself: fronko-verification=<token>. Verified domains route
// sign-ins by email to the organisation's single sign-on, and let single
// sign-on and SCIM mark people's addresses as verified.

const (
	txtPrefix  = "fronko-verification="
	maxDomains = 20
)

// TXTResolver looks up DNS TXT records (net.DefaultResolver; tests fake it).
type TXTResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

var domainPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

// normalizeDomain accepts a bare domain, or an email address or URL to take
// it from.
func normalizeDomain(raw string) (string, bool) {
	d := strings.ToLower(strings.TrimSpace(raw))
	if i := strings.LastIndexByte(d, '@'); i >= 0 {
		d = d[i+1:]
	}
	d = strings.TrimPrefix(strings.TrimPrefix(d, "https://"), "http://")
	d, _, _ = strings.Cut(d, "/")
	d = strings.TrimSuffix(strings.TrimPrefix(d, "www."), ".")
	return d, len(d) <= 253 && domainPattern.MatchString(d)
}

// domainView is a domain with the record to create for it.
type domainView struct {
	*Domain
	TXTName  string `json:"txt_name"`
	TXTValue string `json:"txt_value"`
}

func viewOf(d *Domain) domainView {
	return domainView{Domain: d, TXTName: d.Domain, TXTValue: txtPrefix + d.Token}
}

// Protected (admins): GET /api/org/domains
func (h *Handler) ListDomains(w http.ResponseWriter, r *http.Request) {
	list, err := h.store.listDomains(r.Context(), auth.PrincipalFrom(r.Context()).OrgID)
	if err != nil {
		httpx.Internal("list domains", err).Write(w)
		return
	}
	out := make([]domainView, 0, len(list))
	for _, d := range list {
		out = append(out, viewOf(d))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Protected (admins): POST /api/org/domains {"domain"}
func (h *Handler) AddDomain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	domain, ok := normalizeDomain(req.Domain)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "enter a domain such as example.com")
		return
	}
	p := auth.PrincipalFrom(r.Context())
	n, err := h.store.countDomains(r.Context(), p.OrgID)
	if err != nil {
		httpx.Internal("count domains", err).Write(w)
		return
	}
	if n >= maxDomains {
		httpx.WriteError(w, http.StatusBadRequest, "an organisation can have at most 20 domains")
		return
	}
	d, err := h.store.addDomain(r.Context(), p.OrgID, domain, strings.ToLower(rand.Text()), p.UserID)
	if errors.Is(err, database.ErrConflict) {
		httpx.WriteError(w, http.StatusConflict, "you've already added this domain")
		return
	}
	if err != nil {
		httpx.Internal("add domain", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, viewOf(d))
}

// Protected (admins): POST /api/org/domains/{id}/verify. Looks for the TXT
// record; a domain another organisation verified first can't be verified.
func (h *Handler) VerifyDomain(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, ok := httpx.PathID(w, r, "id", "domain")
	if !ok {
		return
	}
	d, err := h.store.getDomain(r.Context(), p.OrgID, id)
	if errors.Is(err, database.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err != nil {
		httpx.Internal("get domain", err).Write(w)
		return
	}
	if d.VerifiedAt == nil {
		records, err := h.resolver.LookupTXT(r.Context(), d.Domain)
		var dnsErr *net.DNSError
		if err != nil && !(errors.As(err, &dnsErr) && (dnsErr.IsNotFound || dnsErr.IsTemporary)) {
			log.Printf("sso: TXT lookup for %s: %v", d.Domain, err)
		}
		want := txtPrefix + d.Token
		if !slices.ContainsFunc(records, func(rec string) bool { return strings.TrimSpace(rec) == want }) {
			httpx.WriteError(w, http.StatusBadRequest,
				"the TXT record wasn't found yet; DNS changes can take a while to appear, so try again later")
			return
		}
		err = h.store.markVerified(r.Context(), p.OrgID, d.ID)
		if errors.Is(err, database.ErrConflict) {
			httpx.WriteError(w, http.StatusConflict, "another organisation has already verified this domain")
			return
		}
		if err != nil {
			httpx.Internal("verify domain", err).Write(w)
			return
		}
		if d, err = h.store.getDomain(r.Context(), p.OrgID, id); err != nil {
			httpx.Internal("get domain", err).Write(w)
			return
		}
	}
	httpx.WriteJSON(w, http.StatusOK, viewOf(d))
}

// Protected (admins): DELETE /api/org/domains/{id}
func (h *Handler) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "id", "domain")
	if !ok {
		return
	}
	err := h.store.deleteDomain(r.Context(), auth.PrincipalFrom(r.Context()).OrgID, id)
	if errors.Is(err, database.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err != nil {
		httpx.Internal("delete domain", err).Write(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
