package repository

import (
	"context"

	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

// CreateFeedback stores feedback from a signed-in user. The caller fills in
// the sender's email and org name as they are now.
func (r *Repository) CreateFeedback(ctx context.Context, f *models.Feedback) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO feedback (org_id, user_id, sender_email, org_name, category, rating, message, page_path)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING feedback_id, status, created_at, updated_at`,
		f.OrgID, f.UserID, f.SenderEmail, f.OrgName, f.Category, f.Rating, f.Message, f.PagePath,
	).Scan(&f.ID, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	return mapError(err)
}

const feedbackSelect = `
	SELECT f.feedback_id, f.org_id, f.user_id, f.sender_email, f.org_name, f.category, f.rating, f.message,
	       f.page_path, f.status, f.created_at, f.updated_at,
	       (SELECT COUNT(*) FROM feedback_replies fr WHERE fr.feedback_id = f.feedback_id)
	FROM feedback f`

func scanFeedback(row pgx.Row) (*models.Feedback, error) {
	var f models.Feedback
	err := row.Scan(&f.ID, &f.OrgID, &f.UserID, &f.SenderEmail, &f.OrgName, &f.Category, &f.Rating, &f.Message,
		&f.PagePath, &f.Status, &f.CreatedAt, &f.UpdatedAt, &f.ReplyCount)
	if err != nil {
		return nil, mapError(err)
	}
	return &f, nil
}

// ListFeedback returns one page of feedback with the given status ("" for
// all), newest first, and the total matching.
func (r *Repository) ListFeedback(ctx context.Context, status string, limit, offset int) ([]*models.Feedback, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM feedback WHERE ($1::text = '' OR status = $1)`, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, feedbackSelect+`
		WHERE ($1::text = '' OR f.status = $1)
		ORDER BY f.created_at DESC, f.feedback_id DESC LIMIT $2 OFFSET $3`, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := []*models.Feedback{}
	for rows.Next() {
		f, err := scanFeedback(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, f)
	}
	return list, total, rows.Err()
}

// GetFeedback returns one piece of feedback with its replies, oldest first.
func (r *Repository) GetFeedback(ctx context.Context, id int64) (*models.Feedback, error) {
	f, err := scanFeedback(r.db.QueryRow(ctx, feedbackSelect+` WHERE f.feedback_id = $1`, id))
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT fr.reply_id, fr.feedback_id, fr.admin_id, a.email, fr.body, fr.email_sent, fr.created_at
		FROM feedback_replies fr LEFT JOIN platform_admins a ON a.admin_id = fr.admin_id
		WHERE fr.feedback_id = $1 ORDER BY fr.created_at, fr.reply_id`, id)
	if err != nil {
		return nil, err
	}
	f.Replies, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (*models.FeedbackReply, error) {
		var rep models.FeedbackReply
		err := row.Scan(&rep.ID, &rep.FeedbackID, &rep.AdminID, &rep.AdminEmail, &rep.Body, &rep.EmailSent, &rep.CreatedAt)
		return &rep, err
	})
	if err != nil {
		return nil, err
	}
	return f, nil
}

// SetFeedbackStatus moves feedback to new, read or resolved.
func (r *Repository) SetFeedbackStatus(ctx context.Context, id int64, status string) error {
	return r.execOne(ctx, `UPDATE feedback SET status = $2, updated_at = now() WHERE feedback_id = $1`, id, status)
}

// CreateFeedbackReply records a reply and marks new feedback as read.
func (r *Repository) CreateFeedbackReply(ctx context.Context, rep *models.FeedbackReply) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO feedback_replies (feedback_id, admin_id, body, email_sent) VALUES ($1, $2, $3, $4)
			RETURNING reply_id, created_at`, rep.FeedbackID, rep.AdminID, rep.Body, rep.EmailSent,
		).Scan(&rep.ID, &rep.CreatedAt)
		if err != nil {
			return mapError(err)
		}
		_, err = tx.Exec(ctx, `
			UPDATE feedback SET status = 'read', updated_at = now()
			WHERE feedback_id = $1 AND status = 'new'`, rep.FeedbackID)
		return err
	})
}

// CountFeedbackByStatus returns how much feedback is in each status.
func (r *Repository) CountFeedbackByStatus(ctx context.Context) (map[string]int64, error) {
	counts := map[string]int64{models.FeedbackNew: 0, models.FeedbackRead: 0, models.FeedbackResolved: 0}
	rows, err := r.db.Query(ctx, `SELECT status, COUNT(*) FROM feedback GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		counts[status] = n
	}
	return counts, rows.Err()
}
