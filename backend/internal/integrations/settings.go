package integrations

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/faiz-gh/fronko/backend/internal/platform/netguard"
)

// Length limits for setting values.
const (
	maxTextLen     = 500
	maxTextareaLen = 5000
	maxURLLen      = 2000
	maxSecretLen   = 4000
)

// mergeSettings checks a change to a connection's settings against the
// manifest and returns the resulting settings. values and secrets hold only
// the keys being set; a secret set to "" is left as it was. On a new
// connection (current is empty) missing fields get their defaults.
//
// Required fields are not enforced here: a connection can be saved before
// it's complete and stays pending until it is (see missingFields).
func mergeSettings(m Manifest, current Settings, values map[string]any, secrets map[string]string, isNew bool) (Settings, error) {
	out := Settings{
		Values:       map[string]any{},
		Secrets:      map[string]string{},
		AllowPrivate: current.AllowPrivate,
	}
	for k, v := range current.Values {
		out.Values[k] = v
	}
	for k, v := range current.Secrets {
		out.Secrets[k] = v
	}

	for key := range values {
		if f, ok := m.Field(key); !ok || f.Type == FieldSecret {
			return out, fieldErr(key, fmt.Sprintf("unknown setting %q", key))
		}
	}
	for key := range secrets {
		if f, ok := m.Field(key); !ok || f.Type != FieldSecret {
			return out, fieldErr(key, fmt.Sprintf("unknown secret %q", key))
		}
	}

	for _, f := range m.Fields {
		if f.Type == FieldSecret {
			v, ok := secrets[f.Key]
			if !ok {
				continue
			}
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if len(v) > maxSecretLen || strings.ContainsAny(v, "\r\n") {
				return out, fieldErr(f.Key, f.Label+" doesn't look right")
			}
			out.Secrets[f.Key] = v
			continue
		}

		raw, ok := values[f.Key]
		if !ok {
			if isNew && f.Default != nil {
				out.Values[f.Key] = f.Default
			}
			continue
		}
		v, err := checkValue(f, raw, out.AllowPrivate)
		if err != nil {
			return out, err
		}
		if v == nil {
			delete(out.Values, f.Key)
		} else {
			out.Values[f.Key] = v
		}
	}
	return out, nil
}

// checkValue validates and normalises one non-secret value. It returns nil
// for an empty value.
func checkValue(f Field, raw any, allowPrivate bool) (any, error) {
	if raw == nil {
		return nil, nil
	}
	if f.Type == FieldBool {
		b, ok := raw.(bool)
		if !ok {
			return nil, fieldErr(f.Key, f.Label+" must be on or off")
		}
		return b, nil
	}
	s, ok := raw.(string)
	if !ok {
		return nil, fieldErr(f.Key, f.Label+" must be text")
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	switch f.Type {
	case FieldText:
		if utf8.RuneCountInString(s) > maxTextLen || strings.ContainsAny(s, "\r\n") {
			return nil, fieldErr(f.Key, fmt.Sprintf("%s must be one line of at most %d characters", f.Label, maxTextLen))
		}
	case FieldTextarea:
		if utf8.RuneCountInString(s) > maxTextareaLen {
			return nil, fieldErr(f.Key, fmt.Sprintf("%s must be at most %d characters", f.Label, maxTextareaLen))
		}
	case FieldURL:
		if len(s) > maxURLLen {
			return nil, fieldErr(f.Key, f.Label+" is too long")
		}
		u, err := netguard.CheckURL(s, allowPrivate)
		if errors.Is(err, netguard.ErrPrivate) {
			return nil, fieldErr(f.Key, f.Label+" points at a private or local address")
		}
		if err != nil {
			return nil, fieldErr(f.Key, f.Label+": "+strings.TrimPrefix(err.Error(), netguard.ErrInvalidURL.Error()+": "))
		}
		s = u.String()
	case FieldSelect:
		if !slices.ContainsFunc(f.Options, func(o Option) bool { return o.Value == s }) {
			return nil, fieldErr(f.Key, "choose one of the options for "+f.Label)
		}
	}
	return s, nil
}

// missingFields lists the labels of required fields that have no value.
func missingFields(m Manifest, s Settings) []string {
	var missing []string
	for _, f := range m.Fields {
		if !f.Required {
			continue
		}
		var set bool
		if f.Type == FieldSecret {
			set = s.Secrets[f.Key] != ""
		} else {
			switch v := s.Values[f.Key].(type) {
			case string:
				set = v != ""
			case bool:
				set = true
			}
		}
		if !set {
			missing = append(missing, f.Label)
		}
	}
	return missing
}
