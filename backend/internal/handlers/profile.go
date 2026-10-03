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

type ProfileHandler struct {
	repo *repository.Repository
}

func NewProfileHandler(repo *repository.Repository) *ProfileHandler {
	return &ProfileHandler{
		repo: repo,
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

// Public: GET /api/profiles/{slug}
func (h *ProfileHandler) GetProfileBySlug(w http.ResponseWriter, r *http.Request) {
	profile, err := h.repo.GetProfileBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "profile not found")
			return
		}
		log.Printf("get profile by slug: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Resolve only the files this card uses, and only ones its owner still has.
	public := models.PublicProfile{ID: profile.ID, Slug: profile.Slug, Data: profile.Data, Files: []models.PublicFile{}}
	files, err := h.repo.GetFilesByPublicIDs(r.Context(), profile.UserID, referencedFileIDs(profile.Data))
	if err != nil {
		log.Printf("profile files: %v", err)
	}
	for _, f := range files {
		public.Files = append(public.Files, models.PublicFile{ID: f.PublicID, Kind: f.Kind, Name: f.OriginalName, SizeBytes: f.SizeBytes})
	}

	writeJSON(w, http.StatusOK, public)
}

// Protected: GET /api/me/profiles
func (h *ProfileHandler) GetMyProfiles(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r.Context())

	profiles, err := h.repo.GetProfilesByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("list profiles: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, profiles)
}

// Protected: GET /api/me/profiles/{id}
func (h *ProfileHandler) GetMyProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r.Context())
	profileID, ok := profileIDFromPath(w, r)
	if !ok {
		return
	}

	profile, err := h.repo.GetProfileForUser(r.Context(), profileID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "profile not found")
			return
		}
		log.Printf("get profile: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// Protected: POST /api/me/profiles
func (h *ProfileHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r.Context())

	var req profileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	profile := &models.Profile{
		UserID: userID,
		Slug:   req.Slug,
		Data:   req.Data,
	}

	if err := h.repo.CreateProfile(r.Context(), profile); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			writeError(w, http.StatusConflict, "that slug is already taken")
			return
		}
		log.Printf("create profile: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create profile")
		return
	}

	writeJSON(w, http.StatusCreated, profile)
}

// Protected: PUT /api/me/profiles/{id}
func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r.Context())
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

	profile := &models.Profile{
		ID:     profileID,
		UserID: userID,
		Slug:   req.Slug,
		Data:   req.Data,
	}

	// The WHERE clause enforces ownership; another user's profile reads as not found.
	if err := h.repo.UpdateProfile(r.Context(), profile); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			writeError(w, http.StatusNotFound, "profile not found")
		case errors.Is(err, repository.ErrConflict):
			writeError(w, http.StatusConflict, "that slug is already taken")
		default:
			log.Printf("update profile: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to update profile")
		}
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// Protected: DELETE /api/me/profiles/{id}
func (h *ProfileHandler) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r.Context())
	profileID, ok := profileIDFromPath(w, r)
	if !ok {
		return
	}

	if err := h.repo.DeleteProfile(r.Context(), profileID, userID); err != nil {
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
