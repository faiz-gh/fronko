package auth

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// SessionTTL is how long a login lasts; it bounds both the JWT and its cookie.
const SessionTTL = 24 * time.Hour

// AdminSessionTTL is how long a platform admin's login lasts.
const AdminSessionTTL = 8 * time.Hour

// adminAudience marks platform-admin tokens. User tokens never carry it, and
// each validator rejects the other kind, so an admin ID can't pass as the user
// with the same number (or the reverse).
const adminAudience = "platform-admin"

type Service struct {
	jwtSecret []byte
}

func NewService(jwtSecret string) *Service {
	return &Service{
		jwtSecret: []byte(jwtSecret),
	}
}

// passwordCost is the bcrypt work factor for new hashes.
const passwordCost = 12

func (s *Service) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), passwordCost)
	return string(bytes), err
}

// NeedsRehash reports whether a password hash was made with a lower work
// factor than new ones get, so it should be replaced at the next sign-in.
func NeedsRehash(hash string) bool {
	cost, err := bcrypt.Cost([]byte(hash))
	return err == nil && cost < passwordCost
}

func (s *Service) CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// sessionClaims adds the user's session version, so bumping it in the database
// (on a password change or reset) invalidates every token issued before.
type sessionClaims struct {
	SessionVersion int `json:"sv"`
	jwt.RegisteredClaims
}

func (s *Service) GenerateJWT(userID int64, sessionVersion int) (string, error) {
	return s.sign(userID, sessionVersion, SessionTTL, nil)
}

// GenerateAdminJWT issues a platform admin's session token.
func (s *Service) GenerateAdminJWT(adminID int64, sessionVersion int) (string, error) {
	return s.sign(adminID, sessionVersion, AdminSessionTTL, jwt.ClaimStrings{adminAudience})
}

func (s *Service) sign(subject int64, sessionVersion int, ttl time.Duration, aud jwt.ClaimStrings) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, sessionClaims{
		SessionVersion: sessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", subject),
			Audience:  aud,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	return token.SignedString(s.jwtSecret)
}

// ValidateJWT returns the user ID and session version a valid token carries.
// Tokens from before session versions existed read as version 0, which
// matches users who have never changed their password.
func (s *Service) ValidateJWT(tokenString string) (userID int64, sessionVersion int, err error) {
	claims, err := s.parse(tokenString)
	if err != nil {
		return 0, 0, err
	}
	if slices.Contains(claims.Audience, adminAudience) {
		return 0, 0, errors.New("admin token used as a user session")
	}
	return subjectAndVersion(claims)
}

// ValidateAdminJWT returns the admin ID and session version a valid
// platform-admin token carries. User tokens are rejected.
func (s *Service) ValidateAdminJWT(tokenString string) (adminID int64, sessionVersion int, err error) {
	claims, err := s.parse(tokenString, jwt.WithAudience(adminAudience))
	if err != nil {
		return 0, 0, err
	}
	return subjectAndVersion(claims)
}

func (s *Service) parse(tokenString string, opts ...jwt.ParserOption) (*sessionClaims, error) {
	var claims sessionClaims
	opts = append(opts, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	}, opts...)
	if err != nil {
		return nil, err
	}
	return &claims, nil
}

func subjectAndVersion(claims *sessionClaims) (int64, int, error) {
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, 0, errors.New("invalid user id in token")
	}
	return id, claims.SessionVersion, nil
}
