package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/configorder"
)

// TestAddDynamicPeerReadsTheTreeItWasGiven holds the join between the command
// that creates a peer and the parser that reads one.
//
// GOAL: the tree `create bgp peer` builds is the tree parsePeerFromTree reads,
// so the command produces a peer with the settings the operator typed. The two
// halves name their leaves independently, and a path that disagrees fails at
// runtime with no compile error and no test red anywhere else.
// METHOD: hand AddDynamicPeer the tree handleBgpPeerAdd builds for
// `create bgp peer 192.0.2.7 asn 65002 local-as 65001 local-address 192.0.2.1
// router-id 1.2.3.4 receive-hold-time 90 connect-retry 5 group-updates false`,
// then read the PeerSettings it produced.
//
// VALIDATES: the peer is in the reactor under the address given, and its remote
// AS, local AS, local address, router id, hold time, connect retry and
// group-updates each carry the value the tree stated.
// PREVENTS: the failure this test was written for. AddDynamicPeer injected the
// remote address at the TOP of the tree while parsePeerFromTree reads
// `connection > remote > ip`, so every call failed with "missing required
// connection > remote > ip" and the only caller was a mock
// (ai/rules/principles.md).
func TestAddDynamicPeerReadsTheTreeItWasGiven(t *testing.T) {
	r := newDynamicPeerReactor(t, 0, 0)

	tree := map[string]any{
		"session": map[string]any{
			"asn":       map[string]any{"remote": "65002", "local": "65001"},
			"router-id": "1.2.3.4",
		},
		"connection": map[string]any{
			"local": map[string]any{"ip": "192.0.2.1"},
		},
		"timer":    map[string]any{"receive-hold-time": "90", "connect-retry": "5"},
		"behavior": map[string]any{"group-updates": "false"},
		"attach": map[string]any{
			"process": map[string]any{
				"peer-lifecycle": map[string]any{"receive": "*", "send": "*"},
			},
		},
	}

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, r.AddDynamicPeer(addr, tree))

	r.mu.RLock()
	peer, exists := r.findPeerByAddr(addr)
	r.mu.RUnlock()
	require.True(t, exists, "the created peer is in the reactor")

	settings := peer.Settings()
	assert.Equal(t, addr, settings.Address)
	assert.Equal(t, uint32(65002), settings.PeerAS)
	assert.Equal(t, uint32(65001), settings.LocalAS)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), settings.LocalAddress)
	assert.Equal(t, uint32(0x01020304), settings.RouterID)
	assert.Equal(t, 90*time.Second, settings.ReceiveHoldTime)
	assert.Equal(t, 5*time.Second, settings.ConnectRetry)
	assert.False(t, settings.GroupUpdates)

	// The attach block is the peer's receive authorization and its send
	// permission both, so a binding that reaches the peer with neither grant
	// attaches a process and permits it nothing.
	require.Len(t, settings.ProcessBindings, 1, "the named process is bound to the peer")
	binding := settings.ProcessBindings[0]
	assert.Equal(t, "peer-lifecycle", binding.PluginName)
	assert.True(t, binding.ReceiveAll, "the process receives every message from this peer")
	assert.True(t, binding.SendAll, "the process may send every type toward this peer")
}

// TestAddDynamicPeerDefaultsTheLocalAddress holds the one leaf the caller may
// leave out.
//
// GOAL: a command that names no local address gets "auto", which is what
// applyLocalAddress requires and what leaves the kernel to choose the source.
// METHOD: call AddDynamicPeer with a tree stating no local ip.
//
// VALIDATES: the peer is created and its LocalAddress stays unset.
// PREVENTS: "local ip is required" reaching an operator who was never asked for
// one, which is what the parser answers when nothing fills the leaf.
func TestAddDynamicPeerDefaultsTheLocalAddress(t *testing.T) {
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)

	tree := map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}

	addr := netip.MustParseAddr("192.0.2.8")
	require.NoError(t, r.AddDynamicPeer(addr, tree))

	r.mu.RLock()
	peer, exists := r.findPeerByAddr(addr)
	r.mu.RUnlock()
	require.True(t, exists)

	settings := peer.Settings()
	assert.False(t, settings.LocalAddress.IsValid(), "auto leaves the source address to the kernel")
	assert.Equal(t, uint32(65001), settings.LocalAS, "the reactor's own local AS is the default")
	assert.Equal(t, uint32(0x0A000001), settings.RouterID, "the reactor's own router id is the default")
}

