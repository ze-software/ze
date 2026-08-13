package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/clock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDynamicGroup(name string, prefixes []string, maxPeers uint32) *DynamicGroupConfig {
	const localAS = 65000
	var ranges []netip.Prefix
	for _, p := range prefixes {
		ranges = append(ranges, netip.MustParsePrefix(p))
	}
	return &DynamicGroupConfig{
		GroupName: name,
		Ranges:    ranges,
		MaxPeers:  maxPeers,
		Settings: NewPeerSettings(
			netip.Addr{}, // Template has no address
			localAS,
			0, // PeerAS unknown for dynamic
			0x01020304,
		),
	}
}

func TestDynamicGroupContainsAddr(t *testing.T) {
	dg := newTestDynamicGroup("ix", []string{"185.1.69.0/24", "2001:7f8:4::/64"}, 100)

	assert.True(t, dg.containsAddr(netip.MustParseAddr("185.1.69.1")))
	assert.True(t, dg.containsAddr(netip.MustParseAddr("185.1.69.254")))
	assert.False(t, dg.containsAddr(netip.MustParseAddr("185.1.70.1")))
	assert.True(t, dg.containsAddr(netip.MustParseAddr("2001:7f8:4::1")))
	assert.False(t, dg.containsAddr(netip.MustParseAddr("2001:7f8:5::1")))
}

func TestFindDynamicGroup(t *testing.T) {
	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		peers:           make(map[netip.AddrPort]*Peer),
	}

	groupA := newTestDynamicGroup("group-a", []string{"185.1.69.0/24"}, 100)
	groupB := newTestDynamicGroup("group-b", []string{"185.1.69.0/25"}, 50) // More specific
	r.dynamicGroups = []*DynamicGroupConfig{groupA, groupB}

	t.Run("no_match", func(t *testing.T) {
		got := r.findDynamicGroup(netip.MustParseAddr("10.0.0.1"))
		assert.Nil(t, got)
	})

	t.Run("single_match", func(t *testing.T) {
		got := r.findDynamicGroup(netip.MustParseAddr("185.1.69.200"))
		require.NotNil(t, got)
		assert.Equal(t, "group-a", got.GroupName)
	})

	t.Run("longest_prefix_match", func(t *testing.T) {
		got := r.findDynamicGroup(netip.MustParseAddr("185.1.69.10"))
		require.NotNil(t, got)
		assert.Equal(t, "group-b", got.GroupName)
	})
}

func TestDynamicPeerCreation(t *testing.T) {
	r := newTestReactor(t)
	dg := newTestDynamicGroup("ix-peers", []string{"185.1.69.0/24"}, 100)
	dg.Settings.Connection = ConnectionPassive
	r.dynamicGroups = []*DynamicGroupConfig{dg}

	addr := netip.MustParseAddr("185.1.69.42")

	r.mu.Lock()
	peer, err := r.createDynamicPeer(dg, addr)
	r.mu.Unlock()

	require.NoError(t, err)
	require.NotNil(t, peer)
	assert.Equal(t, addr, peer.Settings().Address)
	assert.Equal(t, "dyn-185.1.69.42", peer.Settings().Name)
	assert.True(t, peer.Settings().IsDynamic)
	assert.Equal(t, uint32(0), peer.Settings().PeerAS)
	assert.Equal(t, "ix-peers", peer.Settings().GroupName)
	assert.Equal(t, ConnectionPassive, peer.Settings().Connection)
}

func TestDynamicPeerTTLCopiedFromGroupTemplate(t *testing.T) {
	r := newTestReactor(t)
	dg := newTestDynamicGroup("ix-peers", []string{"185.1.69.0/24"}, 100)
	dg.Template = map[string]any{
		"connection": map[string]any{
			"ttl": map[string]any{"max": "2"},
		},
	}
	r.dynamicGroups = []*DynamicGroupConfig{dg}

	r.mu.Lock()
	peer, err := r.createDynamicPeer(dg, netip.MustParseAddr("185.1.69.42"))
	r.mu.Unlock()

	require.NoError(t, err)
	require.NotNil(t, peer)
	assert.Equal(t, uint8(255), peer.Settings().OutTTL)
	assert.Equal(t, uint8(254), peer.Settings().MinTTL)
}

func TestDynamicPeerTTLConflictRejected(t *testing.T) {
	r := newTestReactor(t)
	dg := newTestDynamicGroup("ix-peers", []string{"185.1.69.0/24"}, 100)
	dg.Template = map[string]any{
		"connection": map[string]any{
			"ttl": map[string]any{"max": "1", "set": "255"},
		},
	}

	r.mu.Lock()
	peer, err := r.createDynamicPeer(dg, netip.MustParseAddr("185.1.69.42"))
	r.mu.Unlock()

	require.Error(t, err)
	assert.Nil(t, peer)
	assert.Contains(t, err.Error(), "ttl max cannot be combined with ttl set or ttl min")
}

func TestDynamicPeerMaxPeers(t *testing.T) {
	r := newTestReactor(t)
	dg := newTestDynamicGroup("ix-peers", []string{"185.1.69.0/24"}, 2)
	dg.Settings.Connection = ConnectionPassive
	r.dynamicGroups = []*DynamicGroupConfig{dg}

	r.mu.Lock()
	_, err := r.createDynamicPeer(dg, netip.MustParseAddr("185.1.69.1"))
	require.NoError(t, err)
	_, err = r.createDynamicPeer(dg, netip.MustParseAddr("185.1.69.2"))
	require.NoError(t, err)
	_, err = r.createDynamicPeer(dg, netip.MustParseAddr("185.1.69.3"))
	r.mu.Unlock()

	assert.ErrorIs(t, err, ErrDynamicMaxPeers)
}

