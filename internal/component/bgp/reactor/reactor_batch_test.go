// rfc-test-change-approved: 2026-08-08 Thomas approved the buildBatchAnnounceUpdate
// signature change that carries the true cause to the caller (an (*message.Update,
// error) pair in place of a bare *message.Update, so a refused build reports WHY
// instead of a silent nil). Every hunk in this RFC4271-5.1.2-2 / RFC4271-5.1.2-3
// file is that caller adaptation, `update :=` becoming `update, _ :=`. No
// assertion, fixture, or expected value changed.
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

// TestBuildBatchASPath_eBGP verifies AS_PATH for eBGP peers.
//
// VALIDATES: LocalAS prepended for eBGP when no explicit AS_PATH.
// PREVENTS: Missing local AS in eBGP announcements.
func TestBuildBatchASPath_eBGP(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	// No explicit AS_PATH, eBGP peer
	asPath := adapter.buildBatchASPath(nil, 0, false, false, 65000)

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
	asPath := adapter.buildBatchASPath(nil, 0, true, false, 65000)

	require.NotNil(t, asPath)
	assert.Empty(t, asPath.Segments, "iBGP should have empty AS_PATH")
}

// TestBuildBatchASPath_Explicit verifies explicit AS_PATH is used.
//
// VALIDATES: RFC 4271 Section 5.1.2 -- a user-provided AS_PATH reaches an ordinary
// eBGP peer with the local AS prepended, not verbatim.
// PREVENTS: re-broadening the RFC 7947 Section 2.2.2 route-server exemption to
// every peer, which is what shipped an as-path with no local AS in it.
func TestBuildBatchASPath_Explicit(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	// Explicit AS_PATH
	userPath := []uint32{65001, 65002, 65003}
	asPath := adapter.buildBatchASPath(userPath, 0, false, false, 65000)

	require.NotNil(t, asPath)
	require.Len(t, asPath.Segments, 1)
	// Corrected: this used to assert userPath verbatim, which is an RFC 4271
	// Section 5.1.2 violation toward an ordinary eBGP peer -- our AS must lead
	// the path we advertise. RFC 7947 Section 2.2.2 transparency covers
	// RS-CLIENTS only (see TestBuildBatchASPath_ExplicitRSClientVerbatim).
	assert.Equal(t, []uint32{65000, 65001, 65002, 65003}, asPath.Segments[0].ASNs)
	assert.Equal(t, []uint32{65001, 65002, 65003}, userPath, "caller slice must not be mutated")
}

// TestBuildBatchASPath_OriginAS_iBGP verifies origin-as gives [originAS] to iBGP.
//
// VALIDATES: origin-as -> iBGP AS_PATH is exactly [originAS] (no prepend, RFC
// 4271 §5.1.2), so a route redistributed as a virtual router keeps its origin.
// PREVENTS: prepending localAS on iBGP for an originated route.
func TestBuildBatchASPath_OriginAS_iBGP(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	asPath := adapter.buildBatchASPath(nil, 112, true, false, 65000)

	require.NotNil(t, asPath)
	require.Len(t, asPath.Segments, 1)
	assert.Equal(t, []uint32{112}, asPath.Segments[0].ASNs)
}

// TestBuildBatchASPath_OriginAS_eBGP verifies origin-as prepends localAS on eBGP.
//
// VALIDATES: origin-as -> eBGP AS_PATH is [localAS, originAS] (normal export
// prepend), so the peer sees ze's AS first (enforce-first-as safe).
// PREVENTS: sending [originAS] verbatim to eBGP (rejected by enforce-first-as).
func TestBuildBatchASPath_OriginAS_eBGP(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	asPath := adapter.buildBatchASPath(nil, 112, false, false, 65000)

	require.NotNil(t, asPath)
	require.Len(t, asPath.Segments, 1)
	assert.Equal(t, []uint32{65000, 112}, asPath.Segments[0].ASNs)
}

