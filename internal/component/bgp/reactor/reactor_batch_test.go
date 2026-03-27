package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/route"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// TestBuildBatchASPath_eBGP verifies AS_PATH for eBGP peers.
//
// VALIDATES: LocalAS prepended for eBGP when no explicit AS_PATH.
// PREVENTS: Missing local AS in eBGP announcements.
func TestBuildBatchASPath_eBGP(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	// No explicit AS_PATH, eBGP peer
	asPath := adapter.buildBatchASPath(nil, false, 65000)

	require.NotNil(t, asPath)
	require.Len(t, asPath.Segments, 1)
	assert.Equal(t, attribute.ASSequence, asPath.Segments[0].Type)
	assert.Equal(t, []uint32{65000}, asPath.Segments[0].ASNs)
}

// TestBuildBatchASPath_iBGP verifies empty AS_PATH for iBGP peers.
//
// VALIDATES: Empty AS_PATH for iBGP (no modification per RFC 4271 §5.1.2).
// PREVENTS: Incorrect AS_PATH modification for iBGP.
func TestBuildBatchASPath_iBGP(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	// No explicit AS_PATH, iBGP peer
	asPath := adapter.buildBatchASPath(nil, true, 65000)

	require.NotNil(t, asPath)
	assert.Empty(t, asPath.Segments, "iBGP should have empty AS_PATH")
}

// TestBuildBatchASPath_Explicit verifies explicit AS_PATH is used.
//
// VALIDATES: User-provided AS_PATH passed through.
// PREVENTS: User AS_PATH being overwritten.
func TestBuildBatchASPath_Explicit(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	// Explicit AS_PATH
	userPath := []uint32{65001, 65002, 65003}
	asPath := adapter.buildBatchASPath(userPath, false, 65000)

	require.NotNil(t, asPath)
	require.Len(t, asPath.Segments, 1)
	assert.Equal(t, userPath, asPath.Segments[0].ASNs)
}

// TestAnnounceNLRIBatch_NoMatchingPeers verifies error when no peers match.
//
// VALIDATES: ErrNoPeersMatch returned for invalid selector.
// PREVENTS: Silent failure on bad peer selector.
func TestAnnounceNLRIBatch_NoMatchingPeers(t *testing.T) {
	r := &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  make(map[netip.AddrPort]*Peer),
	}
	adapter := &reactorAPIAdapter{r: r}

	batch := bgptypes.NLRIBatch{
		Family:  nlri.IPv4Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
	}

	err := adapter.AnnounceNLRIBatch("192.168.1.1", batch)
	assert.ErrorIs(t, err, route.ErrNoPeersMatch)
}

// TestWithdrawNLRIBatch_NoMatchingPeers verifies error when no peers match.
//
// VALIDATES: ErrNoPeersMatch returned for invalid selector.
// PREVENTS: Silent failure on bad peer selector.
func TestWithdrawNLRIBatch_NoMatchingPeers(t *testing.T) {
	r := &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  make(map[netip.AddrPort]*Peer),
	}
	adapter := &reactorAPIAdapter{r: r}

	batch := bgptypes.NLRIBatch{
		Family: nlri.IPv4Unicast,
		NLRIs:  []nlri.NLRI{nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)},
	}

	err := adapter.WithdrawNLRIBatch("192.168.1.1", batch)
	assert.ErrorIs(t, err, route.ErrNoPeersMatch)
}

// TestAnnounceNLRIBatch_FamilyNotNegotiated verifies warning when family not negotiated.
//
// VALIDATES: All peers skipped returns ErrNoPeersAcceptedFamily.
// PREVENTS: Silent failure when no peers support family.
func TestAnnounceNLRIBatch_FamilyNotNegotiated(t *testing.T) {
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))

	// Negotiate ONLY IPv4 unicast, NOT IPv6
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[nlri.Family]bool{nlri.IPv4Unicast: true},
		ExtendedMessage: false,
	})

	r := &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	// Try to announce IPv6 - all peers skipped
	batch := bgptypes.NLRIBatch{
		Family:  nlri.IPv6Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(nlri.IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 0)},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}

	// Should return warning error when all peers skipped
	err := adapter.AnnounceNLRIBatch("*", batch)
	assert.ErrorIs(t, err, route.ErrNoPeersAcceptedFamily)
}

// TestWithdrawNLRIBatch_FamilyNotNegotiated verifies warning when family not negotiated.
//
// VALIDATES: All peers skipped returns ErrNoPeersAcceptedFamily for withdraw.
// PREVENTS: Silent failure when no peers support family.
func TestWithdrawNLRIBatch_FamilyNotNegotiated(t *testing.T) {
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))

	// Negotiate ONLY IPv4 unicast, NOT IPv6
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[nlri.Family]bool{nlri.IPv4Unicast: true},
		ExtendedMessage: false,
	})

	r := &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	// Try to withdraw IPv6 - all peers skipped
	batch := bgptypes.NLRIBatch{
		Family: nlri.IPv6Unicast,
		NLRIs:  []nlri.NLRI{nlri.NewINET(nlri.IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 0)},
	}

	// Should return warning error when all peers skipped
	err := adapter.WithdrawNLRIBatch("*", batch)
	assert.ErrorIs(t, err, route.ErrNoPeersAcceptedFamily)
}

