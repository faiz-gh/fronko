package leads

import (
	"errors"
	"log"
	"math"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/analytics"
	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/cards"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
)

const (
	maxLeadNameLen  = 120
	maxLeadEmailLen = 254
	maxLeadNotesLen = 2000

	defaultLeadPageSize = 25
	maxLeadPageSize     = 100
	maxLeadSearchLen    = 100
)

var (
	dialCodePattern    = regexp.MustCompile(`^\+[1-9][0-9]{0,2}$`)
	phoneNumberPattern = regexp.MustCompile(`^[0-9]{4,14}$`)
	// Visual separators people type or paste; anything else is rejected.
	phoneSeparators = strings.NewReplacer(" ", "", "-", "", ".", "", "(", "", ")", "")
)

// maxPhoneDigits is the E.164 limit for dial code plus national number.
const maxPhoneDigits = 15

// normalizeLeadPhone strips separators and checks the two phone parts. Both
// empty means no phone; otherwise both must be present and well-formed. The
// result is digits only, matching the leads_phone_format check constraint.
func normalizeLeadPhone(code, number string) (string, string, bool) {
	code = phoneSeparators.Replace(strings.TrimSpace(code))
	number = phoneSeparators.Replace(strings.TrimSpace(number))
	if code == "" && number == "" {
		return "", "", true
	}
	if code != "" && !strings.HasPrefix(code, "+") {
		code = "+" + code
	}
	if !dialCodePattern.MatchString(code) || !phoneNumberPattern.MatchString(number) {
		return "", "", false
	}
	if len(code)-1+len(number) > maxPhoneDigits {
		return "", "", false
	}
	return code, number, true
}

// LeadPage is one page of leads plus the paging state needed to render controls.
type LeadPage struct {
	Leads    []*Lead `json:"leads"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}

type LeadHandler struct {
	store *Store
	cards *cards.Store
	// events counts sent contact forms; nil records nothing.
	events *analytics.EventRecorder
}

func NewLeadHandler(store *Store, cardStore *cards.Store, events *analytics.EventRecorder) *LeadHandler {
	return &LeadHandler{
		store:  store,
		cards:  cardStore,
		events: events,
	}
}

// Public: POST /api/profiles/{id}/leads
func (h *LeadHandler) SubmitLead(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromPath(w, r)
	if !ok {
		return
	}

	var req struct {
		Name             string `json:"name"`
		Email            string `json:"email"`
		PhoneCountryCode string `json:"phone_country_code"`
		PhoneNumber      string `json:"phone_number"`
		Notes            string `json:"notes"`
		// How the visitor reached the card (nfc, qr or link) and their visit's id, for analytics.
		Source  string `json:"source"`
		Session string `json:"session"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	req.Notes = strings.TrimSpace(req.Notes)

	if req.Name == "" || len(req.Name) > maxLeadNameLen {
		httpx.WriteError(w, http.StatusBadRequest, "please enter your name")
		return
	}
	if addr, err := mail.ParseAddress(req.Email); err != nil || addr.Address != req.Email || len(req.Email) > maxLeadEmailLen {
		httpx.WriteError(w, http.StatusBadRequest, "please enter a valid email address")
		return
	}
	phoneCode, phoneNumber, ok := normalizeLeadPhone(req.PhoneCountryCode, req.PhoneNumber)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "please enter a valid mobile number with its country code")
		return
	}
	if len(req.Notes) > maxLeadNotesLen {
		httpx.WriteError(w, http.StatusBadRequest, "message is too long")
		return
	}

	lead := &Lead{
		ProfileID:        profileID,
		Name:             req.Name,
		Email:            req.Email,
		PhoneCountryCode: phoneCode,
		PhoneNumber:      phoneNumber,
		Notes:            req.Notes,
		Source:           analytics.NormalizeSource(req.Source),
	}

	// A missing profile surfaces as a foreign key violation, mapped to database.ErrNotFound.
	if err := h.store.CreateLead(r.Context(), lead); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "profile not found")
			return
		}
		log.Printf("create lead: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to submit lead")
		return
	}
	// The lead is the record; the event only puts the send on the visit's timeline.
	h.events.Record(r, profileID, 0, lead.Source, req.Session, analytics.CardEvent{Type: analytics.EventFormSubmit})

	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// Protected: GET /api/me/profiles/{id}/leads. Unpaginated; members only see
// the leads that arrived while they held the card.
func (h *LeadHandler) GetLeads(w http.ResponseWriter, r *http.Request) {
	scope := auth.ScopeOf(r)
	profileID, ok := profileIDFromPath(w, r)
	if !ok {
		return
	}

	if _, err := h.cards.GetProfile(r.Context(), scope, profileID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "profile not found")
			return
		}
		log.Printf("get leads ownership: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	leads, _, err := h.store.ListLeads(r.Context(), scope, LeadFilter{ProfileID: profileID, Limit: math.MaxInt32})
	if err != nil {
		log.Printf("get leads: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, leads)
}

// Protected: GET /api/me/leads?profile_id=&user_id=&q=&since=&page=&page_size=
// Lists the leads the caller can see, newest first: every lead in the
// organisation for admins, a member's own leads otherwise, plus their
// teammates' for team leads. user_id (admins and team leads) keeps the leads
// that arrived while that user held the card; "none" (admins only) keeps those
// that arrived while the organisation held it. team_id (admins and team
// leads) keeps the leads of that team's people. Every parameter is optional;
// page is 1-based.
func (h *LeadHandler) ListLeads(w http.ResponseWriter, r *http.Request) {
	scope := auth.ScopeOf(r)
	principal := auth.PrincipalFrom(r.Context())
	query := r.URL.Query()

	page, pageSize, ok := httpx.PageParams(w, r, defaultLeadPageSize, maxLeadPageSize)
	if !ok {
		return
	}

	filter := LeadFilter{
		Search: strings.TrimSpace(query.Get("q")),
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	}
	if len(filter.Search) > maxLeadSearchLen {
		httpx.WriteError(w, http.StatusBadRequest, "search is too long")
		return
	}
	if raw := query.Get("profile_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			httpx.WriteError(w, http.StatusBadRequest, "invalid profile_id")
			return
		}
		filter.ProfileID = id
	}
	if raw := query.Get("user_id"); raw != "" {
		if !scope.Admin && !(principal.LeadsAnyTeam() && raw != "none") {
			httpx.WriteError(w, http.StatusForbidden, "only your organisation's admins can filter by user")
			return
		}
		if raw == "none" {
			filter.Unassigned = true
		} else {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id < 1 {
				httpx.WriteError(w, http.StatusBadRequest, "invalid user_id")
				return
			}
			filter.UserID = id
		}
	}
	if raw := query.Get("team_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			httpx.WriteError(w, http.StatusBadRequest, "invalid team_id")
			return
		}
		if !scope.Admin && !principal.LeadsTeam(id) {
			httpx.WriteError(w, http.StatusForbidden, "you can only filter by teams you lead")
			return
		}
		filter.TeamID = id
	}
	if raw := query.Get("since"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "since must be an RFC 3339 timestamp")
			return
		}
		filter.Since = t
	}

	leads, total, err := h.store.ListLeads(r.Context(), scope, filter)
	if err != nil {
		log.Printf("list leads: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, LeadPage{Leads: leads, Total: total, Page: page, PageSize: pageSize})
}

func profileIDFromPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid profile ID")
		return 0, false
	}
	return id, true
}
