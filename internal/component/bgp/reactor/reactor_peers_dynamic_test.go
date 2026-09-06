package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
