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
// ze:help argument spans three lines, because a long explanation is the reason
// the extension exists and goyang has to return it whole. The second carries a
// description alone, which is what every unconverted RPC looks like.
const rpcHelpModule = `
module ze-fixture-api {
    namespace "urn:ze:fixture:api";
    prefix zefa;
    import ze-extensions { prefix ze; }

    rpc socket-list {
        description "List the open sockets.";
        ze:help "One row is written for each socket the daemon holds open.

                 The state column names the TCP state.";
        output {
            leaf count {
                type uint32;
                description "How many sockets are open.";
            }
        }
    }

    rpc socket-clear {
        description "Close every idle socket.";
    }
}
`

// TestRPCDescriptionCarriesSummaryAndHelp reads an RPC declaring both help
// texts and asserts each reaches its own field on the extracted metadata.
//
// VALIDATES: goyang exposes the extension statements of an rpc, so an RPC
// declares its long explanation through the same ze:help the command tree uses,
// and the description keeps the one-line summary.
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
	assert.Equal(t, "List the open sockets.", declared.Description)
	assert.Contains(t, declared.LongHelp, "One row is written for each socket")
	assert.Contains(t, declared.LongHelp, "The state column names the TCP state.")
	assert.Contains(t, declared.LongHelp, "\n", "a long explanation keeps the line breaks its author wrote")
	assert.NotContains(t, declared.Description, declared.LongHelp, "neither field is derived from the other")

	silent := byName["socket-clear"]
	assert.Equal(t, "Close every idle socket.", silent.Description)
	assert.Empty(t, silent.LongHelp, "no ze:help statement means no long explanation")
}
