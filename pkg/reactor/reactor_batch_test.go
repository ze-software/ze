package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/pkg/plugin"
)

// TestBuildBatchAttributes verifies attribute conversion for RIB routes.
//
// VALIDATES: PathAttributes correctly converted to attribute.Attribute slice.
// PREVENTS: Attribute loss when queueing routes.
func TestBuildBatchAttributes(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	origin := uint8(0)
	med := uint32(100)
	localPref := uint32(200)

	attrs := plugin.PathAttributes{ //nolint:staticcheck // Testing deprecated fallback path
		Origin:          &origin,
		MED:             &med,
		LocalPreference: &localPref,
		Communities:     []uint32{65000<<16 | 100, 65000<<16 | 200},
		LargeCommunities: []plugin.LargeCommunity{
			{GlobalAdmin: 65000, LocalData1: 1, LocalData2: 2},
		},
		ExtendedCommunities: []attribute.ExtendedCommunity{
			{0x00, 0x02, 0x00, 0x00, 0x00, 0x64, 0x00, 0x65},
		},
	}

	result := adapter.buildBatchAttributes(attrs)

	// Should have 6 attributes: ORIGIN, MED, LOCAL_PREF, COMMUNITY, LARGE_COMMUNITY, EXTENDED_COMMUNITY
	require.Len(t, result, 6)

	// Check ORIGIN
	originAttr, ok := result[0].(attribute.Origin)
	require.True(t, ok, "first attribute should be Origin")
	assert.Equal(t, attribute.Origin(0), originAttr)

	// Check MED
	medAttr, ok := result[1].(attribute.MED)
	require.True(t, ok, "second attribute should be MED")
	assert.Equal(t, attribute.MED(100), medAttr)

	// Check LOCAL_PREF
	lpAttr, ok := result[2].(attribute.LocalPref)
	require.True(t, ok, "third attribute should be LocalPref")
	assert.Equal(t, attribute.LocalPref(200), lpAttr)

	// Check COMMUNITY
	comms, ok := result[3].(attribute.Communities)
	require.True(t, ok, "fourth attribute should be Communities")
	assert.Len(t, comms, 2)
}

// TestBuildBatchAttributes_Minimal verifies default attributes.
//
// VALIDATES: Empty PathAttributes produces default ORIGIN IGP.
// PREVENTS: Missing required attributes.
func TestBuildBatchAttributes_Minimal(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	attrs := plugin.PathAttributes{} //nolint:staticcheck // Testing deprecated fallback path
	result := adapter.buildBatchAttributes(attrs)

	// Should have just ORIGIN IGP
	require.Len(t, result, 1)
	originAttr, ok := result[0].(attribute.Origin)
	require.True(t, ok, "should be Origin")
	assert.Equal(t, attribute.OriginIGP, originAttr)
}

// TestBuildBatchASPath_eBGP verifies AS_PATH for eBGP peers.
//
// VALIDATES: LocalAS prepended for eBGP when no explicit AS_PATH.
// PREVENTS: Missing local AS in eBGP announcements.
func TestBuildBatchASPath_eBGP(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	// No explicit AS_PATH, eBGP peer
	asPath := adapter.buildBatchASPath(nil, false)

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
	asPath := adapter.buildBatchASPath(nil, true)

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
	asPath := adapter.buildBatchASPath(userPath, false)

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
		peers:  make(map[string]*Peer),
	}
	adapter := &reactorAPIAdapter{r: r}

	batch := plugin.NLRIBatch{
		Family:  nlri.IPv4Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)},
		NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
	}

	err := adapter.AnnounceNLRIBatch("192.168.1.1", batch)
	assert.ErrorIs(t, err, plugin.ErrNoPeersMatch)
}

// TestWithdrawNLRIBatch_NoMatchingPeers verifies error when no peers match.
//
// VALIDATES: ErrNoPeersMatch returned for invalid selector.
// PREVENTS: Silent failure on bad peer selector.
func TestWithdrawNLRIBatch_NoMatchingPeers(t *testing.T) {
	r := &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  make(map[string]*Peer),
	}
	adapter := &reactorAPIAdapter{r: r}

	batch := plugin.NLRIBatch{
		Family: nlri.IPv4Unicast,
		NLRIs:  []nlri.NLRI{nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)},
	}

	err := adapter.WithdrawNLRIBatch("192.168.1.1", batch)
	assert.ErrorIs(t, err, plugin.ErrNoPeersMatch)
}