func TestDynamicPeerNaming(t *testing.T) {
	r := newTestReactor(t)
	dg := newTestDynamicGroup("ix", []string{"185.1.69.0/24"}, 100)
	dg.Settings.Connection = ConnectionPassive
	r.dynamicGroups = []*DynamicGroupConfig{dg}

	r.mu.Lock()
	peer, err := r.createDynamicPeer(dg, netip.MustParseAddr("185.1.69.42"))
	r.mu.Unlock()

	require.NoError(t, err)
	assert.Equal(t, "dyn-185.1.69.42", peer.Settings().Name)
}

func TestExplicitPeerPrecedence(t *testing.T) {
	r := newTestReactor(t)
	dg := newTestDynamicGroup("ix", []string{"185.1.69.0/24"}, 100)
	r.dynamicGroups = []*DynamicGroupConfig{dg}

	// Add explicit peer at an address within the dynamic range.
	explicitAddr := netip.MustParseAddr("185.1.69.42")
	explicitPS := NewPeerSettings(explicitAddr, 65000, 64512, 0x01020304)
	explicitPS.Connection = ConnectionPassive
	require.NoError(t, r.AddPeer(explicitPS))

	// tryCreateDynamicPeer returns the existing explicit peer (race protection re-check).
	peer := r.tryCreateDynamicPeer(explicitAddr)
	require.NotNil(t, peer)
	assert.False(t, peer.Settings().IsDynamic, "explicit peer must not be replaced by dynamic")
}

func TestSetDynamicGroups(t *testing.T) {
	r := newTestReactor(t)
	dg := newTestDynamicGroup("ix", []string{"185.1.69.0/24"}, 100)
	dg.Settings.Connection = ConnectionPassive
	r.dynamicGroups = []*DynamicGroupConfig{dg}

	// Create a dynamic peer.
	r.mu.Lock()
	_, err := r.createDynamicPeer(dg, netip.MustParseAddr("185.1.69.1"))
	r.mu.Unlock()
	require.NoError(t, err)

	// Remove the group via SetDynamicGroups.
	r.SetDynamicGroups(nil)

	// Dynamic peer should be gone.
	r.mu.RLock()
	_, exists := r.findPeerByAddr(netip.MustParseAddr("185.1.69.1"))
	r.mu.RUnlock()
	assert.False(t, exists)
}

// newTestReactor creates a minimal Reactor for unit testing.
func newTestReactor(t *testing.T) *Reactor {
	t.Helper()
	return &Reactor{
		peers:     make(map[netip.AddrPort]*Peer),
		listeners: make(map[string]*Listener),
		clock:     clock.RealClock{},
	}
}

// TestDynamicPeerOwnsPrefixMaps verifies each dynamic peer gets its own copy of
// the template's prefix maps.
//
// VALIDATES: AC-8 and R-2. Dynamic peers are built one per accepted connection
// from a single template. Aliasing the maps would let a write on one peer
// change prefix enforcement for every sibling built from the same group.
// PREVENTS: One peer's warn-only choice spreading across an IXP's worth of
// dynamic peers.
func TestDynamicPeerOwnsPrefixMaps(t *testing.T) {
	r := newTestReactor(t)
	dg := newTestDynamicGroup("ix-peers", []string{"185.1.69.0/24"}, 100)
	dg.Settings.PrefixMaximum = map[string]uint32{"ipv4/unicast": 1000}
	dg.Settings.PrefixWarning = map[string]uint32{"ipv4/unicast": 900}
	dg.Settings.PrefixTeardown = map[string]bool{"ipv4/unicast": true}
	dg.Settings.PrefixIdleTimeout = map[string]uint16{"ipv4/unicast": 30}
	dg.Settings.PrefixUpdated = map[string]string{"ipv4/unicast": "2026-07-30"}
	r.dynamicGroups = []*DynamicGroupConfig{dg}

	r.mu.Lock()
	first, err1 := r.createDynamicPeer(dg, netip.MustParseAddr("185.1.69.1"))
	second, err2 := r.createDynamicPeer(dg, netip.MustParseAddr("185.1.69.2"))
	r.mu.Unlock()

	require.NoError(t, err1)
	require.NoError(t, err2)

	// Every per-family prefix value reaches the dynamic peer.
	assert.Equal(t, uint32(1000), first.Settings().PrefixMaximum["ipv4/unicast"])
	assert.True(t, first.Settings().prefixTeardownFor("ipv4/unicast"))
	assert.Equal(t, uint16(30), first.Settings().prefixIdleTimeoutFor("ipv4/unicast"))
	assert.Equal(t, "2026-07-30", first.Settings().OldestPrefixUpdated())

	// Mutating one peer changes neither its sibling nor the template.
	first.Settings().PrefixTeardown["ipv4/unicast"] = false
	first.Settings().PrefixMaximum["ipv4/unicast"] = 1
	first.Settings().PrefixIdleTimeout["ipv4/unicast"] = 1
	first.Settings().PrefixUpdated["ipv4/unicast"] = "2000-01-01"

	assert.True(t, second.Settings().prefixTeardownFor("ipv4/unicast"),
		"the sibling keeps its own enforcement setting")
	assert.Equal(t, uint32(1000), second.Settings().PrefixMaximum["ipv4/unicast"])
	assert.Equal(t, uint16(30), second.Settings().prefixIdleTimeoutFor("ipv4/unicast"))
	assert.Equal(t, "2026-07-30", second.Settings().OldestPrefixUpdated())
	assert.True(t, dg.Settings.prefixTeardownFor("ipv4/unicast"),
		"the template is not mutated either")
}
