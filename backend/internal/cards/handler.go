package cards

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/analytics"
	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/branding"
	"github.com/faiz-gh/fronko/backend/internal/files"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// Slugs are URL path segments: lowercase letters, digits and single hyphens.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const (
	minSlugLen = 3
	maxSlugLen = 48
)

const errSlugTaken = "another card in your organisation already uses that link"

type ProfileHandler struct {
	store    *Store
	files    *files.Store
	branding *branding.Store
	users    *users.Store
	// frontendOrigins are where the app is served when it isn't proxied
	// same-origin (the CORS allow list); used to link back to a card.
	frontendOrigins []string
	// events counts contact saves; nil records nothing.
	events *analytics.EventRecorder
	// bookings finds cards' booking pages; nil shows none.
	bookings BookingFinder
}

func NewProfileHandler(store *Store, fileStore *files.Store, brandingStore *branding.Store, userStore *users.Store, frontendOrigins []string, events *analytics.EventRecorder, bookings BookingFinder) *ProfileHandler {
	return &ProfileHandler{
		bookings:        bookings,
		store:           store,
		files:           fileStore,
		branding:        brandingStore,
		users:           userStore,
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
		httpx.WriteError(w, http.StatusBadRequest, "invalid profile ID")
		return 0, false
	}
	return id, true
}

// Public: GET /api/profiles/{org}/{slug}, the card behind /p/{org}/{slug}.
func (h *ProfileHandler) GetPublicProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := h.store.GetProfileByPath(r.Context(), r.PathValue("org"), r.PathValue("slug"))
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "profile not found")
			return
		}
		log.Printf("get profile by slug: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// A suspended organisation's cards are offline. Visitors only learn that
	// the card is unavailable, not why.
	if profile.OrgSuspended {
		httpx.WriteJSON(w, http.StatusGone, map[string]string{"error": "this card is unavailable", "code": auth.CodeOrgSuspended})
		return
	}

	// Resolve only the files this card uses, and only ones its organisation still has.
	public := PublicProfile{ID: profile.ID, Slug: profile.Slug, OrgHandle: profile.OrgHandle, Data: profile.Data, Files: []files.PublicFile{}}
	if b, err := h.branding.GetOrgBranding(r.Context(), profile.OrgID); err != nil {
		log.Printf("profile org branding: %v", err)
	} else {
		public.Org = &branding.PublicOrg{Name: b.Name, LogoFile: b.LogoFile, LogoPolicy: b.LogoPolicy}
	}
	public.Booking = h.bookingsFor(r.Context(), profile.OrgID)(profile.holderID())
	used, err := h.files.GetOrgFilesByPublicIDs(r.Context(), profile.OrgID, files.ReferencedIDs(profile.Data))
	if err != nil {
		log.Printf("profile files: %v", err)
	}
	for _, f := range used {
		public.Files = append(public.Files, files.PublicFile{ID: f.PublicID, Kind: f.Kind, Name: f.OriginalName, SizeBytes: f.SizeBytes})
	}

	httpx.WriteJSON(w, http.StatusOK, public)
}

// getProfile loads a card the signed-in user can see, writing the error response if it can't.
func (h *ProfileHandler) getProfile(w http.ResponseWriter, r *http.Request, profileID int64) (*Profile, bool) {
	profile, err := h.store.GetProfile(r.Context(), auth.ScopeOf(r), profileID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "profile not found")
			return nil, false
		}
		log.Printf("get profile: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return nil, false
	}
	profile.Booking = h.bookingsFor(r.Context(), profile.OrgID)(profile.holderID())
	return profile, true
}

// bookingsFor returns the lookup of the organisation's booking pages. A
// failure is logged and shows no booking button rather than failing the card.
func (h *ProfileHandler) bookingsFor(ctx context.Context, orgID int64) func(int64) *Booking {
	none := func(int64) *Booking { return nil }
	if h.bookings == nil {
		return none
	}
	find, err := h.bookings(ctx, orgID)
	if err != nil {
		log.Printf("card bookings: %v", err)
		return none
	}
	return find
}

