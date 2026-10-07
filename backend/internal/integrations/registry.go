package integrations

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

// Registry holds every provider the server knows, in catalog order.
type Registry struct {
	byID  map[string]Provider
	order []Provider
}

// NewRegistry returns an empty registry; providers/all.go fills it.
func NewRegistry() *Registry {
	return &Registry{byID: map[string]Provider{}}
}

// Register adds a provider. It panics on a bad or duplicate manifest, since
// that is a programming error caught at start-up.
func (r *Registry) Register(p Provider) {
	m := p.Manifest()
	if err := checkManifest(p, m); err != nil {
		panic(fmt.Sprintf("integrations: provider %q: %v", m.ID, err))
	}
	if _, dup := r.byID[m.ID]; dup {
		panic(fmt.Sprintf("integrations: provider %q registered twice", m.ID))
	}
	r.byID[m.ID] = p
	r.order = append(r.order, p)
}

// Get returns the provider with the given id.
func (r *Registry) Get(id string) (Provider, bool) {
	p, ok := r.byID[id]
	return p, ok
}

// All returns every provider in registration order.
func (r *Registry) All() []Provider { return r.order }

var (
	providerIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	fieldKeyPattern   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

func checkManifest(p Provider, m Manifest) error {
	if !providerIDPattern.MatchString(m.ID) {
		return errors.New("id must be lowercase letters, digits and dashes")
	}
	if m.Name == "" || m.Description == "" {
		return errors.New("name and description are required")
	}
	switch m.Category {
	case CategoryLeadSync, CategoryCalendar, CategoryDirectory, CategorySSO:
	default:
		return fmt.Errorf("unknown category %q", m.Category)
	}
	if len(m.Scopes) == 0 {
		return errors.New("at least one scope is required")
	}
	for _, s := range m.Scopes {
		if s != ScopeOrg && s != ScopeUser {
			return fmt.Errorf("unknown scope %q", s)
		}
	}
	switch m.Status {
	case Available, Beta, ComingSoon:
	default:
		return fmt.Errorf("unknown status %q", m.Status)
	}
	seen := map[string]bool{}
	for _, f := range m.Fields {
		if !fieldKeyPattern.MatchString(f.Key) || seen[f.Key] {
			return fmt.Errorf("field key %q is invalid or repeated", f.Key)
		}
		seen[f.Key] = true
		switch f.Type {
		case FieldText, FieldURL, FieldSecret, FieldTextarea, FieldBool:
		case FieldSelect:
			if len(f.Options) == 0 {
				return fmt.Errorf("select field %q has no options", f.Key)
			}
		default:
			return fmt.Errorf("field %q has unknown type %q", f.Key, f.Type)
		}
	}
	if m.Status == ComingSoon {
		return nil
	}
	if m.Auth == AuthOAuth2 {
		if _, ok := p.(OAuthProvider); !ok {
			return errors.New("oauth2 providers must implement OAuthProvider")
		}
		for _, key := range []string{"client_id", "client_secret"} {
			if f, ok := m.Field(key); !ok || !f.Required {
				return fmt.Errorf("oauth2 providers need a required %q field", key)
			}
		}
	}
	return nil
}

// Placeholder is a provider that is only listed in the catalog, as coming
// soon. It can't be connected.
type Placeholder struct{ M Manifest }

func (p Placeholder) Manifest() Manifest {
	m := p.M
	m.Status = ComingSoon
	return m
}

func (Placeholder) Validate(context.Context, Settings) error {
	return errors.New("this integration isn't available yet")
}
