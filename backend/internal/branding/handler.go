package branding

import (
	"log"
	"net/http"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/files"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
)

// Handler serves the organisation's branding settings.
type Handler struct {
	store *Store
	files *files.Store
}

func NewHandler(store *Store, fileStore *files.Store) *Handler {
	return &Handler{store: store, files: fileStore}
}

// Protected: GET /api/org/branding. Everyone in the organisation reads it,
// since cards and signatures show the logo.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.GetOrgBranding(r.Context(), auth.PrincipalFrom(r.Context()).OrgID)
	if err != nil {
		log.Printf("get org branding: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, b)
}

// Protected (admins): PUT /api/org/branding {"logo_file", "logo_policy", "signature"}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	orgID := auth.PrincipalFrom(r.Context()).OrgID
	var b OrgBranding
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if msg := validateBranding(&b); msg != "" {
		httpx.WriteError(w, http.StatusBadRequest, msg)
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
	orgFiles, err := h.files.GetOrgFilesByPublicIDs(r.Context(), orgID, ids)
	if err != nil {
		log.Printf("branding files: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	images := map[string]bool{}
	for _, f := range orgFiles {
		images[f.PublicID] = f.Kind == "image" && f.Area != files.AreaPersonal
	}
	for _, id := range ids {
		if !images[id] {
			httpx.WriteError(w, http.StatusBadRequest, "the logo and banner must be images in your organisation's files")
			return
		}
	}

	if err := h.store.UpdateOrgBranding(r.Context(), orgID, &b); err != nil {
		log.Printf("update org branding: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, b)
}
