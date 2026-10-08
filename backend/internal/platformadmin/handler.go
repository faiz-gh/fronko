package platformadmin

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/faiz-gh/fronko/backend/internal/feedback"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
	"github.com/faiz-gh/fronko/backend/internal/platform/mail"
)

const (
	maxSuspendReasonLen = 500
	maxReplyLen         = 5000
	defaultTrendDays    = 30
	maxTrendDays        = 366
)

// trendDays reads the `days` query parameter, writing a 400 if it's out of range.
func trendDays(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("days")
	if raw == "" {
		return defaultTrendDays, true
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days < 1 || days > maxTrendDays {
		httpx.WriteError(w, http.StatusBadRequest, "invalid days")
		return 0, false
	}
	return days, true
}

type page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// Admin: GET /api/admin/summary.
func (h *AdminHandler) Summary(w http.ResponseWriter, r *http.Request) {
	s, err := h.store.PlatformSummary(r.Context())
	if err != nil {
		log.Printf("platform summary: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}

// Admin: GET /api/admin/orgs?q=&status=&sort=&page=&page_size=.
func (h *AdminHandler) ListOrgs(w http.ResponseWriter, r *http.Request) {
	pg, size, ok := httpx.PageParams(w, r, 25, 100)
	if !ok {
		return
	}
	q := r.URL.Query()
	f := OrgUsageFilter{
		Search: q.Get("q"), Status: q.Get("status"), Sort: q.Get("sort"),
		Limit: size, Offset: (pg - 1) * size,
	}
	if f.Status != "" && f.Status != "active" && f.Status != "suspended" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid status")
		return
	}
	if !ValidOrgUsageSort(f.Sort) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid sort")
		return
	}
	orgs, total, err := h.store.ListOrgUsage(r.Context(), f)
	if err != nil {
		log.Printf("list org usage: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, page[*OrgUsage]{Items: orgs, Total: total, Page: pg, PageSize: size})
}

// getOrg loads an organisation's usage, writing the error response if it can't.
func (h *AdminHandler) getOrg(w http.ResponseWriter, r *http.Request) (*OrgUsage, bool) {
	id, ok := httpx.PathID(w, r, "id", "organisation")
	if !ok {
		return nil, false
	}
	org, err := h.store.GetOrgUsage(r.Context(), id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "organisation not found")
			return nil, false
		}
		log.Printf("get org usage: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return nil, false
	}
	return org, true
}

// Admin: GET /api/admin/orgs/{id}.
func (h *AdminHandler) GetOrg(w http.ResponseWriter, r *http.Request) {
	if org, ok := h.getOrg(w, r); ok {
		httpx.WriteJSON(w, http.StatusOK, org)
	}
}

// Admin: GET /api/admin/trends?days=.
func (h *AdminHandler) PlatformTrend(w http.ResponseWriter, r *http.Request) {
	days, ok := trendDays(w, r)
	if !ok {
		return
	}
	points, err := h.store.PlatformTrend(r.Context(), days)
	if err != nil {
		log.Printf("platform trend: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"days": days, "points": points})
}

// Admin: GET /api/admin/orgs/{id}/trends?days=.
func (h *AdminHandler) OrgTrend(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "id", "organisation")
	if !ok {
		return
	}
	days, ok := trendDays(w, r)
	if !ok {
		return
	}
	points, err := h.store.OrgTrend(r.Context(), id, days)
	if err != nil {
		log.Printf("org trend: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"days": days, "points": points})
}

// validSuspendReason checks the reason an admin gives: 1-500 characters, one
// line, since it goes into an email to the owner and onto the login screen.
func validSuspendReason(reason string) bool {
	n := utf8.RuneCountInString(reason)
	return n > 0 && n <= maxSuspendReasonLen && !strings.ContainsFunc(reason, unicode.IsControl)
}

// notifyOwner emails an organisation's owner, reporting whether it went out.
func (h *AdminHandler) notifyOwner(r *http.Request, org *OrgUsage, m mail.Message) bool {
	if org.OwnerEmail == nil {
		return false
	}
	m.ReplyTo = h.replyTo
	ctx, cancel := context.WithTimeout(r.Context(), mail.SendTimeout)
	defer cancel()
	if err := h.mailer.Send(ctx, *org.OwnerEmail, m); err != nil {
		log.Printf("sending %q: %v", m.Subject, err)
		return false
	}
	return true
}

// Admin: POST /api/admin/orgs/{id}/suspend {reason}.
func (h *AdminHandler) SuspendOrg(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reason string `json:"reason"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if !validSuspendReason(req.Reason) {
		httpx.WriteError(w, http.StatusBadRequest, "give a reason of 1-500 characters on one line")
		return
	}
	org, ok := h.getOrg(w, r)
	if !ok {
		return
	}
	if org.SuspendedAt != nil {
		httpx.WriteError(w, http.StatusConflict, "this organisation is already suspended")
		return
	}
	if err := h.store.SuspendOrg(r.Context(), org.ID, req.Reason); err != nil {
		log.Printf("suspend org: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	sent := h.notifyOwner(r, org, mail.OrgSuspendedMessage(org.Name, req.Reason))
	admin := AdminFrom(r.Context())
	h.audit(r, admin, AuditOrgSuspend, AuditTarget{Type: "org", ID: org.ID},
		map[string]any{"org_name": org.Name, "reason": req.Reason, "email_sent": sent})
	h.GetOrg(w, r)
}

// Admin: POST /api/admin/orgs/{id}/reinstate.
func (h *AdminHandler) ReinstateOrg(w http.ResponseWriter, r *http.Request) {
	org, ok := h.getOrg(w, r)
	if !ok {
		return
	}
	if org.SuspendedAt == nil {
		httpx.WriteError(w, http.StatusConflict, "this organisation isn't suspended")
		return
	}
	if err := h.store.ReinstateOrg(r.Context(), org.ID); err != nil {
		log.Printf("reinstate org: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	sent := h.notifyOwner(r, org, mail.OrgReinstatedMessage(org.Name))
	admin := AdminFrom(r.Context())
	h.audit(r, admin, AuditOrgReinstate, AuditTarget{Type: "org", ID: org.ID},
		map[string]any{"org_name": org.Name, "email_sent": sent})
	h.GetOrg(w, r)
}

// Admin: GET /api/admin/feedback?status=&page=&page_size=.
func (h *AdminHandler) ListFeedback(w http.ResponseWriter, r *http.Request) {
	pg, size, ok := httpx.PageParams(w, r, 25, 100)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" && !validFeedbackStatus(status) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid status")
		return
	}
	list, total, err := h.feedback.ListFeedback(r.Context(), status, size, (pg-1)*size)
	if err != nil {
		log.Printf("list feedback: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	counts, err := h.feedback.CountFeedbackByStatus(r.Context())
	if err != nil {
		log.Printf("count feedback: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, struct {
		page[*feedback.Feedback]
		Counts map[string]int64 `json:"counts"`
	}{page[*feedback.Feedback]{Items: list, Total: total, Page: pg, PageSize: size}, counts})
}

func validFeedbackStatus(s string) bool {
	return s == feedback.FeedbackNew || s == feedback.FeedbackRead || s == feedback.FeedbackResolved
}

// getFeedback loads feedback with its replies, writing the error response if it can't.
func (h *AdminHandler) getFeedback(w http.ResponseWriter, r *http.Request) (*feedback.Feedback, bool) {
	id, ok := httpx.PathID(w, r, "id", "feedback")
	if !ok {
		return nil, false
	}
	f, err := h.feedback.GetFeedback(r.Context(), id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "feedback not found")
			return nil, false
		}
		log.Printf("get feedback: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return nil, false
	}
	return f, true
}

// Admin: GET /api/admin/feedback/{id}.
func (h *AdminHandler) GetFeedback(w http.ResponseWriter, r *http.Request) {
	if f, ok := h.getFeedback(w, r); ok {
		httpx.WriteJSON(w, http.StatusOK, f)
	}
}

// Admin: PATCH /api/admin/feedback/{id} {status}.
func (h *AdminHandler) SetFeedbackStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status string `json:"status"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if !validFeedbackStatus(req.Status) {
		httpx.WriteError(w, http.StatusBadRequest, "status must be new, read or resolved")
		return
	}
	f, ok := h.getFeedback(w, r)
	if !ok {
		return
	}
	if f.Status != req.Status {
		if err := h.feedback.SetFeedbackStatus(r.Context(), f.ID, req.Status); err != nil {
			log.Printf("set feedback status: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}
		h.audit(r, AdminFrom(r.Context()), AuditFeedbackStatus,
			AuditTarget{Type: "feedback", ID: f.ID}, map[string]any{"from": f.Status, "to": req.Status})
	}
	h.GetFeedback(w, r)
}

// Admin: POST /api/admin/feedback/{id}/replies {body}. The reply is emailed
// to the sender and kept with the feedback, even if the email fails.
func (h *AdminHandler) ReplyFeedback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Body string `json:"body"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" || utf8.RuneCountInString(req.Body) > maxReplyLen {
		httpx.WriteError(w, http.StatusBadRequest, "reply must be 1-5000 characters")
		return
	}
	f, ok := h.getFeedback(w, r)
	if !ok {
		return
	}
	if f.SenderEmail == "" {
		httpx.WriteError(w, http.StatusConflict, "the sender deleted their account, so there's nobody to reply to")
		return
	}

	m := mail.FeedbackReplyMessage(f.Message, req.Body)
	m.ReplyTo = h.replyTo
	ctx, cancel := context.WithTimeout(r.Context(), mail.SendTimeout)
	sendErr := h.mailer.Send(ctx, f.SenderEmail, m)
	cancel()
	if sendErr != nil {
		log.Printf("feedback reply: %v", sendErr)
	}

	admin := AdminFrom(r.Context())
	reply := &feedback.FeedbackReply{FeedbackID: f.ID, AdminID: &admin.ID, Body: req.Body, EmailSent: sendErr == nil}
	if err := h.feedback.CreateFeedbackReply(r.Context(), reply); err != nil {
		log.Printf("save feedback reply: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.audit(r, admin, AuditFeedbackReply, AuditTarget{Type: "feedback", ID: f.ID},
		map[string]any{"email_sent": reply.EmailSent})
	h.GetFeedback(w, r)
}

// Admin: GET /api/admin/audit?page=&page_size=.
func (h *AdminHandler) ListAudit(w http.ResponseWriter, r *http.Request) {
	pg, size, ok := httpx.PageParams(w, r, 50, 200)
	if !ok {
		return
	}
	entries, total, err := h.store.ListAudit(r.Context(), size, (pg-1)*size)
	if err != nil {
		log.Printf("list audit: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, page[*AuditEntry]{Items: entries, Total: total, Page: pg, PageSize: size})
}
