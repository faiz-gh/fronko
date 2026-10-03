package handlers

import (
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
)

const (
	maxLeadNameLen  = 120
	maxLeadEmailLen = 254
	maxLeadNotesLen = 2000

	defaultLeadPageSize = 25
	maxLeadPageSize     = 100
	maxLeadSearchLen    = 100
)

// LeadPage is one page of leads plus the paging state needed to render controls.
type LeadPage struct {
	Leads    []*models.Lead `json:"leads"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

type LeadHandler struct {
	repo *repository.Repository
}

func NewLeadHandler(repo *repository.Repository) *LeadHandler {
	return &LeadHandler{
		repo: repo,
	}
}

// Public: POST /api/profiles/{id}/leads
func (h *LeadHandler) SubmitLead(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromPath(w, r)
	if !ok {
		return
	}

	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Notes string `json:"notes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	req.Notes = strings.TrimSpace(req.Notes)

	if req.Name == "" || len(req.Name) > maxLeadNameLen {
		writeError(w, http.StatusBadRequest, "please enter your name")
		return
	}
	if addr, err := mail.ParseAddress(req.Email); err != nil || addr.Address != req.Email || len(req.Email) > maxLeadEmailLen {
		writeError(w, http.StatusBadRequest, "please enter a valid email address")
		return
	}
	if len(req.Notes) > maxLeadNotesLen {
		writeError(w, http.StatusBadRequest, "message is too long")
		return
	}

	lead := &models.Lead{
		ProfileID: profileID,
		Name:      req.Name,
		Email:     req.Email,
		Notes:     req.Notes,
	}

	// A missing profile surfaces as a foreign key violation, mapped to ErrNotFound.
	if err := h.repo.CreateLead(r.Context(), lead); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "profile not found")
			return
		}
		log.Printf("create lead: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to submit lead")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// Protected: GET /api/me/profiles/{id}/leads
func (h *LeadHandler) GetLeads(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r.Context())
	profileID, ok := profileIDFromPath(w, r)
	if !ok {
		return
	}

	if _, err := h.repo.GetProfileForUser(r.Context(), profileID, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "profile not found")
			return
		}
		log.Printf("get leads ownership: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	leads, err := h.repo.GetLeadsByProfileID(r.Context(), profileID)
	if err != nil {
		log.Printf("get leads: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, leads)
}

// Protected: GET /api/me/leads?profile_id=&q=&since=&page=&page_size=
// Lists leads across all of the caller's profiles, newest first. Every
// parameter is optional; page is 1-based.
func (h *LeadHandler) ListLeads(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r.Context())
	query := r.URL.Query()

	page, pageSize, ok := pageParams(w, r, defaultLeadPageSize, maxLeadPageSize)
	if !ok {
		return
	}

	filter := repository.LeadFilter{
		Search: strings.TrimSpace(query.Get("q")),
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	}
	if len(filter.Search) > maxLeadSearchLen {
		writeError(w, http.StatusBadRequest, "search is too long")
		return
	}
	if raw := query.Get("profile_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			writeError(w, http.StatusBadRequest, "invalid profile_id")
			return
		}
		filter.ProfileID = id
	}
	if raw := query.Get("since"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "since must be an RFC 3339 timestamp")
			return
		}
		filter.Since = t
	}

	leads, total, err := h.repo.ListLeadsForUser(r.Context(), userID, filter)
	if err != nil {
		log.Printf("list leads: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, LeadPage{Leads: leads, Total: total, Page: page, PageSize: pageSize})
}
