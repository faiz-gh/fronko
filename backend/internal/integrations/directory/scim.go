package directory

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SCIM 2.0 (RFC 7643, RFC 7644): the resources, filters and PATCH
// operations Fronko understands. It's the subset identity providers use to
// provision people: Microsoft Entra ID is the reference, and Okta and
// OneLogin send the same requests.

const (
	schemaUser       = "urn:ietf:params:scim:schemas:core:2.0:User"
	schemaGroup      = "urn:ietf:params:scim:schemas:core:2.0:Group"
	schemaList       = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	schemaPatch      = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	schemaError      = "urn:ietf:params:scim:api:messages:2.0:Error"
	schemaSPConfig   = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	schemaResType    = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	schemaSchema     = "urn:ietf:params:scim:schemas:core:2.0:Schema"
	contentType      = "application/scim+json"
	defaultPageSize  = 100
	maxPageSize      = 200
	maxBodyBytes     = 1 << 20
	maxNameLen       = 200
	maxExternalIDLen = 255
)

// scimError is an error response (RFC 7644 §3.12).
type scimError struct {
	Status   int
	ScimType string
	Detail   string
}

func (e *scimError) Error() string { return e.Detail }

func badRequest(scimType, format string, args ...any) *scimError {
	return &scimError{http.StatusBadRequest, scimType, fmt.Sprintf(format, args...)}
}

func notFound(what string) *scimError {
	return &scimError{http.StatusNotFound, "", what + " not found"}
}

func conflict(format string, args ...any) *scimError {
	return &scimError{http.StatusConflict, "uniqueness", fmt.Sprintf(format, args...)}
}

func writeSCIM(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeSCIMError(w http.ResponseWriter, e *scimError) {
	body := map[string]any{"schemas": []string{schemaError}, "status": strconv.Itoa(e.Status), "detail": e.Detail}
	if e.ScimType != "" {
		body["scimType"] = e.ScimType
	}
	writeSCIM(w, e.Status, body)
}

// decode reads a SCIM request body (application/scim+json or application/json).
func decode(w http.ResponseWriter, r *http.Request, dst any) *scimError {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return badRequest("invalidSyntax", "the request body isn't valid JSON")
	}
	return nil
}

type meta struct {
	ResourceType string    `json:"resourceType"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
	Location     string    `json:"location"`
}

type listResponse struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
	Resources    []any    `json:"Resources"`
}

// flexBool reads a boolean that some identity providers send as a string
// ("True", "False").
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch t := v.(type) {
	case bool:
		*b = flexBool(t)
	case string:
		parsed, err := strconv.ParseBool(strings.ToLower(t))
		if err != nil {
			return fmt.Errorf("%q isn't true or false", t)
		}
		*b = flexBool(parsed)
	default:
		return errors.New("expected true or false")
	}
	return nil
}

type nameAttr struct {
	Formatted  string `json:"formatted,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

type emailAttr struct {
	Value   string   `json:"value"`
	Type    string   `json:"type,omitempty"`
	Primary flexBool `json:"primary,omitempty"`
}

type ref struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
	Ref     string `json:"$ref,omitempty"`
}

// userIn is a User a client sends; unknown attributes are ignored.
type userIn struct {
	ExternalID  *string     `json:"externalId"`
	UserName    string      `json:"userName"`
	Name        *nameAttr   `json:"name"`
	DisplayName *string     `json:"displayName"`
	Emails      []emailAttr `json:"emails"`
	Active      *flexBool   `json:"active"`
}

// userOut is a User Fronko returns.
type userOut struct {
	Schemas     []string    `json:"schemas"`
	ID          string      `json:"id"`
	ExternalID  string      `json:"externalId,omitempty"`
	UserName    string      `json:"userName"`
	Name        *nameAttr   `json:"name,omitempty"`
	DisplayName string      `json:"displayName,omitempty"`
	Emails      []emailAttr `json:"emails,omitempty"`
	Active      bool        `json:"active"`
	Groups      []ref       `json:"groups"`
	Meta        meta        `json:"meta"`
}

type groupIn struct {
	ExternalID  *string `json:"externalId"`
	DisplayName string  `json:"displayName"`
	Members     []ref   `json:"members"`
}

type groupOut struct {
	Schemas     []string `json:"schemas"`
	ID          string   `json:"id"`
	ExternalID  string   `json:"externalId,omitempty"`
	DisplayName string   `json:"displayName"`
	Members     []ref    `json:"members,omitempty"`
	Meta        meta     `json:"meta"`
}

// splitName guesses given and family names from a full name.
func splitName(full string) nameAttr {
	given, family, _ := strings.Cut(strings.TrimSpace(full), " ")
	return nameAttr{Formatted: full, GivenName: given, FamilyName: strings.TrimSpace(family)}
}

// primaryEmail picks the address to use from a SCIM emails list: the
// primary one, else the work one, else the first.
func primaryEmail(emails []emailAttr) string {
	for _, e := range emails {
		if e.Primary {
			return e.Value
		}
	}
	for _, e := range emails {
		if strings.EqualFold(e.Type, "work") {
			return e.Value
		}
	}
	if len(emails) > 0 {
		return emails[0].Value
	}
	return ""
}

// filter is a parsed `attr eq "value"` filter, the only kind identity
// providers send when provisioning.
type filter struct {
	attr  string // lower case
	value string
}

