package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
)

// Slugs are URL path segments: lowercase letters, digits and single hyphens.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const (
	minSlugLen = 3
	maxSlugLen = 48
)

const errSlugTaken = "another card in your organisation already uses that link"

// validHandle checks an organisation's link handle: the same characters as
// a card slug, 3-32 long.
func validHandle(handle string) bool {
	return len(handle) >= repository.MinHandleLen && len(handle) <= repository.MaxHandleLen && slugPattern.MatchString(handle)
}

type ProfileHandler struct {
	repo *repository.Repository
	// frontendOrigins are where the app is served when it isn't proxied
	// same-origin (the CORS allow list); used to link back to a card.
	frontendOrigins []string
	// events counts contact saves; nil records nothing.
	events *EventRecorder
}

func NewProfileHandler(repo *repository.Repository, frontendOrigins []string, events *EventRecorder) *ProfileHandler {
	return &ProfileHandler{
		repo:            repo,
		frontendOrigins: frontendOrigins,
		events:          events,
	}
}

type profileRequest struct {
	Slug string          `json:"slug"`
	Data json.RawMessage `json:"data"`
}

// validate normalizes the request and returns a user-facing error message, or "".
func (req *profileRequest) validate() string {
	req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	if len(req.Slug) < minSlugLen || len(req.Slug) > maxSlugLen || !slugPattern.MatchString(req.Slug) {
		return "slug must be 3-48 characters: lowercase letters, numbers and hyphens"
	}

	data := bytes.TrimSpace(req.Data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		req.Data = json.RawMessage(`{}`)
		return ""
	}
	if data[0] != '{' {
		return "data must be a JSON object"
	}
	return ""
}

func profileIDFromPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid profile ID")
		return 0, false
	}
	return id, true
}

// Public: GET /api/profiles/{org}/{slug}, the card behind /p/{org}/{slug}.
func (h *ProfileHandler) GetPublicProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := h.repo.GetProfileByPath(r.Context(), r.PathValue("org"), r.PathValue("slug"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "profile not found")
			return
		}
		log.Printf("get profile by slug: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// A suspended organisation's cards are offline. Visitors only learn that
	// the card is unavailable, not why.
	if profile.OrgSuspended {
		writeJSON(w, http.StatusGone, map[string]string{"error": "this card is unavailable", "code": middleware.CodeOrgSuspended})
		return
	}

	// Resolve only the files this card uses, and only ones its organisation still has.
	public := models.PublicProfile{ID: profile.ID, Slug: profile.Slug, OrgHandle: profile.OrgHandle, Data: profile.Data, Files: []models.PublicFile{}}
	if b, err := h.repo.GetOrgBranding(r.Context(), profile.OrgID); err != nil {
		log.Printf("profile org branding: %v", err)
	} else {
		public.Org = &models.PublicOrg{Name: b.Name, LogoFile: b.LogoFile, LogoPolicy: b.LogoPolicy}
	}
	files, err := h.repo.GetOrgFilesByPublicIDs(r.Context(), profile.OrgID, referencedFileIDs(profile.Data))
	if err != nil {
		log.Printf("profile files: %v", err)
	}
	for _, f := range files {
		public.Files = append(public.Files, models.PublicFile{ID: f.PublicID, Kind: f.Kind, Name: f.OriginalName, SizeBytes: f.SizeBytes})
	}

	writeJSON(w, http.StatusOK, public)
}

// getProfile loads a card the signed-in user can see, writing the error response if it can't.
func (h *ProfileHandler) getProfile(w http.ResponseWriter, r *http.Request, profileID int64) (*models.Profile, bool) {
	profile, err := h.repo.GetProfile(r.Context(), scopeOf(r), profileID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "profile not found")
			return nil, false
		}
		log.Printf("get profile: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return nil, false
	}
	return profile, true
}

