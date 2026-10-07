package users

import (
	"net/http"
	netmail "net/mail"
	"regexp"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)

// ValidUsername checks a username: 3-32 letters, numbers, '.', '_' or '-'.
func ValidUsername(username string) bool { return usernamePattern.MatchString(username) }

// NormalizeEmail accepts a bare address (no display name) and lower-cases it.
func NormalizeEmail(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > 254 {
		return "", false
	}
	addr, err := netmail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@")+1:], ".") {
		return "", false
	}
	return email, true
}

// ValidPassword writes a 400 and returns false if the password is out of range.
// bcrypt ignores everything past 72 bytes, so longer passwords are rejected outright.
func ValidPassword(w http.ResponseWriter, password string) bool {
	if len(password) < 8 || len(password) > 72 {
		httpx.WriteError(w, http.StatusBadRequest, "password must be 8-72 characters")
		return false
	}
	return true
}