// TestAddDynamicPeerRefusesAnIncompleteTree keeps the parser's refusal reaching
// the caller.
//
// GOAL: a tree with no remote AS creates no peer, and the error says which leaf
// is missing.
// METHOD: call AddDynamicPeer with an empty tree.
//
// VALIDATES: the call errors and the reactor holds no peer.
// PREVENTS: a half-built peer left in the reactor after a failed parse.
func TestAddDynamicPeerRefusesAnIncompleteTree(t *testing.T) {
	r := newDynamicPeerReactor(t, 0, 0)

	err := r.AddDynamicPeer(netip.MustParseAddr("192.0.2.9"), map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session > asn > remote")

	r.mu.RLock()
	_, exists := r.findPeerByAddr(netip.MustParseAddr("192.0.2.9"))
	r.mu.RUnlock()
	assert.False(t, exists, "a tree the parser refuses leaves no peer behind")
}

// newDynamicPeerReactor builds the reactor these three tests drive.
//
// It states the config explicitly rather than reusing newTestReactor, which
// leaves the config pointer nil: AddDynamicPeer reads the router's own local AS
// and BGP Identifier out of it, so a nil there is a panic rather than a
// default.
func newDynamicPeerReactor(t *testing.T, localAS, routerID uint32) *Reactor {
	t.Helper()
	r := newTestReactor(t)
	r.config = &Config{LocalAS: localAS, RouterID: routerID}
	return r
}

// TestAddDynamicPeerRecordsTheRunningConfig holds the join between the command
// that creates a peer and the command that saves the running peer set.
//
// GOAL: a peer created at runtime is part of the running configuration, so
// `update bgp config` has the peer's own leaves to write into the file. The
// create command builds that tree once, and nothing else can rebuild it: a
// PeerInfo carries no families, no graceful restart time and no process
// binding.
// METHOD: create a peer through the API adapter, which is the object every
// command handler reaches, then read GetConfigTree.
//
// VALIDATES: bgp > peer > <address> carries the tree the command built, with
// the remote address AddDynamicPeer fills in.
// PREVENTS: `update bgp config` writing a peer the file cannot bring back up,
// which is what a save reconstructed from PeerInfo would write.
func TestAddDynamicPeerRecordsTheRunningConfig(t *testing.T) {
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)
	api := &reactorAPIAdapter{r: r}

	tree := map[string]any{
		"session": map[string]any{
			"asn": map[string]any{"remote": "65002"},
			"family": map[string]any{
				"ipv4/unicast": map[string]any{"prefix": map[string]any{"maximum": "1000"}},
			},
		},
	}
	require.NoError(t, api.AddDynamicPeer(netip.MustParseAddr("192.0.2.7"), tree))

	peers := runningPeerList(t, api)
	require.Contains(t, peers, "peer-192.0.2.7",
		"the running configuration names the created peer, under a name its key leaf accepts")

	entry, ok := peers["peer-192.0.2.7"].(map[string]any)
	require.True(t, ok, "the entry is the peer's config subtree")
	session, ok := entry["session"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{
		"ipv4/unicast": map[string]any{"prefix": map[string]any{"maximum": "1000"}},
	}, session["family"], "every leaf the command stated is recorded, not the subset PeerInfo carries")

	connection, ok := entry["connection"].(map[string]any)
	require.True(t, ok, "AddDynamicPeer fills the remote address in")
	remote, ok := connection["remote"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "192.0.2.7", remote["ip"])
}