// parseFilter reads a filter, allowing only the given attributes.
func parseFilter(raw string, allowed map[string]string) (*filter, *scimError) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	attr, rest, ok := strings.Cut(raw, " ")
	if !ok {
		return nil, badRequest("invalidFilter", "the filter must look like: attribute eq \"value\"")
	}
	op, value, ok := strings.Cut(strings.TrimSpace(rest), " ")
	if !ok || !strings.EqualFold(op, "eq") {
		return nil, badRequest("invalidFilter", "only the eq operator is supported")
	}
	attr = strings.ToLower(attr)
	// Attributes may be written with their schema URN.
	attr = strings.TrimPrefix(attr, strings.ToLower(schemaUser)+":")
	attr = strings.TrimPrefix(attr, strings.ToLower(schemaGroup)+":")
	if _, ok := allowed[attr]; !ok {
		return nil, badRequest("invalidFilter", "filtering on %s isn't supported", attr)
	}
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, `"`) {
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return nil, badRequest("invalidFilter", "the filter's value isn't a valid string")
		}
		value = unquoted
	}
	return &filter{attr: attr, value: value}, nil
}

// page reads startIndex (1-based) and count into an offset and limit.
func page(r *http.Request) (start, offset, limit int, err *scimError) {
	q := r.URL.Query()
	start, limit = 1, defaultPageSize
	if raw := q.Get("startIndex"); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil {
			return 0, 0, 0, badRequest("invalidValue", "startIndex must be a number")
		}
		start = max(n, 1)
	}
	if raw := q.Get("count"); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil {
			return 0, 0, 0, badRequest("invalidValue", "count must be a number")
		}
		limit = min(max(n, 0), maxPageSize)
	}
	return start, start - 1, limit, nil
}

type patchRequest struct {
	Schemas    []string  `json:"schemas"`
	Operations []patchOp `json:"Operations"`
}

type patchOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// expand turns an operation without a path, whose value is an object of
// attributes, into one operation per attribute.
func (op patchOp) expand() ([]patchOp, *scimError) {
	if op.Path != "" {
		return []patchOp{op}, nil
	}
	var attrs map[string]json.RawMessage
	if err := json.Unmarshal(op.Value, &attrs); err != nil {
		return nil, badRequest("invalidValue", "an operation without a path needs an object of attributes")
	}
	out := make([]patchOp, 0, len(attrs))
	for k, v := range attrs {
		if k == "name" {
			// {"name": {"givenName": …}} sets each part.
			var parts map[string]json.RawMessage
			if json.Unmarshal(v, &parts) == nil {
				for pk, pv := range parts {
					out = append(out, patchOp{Op: op.Op, Path: "name." + pk, Value: pv})
				}
				continue
			}
		}
		out = append(out, patchOp{Op: op.Op, Path: k, Value: v})
	}
	return out, nil
}

// kind is the operation, lower case: add, replace or remove.
func (op patchOp) kind() (string, *scimError) {
	k := strings.ToLower(op.Op)
	if k != "add" && k != "replace" && k != "remove" {
		return "", badRequest("invalidSyntax", "unknown operation %q", op.Op)
	}
	return k, nil
}

// path is the operation's path, lower case and without the core schema URN.
func (op patchOp) path() string {
	p := strings.ToLower(strings.TrimSpace(op.Path))
	p = strings.TrimPrefix(p, strings.ToLower(schemaUser)+":")
	return strings.TrimPrefix(p, strings.ToLower(schemaGroup)+":")
}

func (op patchOp) str() (string, *scimError) {
	var s string
	if err := json.Unmarshal(op.Value, &s); err != nil {
		// Some providers wrap single values: [{"value": "x"}].
		var wrapped []ref
		if json.Unmarshal(op.Value, &wrapped) == nil && len(wrapped) == 1 {
			return wrapped[0].Value, nil
		}
		return "", badRequest("invalidValue", "%s must be a string", op.Path)
	}
	return s, nil
}

func (op patchOp) boolean() (bool, *scimError) {
	var b flexBool
	if err := json.Unmarshal(op.Value, &b); err != nil {
		return false, badRequest("invalidValue", "%s must be true or false", op.Path)
	}
	return bool(b), nil
}

// memberFilter reads the user id out of a path like members[value eq "42"].
func memberFilter(path string) (string, bool) {
	inner, ok := strings.CutPrefix(path, "members[")
	if !ok || !strings.HasSuffix(inner, "]") {
		return "", false
	}
	f, err := parseFilter(strings.TrimSuffix(inner, "]"), map[string]string{"value": ""})
	if err != nil {
		return "", false
	}
	return f.value, true
}

// refIDs reads the user ids out of a list of member references.
func refIDs(refs []ref) []int64 {
	ids := make([]int64, 0, len(refs))
	for _, r := range refs {
		if id, err := strconv.ParseInt(r.Value, 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// cleanText checks a single-line value from the identity provider.
func cleanText(s string, maxLen int, what string) (string, *scimError) {
	s = strings.TrimSpace(s)
	if len(s) > maxLen || strings.ContainsAny(s, "\r\n\x00") {
		return "", badRequest("invalidValue", "%s is too long or isn't one line", what)
	}
	return s, nil
}

func optText(s *string, maxLen int, what string) (*string, *scimError) {
	if s == nil {
		return nil, nil
	}
	v, err := cleanText(*s, maxLen, what)
	if err != nil || v == "" {
		return nil, err
	}
	return &v, nil
}
