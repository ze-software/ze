package aihelp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	// Registers ze-plugin-cmd.yang and ze-system-cmd.yang, which document
	// builtin RPCs, so TestBuildPublishesEachRPCOnce meets the overlap.
	_ "github.com/ze-software/ze/internal/core/ipc/yang"
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
		Commands:     []CLICommand{{Name: "show", Mode: "read-only", ShortHelp: "Show state", Subs: "ze show help"}},
		RPCs:         []RPC{{WireMethod: "ze-cmd:show-version", ShortHelp: "Show version"}, {WireMethod: "ze-cmd:show-uptime"}},
		DispatchKeys: map[string]string{"ze-cmd:show-version": "show version"},
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
	assert.Contains(t, s, `"wire-method":"ze-cmd:show-version"`)
	assert.Contains(t, s, `"dispatch-keys":{"ze-cmd:show-version":"show version"}`)
}

// TestBuildRunsAndInitializesDispatchKeys verifies Build assembles from the live
// registries without panicking and always returns a non-nil dispatch-keys map
// (so the JSON has a stable {} rather than null).
func TestBuildRunsAndInitializesDispatchKeys(t *testing.T) {
	ref, err := Build()
	require.NoError(t, err)
	require.NotNil(t, ref.DispatchKeys, "DispatchKeys must be initialized (never nil) for stable JSON")

	data, err := json.Marshal(ref)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

// TestReferenceRPCCarriesDescription pins the two RPC help keys in the JSON both
// `ze help ai --json` and the MCP ze_reference tool emit. They are the RPC half
// of the pair `ze help command --json` already carries for a command.
//
// VALIDATES: an RPC's summary and its long explanation are two distinct
// kebab-case keys, and the long one is omitted when nobody wrote it.
// PREVENTS: a consumer reading a one-line summary where a page of explanation
// was declared, or an empty "description" key implying an authored empty string.
func TestReferenceRPCCarriesDescription(t *testing.T) {
	ref := Reference{RPCs: []RPC{
		{WireMethod: "ze-bgp:peer-list", ShortHelp: "List the configured peers.", Description: "One row per peer."},
		{WireMethod: "ze-bgp:peer-detail", ShortHelp: "Show one peer."},
	}}

	data, err := json.Marshal(ref)
	require.NoError(t, err)

	s := string(data)
	assert.Contains(t, s, `"short-help":"List the configured peers.","description":"One row per peer."`)
	assert.Contains(t, s, `"short-help":"Show one peer."}`, "an RPC with no explanation carries no description key")
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
        ze:help "Summarize the fixture in one line.";
        description "The long explanation the reference carries whole.";
    }

    rpc fixture-summary-only {
        ze:help "Summarize the second fixture in one line.";
    }
}`

// helpFixtureCmdModule points one command node at each fixture rpc. An rpc
// is published under the method of the node that points at it, so without
// these nodes the fixture rpcs would be published under no name.
const helpFixtureCmdModule = `module ze-aihelpfixture-cmd {
    namespace "urn:ze:aihelpfixture:cmd";
    prefix ahfc;

    import ze-extensions { prefix ze; }

    description "Fixture command nodes for the aihelp reference tests.";

    revision 2026-10-08 { description "Initial revision"; }

    container aihelpfixture {
        config false;
        container both {
            config false;
            ze:command "ze-aihelpfixture:fixture-both";
            ze:rpc "ze-aihelpfixture-api:fixture-both";
        }
        container summary-only {
            config false;
            ze:command "ze-aihelpfixture:fixture-summary-only";
            ze:rpc "ze-aihelpfixture-api:fixture-summary-only";
        }
    }
}`

func init() {
	yang.RegisterModule("ze-aihelpfixture-api", helpFixtureModule)
	yang.RegisterModule("ze-aihelpfixture-cmd", helpFixtureCmdModule)
}

// TestBuildCarriesEveryRegisteredRPCHelpText verifies that the reference an
// agent reads carries what the schema registry holds, for both help texts and
// for every RPC. The registry is the single declaration; Build only projects it.
//
// VALIDATES: Build copies ShortHelp and Description for each registered RPC.
// PREVENTS: the long explanation reaching the registry and stopping there,
// which is where it stopped before this test existed.
func TestBuildCarriesEveryRegisteredRPCHelpText(t *testing.T) {
	published := make(map[string]RPC)
	ref, err := Build()
	require.NoError(t, err)
	for _, rpc := range ref.RPCs {
		published[rpc.WireMethod] = rpc
	}

	schemaReg, err := SchemaRegistry()
	require.NoError(t, err)
	longForms := 0
	for _, registered := range schemaReg.ListRPCs("") {
		got, ok := published[registered.WireMethod]
		if !ok {
			t.Errorf("the reference omits the registered RPC %q", registered.WireMethod)
			continue
		}
		assert.Equal(t, registered.ShortHelp, got.ShortHelp, "summary for %q", registered.WireMethod)
		assert.Equal(t, registered.Description, got.Description, "long help for %q", registered.WireMethod)
		if registered.Description != "" {
			longForms++
		}
	}

	// The comparison is only discriminating while some RPC declares a
	// description. helpFixtureModule guarantees one, whatever the build tags load.
	assert.Positive(t, longForms, "no registered RPC declares a description, so this test proved nothing")
	assert.Equal(t, "The long explanation the reference carries whole.",
		published["ze-aihelpfixture:fixture-both"].Description,
		"the fixture RPC's long form did not reach the reference")
	assert.Empty(t, published["ze-aihelpfixture:fixture-summary-only"].Description,
		"an RPC with no description gained a long form from somewhere")
}

// TestCLISubcommandModeMatchesTheVerbRegistry asserts that the mode this
// reference publishes for a top-level verb root is the role command.Verbs gives
// that verb. An agent picks a command by this field, so a read verb published
// as "daemon" tells it the command needs a running daemon and an operator with
// edit rights, when a read-only operator reaches it offline.
//
// Until 2026-09-14 the mode came from a three-word list that omitted resolve,
// so the resolve root published "daemon" (docs/architecture/cli/command-verbs.md,
// T-7). The assertion is against Verbs, so a verb whose role changes there, or
// a verb added to it, moves this test with it. A verb root the build does not
// carry is skipped: resolve itself is proven in
// internal/component/plugin/server, over the pure predicate this reference
// reads.
func TestCLISubcommandModeMatchesTheVerbRegistry(t *testing.T) {
	byName := make(map[string]CLICommand)
	commands, err := CLISubcommands()
	require.NoError(t, err)
	for _, cmd := range commands {
		byName[cmd.Name] = cmd
	}
	require.NotEmpty(t, byName, "the reference must publish at least one subcommand")

	checked := 0
	for verb, role := range command.Verbs {
		cmd, found := byName[verb]
		if !found {
			continue // The vocabulary holds verbs no command roots at yet.
		}
		checked++
		want := "daemon"
		if role == command.RoleRead {
			want = "read-only"
		}
		assert.Equal(t, want, cmd.Mode,
			"verb %q carries role %d, so `ze help ai` must publish mode %q", verb, role, want)
		assert.Equal(t, pluginserver.IsReadOnlyPath(verb), cmd.Mode == "read-only",
			"the mode published for %q must be the answer authorization gives for it", verb)
	}
	assert.Positive(t, checked, "no verb root reached the reference; the tree cannot be empty")
}

// TestBuildPublishesEachRPCOnce asserts that every wire method appears in the
// reference's RPC list exactly once. A builtin RPC that the schema registry
// also documents is published by its documented row, and the bare builtin row
// is dropped, the same rule `ze help ai` text output applies.
//
// VALIDATES: Build adds a builtin RPC only when no documented row names it.
// PREVENTS: `ze help ai --json` and the MCP ze_reference tool listing a method
// twice, once with its help text and once bare, so an agent sees two
// contradictory descriptions of one method.
func TestBuildPublishesEachRPCOnce(t *testing.T) {
	schemaReg, err := SchemaRegistry()
	require.NoError(t, err)
	documented := make(map[string]bool)
	for _, rpc := range schemaReg.ListRPCs("") {
		documented[rpc.WireMethod] = true
	}
	overlap := 0
	for _, brpc := range pluginserver.AllBuiltinRPCs() {
		if documented[brpc.WireMethod] {
			overlap++
		}
	}
	// Only discriminating while some builtin RPC is also documented: that is
	// the case which produced the duplicate rows.
	require.Positive(t, overlap, "no builtin RPC is also documented, so this test proves nothing")

	ref, err := Build()
	require.NoError(t, err)
	count := make(map[string]int, len(ref.RPCs))
	for _, rpc := range ref.RPCs {
		count[rpc.WireMethod]++
	}
	for method, n := range count {
		assert.Equal(t, 1, n, "the reference publishes %q %d times", method, n)
	}
}

// TestSchemaRegistryRefusesABrokenRPCPointer proves a ze:rpc pointer the schema
// cannot honour reaches the caller as an error.
//
// VALIDATES: schemaRegistryFrom, the step SchemaRegistry and Build publish
// through, returns yang.ErrRPCPointer for a ze:command node whose ze:rpc names
// an rpc its loaded module does not declare.
// PREVENTS: `ze help ai` and the MCP ze_reference tool publishing no documented
// RPC, with no error, because one pointer is mistyped.
func TestSchemaRegistryRefusesABrokenRPCPointer(t *testing.T) {
	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	const module = `module ze-fixture-cmd {
    namespace "urn:ze:fixture:cmd";
    prefix zefc;
    import ze-extensions { prefix ze; }
    container show {
        config false;
        container gone { config false; ze:command "ze-fixture:show-gone"; ze:rpc "ze-extensions:no-such-rpc"; }
    }
}`
	require.NoError(t, loader.AddModuleFromText("ze-fixture-cmd.yang", module))
	require.NoError(t, loader.Resolve())

	schemaReg, err := schemaRegistryFrom(loader)
	require.ErrorIs(t, err, yang.ErrRPCPointer)
	assert.Nil(t, schemaReg)
}
