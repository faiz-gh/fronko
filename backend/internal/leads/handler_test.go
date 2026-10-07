package leads

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeLeadPhone(t *testing.T) {
	ok := []struct{ code, number, wantCode, wantNumber string }{
		{"", "", "", ""},
		{"  ", " ", "", ""},
		{"+91", "9876543210", "+91", "9876543210"},
		{"91", "98765 43210", "+91", "9876543210"},
		{"+1", "(512) 555-0199", "+1", "5125550199"},
		{"+44", "7700.900123", "+44", "7700900123"},
	}
	for _, tc := range ok {
		code, number, valid := normalizeLeadPhone(tc.code, tc.number)
		assert.True(t, valid, "%q %q", tc.code, tc.number)
		assert.Equal(t, tc.wantCode, code)
		assert.Equal(t, tc.wantNumber, number)
	}

	bad := []struct{ code, number string }{
		{"+91", ""},                 // number without code is half a phone
		{"", "9876543210"},          // and so is a code-less number
		{"+0", "9876543210"},        // dial codes never start with 0
		{"+1234", "98765"},          // dial codes are at most 3 digits
		{"+91", "123"},              // too short
		{"+91", "98765abc10"},       // letters
		{"+91", "1234567890123456"}, // too long
		{"+353", "12345678901234"},  // over 15 digits in total
		{"+91", "+919876543210"},    // a full number in the national field
	}
	for _, tc := range bad {
		_, _, valid := normalizeLeadPhone(tc.code, tc.number)
		assert.False(t, valid, "%q %q", tc.code, tc.number)
	}
}
