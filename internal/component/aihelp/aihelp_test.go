package aihelp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
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

// TestReferenceRPCCarriesLongHelp pins the two RPC help keys in the JSON both
// `ze help ai --json` and the MCP ze_reference tool emit. They are the RPC half
// of the pair `ze help command --json` already carries for a command.
//
// VALIDATES: an RPC's summary and its long explanation are two distinct
// kebab-case keys, and the long one is omitted when nobody wrote it.
// PREVENTS: a consumer reading a one-line summary where a page of explanation
// was declared, or an empty "long-help" key implying an authored empty string.
func TestReferenceRPCCarriesLongHelp(t *testing.T) {
	ref := Reference{RPCs: []RPC{
		{WireMethod: "ze-bgp:peer-list", Description: "List the configured peers.", LongHelp: "One row per peer."},
		{WireMethod: "ze-bgp:peer-detail", Description: "Show one peer."},
	}}

	data, err := json.Marshal(ref)
	require.NoError(t, err)

	s := string(data)
	assert.Contains(t, s, `"description":"List the configured peers.","long-help":"One row per peer."`)
	assert.Contains(t, s, `"description":"Show one peer."}`, "an RPC with no explanation carries no long-help key")
}

// helpFixtureModule is a YANG API module registered by this test file alone. It
// gives the package a registered RPC that declares BOTH help texts, so the
// end-to-end check below cannot pass by finding nothing to compare. The RPC
// names are prefixed so no production wire method collides with them.
const helpFixtureModule = `module ze-aihelpfixture-api {
    namespace "urn:ze:aihelpfixture:api";
    prefix ahf;

    import ze-extensions { prefix ze; }

    description "Fixture module for the aihelp reference tests.";

    revision 2026-08-31 { description "Initial revision"; }

    rpc fixture-both {
        description "Summarize the fixture in one line.";
        ze:help "The long explanation the reference carries whole.";
    }

    rpc fixture-summary-only {
        description "Summarize the second fixture in one line.";
    }
}`

func init() { yang.RegisterModule("ze-aihelpfixture-api", helpFixtureModule) }

// TestBuildCarriesEveryRegisteredRPCHelpText verifies that the reference an
// agent reads carries what the schema registry holds, for both help texts and
// for every RPC. The registry is the single declaration; Build only projects it.
//
// VALIDATES: Build copies Description and LongHelp for each registered RPC.
// PREVENTS: the long explanation reaching the registry and stopping there,
// which is where it stopped before this test existed.
func TestBuildCarriesEveryRegisteredRPCHelpText(t *testing.T) {
	published := make(map[string]RPC)
	for _, rpc := range Build().RPCs {
		published[rpc.WireMethod] = rpc
	}

	longForms := 0
	for _, registered := range SchemaRegistry().ListRPCs("") {
		got, ok := published[registered.WireMethod]
		if !ok {
			t.Errorf("the reference omits the registered RPC %q", registered.WireMethod)
			continue
		}
		assert.Equal(t, registered.Description, got.Description, "summary for %q", registered.WireMethod)
		assert.Equal(t, registered.LongHelp, got.LongHelp, "long help for %q", registered.WireMethod)
		if registered.LongHelp != "" {
			longForms++
		}
	}

	// The comparison is only discriminating while some RPC declares a ze:help.
	// helpFixtureModule guarantees one, whatever the build tags load.
	assert.Positive(t, longForms, "no registered RPC declares a ze:help, so this test proved nothing")
	assert.Equal(t, "The long explanation the reference carries whole.",
		published["ze-aihelpfixture:fixture-both"].LongHelp,
		"the fixture RPC's long form did not reach the reference")
	assert.Empty(t, published["ze-aihelpfixture:fixture-summary-only"].LongHelp,
		"an RPC with no ze:help gained a long form from somewhere")
}
