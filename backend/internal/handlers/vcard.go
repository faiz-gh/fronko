package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
)

// vcardFields are the card data keys that end up in a contact file.
type vcardFields struct {
	Name             string `json:"name"`
	Title            string `json:"title"`
	Company          string `json:"company"`
	Bio              string `json:"bio"`
	Email            string `json:"email"`
	PhoneCountryCode string `json:"phone_country_code"`
	PhoneNumber      string `json:"phone_number"`
	Website          string `json:"website"`
	Location         string `json:"location"`
}

var vcardEscaper = strings.NewReplacer(`\`, `\\`, "\r\n", `\n`, "\n", `\n`, "\r", `\n`, ",", `\,`, ";", `\;`)

func vEscape(s string) string { return vcardEscaper.Replace(s) }

// safeWebURL returns raw as an absolute http(s) URL, adding https:// to bare
// hosts the way the card editor does, or "" if it isn't one.
func safeWebURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

// buildVCard renders card data as a vCard 3.0 file, which iOS and Android
// both import as a contact.
func buildVCard(data []byte, profileURL string) string {
	var c vcardFields
	_ = json.Unmarshal(data, &c) // a malformed card still gets a named contact

	name := strings.Join(strings.Fields(c.Name), " ")
	if name == "" {
		name = "Contact"
	}
	parts := strings.Fields(name)
	first, last := parts[0], ""
	if len(parts) > 1 {
		first, last = strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
	}

	lines := []string{"BEGIN:VCARD", "VERSION:3.0", "N:" + vEscape(last) + ";" + vEscape(first) + ";;;", "FN:" + vEscape(name)}
	add := func(prefix, value string) {
		if value = strings.TrimSpace(value); value != "" {
			lines = append(lines, prefix+vEscape(value))
		}
	}
	add("ORG:", c.Company)
	add("TITLE:", c.Title)
	add("EMAIL;TYPE=INTERNET:", c.Email)
	if c.PhoneNumber != "" {
		add("TEL;TYPE=CELL:", c.PhoneCountryCode+c.PhoneNumber)
	}
	add("URL:", safeWebURL(c.Website))
	add("URL:", profileURL)
	if loc := strings.TrimSpace(c.Location); loc != "" {
		lines = append(lines, "ADR;TYPE=WORK:;;;"+vEscape(loc)+";;;")
	}
	add("NOTE:", c.Bio)
	lines = append(lines, "END:VCARD")
	return strings.Join(lines, "\r\n") + "\r\n"
}

// frontendOrigin is where the visitor is viewing the card: the page that sent
// them here when it's a known frontend origin, otherwise this request's own
// origin (nginx proxies the API same-origin and keeps the Host header).
func frontendOrigin(r *http.Request, allowed []string) string {
	if ref, err := url.Parse(r.Referer()); err == nil && ref.Host != "" {
		origin := ref.Scheme + "://" + ref.Host
		for _, a := range allowed {
			if strings.TrimSpace(a) == origin {
				return origin
			}
		}
	}
	return requestOrigin(r)
}

func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// Public: GET /api/profiles/{org}/{slug}/vcard[?via=nfc|qr|link&s=<session>].
// Opening it shows the phone's "Add contact" sheet, which is how a card's
// "save contact" tap works. Each download counts as a contact save; via and s
// tie it to the visit that asked for it.
func (h *ProfileHandler) VCard(w http.ResponseWriter, r *http.Request) {
	profile, err := h.repo.GetProfileByPath(r.Context(), r.PathValue("org"), r.PathValue("slug"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "profile not found")
			return
		}
		log.Printf("vcard: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if profile.OrgSuspended {
		writeError(w, http.StatusGone, "this card is unavailable")
		return
	}

	q := r.URL.Query()
	h.events.Record(r, profile.ID, profile.OrgID, q.Get("via"), q.Get("s"), repository.CardEvent{Type: models.EventVCard})

	w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="`+profile.Slug+`.vcf"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(buildVCard(profile.Data, frontendOrigin(r, h.frontendOrigins)+"/p/"+profile.OrgHandle+"/"+profile.Slug)))
}
