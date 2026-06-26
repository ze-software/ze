package aihelp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReferenceJSONShape locks the wire shape of the AI reference so the CLI
// (ze help ai --json) and the MCP ze_reference tool stay byte-compatible. The
// top-level keys and the kebab-case "wire-method"/"dispatch-keys" tags are the
// contract both surfaces depend on.
//
// VALIDATES: the Reference JSON keys and tags that the CLI and MCP both emit.
// PREVENTS: a struct-tag change silently diverging ze help ai --json from the
// ze_reference MCP tool.
func TestReferenceJSONShape(t *testing.T) {
	ref := Reference{
		Commands:     []CLICommand{{Name: "show", Mode: "read-only", Description: "Show state", Subs: "ze show help"}},
		RPCs:         []RPC{{WireMethod: "ze-show:version", Description: "Show version"}, {WireMethod: "ze-show:uptime"}},
		DispatchKeys: map[string]string{"ze-show:version": "show version"},
		Plugins:      []Plugin{{Name: "bgp", Description: "core", Families: []string{"ipv4/unicast"}}},
		Families:     []string{"ipv4/unicast"},
		Services:     []ServiceRef{{Name: "web", Leaves: []string{"listen"}}},
	}
	data, err := json.Marshal(ref)
	require.NoError(t, err)

	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &got))
	for _, key := range []string{"commands", "rpcs", "dispatch-keys", "plugins", "families", "services"} {
		_, ok := got[key]
		assert.True(t, ok, "reference JSON must contain top-level key %q", key)
	}

	s := string(data)
	assert.Contains(t, s, `"wire-method":"ze-show:version"`)
	assert.Contains(t, s, `"dispatch-keys":{"ze-show:version":"show version"}`)
}

// TestBuildRunsAndInitializesDispatchKeys verifies Build assembles from the live
// registries without panicking and always returns a non-nil dispatch-keys map
// (so the JSON has a stable {} rather than null).
func TestBuildRunsAndInitializesDispatchKeys(t *testing.T) {
	ref := Build()
	require.NotNil(t, ref.DispatchKeys, "DispatchKeys must be initialized (never nil) for stable JSON")

	data, err := json.Marshal(ref)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}
