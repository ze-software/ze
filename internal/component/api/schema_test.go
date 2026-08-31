package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: AC-6 -- OpenAPISchema returns valid OpenAPI 3.1 JSON.
// PREVENTS: malformed OpenAPI spec.
func TestOpenAPISchemaValid(t *testing.T) {
	commands := []CommandMeta{
		{Name: "bgp summary", Description: "Show BGP summary", ReadOnly: true},
		{Name: "daemon reload", Description: "Reload config", ReadOnly: false},
	}

	data, err := OpenAPISchema(commands)
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	assert.Equal(t, "3.1.0", spec["openapi"])

	info, ok := spec["info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Ze API", info["title"])

	paths, ok := spec["paths"].(map[string]any)
	require.True(t, ok)
	// 2 per-command paths + 1 generic execute.
	assert.Len(t, paths, 3)
	assert.Contains(t, paths, "/api/v1/execute/bgp/summary")
	assert.Contains(t, paths, "/api/v1/execute/daemon/reload")
	assert.Contains(t, paths, "/api/v1/execute")
}

// VALIDATES: read-only commands use GET, write commands use POST.
// PREVENTS: wrong HTTP method in OpenAPI spec.
func TestOpenAPISchemaHTTPMethods(t *testing.T) {
	commands := []CommandMeta{
		{Name: "bgp summary", Description: "Show BGP summary", ReadOnly: true},
		{Name: "daemon reload", Description: "Reload config", ReadOnly: false},
	}

	data, err := OpenAPISchema(commands)
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	paths, ok := spec["paths"].(map[string]any)
	require.True(t, ok)

	bgpPath, ok := paths["/api/v1/execute/bgp/summary"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, bgpPath, "get")
	assert.NotContains(t, bgpPath, "post")

	reloadPath, ok := paths["/api/v1/execute/daemon/reload"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, reloadPath, "post")
	assert.NotContains(t, reloadPath, "get")
}

// VALIDATES: CommandSchema maps YANG params to JSON Schema properties.
// PREVENTS: lost or mistyped parameters.
func TestCommandSchemaMatchesYANG(t *testing.T) {
	cmd := CommandMeta{
		Name: "show bgp rib",
		Params: []ParamMeta{
			{Name: "family", Type: "string", Description: "Address family", Required: false},
			{Name: "limit", Type: "uint32", Description: "Max results", Required: true},
			{Name: "active", Type: "boolean", Description: "Active only", Required: false},
		},
	}

	schema := CommandSchema(cmd)

	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, props, 3)

	familyProp, ok := props["family"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "string", familyProp["type"])

	limitProp, ok := props["limit"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "integer", limitProp["type"])

	activeProp, ok := props["active"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "boolean", activeProp["type"])

	required, ok := schema["required"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"limit"}, required)
}

// VALIDATES: CommandSchema with no params returns empty properties.
// PREVENTS: nil pointer on empty param list.
func TestCommandSchemaNoParams(t *testing.T) {
	cmd := CommandMeta{Name: "bgp summary"}

	schema := CommandSchema(cmd)

	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Empty(t, props)
	assert.Nil(t, schema["required"])
}

// VALIDATES: operationID converts command names to camelCase.
// PREVENTS: invalid OpenAPI operationId values.
func TestOperationID(t *testing.T) {
	assert.Equal(t, "showBgpRib", operationID("show bgp rib"))
	assert.Equal(t, "bgpSummary", operationID("bgp summary"))
	assert.Equal(t, "daemonReload", operationID("daemon reload"))
	assert.Equal(t, "peer", operationID("peer"))
	assert.Equal(t, "", operationID(""))
}

// VALIDATES: OpenAPI spec includes security scheme.
// PREVENTS: missing auth documentation.
func TestOpenAPISchemaSecurityScheme(t *testing.T) {
	data, err := OpenAPISchema(nil)
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	components, ok := spec["components"].(map[string]any)
	require.True(t, ok)
	schemes, ok := components["securitySchemes"].(map[string]any)
	require.True(t, ok)
	bearer, ok := schemes["bearerAuth"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "http", bearer["type"])
	assert.Equal(t, "bearer", bearer["scheme"])
}

// VALIDATES: story 8 -- the OpenAPI operation carries the one-line summary in
// `summary` and the long explanation in `description`, the two roles OpenAPI
// defines for them.
// PREVENTS: a client reading the whole explanation as a list-row label, or
// finding no explanation at all because both halves went into one key.
func TestOpenAPICarriesSummaryAndDescription(t *testing.T) {
	commands := []CommandMeta{
		{
			Name:        "bgp summary",
			Description: "Show one line for each configured peer.",
			LongHelp:    "The state column is the FSM state.\nThe prefix counts are what the peer has sent.",
			ReadOnly:    true,
		},
		{Name: "daemon reload", Description: "Reload the running configuration."},
	}

	data, err := OpenAPISchema(commands)
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	paths, ok := spec["paths"].(map[string]any)
	require.True(t, ok)

	bgpPath, ok := paths["/api/v1/execute/bgp/summary"].(map[string]any)
	require.True(t, ok)
	get, ok := bgpPath["get"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "Show one line for each configured peer.", get["summary"],
		"summary is the one-line half, verbatim")
	assert.Equal(t,
		"The state column is the FSM state.\nThe prefix counts are what the peer has sent.",
		get["description"], "description is the long half, verbatim and with its newlines")

	// A command declaring no explanation carries no description key at all,
	// rather than an empty string a client would render as a blank paragraph.
	reloadPath, ok := paths["/api/v1/execute/daemon/reload"].(map[string]any)
	require.True(t, ok)
	post, ok := reloadPath["post"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Reload the running configuration.", post["summary"])
	assert.NotContains(t, post, "description")
}
