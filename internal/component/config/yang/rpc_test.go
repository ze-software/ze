package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWireModule verifies YANG module name to wire method prefix conversion.
//
// VALIDATES: Module names are correctly stripped of -api/-conf suffixes.
// PREVENTS: Wrong method prefixes on the wire (e.g., "ze-bgp-api:peer-list" instead of "ze-bgp:peer-list").
func TestWireModule(t *testing.T) {
	tests := []struct {
		name   string
		module string
		want   string
	}{
		{"bgp-api", "ze-bgp-api", "ze-bgp"},
		{"system-api", "ze-system-api", "ze-system"},
		{"rib-api", "ze-rib-api", "ze-rib"},
		{"plugin-api", "ze-plugin-api", "ze-plugin"},
		{"bgp-conf", "ze-bgp-conf", "ze-bgp"},
		{"no-suffix", "ze-types", "ze-types"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, WireModule(tt.module))
		})
	}
}

// TestExtractRPCsNonexistentModule verifies graceful handling of missing modules.
//
// VALIDATES: Returns empty slice for nonexistent module.
// PREVENTS: Nil pointer panic when module doesn't exist.
func TestExtractRPCsNonexistentModule(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.Resolve())

	rpcs := ExtractRPCs(loader, "nonexistent-module")
	assert.Empty(t, rpcs, "should return empty for nonexistent module")
}

// TestExtractNotificationsNonexistentModule verifies graceful handling of missing modules.
//
// VALIDATES: Returns empty slice for nonexistent module.
// PREVENTS: Nil pointer panic when module doesn't exist.
func TestExtractNotificationsNonexistentModule(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.Resolve())

	notifs := ExtractNotifications(loader, "nonexistent-module")
	assert.Empty(t, notifs, "should return empty for nonexistent module")
}

// rpcHelpModule declares two RPCs. The first carries both help texts, and its
// description argument spans three lines, because a long explanation is what
// the statement carries and goyang has to return it whole. The second carries
// a ze:help summary alone, which is what every unconverted RPC looks like.
const rpcHelpModule = `
module ze-fixture-api {
    namespace "urn:ze:fixture:api";
    prefix zefa;
    import ze-extensions { prefix ze; }

    rpc socket-list {
        ze:help "List the open sockets.";
        description "One row is written for each socket the daemon holds open.

                 The state column names the TCP state.";
        output {
            leaf count {
                type uint32;
                ze:help "How many sockets are open.";
            }
        }
    }

    rpc socket-clear {
        ze:help "Close every idle socket.";
    }
}
`

// TestRPCDescriptionCarriesSummaryAndHelp reads an RPC declaring both help
// texts and asserts each reaches its own field on the extracted metadata.
//
// VALIDATES: goyang exposes the extension statements of an rpc, so an RPC
// declares its one-line summary through the same ze:help the command tree uses,
// and the description carries the long explanation.
// PREVENTS: a second mechanism for the long form on the RPC side, and an RPC
// whose summary and explanation share one string, which is the state every
// renderer guesses its way out of (AC-16).
func TestRPCDescriptionCarriesSummaryAndHelp(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText("ze-fixture-api.yang", rpcHelpModule))
	require.NoError(t, loader.Resolve())

	rpcs := ExtractRPCs(loader, "ze-fixture-api")
	require.Len(t, rpcs, 2)

	byName := map[string]RPCMeta{}
	for _, rpc := range rpcs {
		byName[rpc.Name] = rpc
	}

	declared := byName["socket-list"]
	assert.Equal(t, "List the open sockets.", declared.ShortHelp)
	assert.Contains(t, declared.Description, "One row is written for each socket")
	assert.Contains(t, declared.Description, "The state column names the TCP state.")
	assert.Contains(t, declared.Description, "\n", "a long explanation keeps the line breaks its author wrote")
	assert.NotContains(t, declared.ShortHelp, declared.Description, "neither field is derived from the other")

	silent := byName["socket-clear"]
	assert.Equal(t, "Close every idle socket.", silent.ShortHelp)
	assert.Empty(t, silent.Description, "no description statement means no long explanation")
}

