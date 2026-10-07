package integrations

import (
	"context"
	"errors"
	"log"
	"strings"
	"unicode"
	"unicode/utf8"
)

// userError is a message written for people, shown as is.
type userError struct{ msg string }

func (e *userError) Error() string { return e.msg }

func userErr(msg string) error { return &userError{msg: msg} }

// asUserError treats a provider's validation error as a message for people.
func asUserError(err error) error {
	var fe *FieldError
	if err == nil || errors.As(err, &fe) {
		return err
	}
	return userErr(err.Error())
}

// detailedError carries troubleshooting details for the activity log.
type detailedError struct {
	err    error
	detail map[string]any
}

func (e *detailedError) Error() string { return e.err.Error() }
func (e *detailedError) Unwrap() error { return e.err }

// WithDetail attaches details (an HTTP status, a response excerpt) to a
// provider error; they're shown in the activity log. It keeps errors.Is and
// jobs.IsPermanent working.
func WithDetail(err error, detail map[string]any) error {
	if err == nil {
		return nil
	}
	return &detailedError{err: err, detail: detail}
}

func errorDetail(err error) map[string]any {
	var d *detailedError
	if errors.As(err, &d) {
		return d.detail
	}
	return nil
}

// describeError is the one line shown for a failure.
func describeError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "The request timed out"
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "Something went wrong"
	}
	r, size := utf8.DecodeRuneInString(msg)
	return string(unicode.ToUpper(r)) + msg[size:]
}

func logf(format string, args ...any) { log.Printf("integrations: "+format, args...) }
