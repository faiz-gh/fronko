package cards

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildVCard(t *testing.T) {
	t.Run("full card", func(t *testing.T) {
		data := []byte(`{"name":"  Ada  King Lovelace ","title":"Engineer","company":"Acme, Inc; Ltd","email":"ada@example.com",
			"phone_country_code":"+44","phone_number":"7700900123","website":"ada.dev","location":"London",
			"bio":"Line one\nLine two \\ done"}`)
		got := buildVCard(data, "https://fronko.app/p/ada")
		assert.Equal(t, strings.Join([]string{
			"BEGIN:VCARD",
			"VERSION:3.0",
			"N:Lovelace;Ada King;;;",
			"FN:Ada King Lovelace",
			`ORG:Acme\, Inc\; Ltd`,
			"TITLE:Engineer",
			"EMAIL;TYPE=INTERNET:ada@example.com",
			"TEL;TYPE=CELL:+447700900123",
			"URL:https://ada.dev",
			"URL:https://fronko.app/p/ada",
			"ADR;TYPE=WORK:;;;London;;;",
			`NOTE:Line one\nLine two \\ done`,
			"END:VCARD",
		}, "\r\n")+"\r\n", got)
	})

	t.Run("empty card still makes a contact", func(t *testing.T) {
		got := buildVCard([]byte(`{}`), "https://fronko.app/p/x")
		assert.Contains(t, got, "N:;Contact;;;\r\nFN:Contact\r\n")
		assert.NotContains(t, got, "TEL")
		assert.NotContains(t, got, "EMAIL")
	})

	t.Run("unsafe website dropped", func(t *testing.T) {
		got := buildVCard([]byte(`{"name":"X","website":"javascript:alert(1)"}`), "https://fronko.app/p/x")
		assert.Equal(t, 1, strings.Count(got, "URL:"))
	})
}

func TestRequestOrigin(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "http://localhost:3000/api/profiles/x/vcard", nil)
	assert.Equal(t, "http://localhost:3000", requestOrigin(r))
	r.Header.Set("X-Forwarded-Proto", "https")
	assert.Equal(t, "https://localhost:3000", requestOrigin(r))
	r.Header.Del("X-Forwarded-Proto")
	r.TLS = &tls.ConnectionState{}
	assert.Equal(t, "https://localhost:3000", requestOrigin(r))
}

func TestFrontendOrigin(t *testing.T) {
	allowed := []string{"https://app.fronko.dev"}
	r := httptest.NewRequest(http.MethodGet, "http://api.fronko.dev/api/profiles/x/vcard", nil)
	assert.Equal(t, "http://api.fronko.dev", frontendOrigin(r, allowed), "no referer")

	r.Header.Set("Referer", "https://app.fronko.dev/p/x?via=nfc")
	assert.Equal(t, "https://app.fronko.dev", frontendOrigin(r, allowed))

	r.Header.Set("Referer", "https://evil.example/p/x")
	assert.Equal(t, "http://api.fronko.dev", frontendOrigin(r, allowed), "unknown referer ignored")
}
