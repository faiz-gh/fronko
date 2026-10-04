package mail

import (
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMessage(t *testing.T) {
	from, err := mail.ParseAddress("Fronko <no-reply@fronko.app>")
	require.NoError(t, err)

	msg := VerifyEmailMessage("042917", 15*time.Minute)
	raw, err := buildMessage(from, "ana@example.com", msg, time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	parsed, err := mail.ReadMessage(strings.NewReader(string(raw)))
	require.NoError(t, err)
	assert.Equal(t, `"Fronko" <no-reply@fronko.app>`, parsed.Header.Get("From"))
	assert.Equal(t, "<ana@example.com>", parsed.Header.Get("To"))
	assert.Equal(t, "042917 is your Fronko verification code", parsed.Header.Get("Subject"))
	assert.Contains(t, parsed.Header.Get("Content-Type"), "multipart/alternative")
	assert.True(t, strings.HasSuffix(parsed.Header.Get("Message-ID"), "@fronko.app>"))

	body := string(raw)
	assert.Contains(t, body, "text/plain")
	assert.Contains(t, body, "text/html")
	assert.Contains(t, body, "042917")
}

func TestBuildMessageRejectsHeaderInjection(t *testing.T) {
	from, err := mail.ParseAddress("no-reply@fronko.app")
	require.NoError(t, err)

	for _, to := range []string{"a@example.com\r\nBcc: victim@example.com", "not an address"} {
		_, err := buildMessage(from, to, ResetPasswordMessage("123456", time.Minute), time.Now())
		assert.Error(t, err, to)
	}
}

func TestNewSMTPSenderValidatesFrom(t *testing.T) {
	_, err := NewSMTPSender(SMTPConfig{Host: "smtp.example.com", Port: 587, From: "nope"})
	assert.Error(t, err)
}