// TestAnnounceNLRIBatch_QueueForNonEstablished verifies queueing behavior.
//
// VALIDATES: Non-established peers receive queued routes.
// PREVENTS: Routes lost when peer not yet connected.
func TestAnnounceNLRIBatch_QueueForNonEstablished(t *testing.T) {
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	// NOT established - should queue
	peer.state.Store(int32(PeerStateActive))

	r := &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	batch := bgptypes.NLRIBatch{
		Family: nlri.IPv4Unicast,
		NLRIs: []nlri.NLRI{
			nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0),
			nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.1.0/24"), 0),
		},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
	}

	err := adapter.AnnounceNLRIBatch("*", batch)
	require.NoError(t, err)

	// Check queue has 2 routes (one per NLRI)
	peer.mu.Lock()
	queueLen := len(peer.opQueue)
	peer.mu.Unlock()

	assert.Equal(t, 2, queueLen, "should queue 2 routes for non-established peer")
}

// TestWithdrawNLRIBatch_QueueForNonEstablished verifies withdrawal queueing.
//
// VALIDATES: Non-established peers receive queued withdrawals.
// PREVENTS: Withdrawals lost when peer not yet connected.
func TestWithdrawNLRIBatch_QueueForNonEstablished(t *testing.T) {
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	// NOT established - should queue
	peer.state.Store(int32(PeerStateActive))

	r := &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	batch := bgptypes.NLRIBatch{
		Family: nlri.IPv4Unicast,
		NLRIs: []nlri.NLRI{
			nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0),
			nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.1.0/24"), 0),
		},
	}

	err := adapter.WithdrawNLRIBatch("*", batch)
	require.NoError(t, err)

	// Check queue has 2 withdrawals
	peer.mu.Lock()
	queueLen := len(peer.opQueue)
	peer.mu.Unlock()

	assert.Equal(t, 2, queueLen, "should queue 2 withdrawals for non-established peer")
}

// =============================================================================
// Phase 5: Wire mode tests
// =============================================================================

// TestBuildBatchAnnounceUpdate_WireMode_IPv4 verifies wire mode for IPv4 unicast.
//
// VALIDATES: Wire attrs used when batch.Wire is set.
// PREVENTS: Wire bytes being ignored or re-encoded.
func TestBuildBatchAnnounceUpdate_WireMode_IPv4(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	// Wire attributes: ORIGIN IGP (0x40 0x01 0x01 0x00) + AS_PATH empty (0x40 0x02 0x00)
	wireAttrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00}
	attrsWire := attribute.NewAttributesWire(wireAttrs, 0)

	// Create wire NLRI (10.0.0.0/24)
	wn, err := nlri.NewWireNLRI(nlri.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)

	batch := bgptypes.NLRIBatch{
		Family:  nlri.IPv4Unicast,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
		Wire:    attrsWire,
	}

	// Use nil context (default ASN4=true, no ADD-PATH)
	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch, netip.MustParseAddr("10.0.0.1"), false, true, false, 65000)

	require.NotNil(t, update)

	// Wire mode: PathAttributes should contain wire bytes + NEXT_HOP
	// The wire bytes should be preserved (wire attrs come first)
	assert.True(t, bytes.HasPrefix(update.PathAttributes, wireAttrs), "wire attrs should be preserved at start")
	assert.Len(t, update.NLRI, 4, "IPv4 unicast NLRI should be in NLRI field")
}

// TestBuildBatchAnnounceUpdate_WireMode_IPv6 verifies wire mode for IPv6 unicast.
//
// VALIDATES: Wire mode uses MP_REACH_NLRI for non-IPv4 families.
// PREVENTS: Wrong attribute construction for MP families.
func TestBuildBatchAnnounceUpdate_WireMode_IPv6(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	// Wire attributes: ORIGIN IGP
	wireAttrs := []byte{0x40, 0x01, 0x01, 0x00}
	attrsWire := attribute.NewAttributesWire(wireAttrs, 0)

	// Create wire NLRI for IPv6 (2001:db8::/32)
	wn, err := nlri.NewWireNLRI(nlri.IPv6Unicast, []byte{0x20, 0x20, 0x01, 0x0d, 0xb8}, false)
	require.NoError(t, err)

	batch := bgptypes.NLRIBatch{
		Family:  nlri.IPv6Unicast,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
		Wire:    attrsWire,
	}

	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch, netip.MustParseAddr("2001:db8::1"), false, true, false, 65000)

	require.NotNil(t, update)

	// IPv6: NLRI field should be empty (NLRIs go in MP_REACH_NLRI)
	assert.Empty(t, update.NLRI, "IPv6 unicast should use MP_REACH_NLRI, not NLRI field")
	// PathAttributes should contain wire attrs + MP_REACH_NLRI
	assert.NotEmpty(t, update.PathAttributes)
}

// TestBuildBatchWithdrawUpdate_WireMode verifies wire mode for withdrawals.
//
// VALIDATES: Wire NLRIs correctly packed for withdrawal.
// PREVENTS: Withdrawal parsing failures.
func TestBuildBatchWithdrawUpdate_WireMode(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	// Create wire NLRI (10.0.0.0/24)
	wn, err := nlri.NewWireNLRI(nlri.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)

	batch := bgptypes.NLRIBatch{
		Family: nlri.IPv4Unicast,
		NLRIs:  []nlri.NLRI{wn},
	}

	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update := adapter.buildBatchWithdrawUpdate(attrBuf, nlriBuf, batch, false)

	require.NotNil(t, update)
	// IPv4 unicast: withdrawals go in WithdrawnRoutes field
	assert.Equal(t, []byte{0x18, 0x0a, 0x00, 0x00}, update.WithdrawnRoutes)
	assert.Empty(t, update.PathAttributes)
	assert.Empty(t, update.NLRI)
}
