package feedback

import (
	"time"

	"github.com/faiz-gh/fronko/backend/internal/app"
)

// Feedback is authenticated; this just keeps one user from flooding the inbox.
const (
	feedbackBurst    = 5
	feedbackInterval = 12 * time.Minute
)

// Routes registers sending product feedback. The admin inbox is in platformadmin.
func (h *FeedbackHandler) Routes(r *app.Routes) {
	r.User("POST /api/me/feedback", r.RateLimit(feedbackInterval, feedbackBurst)(h.Create))
}
