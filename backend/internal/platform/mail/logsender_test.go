package mail

import (
	"bytes"
	"context"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogSenderHidesBodiesOutsideDevelopment(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	m := Message{Subject: "Your code", Text: "123456 temporary-password"}

	assert.NoError(t, LogSender{}.Send(context.Background(), "nina@example.com", m))
	assert.NotContains(t, buf.String(), "123456")
	assert.NotContains(t, buf.String(), "nina")
	assert.Contains(t, buf.String(), "*@example.com")

	buf.Reset()
	assert.NoError(t, LogSender{ShowBody: true}.Send(context.Background(), "nina@example.com", m))
	assert.Contains(t, buf.String(), "123456")
}
