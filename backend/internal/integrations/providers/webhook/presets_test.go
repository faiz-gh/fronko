package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/faiz-gh/fronko/backend/internal/integrations"
)

func TestPresetsCheckTheirToolsURL(t *testing.T) {
	ctx := context.Background()
	check := func(p integrations.Provider, url string) error {
		return p.Validate(ctx, integrations.Settings{Values: map[string]any{"url": url}})
	}
	assert.NoError(t, check(Zapier(), "https://hooks.zapier.com/hooks/catch/123/abc/"))
	assert.Error(t, check(Zapier(), "https://hook.eu1.make.com/abc"))
	assert.NoError(t, check(Make(), "https://hook.eu1.make.com/abc"))
	assert.NoError(t, check(Make(), "https://hook.us2.make.com/abc"))
	assert.Error(t, check(Make(), "https://example.com/make.com"))
	assert.NoError(t, check(N8n(), "https://n8n.internal.example/webhook/abc"), "n8n can be self-hosted")

	for _, p := range []integrations.Provider{Zapier(), Make(), N8n()} {
		_, pushes := p.(integrations.LeadPusher)
		assert.True(t, pushes, p.Manifest().ID)
	}
	_, hasSecret := N8n().Manifest().Field("signing_secret")
	assert.True(t, hasSecret)
	_, hasSecret = Zapier().Manifest().Field("signing_secret")
	assert.False(t, hasSecret, "Zapier can't check a signature, so it isn't offered")
}
