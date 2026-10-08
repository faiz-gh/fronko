package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"
)

// One-time codes emailed for verification and password resets.
const (
	CodeTTL = 15 * time.Minute
	// CodeMaxAttempts wrong guesses burn the code; with 10^6 possibilities and
	// the per-IP rate limit, guessing is impractical.
	CodeMaxAttempts = 5
	// CodeResendCooldown is the minimum gap between two codes for the same purpose.
	CodeResendCooldown = 60 * time.Second
)

// Code purposes, matching the email_codes.purpose CHECK constraint.
const (
	PurposeVerifyEmail   = "verify_email"
	PurposeResetPassword = "reset_password"
	// PurposeChangeEmail codes go to the new address of an already verified account.
	PurposeChangeEmail = "change_email"
	// PurposeDeleteAccount codes confirm deleting an account that has no
	// password (single sign-on only).
	PurposeDeleteAccount = "delete_account"
)

// NewCode returns a uniformly random 6-digit code.
func NewCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// HashCode keys the hash with the JWT secret so a database dump alone can't
// be brute-forced back into live codes. The purpose and user are mixed in so a
// hash can't be replayed onto another row.
func (s *Service) HashCode(userID int64, purpose, code string) string {
	mac := hmac.New(sha256.New, s.jwtSecret)
	fmt.Fprintf(mac, "%d:%s:%s", userID, purpose, code)
	return hex.EncodeToString(mac.Sum(nil))
}

// CheckCode compares in constant time.
func (s *Service) CheckCode(userID int64, purpose, code, hash string) bool {
	return hmac.Equal([]byte(s.HashCode(userID, purpose, code)), []byte(hash))
}
