package cards

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

// schemePattern matches a leading URL scheme such as "https:" or "mailto:".
var schemePattern = regexp.MustCompile(`^(?i)[a-z][a-z0-9+.-]*:`)

// hostPattern is a dotted domain or IPv4 address, e.g. "ada.dev" or "10.0.0.1".
var hostPattern = regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*\.[a-z0-9-]{2,}$`)

// validWebURL reports whether s is a usable web address, as the card editor
// accepts it: http(s), or a bare host that gets https:// added. Text that
// isn't an address ("not a url") is rejected. Mirrors safeUrl in the frontend.
func validWebURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	if !schemePattern.MatchString(s) {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || strings.Contains(u.Host, "[") || hostPattern.MatchString(host)
}

// validateCardURLs checks the web addresses in a card's data and returns a
// user-facing error message, or "". Empty values are allowed.
func validateCardURLs(data json.RawMessage) string {
	var card struct {
		Website *string `json:"website"`
		Links   []struct {
			URL string `json:"url"`
		} `json:"links"`
	}
	if err := json.Unmarshal(data, &card); err != nil {
		return "data is not a valid card"
	}
	if card.Website != nil && strings.TrimSpace(*card.Website) != "" && !validWebURL(*card.Website) {
		return "website must be a valid web address"
	}
	for _, l := range card.Links {
		if strings.TrimSpace(l.URL) != "" && !validWebURL(l.URL) {
			return "links must be valid web addresses"
		}
	}
	return ""
}