// TestBuildBatchASPath_ExplicitBeatsOriginAS verifies an explicit as-path takes
// precedence over origin-as.
//
// VALIDATES: userASPath wins when both an explicit as-path and origin-as are set,
// and (RFC 4271 Section 5.1.2) still carries the local AS toward an eBGP peer.
// PREVENTS: origin-as replacing a deliberately-crafted AS_PATH.
func TestBuildBatchASPath_ExplicitBeatsOriginAS(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	userPath := []uint32{100, 200}
	asPath := adapter.buildBatchASPath(userPath, 112, false, false, 65000)

	require.NotNil(t, asPath)
	require.Len(t, asPath.Segments, 1)
	// Corrected alongside TestBuildBatchASPath_Explicit: the explicit path still
	// beats origin-as, but toward an ordinary eBGP peer it carries our AS first.
	assert.Equal(t, []uint32{65000, 100, 200}, asPath.Segments[0].ASNs)
}

// TestBuildBatchASPath_ExplicitRSClientVerbatim verifies the transparency
// exemption is scoped to the peers the RFC grants it to.
//
// VALIDATES: RFC 7947 Section 2.2.2 -- a route server does not prepend its own AS
// for an RS-CLIENT, so an explicit as-path reaches that peer untouched.
// PREVENTS: a future change prepending for RS-clients too. NOTE this test is a
// ratchet, not a gate for the fix that introduced it: the pre-fix code emitted
// every explicit path verbatim, so this case passed then as well. The two tests
// that actually gate the fix are TestBuildBatchASPath_Explicit and
// _ExplicitBeatsOriginAS (both flip red when the prepend is disabled).
func TestBuildBatchASPath_ExplicitRSClientVerbatim(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	userPath := []uint32{65001, 65002}
	asPath := adapter.buildBatchASPath(userPath, 0, false, true /*rsClient*/, 65000)

	require.NotNil(t, asPath)
	require.Len(t, asPath.Segments, 1)
	assert.Equal(t, []uint32{65001, 65002}, asPath.Segments[0].ASNs)
}

// TestBuildBatchASPath_ExplicitIBGPVerbatim pins the iBGP half of the same rule.
//
// VALIDATES: RFC 4271 Section 5.1.2 -- "When advertising a route to an internal
// peer, the speaker SHALL NOT modify the AS_PATH attribute".
// PREVENTS: prepending localAS toward an iBGP peer, which would corrupt the path
// for every downstream speaker in the AS.
func TestBuildBatchASPath_ExplicitIBGPVerbatim(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	userPath := []uint32{65001, 65002}
	asPath := adapter.buildBatchASPath(userPath, 0, true /*iBGP*/, false, 65000)

	require.NotNil(t, asPath)
	require.Len(t, asPath.Segments, 1)
	assert.Equal(t, []uint32{65001, 65002}, asPath.Segments[0].ASNs)
}

// TestBuildBatchASPath_ExplicitAlreadyLeadingNotDoubled verifies an operator who
// already spelled out the full path is not prepended a second time.
//
// VALIDATES: the local AS appears exactly once at the head when the supplied path
// already starts with it.
// PREVENTS: a double prepend, which inflates AS_PATH length and silently changes
// best-path selection at every receiver.
func TestBuildBatchASPath_ExplicitAlreadyLeadingNotDoubled(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	userPath := []uint32{65000, 65001}
	asPath := adapter.buildBatchASPath(userPath, 0, false, false, 65000)

	require.NotNil(t, asPath)
	require.Len(t, asPath.Segments, 1)
	assert.Equal(t, []uint32{65000, 65001}, asPath.Segments[0].ASNs)
}

