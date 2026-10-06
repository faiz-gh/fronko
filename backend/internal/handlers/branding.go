package handlers

import (
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
)

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
func validateBranding(b *models.OrgBranding) string {
	if b.LogoPolicy == "" {
		b.LogoPolicy = models.LogoOptional
	}
	if b.LogoPolicy != models.LogoRequired && b.LogoPolicy != models.LogoOptional {
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

// Protected: GET /api/org/branding. Everyone in the organisation reads it,
// since cards and signatures show the logo.
func (h *OrgHandler) GetBranding(w http.ResponseWriter, r *http.Request) {
	b, err := h.repo.GetOrgBranding(r.Context(), middleware.PrincipalFrom(r.Context()).OrgID)
	if err != nil {
		log.Printf("get org branding: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// Protected (admins): PUT /api/org/branding {"logo_file", "logo_policy", "signature"}
func (h *OrgHandler) UpdateBranding(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.PrincipalFrom(r.Context()).OrgID
	var b models.OrgBranding
	if !decodeJSON(w, r, &b) {
		return
	}
	if msg := validateBranding(&b); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	// The logo and banner must be images in the organisation's own files,
	// not someone's personal ones.
	var ids []string
	if b.LogoFile != nil {
		ids = append(ids, *b.LogoFile)
	}
	if b.Signature.BannerFile != "" {
		ids = append(ids, b.Signature.BannerFile)
	}
	files, err := h.repo.GetOrgFilesByPublicIDs(r.Context(), orgID, ids)
	if err != nil {
		log.Printf("branding files: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	images := map[string]bool{}
	for _, f := range files {
		images[f.PublicID] = f.Kind == "image" && f.Area != models.AreaPersonal
	}
	for _, id := range ids {
		if !images[id] {
			writeError(w, http.StatusBadRequest, "the logo and banner must be images in your organisation's files")
			return
		}
	}

	if err := h.repo.UpdateOrgBranding(r.Context(), orgID, &b); err != nil {
		log.Printf("update org branding: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, b)
}
