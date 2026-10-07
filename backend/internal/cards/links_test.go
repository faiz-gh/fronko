package cards

import (
	"encoding/json"
	"testing"
)

func TestValidWebURL(t *testing.T) {
	good := []string{"ada.dev", "https://ada.dev/about", "http://sub.ada-lovelace.co.uk/x?y=1", "github.com/you", "http://localhost:3000/p", "https://192.168.1.10/x"}
	for _, s := range good {
		if !validWebURL(s) {
			t.Errorf("validWebURL(%q) = false, want true", s)
		}
	}
	bad := []string{"", "not a url", "https://not a url", "hello", "https://ada", "javascript:alert(1)", "mailto:a@b.co", "ftp://ada.dev", "https://%20.com"}
	for _, s := range bad {
		if validWebURL(s) {
			t.Errorf("validWebURL(%q) = true, want false", s)
		}
	}
}

func TestValidateCardURLs(t *testing.T) {
	cases := []struct {
		data string
		ok   bool
	}{
		{`{}`, true},
		{`{"website":"","links":[{"url":""}]}`, true},
		{`{"website":"acme.com","links":[{"label":"GitHub","url":"github.com/ada"}]}`, true},
		{`{"links":[{"url":"not a url"}]}`, false},
		{`{"website":"not a url"}`, false},
	}
	for _, c := range cases {
		if got := validateCardURLs(json.RawMessage(c.data)) == ""; got != c.ok {
			t.Errorf("validateCardURLs(%s) ok = %v, want %v", c.data, got, c.ok)
		}
	}
}
