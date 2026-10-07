package integrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddressesFollowTheAPIURL(t *testing.T) {
	same := &Service{opts: Options{PublicURL: "https://cards.example.com"}}
	assert.Equal(t, "https://cards.example.com/api/integrations/oauth/callback", same.OAuthRedirectURL(),
		"without PUBLIC_API_URL, the API is on the site's origin")

	split := &Service{opts: Options{PublicURL: "https://example.com", APIURL: "https://api.example.com"}}
	assert.Equal(t, "https://api.example.com/api/integrations/oauth/callback", split.OAuthRedirectURL())
	assert.Equal(t, "https://example.com/p/acme/jane", split.cardURL("acme", "jane"), "cards are on the site")

	none := &Service{}
	assert.Empty(t, none.OAuthRedirectURL())
	m := Manifest{Status: Available, Requires: []Requirement{RequiresPublicURL}}
	assert.Equal(t, "Needs FRONTEND_URL (or PUBLIC_URL) to be set on the server", none.Unavailable(m))
	assert.Empty(t, split.Unavailable(m))
}
