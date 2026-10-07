package users

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/mail"
)

var codePattern = regexp.MustCompile(`^[0-9]{6}$`)

// Codes issues and checks the 6-digit codes emailed for verifying an address,
// resetting a password and changing an email.
type Codes struct {
	store  *Store
	auth   *auth.Service
	mailer mail.Sender
}

func NewCodes(store *Store, authService *auth.Service, mailer mail.Sender) *Codes {
	return &Codes{store: store, auth: authService, mailer: mailer}
}

// SendAsync delivers mail in the background. See mail.SendAsync.
func (c *Codes) SendAsync(to string, m mail.Message) { mail.SendAsync(c.mailer, to, m) }

// Issue emails the user a fresh code for purpose at their current
// address, replacing any older one. See IssueTo.
func (c *Codes) Issue(ctx context.Context, user *User, purpose string, force bool) (time.Duration, error) {
	if user.Email == nil {
		return 0, errors.New("user has no email")
	}
	return c.IssueTo(ctx, user.ID, *user.Email, purpose, force)
}

// IssueTo emails a fresh code for purpose to the given address, replacing
// any older one. Unless force is set, it refuses while the previous code to the
// same address is inside the resend cooldown and returns how long is left.
// A change_email code remembers the address it was sent to.
func (c *Codes) IssueTo(ctx context.Context, userID int64, to, purpose string, force bool) (time.Duration, error) {
	if !force {
		prev, err := c.store.GetEmailCode(ctx, userID, purpose)
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return 0, err
		}
		if prev != nil && (prev.Email == nil || *prev.Email == to) {
			if wait := auth.CodeResendCooldown - time.Since(prev.CreatedAt); wait > 0 {
				return wait, nil
			}
		}
	}

	code, err := auth.NewCode()
	if err != nil {
		return 0, err
	}
	stored := &EmailCode{
		UserID:    userID,
		Purpose:   purpose,
		CodeHash:  c.auth.HashCode(userID, purpose, code),
		ExpiresAt: time.Now().Add(auth.CodeTTL),
	}
	if purpose == auth.PurposeChangeEmail {
		stored.Email = &to
	}
	if err := c.store.UpsertEmailCode(ctx, stored); err != nil {
		return 0, err
	}

	var msg mail.Message
	switch purpose {
	case auth.PurposeResetPassword:
		msg = mail.ResetPasswordMessage(code, auth.CodeTTL)
	case auth.PurposeChangeEmail:
		msg = mail.ChangeEmailMessage(code, auth.CodeTTL)
	default:
		msg = mail.VerifyEmailMessage(code, auth.CodeTTL)
	}
	c.SendAsync(to, msg)
	return 0, nil
}

// Consume checks a code, spending one attempt. A correct code is deleted
// so it can't be used twice.
func (c *Codes) Consume(ctx context.Context, userID int64, purpose, code string) (bool, error) {
	stored, err := c.ConsumeRow(ctx, userID, purpose, code)
	return stored != nil, err
}

// ConsumeRow is Consume, returning the matched code (nil if invalid).
func (c *Codes) ConsumeRow(ctx context.Context, userID int64, purpose, code string) (*EmailCode, error) {
	code = strings.TrimSpace(code)
	if !codePattern.MatchString(code) {
		return nil, nil
	}
	stored, err := c.store.UseEmailCodeAttempt(ctx, userID, purpose, auth.CodeMaxAttempts)
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !c.auth.CheckCode(userID, purpose, code, stored.CodeHash) {
		return nil, nil
	}
	return stored, c.store.DeleteEmailCode(ctx, userID, purpose)
}
