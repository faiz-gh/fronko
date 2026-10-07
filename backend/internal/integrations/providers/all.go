// Package providers registers every integration provider. Adding one is a
// new folder under providers/ and one line in All.
package providers

import (
	"github.com/faiz-gh/fronko/backend/internal/integrations"
	"github.com/faiz-gh/fronko/backend/internal/integrations/providers/webhook"
)

// All registers every provider with r, in catalog order within each category.
func All(r *integrations.Registry) {
	r.Register(webhook.New())
	for _, p := range soon() {
		r.Register(p)
	}
}
