package feedback

import (
	"context"
	"log"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/orgs"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
	"github.com/faiz-gh/fronko/backend/internal/platform/mail"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

const (
	maxFeedbackLen = 5000
	maxPagePathLen = 200
)

// FeedbackHandler takes product feedback from signed-in users.
type FeedbackHandler struct {
	store  *Store
	users  *users.Store
	orgs   *orgs.Store
	mailer mail.Sender
	// notifyEmail gets an email for each piece of feedback; empty sends none.
	notifyEmail string
}

func NewFeedbackHandler(store *Store, userStore *users.Store, orgStore *orgs.Store, mailer mail.Sender, notifyEmail string) *FeedbackHandler {
	return &FeedbackHandler{store: store, users: userStore, orgs: orgStore, mailer: mailer, notifyEmail: notifyEmail}
}

type feedbackRequest struct {
	Category string `json:"category"`
	Rating   *int16 `json:"rating"`
	Message  string `json:"message"`
	PagePath string `json:"page_path"`
}

// validate trims the request and returns a message for the first problem, or "".
func (req *feedbackRequest) validate() string {
	req.Message = strings.TrimSpace(req.Message)
	req.PagePath = strings.TrimSpace(req.PagePath)
	switch req.Category {
	case FeedbackBug, FeedbackIdea, FeedbackOther:
	default:
		return "choose bug, idea or other"
	}
	if req.Rating != nil && (*req.Rating < 1 || *req.Rating > 5) {
		return "rating must be from 1 to 5"
	}
	if req.Message == "" {
		return "write a message"
	}
	if utf8.RuneCountInString(req.Message) > maxFeedbackLen {
		return "message must be at most 5000 characters"
	}
	// The page is only context; drop anything that doesn't look like an app path.
	if !strings.HasPrefix(req.PagePath, "/") || len(req.PagePath) > maxPagePathLen ||
		strings.ContainsFunc(req.PagePath, unicode.IsControl) {
		req.PagePath = ""
	}
	return ""
}

// Protected: POST /api/me/feedback.
func (h *FeedbackHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req feedbackRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		httpx.WriteError(w, http.StatusBadRequest, msg)
		return
	}

	p := auth.PrincipalFrom(r.Context())
	user, err := h.users.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		log.Printf("feedback user: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	org, err := h.orgs.GetOrganization(r.Context(), p.OrgID)
	if err != nil {
		log.Printf("feedback org: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// Verified users always have an email; this only covers very old accounts.
	sender := user.Username
	if user.Email != nil {
		sender = *user.Email
	}

	f := &Feedback{
		OrgID: &org.ID, UserID: &user.ID, SenderEmail: sender, OrgName: org.Name,
		Category: req.Category, Rating: req.Rating, Message: req.Message,
	}
	if req.PagePath != "" {
		f.PagePath = &req.PagePath
	}
	if err := h.store.CreateFeedback(r.Context(), f); err != nil {
		log.Printf("create feedback: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to send feedback")
		return
	}

	if h.notifyEmail != "" {
		msg := mail.FeedbackNotice(mail.FeedbackDetails{
			SenderEmail: f.SenderEmail, OrgName: f.OrgName, Category: f.Category,
			Rating: f.Rating, PagePath: req.PagePath, Message: f.Message,
		})
		if user.Email != nil {
			msg.ReplyTo = *user.Email
		}
		// The feedback is saved either way; a lost notice only means checking the inbox.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), mail.SendTimeout)
			defer cancel()
			if err := h.mailer.Send(ctx, h.notifyEmail, msg); err != nil {
				log.Printf("feedback notice: %v", err)
			}
		}()
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": f.ID})
}
