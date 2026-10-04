package auth

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCode(t *testing.T) {
	digits := regexp.MustCompile(`^[0-9]{6}$`)
	for range 200 {
		code, err := NewCode()
		require.NoError(t, err)
		assert.Regexp(t, digits, code)
	}
}

func TestCheckCode(t *testing.T) {
	s := NewService("test-secret-0123456789")
	hash := s.HashCode(7, PurposeVerifyEmail, "123456")

	assert.True(t, s.CheckCode(7, PurposeVerifyEmail, "123456", hash))
	assert.False(t, s.CheckCode(7, PurposeVerifyEmail, "123457", hash), "wrong code")
	assert.False(t, s.CheckCode(8, PurposeVerifyEmail, "123456", hash), "other user")
	assert.False(t, s.CheckCode(7, PurposeResetPassword, "123456", hash), "other purpose")
	assert.False(t, NewService("another-secret-0123456").CheckCode(7, PurposeVerifyEmail, "123456", hash), "other key")
}

func TestJWTCarriesSessionVersion(t *testing.T) {
	s := NewService("test-secret-0123456789")
	token, err := s.GenerateJWT(42, 3)
	require.NoError(t, err)

	id, version, err := s.ValidateJWT(token)
	require.NoError(t, err)
	assert.Equal(t, int64(42), id)
	assert.Equal(t, 3, version)

	_, _, err = NewService("another-secret-0123456").ValidateJWT(token)
	assert.Error(t, err)
}