// TestExtractRPCsCarriesBothLeafTexts proves an rpc input leaf, an rpc output
// leaf and a notification leaf each carry LeafMeta.ShortHelp from ze:help and
// LeafMeta.Description from description, and that a leaf declaring one text
// leaves the other empty.
func TestExtractRPCsCarriesBothLeafTexts(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())

	module := `
module ze-leaftexts-api {
    namespace "urn:test:leaftexts";
    prefix lt;

    import ze-extensions { prefix ze; }

    rpc socket-open {
        ze:help "Open a socket.";
        input {
            leaf port {
                type uint16;
                mandatory true;
                ze:help "The TCP port to listen on";
                description "The port the socket binds.";
            }
        }
        output {
            leaf fd {
                type int32;
                ze:help "The descriptor of the open socket";
                description "A descriptor the caller closes when it is done.";
            }
        }
    }

    notification socket-closed {
        ze:help "A socket closed.";
        leaf reason {
            type string;
            ze:help "Why the socket closed";
        }
    }
}
`
	require.NoError(t, loader.AddModuleFromText("ze-leaftexts-api.yang", module))
	require.NoError(t, loader.Resolve())

	rpcs := ExtractRPCs(loader, "ze-leaftexts-api")
	require.Len(t, rpcs, 1)
	require.Len(t, rpcs[0].Input, 1)
	assert.Equal(t, "The TCP port to listen on", rpcs[0].Input[0].ShortHelp)
	assert.Equal(t, "The port the socket binds.", rpcs[0].Input[0].Description)
	require.Len(t, rpcs[0].Output, 1)
	assert.Equal(t, "The descriptor of the open socket", rpcs[0].Output[0].ShortHelp)
	assert.Equal(t, "A descriptor the caller closes when it is done.", rpcs[0].Output[0].Description)

	notifs := ExtractNotifications(loader, "ze-leaftexts-api")
	require.Len(t, notifs, 1)
	require.Len(t, notifs[0].Leaves, 1)
	assert.Equal(t, "Why the socket closed", notifs[0].Leaves[0].ShortHelp)
	assert.Empty(t, notifs[0].Leaves[0].Description, "no description statement means no explanation")
}

// TestRPCInputKeepsTheOrderTheModuleDeclared pins the order an RPC's
// parameters come back in.
//
// VALIDATES: the leaves arrive in source order, and a leaf reached through a
// `uses` arrives after them by name rather than wherever a map put it.
// PREVENTS: an order decided by Go's map seed. Entry.Dir is a map, and ranging
// it was the whole source of this order, so the same schema described its own
// arguments differently on each process: the MCP tool's parameter list and the
// generated command help both read it. It also made a test of the first
// parameter pass or fail on the toss of that seed, which is how this was found
// (cmd/ze/hub, TestBuildParamMetaCarriesBothLeafTexts).
//
// The four declared names are in neither sorted nor reverse-sorted order, so a
// map iteration agreeing with the assertion by chance is a 1-in-24 event, and
// the loop repeats it until that is not worth arguing about.
func TestRPCInputKeepsTheOrderTheModuleDeclared(t *testing.T) {
	const module = `module ze-order-api {
  namespace "urn:ze:order"; prefix zo;
  grouping shared { leaf alpha { type string; } }
  rpc show-order {
    input {
      leaf zulu { type string; mandatory true; }
      leaf mike { type uint16; }
      leaf bravo { type string; }
      uses shared;
    }
  }
}
`
	want := []string{"zulu", "mike", "bravo", "alpha"}
	for range 20 {
		loader := NewLoader()
		require.NoError(t, loader.AddModuleFromText("ze-order-api", module))
		require.NoError(t, loader.Resolve())

		rpcs := ExtractRPCs(loader, "ze-order-api")
		require.Len(t, rpcs, 1, "the module declares one rpc")

		got := make([]string, 0, len(rpcs[0].Input))
		for _, leaf := range rpcs[0].Input {
			got = append(got, leaf.Name)
		}
		require.Equal(t, want, got,
			"declared order first, then what only the entry tree holds, by name")
	}
}
