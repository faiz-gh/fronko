package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/faiz-gh/fronko/backend/internal/integrations"
)

func TestEveryProviderRegisters(t *testing.T) {
	r := integrations.NewRegistry()
	All(r)
	categories := map[integrations.Category]int{}
	for _, p := range r.All() {
		categories[p.Manifest().Category]++
	}
	for _, c := range integrations.Categories {
		assert.Positive(t, categories[c], "the catalog has a %s section", c)
	}
	webhook, ok := r.Get("webhook")
	assert.True(t, ok)
	assert.Equal(t, integrations.Available, webhook.Manifest().Status)
}
