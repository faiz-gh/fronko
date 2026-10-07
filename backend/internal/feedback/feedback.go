package feedback

import (
	"time"
)

// Feedback categories and statuses.
const (
	FeedbackBug   = "bug"
	FeedbackIdea  = "idea"
	FeedbackOther = "other"

	FeedbackNew      = "new"
	FeedbackRead     = "read"
	FeedbackResolved = "resolved"
)

// Feedback is a message a signed-in user sent about the product.
type Feedback struct {
	ID          int64            `json:"id"`
	OrgID       *int64           `json:"org_id"`
	UserID      *int64           `json:"-"`
	SenderEmail string           `json:"sender_email"`
	OrgName     string           `json:"org_name"`
	Category    string           `json:"category"`
	Rating      *int16           `json:"rating"`
	Message     string           `json:"message"`
	PagePath    *string          `json:"page_path"`
	Status      string           `json:"status"`
	ReplyCount  int64            `json:"reply_count"`
	Replies     []*FeedbackReply `json:"replies,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// FeedbackReply is an answer a platform admin emailed to the sender.
type FeedbackReply struct {
	ID         int64     `json:"id"`
	FeedbackID int64     `json:"-"`
	AdminID    *int64    `json:"-"`
	AdminEmail *string   `json:"admin_email"`
	Body       string    `json:"body"`
	EmailSent  bool      `json:"email_sent"`
	CreatedAt  time.Time `json:"created_at"`
}