// TestAnnounceNLRIBatch_NoMatchingPeers verifies error when no peers match.
//
// VALIDATES: ErrNoPeersMatch returned for invalid selector.
// PREVENTS: Silent failure on bad peer selector.
func TestAnnounceNLRIBatch_NoMatchingPeers(t *testing.T) {
	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		config:          &Config{LocalAS: 65000},
		peers:           make(map[netip.AddrPort]*Peer),
	}
	adapter := &reactorAPIAdapter{r: r}

	batch := bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
	}

	err := adapter.AnnounceNLRIBatch(selector.Addr(netip.MustParseAddr("192.168.1.1")), batch, plugin.OperatorSender())
	assert.ErrorIs(t, err, route.ErrNoPeersMatch)
}

// TestWithdrawNLRIBatch_NoMatchingPeers verifies error when no peers match.
//
// VALIDATES: ErrNoPeersMatch returned for invalid selector.
// PREVENTS: Silent failure on bad peer selector.
func TestWithdrawNLRIBatch_NoMatchingPeers(t *testing.T) {
	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		config:          &Config{LocalAS: 65000},
		peers:           make(map[netip.AddrPort]*Peer),
	}
	adapter := &reactorAPIAdapter{r: r}

	batch := bgptypes.NLRIBatch{
		Family: family.IPv4Unicast,
		NLRIs:  []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)},
	}

	err := adapter.WithdrawNLRIBatch(selector.Addr(netip.MustParseAddr("192.168.1.1")), batch, plugin.OperatorSender())
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
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})

	r := &Reactor{

		attrModHandlers: attrModHandlersWithDefaults(),
		config:          &Config{LocalAS: 65000},
		peers:           map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	// Try to announce IPv6 - all peers skipped
	batch := bgptypes.NLRIBatch{
		Family:  family.IPv6Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 0)},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}

	// Should return warning error when all peers skipped
	err := adapter.AnnounceNLRIBatch(selector.All(), batch, plugin.OperatorSender())
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
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})

	r := &Reactor{

		attrModHandlers: attrModHandlersWithDefaults(),
		config:          &Config{LocalAS: 65000},
		peers:           map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	// Try to withdraw IPv6 - all peers skipped
	batch := bgptypes.NLRIBatch{
		Family: family.IPv6Unicast,
		NLRIs:  []nlri.NLRI{nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 0)},
	}

	// Should return warning error when all peers skipped
	err := adapter.WithdrawNLRIBatch(selector.All(), batch, plugin.OperatorSender())
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

		attrModHandlers: attrModHandlersWithDefaults(),
		config:          &Config{LocalAS: 65000},
		peers:           map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	batch := bgptypes.NLRIBatch{
		Family: family.IPv4Unicast,
		NLRIs: []nlri.NLRI{
			nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0),
			nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.1.0/24"), 0),
		},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
	}

	err := adapter.AnnounceNLRIBatch(selector.All(), batch, plugin.OperatorSender())
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

		attrModHandlers: attrModHandlersWithDefaults(),
		config:          &Config{LocalAS: 65000},
		peers:           map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	batch := bgptypes.NLRIBatch{
		Family: family.IPv4Unicast,
		NLRIs: []nlri.NLRI{
			nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0),
			nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.1.0/24"), 0),
		},
	}

	err := adapter.WithdrawNLRIBatch(selector.All(), batch, plugin.OperatorSender())
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
	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)

	batch := bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
		Wire:    attrsWire,
	}

	// Use nil context (default ASN4=true, no ADD-PATH)
	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update, _ := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch, netip.MustParseAddr("10.0.0.1"), false, false, true, false, 65000)

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
	wn, err := nlri.NewWireNLRI(family.IPv6Unicast, []byte{0x20, 0x20, 0x01, 0x0d, 0xb8}, false)
	require.NoError(t, err)

	batch := bgptypes.NLRIBatch{
		Family:  family.IPv6Unicast,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
		Wire:    attrsWire,
	}

	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update, _ := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch, netip.MustParseAddr("2001:db8::1"), false, false, true, false, 65000)

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
	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)

	batch := bgptypes.NLRIBatch{
		Family: family.IPv4Unicast,
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

// announceWithExplicitASPath drives the ESTABLISHED announce path (the one that
// builds wire bytes directly, as opposed to the queued path that goes through
// buildBatchASPath) with an operator-supplied AS_PATH already present in the
// attributes, and returns the AS_PATH actually written to the wire.
func announceWithExplicitASPath(t *testing.T, userPath []uint32, isIBGP, rsClient bool) []uint32 {
	t.Helper()
	const localAS = uint32(65000)
	b := attribute.NewBuilder()
	b.SetOrigin(uint8(attribute.OriginIGP))
	b.SetASPath(userPath)

	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)
	batch := bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
		Attrs:   b,
	}

	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: localAS}}}
	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update, _ := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch,
		netip.MustParseAddr("10.0.0.1"), isIBGP, rsClient, true /*asn4*/, false, localAS)
	require.NotNil(t, update)

	_, value, ok := findPathAttr(update.PathAttributes, byte(attribute.AttrASPath))
	require.True(t, ok, "no AS_PATH on the wire")
	parsed, err := attribute.ParseASPath(value, true)
	require.NoError(t, err)
	require.Len(t, parsed.Segments, 1)
	return parsed.Segments[0].ASNs
}

