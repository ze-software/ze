package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
)

// VALIDATES: AC-6 -- OpenAPISchema returns valid OpenAPI 3.1 JSON.
// PREVENTS: malformed OpenAPI spec.
func TestOpenAPISchemaValid(t *testing.T) {
	commands := []CommandMeta{
		{Name: "bgp summary", ShortHelp: "Show BGP summary", ReadOnly: true},
		{Name: "daemon reload", ShortHelp: "Reload config", ReadOnly: false},
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
	// The one execute route REST serves; a command has no path of its own.
	assert.Len(t, paths, 1)
	assert.Contains(t, paths, "/api/v1/execute")
}

// VALIDATES: each command is published as the request body that runs it on
// POST /api/v1/execute: `command` fixed to its name, its parameters under
// `params`, the pipe list, and `params` required only when a parameter is.
// PREVENTS: a command's parameter schema lost with its unserved path, or a
// schema that does not name the command it runs.
func TestOpenAPIPublishesEachCommandRequestBody(t *testing.T) {
	commands := []CommandMeta{
		{Name: "bgp summary", ShortHelp: "Show BGP summary", ReadOnly: true},
		{Name: "show bgp rib", ShortHelp: "Show routes", ReadOnly: true, Params: []ParamMeta{
			{Name: "limit", Type: "uint32", ShortHelp: "Max results", Required: true},
		}},
	}

	data, err := OpenAPISchema(commands)
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))
	components, ok := spec["components"].(map[string]any)
	require.True(t, ok)
	schemas, ok := components["schemas"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, schemas, "ExecResult")

	summary, ok := schemas["bgpSummary"].(map[string]any)
	require.True(t, ok, "a command without parameters is still published")
	summaryProps, ok := summary["properties"].(map[string]any)
	require.True(t, ok)
	summaryCommand, ok := summaryProps["command"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "bgp summary", summaryCommand["const"])
	assert.Contains(t, summaryProps, command.PipeListParameter)
	assert.Equal(t, []any{"command"}, summary["required"],
		"params is not required when no parameter is")

	rib, ok := schemas["showBgpRib"].(map[string]any)
	require.True(t, ok)
	ribProps, ok := rib["properties"].(map[string]any)
	require.True(t, ok)
	ribCommand, ok := ribProps["command"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "show bgp rib", ribCommand["const"])
	params, ok := ribProps["params"].(map[string]any)
	require.True(t, ok)
	paramProps, ok := params["properties"].(map[string]any)
	require.True(t, ok)
	limit, ok := paramProps["limit"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "integer", limit["type"])
	assert.Equal(t, []any{"limit"}, params["required"])
	assert.Equal(t, []any{"command", "params"}, rib["required"],
		"params is required when one of its parameters is")
}

// VALIDATES: OpenAPISchema publishes no path of its own for a command: the
// one execute route REST serves runs every command.
// PREVENTS: a published /api/v1/execute/<command path> that no handler
// serves, so a client following the document meets a 404.
func TestOpenAPIPublishesNoPerCommandPath(t *testing.T) {
	commands := []CommandMeta{
		{Name: "bgp summary", ShortHelp: "Show BGP summary", ReadOnly: true},
		{Name: "daemon reload", ShortHelp: "Reload config", ReadOnly: false},
	}

	data, err := OpenAPISchema(commands)
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))
	paths, ok := spec["paths"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, paths, 1)
	assert.Contains(t, paths, "/api/v1/execute")
	assert.NotContains(t, paths, "/api/v1/execute/bgp/summary")
	assert.NotContains(t, paths, "/api/v1/execute/daemon/reload")
}

// VALIDATES: OpenAPI's command request body carries the command's one-line
// summary as `title` and its long explanation as `description`, the two roles
// JSON Schema defines for them, and no description key when none is declared.
// PREVENTS: a client reading the whole explanation as a list-row label, or
// finding no explanation at all because both halves went into one key.
func TestOpenAPICommandSchemaCarriesSummaryAndDescription(t *testing.T) {
	commands := []CommandMeta{
		{
			Name:        "bgp summary",
			ShortHelp:   "Show one line for each configured peer.",
			Description: "The state column is the FSM state.\nThe prefix counts are what the peer has sent.",
			ReadOnly:    true,
		},
		{Name: "daemon reload", ShortHelp: "Reload the running configuration."},
	}

	data, err := OpenAPISchema(commands)
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))
	components, ok := spec["components"].(map[string]any)
	require.True(t, ok)
	schemas, ok := components["schemas"].(map[string]any)
	require.True(t, ok)

	summary, ok := schemas["bgpSummary"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Show one line for each configured peer.", summary["title"],
		"title is the one-line half, verbatim")
	assert.Equal(t,
		"The state column is the FSM state.\nThe prefix counts are what the peer has sent.",
		summary["description"], "description is the long half, verbatim and with its newlines")

	reload, ok := schemas["daemonReload"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Reload the running configuration.", reload["title"])
	assert.NotContains(t, reload, "description")
}

// VALIDATES: CommandSchema maps YANG params to JSON Schema properties.
// PREVENTS: lost or mistyped parameters.
func TestCommandSchemaMatchesYANG(t *testing.T) {
	cmd := CommandMeta{
		Name: "show bgp rib",
		Params: []ParamMeta{
			{Name: "family", Type: "string", ShortHelp: "Address family", Required: false},
			{Name: "limit", Type: "uint32", ShortHelp: "Max results", Required: true},
			{Name: "active", Type: "boolean", ShortHelp: "Active only", Required: false},
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

// TestCommandSchemaWritesTitleAndDescription proves a property carries the
// parameter's summary as JSON Schema `title` and its explanation as
// `description`, each only when the parameter declares it, and that neither
// is derived from the other.
func TestCommandSchemaWritesTitleAndDescription(t *testing.T) {
	cmd := CommandMeta{
		Name: "request socket open",
		Params: []ParamMeta{
			{Name: "port", Type: "uint16", ShortHelp: "The TCP port to listen on", Description: "The port the socket binds.", Required: true},
			{Name: "label", Type: "string", ShortHelp: "A label for the socket"},
			{Name: "owner", Type: "string", Description: "The user the socket is opened for."},
			{Name: "silent", Type: "string"},
		},
	}

	props, ok := CommandSchema(cmd)["properties"].(map[string]any)
	require.True(t, ok)

	port, ok := props["port"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "The TCP port to listen on", port["title"])
	assert.Equal(t, "The port the socket binds.", port["description"])

	label, ok := props["label"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "A label for the socket", label["title"])
	_, hasDescription := label["description"]
	assert.False(t, hasDescription, "an undeclared explanation is not written, and not derived from the summary")

	owner, ok := props["owner"].(map[string]any)
	require.True(t, ok)
	_, hasTitle := owner["title"]
	assert.False(t, hasTitle, "an undeclared summary is not written, and not derived from the explanation")
	assert.Equal(t, "The user the socket is opened for.", owner["description"])

	silent, ok := props["silent"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"type": "string"}, silent)
}
