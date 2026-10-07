package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleFromName(t *testing.T) {
	cases := map[string]string{
		"Acme":                "acme",
		"  Acme Corp, Ltd.  ": "acme-corp-ltd",
		"faiz@example.com":    "faiz-example-com",
		"Ünïcödé Café":        "n-c-d-caf",
		"AB":                  "",
		"!!!":                 "",
		"The Very Long Company Name Incorporated": "the-very-long-company-name-incor",
		"abcdefghijklmnopqrstuvwxyz12345 6":       "abcdefghijklmnopqrstuvwxyz12345",
	}
	for in, want := range cases {
		assert.Equal(t, want, HandleFromName(in), in)
	}
}