// checkFiles makes sure every file a card newly points at is one the signed-in
// user can see. Files the card already used (before) stay allowed, so losing
// access to one doesn't block saving the rest of the card.
func (h *ProfileHandler) checkFiles(w http.ResponseWriter, r *http.Request, data, before []byte) bool {
	existing := map[string]bool{}
	for _, id := range referencedFileIDs(before) {
		existing[id] = true
	}
	var added []string
	for _, id := range referencedFileIDs(data) {
		if !existing[id] {
			added = append(added, id)
		}
	}
	n, err := h.repo.CountVisibleFiles(r.Context(), scopeOf(r), added)
	if err != nil {
		log.Printf("check card files: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return false
	}
	if n != len(added) {
		writeError(w, http.StatusBadRequest, "this card uses a file you don't have access to")
		return false
	}
	return true
}

// Protected: GET /api/me/profiles. Admins get every card in the organisation;
// members only the cards assigned to them.
func (h *ProfileHandler) GetMyProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.repo.ListProfiles(r.Context(), scopeOf(r))
	if err != nil {
		log.Printf("list profiles: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, profiles)
}

// Protected: GET /api/me/profiles/{id}
func (h *ProfileHandler) GetMyProfile(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromPath(w, r)
	if !ok {
		return
	}
	if profile, ok := h.getProfile(w, r, profileID); ok {
		writeJSON(w, http.StatusOK, profile)
	}
}

// Protected (admins): POST /api/me/profiles. Optionally assigns the new card
// to a user straight away.
func (h *ProfileHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	principal := middleware.PrincipalFrom(r.Context())

	var req struct {
		profileRequest
		AssignedUserID *int64 `json:"assigned_user_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if req.AssignedUserID != nil && !h.checkAssignee(w, r, *req.AssignedUserID) {
		return
	}
	if !h.checkFiles(w, r, req.Data, nil) {
		return
	}

	profile := &models.Profile{
		OrgID:          principal.OrgID,
		UserID:         principal.UserID,
		AssignedUserID: req.AssignedUserID,
		Slug:           req.Slug,
		Data:           req.Data,
	}

	if err := h.repo.CreateProfile(r.Context(), profile); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			writeError(w, http.StatusConflict, errSlugTaken)
			return
		}
		log.Printf("create profile: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create profile")
		return
	}

	if created, ok := h.getProfile(w, r, profile.ID); ok {
		writeJSON(w, http.StatusCreated, created)
	}
}

// Protected: PUT /api/me/profiles/{id}. Members can edit everything on their
// cards except the slug, which may already be printed on cards and QR codes.
func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	scope := scopeOf(r)
	profileID, ok := profileIDFromPath(w, r)
	if !ok {
		return
	}

	var req profileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	current, ok := h.getProfile(w, r, profileID)
	if !ok {
		return
	}
	if !scope.Admin && req.Slug != current.Slug {
		writeError(w, http.StatusForbidden, "your organisation manages this card's link")
		return
	}
	if !h.checkFiles(w, r, req.Data, current.Data) {
		return
	}

	profile := &models.Profile{
		ID:   profileID,
		Slug: req.Slug,
		Data: req.Data,
	}

	// The WHERE clause enforces access; a card outside the scope reads as not found.
	if err := h.repo.UpdateProfile(r.Context(), scope, profile); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			writeError(w, http.StatusNotFound, "profile not found")
		case errors.Is(err, repository.ErrConflict):
			writeError(w, http.StatusConflict, errSlugTaken)
		default:
			log.Printf("update profile: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to update profile")
		}
		return
	}

	if updated, ok := h.getProfile(w, r, profileID); ok {
		writeJSON(w, http.StatusOK, updated)
	}
}

// Protected (admins): DELETE /api/me/profiles/{id}
func (h *ProfileHandler) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromPath(w, r)
	if !ok {
		return
	}

	if err := h.repo.DeleteProfile(r.Context(), profileID, middleware.PrincipalFrom(r.Context()).OrgID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "profile not found")
			return
		}
		log.Printf("delete profile: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete profile")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// checkAssignee makes sure userID is someone in the caller's organisation.
func (h *ProfileHandler) checkAssignee(w http.ResponseWriter, r *http.Request, userID int64) bool {
	_, err := h.repo.GetUserInOrg(r.Context(), userID, middleware.PrincipalFrom(r.Context()).OrgID)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "that user isn't in your organisation")
		return false
	}
	if err != nil {
		log.Printf("check assignee: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return false
	}
	return true
}

// Protected (admins): PUT /api/org/profiles/{id}/assignee. Hands the card to
// a user, or back to the organisation with {"user_id": null}. Leads that
// already arrived stay with whoever held the card at the time.
func (h *ProfileHandler) SetAssignee(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromPath(w, r)
	if !ok {
		return
	}
	var req struct {
		UserID *int64 `json:"user_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.UserID != nil && !h.checkAssignee(w, r, *req.UserID) {
		return
	}
	err := h.repo.SetProfileAssignee(r.Context(), profileID, middleware.PrincipalFrom(r.Context()).OrgID, req.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "profile not found")
			return
		}
		log.Printf("set assignee: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if updated, ok := h.getProfile(w, r, profileID); ok {
		writeJSON(w, http.StatusOK, updated)
	}
}
