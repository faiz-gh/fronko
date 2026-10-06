package handlers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeedbackRequestValidate(t *testing.T) {
	rating := func(n int16) *int16 { return &n }
	cases := []struct {
		name string
		req  feedbackRequest
		ok   bool
	}{
		{"valid", feedbackRequest{Category: "idea", Message: "CSV export please"}, true},
		{"valid with rating", feedbackRequest{Category: "bug", Rating: rating(5), Message: "x"}, true},
		{"bad category", feedbackRequest{Category: "rant", Message: "x"}, false},
		{"rating too low", feedbackRequest{Category: "bug", Rating: rating(0), Message: "x"}, false},
		{"rating too high", feedbackRequest{Category: "bug", Rating: rating(6), Message: "x"}, false},
		{"blank message", feedbackRequest{Category: "other", Message: "   "}, false},
		{"too long", feedbackRequest{Category: "other", Message: strings.Repeat("é", maxFeedbackLen+1)}, false},
		{"max length", feedbackRequest{Category: "other", Message: strings.Repeat("é", maxFeedbackLen)}, true},
	}
	for _, c := range cases {
		assert.Equal(t, c.ok, c.req.validate() == "", c.name)
	}
}

func TestFeedbackRequestDropsOddPagePaths(t *testing.T) {
	for path, want := range map[string]string{
		"/dashboard/leads":                        "/dashboard/leads",
		"https://evil.example/":                   "",
		"/dashboard\r\nX-Injected":                "",
		"/" + strings.Repeat("a", maxPagePathLen): "",
	} {
		req := feedbackRequest{Category: "idea", Message: "x", PagePath: path}
		assert.Empty(t, req.validate())
		assert.Equal(t, want, req.PagePath, path)
	}
}