// TestEstablishedAnnounce_ExplicitASPath_PrependsLocalAS is the regression guard
// for the half of this fix that the queued-path tests cannot reach.
//
// RFC requirement: RFC4271-5.1.2-3 positive -- an explicit as-path announced to an
// established ordinary eBGP peer leaves with the local AS prepended to the leading
// AS_SEQUENCE.
// VALIDATES: RFC 4271 Section 5.1.2 -- an explicit as-path announced to an
// established ordinary eBGP peer still leaves with the local AS at its head.
// PREVENTS: the established path diverging from the queued one. writeMandatoryAttrs
// used to copy the packed attributes verbatim whenever ORIGIN and AS_PATH were
// both present, so the SAME announce produced conformant wire when the peer was
// still queueing and non-conformant wire once it established -- a difference no
// operator could see or predict.
func TestEstablishedAnnounce_ExplicitASPath_PrependsLocalAS(t *testing.T) {
	got := announceWithExplicitASPath(t, []uint32{65001, 65002}, false /*eBGP*/, false /*not RS*/)
	assert.Equal(t, []uint32{65000, 65001, 65002}, got)
}

// TestEstablishedAnnounce_ExplicitASPath_RSClientVerbatim pins the RFC 7947
// Section 2.2.2 exemption on the established path.
func TestEstablishedAnnounce_ExplicitASPath_RSClientVerbatim(t *testing.T) {
	got := announceWithExplicitASPath(t, []uint32{65001, 65002}, false /*eBGP*/, true /*RS-client*/)
	assert.Equal(t, []uint32{65001, 65002}, got)
}

// TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim pins RFC 4271 Section 5.1.2
// "SHALL NOT modify the AS_PATH" toward an internal peer, on the established path.
// RFC requirement: RFC4271-5.1.2-3 negative -- the prepend is confined to EXTERNAL
// peers; an internal peer's AS_PATH is left byte-identical.
// RFC requirement: RFC4271-5.1.2-2 positive -- the internal-peer SHALL NOT.
func TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim(t *testing.T) {
	got := announceWithExplicitASPath(t, []uint32{65001, 65002}, true /*iBGP*/, false)
	assert.Equal(t, []uint32{65001, 65002}, got)
}

// TestEstablishedAnnounce_ExplicitASPath_AlreadyLeadingNotDoubled guards against
// the fix double-prepending a path the operator already spelled out in full.
func TestEstablishedAnnounce_ExplicitASPath_AlreadyLeadingNotDoubled(t *testing.T) {
	got := announceWithExplicitASPath(t, []uint32{65000, 65001}, false, false)
	assert.Equal(t, []uint32{65000, 65001}, got)
}
