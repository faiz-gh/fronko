package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// SessionTTL is how long a login lasts; it bounds both the JWT and its cookie.
const SessionTTL = 24 * time.Hour

type Service struct {
	jwtSecret []byte
}

func NewService(jwtSecret string) *Service {
	return &Service{
		jwtSecret: []byte(jwtSecret),
	}
}

func (s *Service) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
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
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, sessionClaims{
		SessionVersion: sessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(SessionTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	return token.SignedString(s.jwtSecret)
}

// ValidateJWT returns the user ID and session version a valid token carries.
// Tokens from before session versions existed read as version 0, which
// matches users who have never changed their password.
func (s *Service) ValidateJWT(tokenString string) (userID int64, sessionVersion int, err error) {
	var claims sessionClaims
	_, err = jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return 0, 0, err
	}

	userID, err = strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, 0, errors.New("invalid user id in token")
	}
	return userID, claims.SessionVersion, nil
}