// TestAnnounceNLRIBatch_FamilyNotNegotiated verifies warning when family not negotiated.
//
// VALIDATES: All peers skipped returns ErrNoPeersAcceptedFamily.
// PREVENTS: Silent failure when no peers support family.
func TestAnnounceNLRIBatch_FamilyNotNegotiated(t *testing.T) {
	settings := &PeerSettings{
		Address:  netip.MustParseAddr("10.0.0.2"),
		LocalAS:  65000,
		PeerAS:   65001,
		RouterID: 0x01020301,
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
		peers:  map[string]*Peer{settings.Address.String(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	// Try to announce IPv6 - all peers skipped
	batch := plugin.NLRIBatch{
		Family:  nlri.IPv6Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(nlri.IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 0)},
		NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}

	// Should return warning error when all peers skipped
	err := adapter.AnnounceNLRIBatch("*", batch)
	assert.ErrorIs(t, err, plugin.ErrNoPeersAcceptedFamily)
}

// TestWithdrawNLRIBatch_FamilyNotNegotiated verifies warning when family not negotiated.
//
// VALIDATES: All peers skipped returns ErrNoPeersAcceptedFamily for withdraw.
// PREVENTS: Silent failure when no peers support family.
func TestWithdrawNLRIBatch_FamilyNotNegotiated(t *testing.T) {
	settings := &PeerSettings{
		Address:  netip.MustParseAddr("10.0.0.2"),
		LocalAS:  65000,
		PeerAS:   65001,
		RouterID: 0x01020301,
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
		peers:  map[string]*Peer{settings.Address.String(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	// Try to withdraw IPv6 - all peers skipped
	batch := plugin.NLRIBatch{
		Family: nlri.IPv6Unicast,
		NLRIs:  []nlri.NLRI{nlri.NewINET(nlri.IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 0)},
	}

	// Should return warning error when all peers skipped
	err := adapter.WithdrawNLRIBatch("*", batch)
	assert.ErrorIs(t, err, plugin.ErrNoPeersAcceptedFamily)
}

// TestAnnounceNLRIBatch_QueueForNonEstablished verifies queueing behavior.
//
// VALIDATES: Non-established peers receive queued routes.
// PREVENTS: Routes lost when peer not yet connected.
func TestAnnounceNLRIBatch_QueueForNonEstablished(t *testing.T) {
	settings := &PeerSettings{
		Address:  netip.MustParseAddr("10.0.0.2"),
		LocalAS:  65000,
		PeerAS:   65001,
		RouterID: 0x01020301,
	}
	peer := NewPeer(settings)
	// NOT established - should queue
	peer.state.Store(int32(PeerStateActive))

	r := &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  map[string]*Peer{settings.Address.String(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	batch := plugin.NLRIBatch{
		Family: nlri.IPv4Unicast,
		NLRIs: []nlri.NLRI{
			nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0),
			nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.1.0/24"), 0),
		},
		NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
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
		Address:  netip.MustParseAddr("10.0.0.2"),
		LocalAS:  65000,
		PeerAS:   65001,
		RouterID: 0x01020301,
	}
	peer := NewPeer(settings)
	// NOT established - should queue
	peer.state.Store(int32(PeerStateActive))

	r := &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  map[string]*Peer{settings.Address.String(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	batch := plugin.NLRIBatch{
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
// VALIDATES: Wire attrs used when PathAttributes.Wire is set.
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

	batch := plugin.NLRIBatch{
		Family:  nlri.IPv4Unicast,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
		Attrs:   plugin.PathAttributes{Wire: attrsWire}, //nolint:staticcheck // Testing wire passthrough
	}

	// Use nil context (default ASN4=true, no ADD-PATH)
	update := adapter.buildBatchAnnounceUpdate(batch, netip.MustParseAddr("10.0.0.1"), false, nil)

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

	batch := plugin.NLRIBatch{
		Family:  nlri.IPv6Unicast,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
		Attrs:   plugin.PathAttributes{Wire: attrsWire}, //nolint:staticcheck // Testing wire passthrough
	}

	update := adapter.buildBatchAnnounceUpdate(batch, netip.MustParseAddr("2001:db8::1"), false, nil)

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

	batch := plugin.NLRIBatch{
		Family: nlri.IPv4Unicast,
		NLRIs:  []nlri.NLRI{wn},
	}

	update := adapter.buildBatchWithdrawUpdate(batch, nil)

	require.NotNil(t, update)
	// IPv4 unicast: withdrawals go in WithdrawnRoutes field
	assert.Equal(t, []byte{0x18, 0x0a, 0x00, 0x00}, update.WithdrawnRoutes)
	assert.Empty(t, update.PathAttributes)
	assert.Empty(t, update.NLRI)
}
