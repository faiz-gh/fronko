// Package directory is Fronko's SCIM 2.0 server: an organisation's identity
// provider (Microsoft Entra ID, Okta…) adds, updates, deactivates and
// removes people, and keeps teams in step with its groups.
//
// The provider authenticates with the bearer token of the organisation's
// directory connection (one per organisation). People it adds are members;
// they get a temporary password by email, or none when the organisation
// signs in with single sign-on. Deactivating someone suspends them.
package directory

import (
	"net/http"
	"strings"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/app"
)

// The per-IP budget: identity providers sync in bursts.
const (
	burst    = 200
	interval = 20 * time.Millisecond
)

// Routes serves the SCIM API under /scim/v2.
func (h *Handler) Routes(r *app.Routes) {
	limit := r.RateLimit(interval, burst)
	route := func(pattern string, fn handlerFunc) {
		method, path, _ := strings.Cut(pattern, " ")
		r.External(method+" "+BasePath+path, limit(h.authed(fn)))
	}
	route("GET /ServiceProviderConfig", serviceProviderConfig)
	route("GET /ResourceTypes", resourceTypes)
	route("GET /Schemas", schemas)

	route("GET /Users", h.listUsers)
	route("POST /Users", h.createUser)
	route("GET /Users/{id}", h.getUser)
	route("PUT /Users/{id}", h.replaceUser)
	route("PATCH /Users/{id}", h.patchUser)
	route("DELETE /Users/{id}", h.deleteUser)

	route("GET /Groups", h.listGroups)
	route("POST /Groups", h.createGroup)
	route("GET /Groups/{id}", h.getGroup)
	route("PUT /Groups/{id}", h.replaceGroup)
	route("PATCH /Groups/{id}", h.patchGroup)
	route("DELETE /Groups/{id}", h.deleteGroup)
}

func serviceProviderConfig(w http.ResponseWriter, _ *http.Request, c *call) *scimError {
	writeSCIM(w, http.StatusOK, map[string]any{
		"schemas":          []string{schemaSPConfig},
		"documentationUri": "https://github.com/faiz-gh/fronko/blob/master/docs/integrations/entra-scim.md",
		"patch":            map[string]bool{"supported": true},
		"bulk":             map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
		"filter":           map[string]any{"supported": true, "maxResults": maxPageSize},
		"changePassword":   map[string]bool{"supported": false},
		"sort":             map[string]bool{"supported": false},
		"etag":             map[string]bool{"supported": false},
		"authenticationSchemes": []map[string]any{{
			"type": "oauthbearertoken", "name": "Bearer token", "primary": true,
			"description": "The token generated on the directory connection in Fronko's Integrations page.",
		}},
		"meta": map[string]string{"resourceType": "ServiceProviderConfig", "location": c.base + "/ServiceProviderConfig"},
	})
	return nil
}

func resourceTypes(w http.ResponseWriter, _ *http.Request, c *call) *scimError {
	types := []any{
		map[string]any{"schemas": []string{schemaResType}, "id": "User", "name": "User", "endpoint": "/Users",
			"schema": schemaUser, "meta": map[string]string{"resourceType": "ResourceType", "location": c.base + "/ResourceTypes/User"}},
		map[string]any{"schemas": []string{schemaResType}, "id": "Group", "name": "Group", "endpoint": "/Groups",
			"schema": schemaGroup, "meta": map[string]string{"resourceType": "ResourceType", "location": c.base + "/ResourceTypes/Group"}},
	}
	writeSCIM(w, http.StatusOK, listResponse{Schemas: []string{schemaList}, TotalResults: len(types), StartIndex: 1,
		ItemsPerPage: len(types), Resources: types})
	return nil
}

func attr(name, typ string, required bool, sub ...map[string]any) map[string]any {
	a := map[string]any{"name": name, "type": typ, "multiValued": false, "required": required,
		"caseExact": false, "mutability": "readWrite", "returned": "default", "uniqueness": "none"}
	if len(sub) > 0 {
		a["subAttributes"] = sub
	}
	return a
}

func multi(a map[string]any) map[string]any {
	a["multiValued"] = true
	return a
}

func schemas(w http.ResponseWriter, _ *http.Request, c *call) *scimError {
	userName := attr("userName", "string", true)
	userName["uniqueness"] = "server"
	list := []any{
		map[string]any{
			"schemas": []string{schemaSchema}, "id": schemaUser, "name": "User", "description": "A person in the organisation",
			"attributes": []any{
				userName,
				attr("externalId", "string", false),
				attr("displayName", "string", false),
				attr("name", "complex", false, attr("formatted", "string", false),
					attr("givenName", "string", false), attr("familyName", "string", false)),
				multi(attr("emails", "complex", true, attr("value", "string", true),
					attr("type", "string", false), attr("primary", "boolean", false))),
				attr("active", "boolean", false),
			},
			"meta": map[string]string{"resourceType": "Schema", "location": c.base + "/Schemas/" + schemaUser},
		},
		map[string]any{
			"schemas": []string{schemaSchema}, "id": schemaGroup, "name": "Group", "description": "A team",
			"attributes": []any{
				attr("displayName", "string", true),
				attr("externalId", "string", false),
				multi(attr("members", "complex", false, attr("value", "string", false), attr("display", "string", false))),
			},
			"meta": map[string]string{"resourceType": "Schema", "location": c.base + "/Schemas/" + schemaGroup},
		},
	}
	writeSCIM(w, http.StatusOK, listResponse{Schemas: []string{schemaList}, TotalResults: len(list), StartIndex: 1,
		ItemsPerPage: len(list), Resources: list})
	return nil
}
