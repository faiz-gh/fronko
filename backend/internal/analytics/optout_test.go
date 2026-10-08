package analytics

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptedOut(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/profiles/o/s/events", nil)
	assert.False(t, optedOut(r))
	r.Header.Set("Sec-GPC", "1")
	assert.True(t, optedOut(r))
	r = httptest.NewRequest("POST", "/api/profiles/o/s/events", nil)
	r.Header.Set("DNT", "1")
	assert.True(t, optedOut(r))
}
