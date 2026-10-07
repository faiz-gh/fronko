package branding

import (
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Logo policies: whether every card and signature shows the organisation's
// logo, or each card chooses.
const (
	LogoRequired = "required"
	LogoOptional = "optional"
)

// OrgSignature is the organisation's email signature settings.
type OrgSignature struct {
	// LockedTemplate, when set, is the only signature template employees can use.
	LockedTemplate string `json:"locked_template,omitempty"`
	// BrandColor (#rrggbb), when set, replaces each card's accent in signatures.
	BrandColor string `json:"brand_color,omitempty"`
	Disclaimer string `json:"disclaimer,omitempty"`
	// BannerFile is an org image shown under every signature, linking to BannerURL.
	BannerFile string `json:"banner_file,omitempty"`
	BannerURL  string `json:"banner_url,omitempty"`
}

// OrgBranding is how an organisation appears on its cards and email signatures.
type OrgBranding struct {
	Name       string       `json:"name"`
	LogoFile   *string      `json:"logo_file"`
	LogoPolicy string       `json:"logo_policy"`
	Signature  OrgSignature `json:"signature"`
}

// PublicOrg is the branding a public card needs.
type PublicOrg struct {
	Name       string  `json:"name"`
	LogoFile   *string `json:"logo_file"`
	LogoPolicy string  `json:"logo_policy"`
}

// signatureTemplates are the email signature templates the frontend renders;
// an organisation can lock its employees to one of them.
var signatureTemplates = map[string]bool{
	"classic": true, "corporate": true, "compact": true, "bold": true, "minimal": true,
}

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

const (
	maxDisclaimerLen = 1000
	maxBannerURLLen  = 2048
)

// validateBranding normalises b in place and returns a message for the
// first invalid field, or "" when it's fine.
func validateBranding(b *OrgBranding) string {
	if b.LogoPolicy == "" {
		b.LogoPolicy = LogoOptional
	}
	if b.LogoPolicy != LogoRequired && b.LogoPolicy != LogoOptional {
		return "logo policy must be required or optional"
	}
	if b.LogoFile != nil && strings.TrimSpace(*b.LogoFile) == "" {
		b.LogoFile = nil
	}

	s := &b.Signature
	s.LockedTemplate = strings.TrimSpace(s.LockedTemplate)
	if s.LockedTemplate != "" && !signatureTemplates[s.LockedTemplate] {
		return "unknown signature template"
	}
	s.BrandColor = strings.TrimSpace(s.BrandColor)
	if s.BrandColor != "" && !hexColorPattern.MatchString(s.BrandColor) {
		return "brand colour must look like #1a2b3c"
	}
	s.Disclaimer = strings.TrimSpace(s.Disclaimer)
	if utf8.RuneCountInString(s.Disclaimer) > maxDisclaimerLen {
		return "disclaimer must be at most 1000 characters"
	}
	s.BannerFile = strings.TrimSpace(s.BannerFile)
	s.BannerURL = strings.TrimSpace(s.BannerURL)
	if s.BannerURL != "" {
		u, err := url.Parse(s.BannerURL)
		if err != nil || len(s.BannerURL) > maxBannerURLLen || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return "banner link must be a full http(s) URL"
		}
	}
	return ""
}