// checkFiles makes sure every file a card newly points at is one the signed-in
// user can see. Files the card already used (before) stay allowed, so losing
// access to one doesn't block saving the rest of the card.
func (h *ProfileHandler) checkFiles(w http.ResponseWriter, r *http.Request, data, before []byte) bool {
	existing := map[string]bool{}
	for _, id := range files.ReferencedIDs(before) {
		existing[id] = true
	}
	var added []string
	for _, id := range files.ReferencedIDs(data) {
		if !existing[id] {
			added = append(added, id)
		}
	}
	n, err := h.files.CountVisibleFiles(r.Context(), auth.ScopeOf(r), added)
	if err != nil {
		log.Printf("check card files: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return false
	}
	if n != len(added) {
		httpx.WriteError(w, http.StatusBadRequest, "this card uses a file you don't have access to")
		return false
	}
	return true
}

// Protected: GET /api/me/profiles. Admins get every card in the organisation;
// members only the cards assigned to them.
func (h *ProfileHandler) GetMyProfiles(w http.ResponseWriter, r *http.Request) {
	scope := auth.ScopeOf(r)
	profiles, err := h.store.ListProfiles(r.Context(), scope)
	if err != nil {
		log.Printf("list profiles: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	booking := h.bookingsFor(r.Context(), scope.OrgID)
	for _, p := range profiles {
		p.Booking = booking(p.holderID())
	}

	httpx.WriteJSON(w, http.StatusOK, profiles)
}

// Protected: GET /api/me/profiles/{id}
func (h *ProfileHandler) GetMyProfile(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromPath(w, r)
	if !ok {
		return
	}
	if profile, ok := h.getProfile(w, r, profileID); ok {
		httpx.WriteJSON(w, http.StatusOK, profile)
	}
}

// Protected (admins): POST /api/me/profiles. Optionally assigns the new card
// to a user straight away.
func (h *ProfileHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFrom(r.Context())

	var req struct {
		profileRequest
		AssignedUserID *int64 `json:"assigned_user_id"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		httpx.WriteError(w, http.StatusBadRequest, msg)
		return
	}
	if req.AssignedUserID != nil && !h.checkAssignee(w, r, *req.AssignedUserID) {
		return
	}
	if !h.checkFiles(w, r, req.Data, nil) {
		return
	}

	profile := &Profile{
		OrgID:          principal.OrgID,
		UserID:         principal.UserID,
		AssignedUserID: req.AssignedUserID,
		Slug:           req.Slug,
		Data:           req.Data,
	}

	if err := h.store.CreateProfile(r.Context(), profile); err != nil {
		if errors.Is(err, database.ErrConflict) {
			httpx.WriteError(w, http.StatusConflict, errSlugTaken)
			return
		}
		log.Printf("create profile: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to create profile")
		return
	}

	if created, ok := h.getProfile(w, r, profile.ID); ok {
		httpx.WriteJSON(w, http.StatusCreated, created)
	}
}

// Protected: PUT /api/me/profiles/{id}. Members can edit everything on their
// cards except the slug, which may already be printed on cards and QR codes.
func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	scope := auth.ScopeOf(r)
	profileID, ok := profileIDFromPath(w, r)
	if !ok {
		return
	}

	var req profileRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		httpx.WriteError(w, http.StatusBadRequest, msg)
		return
	}

	current, ok := h.getProfile(w, r, profileID)
	if !ok {
		return
	}
	if !scope.Admin && req.Slug != current.Slug {
		httpx.WriteError(w, http.StatusForbidden, "your organisation manages this card's link")
		return
	}
	if !h.checkFiles(w, r, req.Data, current.Data) {
		return
	}

	profile := &Profile{
		ID:   profileID,
		Slug: req.Slug,
		Data: req.Data,
	}

	// The WHERE clause enforces access; a card outside the scope reads as not found.
	if err := h.store.UpdateProfile(r.Context(), scope, profile); err != nil {
		switch {
		case errors.Is(err, database.ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, "profile not found")
		case errors.Is(err, database.ErrConflict):
			httpx.WriteError(w, http.StatusConflict, errSlugTaken)
		default:
			log.Printf("update profile: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "failed to update profile")
		}
		return
	}

	if updated, ok := h.getProfile(w, r, profileID); ok {
		httpx.WriteJSON(w, http.StatusOK, updated)
	}
}

// Protected (admins): DELETE /api/me/profiles/{id}
func (h *ProfileHandler) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromPath(w, r)
	if !ok {
		return
	}

	if err := h.store.DeleteProfile(r.Context(), profileID, auth.PrincipalFrom(r.Context()).OrgID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "profile not found")
			return
		}
		log.Printf("delete profile: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to delete profile")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// checkAssignee makes sure userID is someone in the caller's organisation.
func (h *ProfileHandler) checkAssignee(w http.ResponseWriter, r *http.Request, userID int64) bool {
	_, err := h.users.GetUserInOrg(r.Context(), userID, auth.PrincipalFrom(r.Context()).OrgID)
	if errors.Is(err, database.ErrNotFound) {
		httpx.WriteError(w, http.StatusBadRequest, "that user isn't in your organisation")
		return false
	}
	if err != nil {
		log.Printf("check assignee: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
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
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.UserID != nil && !h.checkAssignee(w, r, *req.UserID) {
		return
	}
	err := h.store.SetProfileAssignee(r.Context(), profileID, auth.PrincipalFrom(r.Context()).OrgID, req.UserID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "profile not found")
			return
		}
		log.Printf("set assignee: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if updated, ok := h.getProfile(w, r, profileID); ok {
		httpx.WriteJSON(w, http.StatusOK, updated)
	}
}