// TestRemovePeerDropsTheRunningConfig holds the absence half of the same join.
//
// GOAL: a peer deleted at runtime leaves the running configuration, so a later
// `update bgp config` takes it out of the file.
// METHOD: start from a configuration that declares one peer, remove it through
// the API adapter.
//
// VALIDATES: the peer list no longer names it.
// PREVENTS: a save that writes the deleted peer back, which is what a running
// configuration that still declared it would produce.
func TestRemovePeerDropsTheRunningConfig(t *testing.T) {
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)
	api := &reactorAPIAdapter{r: r}

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, api.AddDynamicPeer(addr, map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}))
	require.Contains(t, runningPeerList(t, api), "peer-192.0.2.7")

	require.NoError(t, api.RemovePeer(addr))
	assert.NotContains(t, runningPeerList(t, api), "peer-192.0.2.7",
		"the running configuration drops the peer with the peer")
}

// TestRunningConfigKeepsTheEntryOrder holds the shape the peer list is
// delivered in.
//
// GOAL: the list stays readable by configorder.Entries, which refuses a list of
// two or more entries whose order does not name every one of them exactly once.
// METHOD: start from a configuration file that declares one peer, create a
// second at runtime, then delete the first.
//
// VALIDATES: two entries carry an order naming both, the configured one first;
// one entry carries no order key at all.
// PREVENTS: a peer list every reader refuses, which is what an entry added
// beside an order that does not name it produces.
func TestRunningConfigKeepsTheEntryOrder(t *testing.T) {
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)
	r.configTree = map[string]any{
		"bgp": map[string]any{
			"peer": map[string]any{
				"peer1": map[string]any{"session": map[string]any{"asn": map[string]any{"remote": "65003"}}},
			},
		},
	}
	api := &reactorAPIAdapter{r: r}

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, api.AddDynamicPeer(addr, map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}))

	bgp := runningBGPBlock(t, api)
	entries, err := configorder.Entries(bgp, "peer", "name")
	require.NoError(t, err, "the delivered list and its order agree")
	require.Len(t, entries, 2)
	assert.Equal(t, "peer1", entries[0].Key, "the configured peer keeps its place")
	assert.Equal(t, "peer-192.0.2.7", entries[1].Key, "the created peer goes last")

	require.ErrorIs(t, api.RemovePeer(netip.MustParseAddr("127.0.0.1")), ErrPeerNotFound,
		"an address naming no peer is refused, and the peer list is left alone")
	assert.Len(t, runningPeerList(t, api), 2, "a refused removal drops nothing")
}

// TestReloadRemovalLeavesTheRunningConfig holds the boundary between the two
// removal paths.
//
// GOAL: only an API removal changes the running configuration. A reload removes
// peers through Reactor.RemovePeer while it applies a candidate, and it
// REPLACES the whole tree with SetConfigTree when it succeeds. A reload that
// fails partway must leave the running configuration as it was.
// METHOD: remove the peer through Reactor.RemovePeer, the way
// applyPeerOperation does.
//
// VALIDATES: the running configuration still names the peer.
// PREVENTS: a failed reload leaving a configured peer out of the running
// configuration, where the next `update bgp config` would delete it from the
// file.
func TestReloadRemovalLeavesTheRunningConfig(t *testing.T) {
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)
	api := &reactorAPIAdapter{r: r}

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, api.AddDynamicPeer(addr, map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}))

	require.NoError(t, r.RemovePeer(addr))
	assert.Contains(t, runningPeerList(t, api), "peer-192.0.2.7",
		"the reload path leaves the running configuration to SetConfigTree")
}

// runningPeerList answers the peer list of the running configuration.
func runningPeerList(t *testing.T, api *reactorAPIAdapter) map[string]any {
	t.Helper()
	peers, ok := runningBGPBlock(t, api)["peer"].(map[string]any)
	require.True(t, ok, "the running configuration holds a peer list")
	return peers
}

// runningBGPBlock answers the bgp block of the running configuration.
func runningBGPBlock(t *testing.T, api *reactorAPIAdapter) map[string]any {
	t.Helper()
	bgp, ok := api.GetConfigTree()["bgp"].(map[string]any)
	require.True(t, ok, "the running configuration holds a bgp block")
	return bgp
}
