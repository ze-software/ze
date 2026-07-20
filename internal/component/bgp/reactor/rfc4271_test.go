package reactor

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/filterapi"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/core/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// rfc4271Announce encodes an announce UPDATE for 10.0.0.0/24 toward an internal or
// external peer and returns the path-attribute section.
func rfc4271Announce(t *testing.T, isIBGP bool) []byte {
	t.Helper()
	buf := make([]byte, 4096)
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
	}
	n := WriteAnnounceUpdate(buf, 0, route, 65001, isIBGP, true, false)
	require.Greater(t, n, message.HeaderLen+4)

	// Header(19) + WithdrawnLen(2) + AttrLen(2) then the attribute section.
	attrLen := int(buf[message.HeaderLen+2])<<8 | int(buf[message.HeaderLen+3])
	start := message.HeaderLen + 4
	require.LessOrEqual(t, start+attrLen, n)
	return buf[start : start+attrLen]
}

// rfc4271FindAttr returns the value bytes of the first attribute with the given type code,
// and whether it was present.
func rfc4271FindAttr(attrs []byte, code attribute.AttributeCode) ([]byte, bool) {
	pos := 0
	for pos+3 <= len(attrs) {
		flags := attrs[pos]
		typeCode := attribute.AttributeCode(attrs[pos+1])
		var length, hdr int
		if flags&0x10 != 0 {
			if pos+4 > len(attrs) {
				return nil, false
			}
			length = int(attrs[pos+2])<<8 | int(attrs[pos+3])
			hdr = 4
		} else {
			length = int(attrs[pos+2])
			hdr = 3
		}
		if pos+hdr+length > len(attrs) {
			return nil, false
		}
		if typeCode == code {
			return attrs[pos+hdr : pos+hdr+length], true
		}
		pos += hdr + length
	}
	return nil, false
}

// TestRFC4271LocalPrefIncludedForInternalPeers verifies LOCAL_PREF is present in an UPDATE
// built for an internal peer and absent for an external one.
//
// VALIDATES: The iBGP announce carries LOCAL_PREF (default degree of preference 100); the
// eBGP announce carries none.
//
// PREVENTS: Leaking local preference across an AS boundary, or omitting it inside the AS.
//
// RFC requirement: RFC4271-5.1.5-1 positive -- WriteAnnounceUpdate writes LOCAL_PREF into
// every announce built for an internal peer (internal/component/bgp/reactor/reactor_wire.go:347-354).
// RFC requirement: RFC4271-5.1.5-2 negative -- the same gate is what keeps LOCAL_PREF out
// of an external peer's UPDATE, so the attribute's presence is conditional on the session
// being internal rather than unconditional (reactor_wire.go:347).
// RFC requirement: RFC4271-9.1.1-2 positive -- the value carried to the internal peer is
// the computed degree of preference for the route (the stored LOCAL_PREF, defaulting to
// 100) (internal/component/bgp/reactor/reactor_wire.go:348-352).
func TestRFC4271LocalPrefIncludedForInternalPeers(t *testing.T) {
	attrs := rfc4271Announce(t, true)
	value, ok := rfc4271FindAttr(attrs, attribute.AttrLocalPref)
	require.True(t, ok, "iBGP announce must carry LOCAL_PREF")
	require.Len(t, value, 4)
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x64}, value, "degree of preference 100")
}

// TestRFC4271LocalPrefOmittedForExternalPeers verifies no LOCAL_PREF reaches an external
// peer.
//
// VALIDATES: The eBGP announce has no LOCAL_PREF attribute at all.
//
// PREVENTS: Advertising an internal preference value to another AS.
//
// RFC requirement: RFC4271-5.1.5-2 positive -- an announce built for an external peer
// contains no LOCAL_PREF, because the write is gated on isIBGP
// (internal/component/bgp/reactor/reactor_wire.go:347-354).
// RFC requirement: RFC4271-5.1.5-1 negative -- the SHALL-include obligation is not applied
// blindly: the same producer omits the attribute when the peer is external, so its
// presence for internal peers is a real decision (reactor_wire.go:347).
// RFC requirement: RFC4271-9.1.1-2 negative -- the computed degree of preference is not
// emitted as a LOCAL_PREF toward an external peer (reactor_wire.go:347-354).
func TestRFC4271LocalPrefOmittedForExternalPeers(t *testing.T) {
	attrs := rfc4271Announce(t, false)
	_, ok := rfc4271FindAttr(attrs, attribute.AttrLocalPref)
	assert.False(t, ok, "eBGP announce must not carry LOCAL_PREF")

	// The mandatory attributes are still there, so the absence is specific.
	_, hasOrigin := rfc4271FindAttr(attrs, attribute.AttrOrigin)
	_, hasNextHop := rfc4271FindAttr(attrs, attribute.AttrNextHop)
	assert.True(t, hasOrigin, "ORIGIN still present")
	assert.True(t, hasNextHop, "NEXT_HOP still present")
}

// TestRFC4271ASPathUnmodifiedTowardInternalPeer verifies the local AS is not prepended when
// advertising to an internal peer.
//
// VALIDATES: The iBGP announce carries an empty AS_PATH (no local AS added).
//
// PREVENTS: Breaking iBGP loop semantics and path length by prepending inside the AS.
//
// RFC requirement: RFC4271-5.1.2-2 positive -- WriteAnnounceUpdate leaves the AS_PATH
// unmodified for an internal peer: the eBGP prepend branch is not taken and the AS number
// list stays empty (internal/component/bgp/reactor/reactor_wire.go:322-332).
func TestRFC4271ASPathUnmodifiedTowardInternalPeer(t *testing.T) {
	attrs := rfc4271Announce(t, true)
	value, ok := rfc4271FindAttr(attrs, attribute.AttrASPath)
	require.True(t, ok, "AS_PATH is well-known mandatory and must be present")
	assert.Empty(t, value, "AS_PATH toward an internal peer is not modified (stays empty)")
}

// TestRFC4271ASPathPrependedTowardExternalPeer verifies the no-modify rule for internal
// peers is a real branch, not an implementation that never prepends.
//
// VALIDATES: The eBGP announce carries an AS_SEQUENCE holding the local AS 65001.
//
// PREVENTS: Reading the empty iBGP AS_PATH as "AS_PATH is never written".
//
// RFC requirement: RFC4271-5.1.2-2 negative -- the same producer does modify the AS_PATH
// when the peer is external, prepending the local AS, so the internal-peer no-modify rule
// is a deliberate branch (internal/component/bgp/reactor/reactor_wire.go:326-332).
func TestRFC4271ASPathPrependedTowardExternalPeer(t *testing.T) {
	attrs := rfc4271Announce(t, false)
	value, ok := rfc4271FindAttr(attrs, attribute.AttrASPath)
	require.True(t, ok)
	require.NotEmpty(t, value, "eBGP AS_PATH must carry the local AS")
	// Segment: type(1) length(1) then 4-octet ASNs (asn4 negotiated).
	require.GreaterOrEqual(t, len(value), 6)
	assert.Equal(t, byte(2), value[0], "AS_SEQUENCE")
	assert.Equal(t, byte(1), value[1], "one AS number")
	assert.Equal(t, []byte{0x00, 0x00, 0xFD, 0xE9}, value[2:6], "local AS 65001")
}

// TestRFC4271NegotiatedHoldTimeDrivesTimers verifies the negotiated hold time, not the
// locally configured one, is what the session's timers run on.
//
// VALIDATES: With a local 90s and a peer 30s proposal the negotiated value is 30s and the
// hold timer is set to 30s.
//
// PREVENTS: Running the hold timer on the local proposal and tearing down a healthy peer.
//
// RFC requirement: RFC4271-6.2-2 positive -- negotiateWith stores the negotiated hold time
// and installs exactly that value into the session timers
// (internal/component/bgp/reactor/session_negotiate.go:47-62).
func TestRFC4271NegotiatedHoldTimeDrivesTimers(t *testing.T) {
	s := newNegotiateSession(90*time.Second, 30*time.Second)
	s.negotiateWith(nil, nil)

	neg := s.Negotiated()
	require.NotNil(t, neg)
	assert.Equal(t, uint16(30), neg.HoldTime)
	assert.Equal(t, 30*time.Second, s.timers.HoldTime(), "timers run on the negotiated value")
}

// TestRFC4271LocalHoldTimeNotUsedWhenPeerProposesSmaller verifies the configured value is
// not what reaches the timers when the peer proposes less.
//
// VALIDATES: A local 90s configuration never appears in the negotiated value or the timer
// when the peer proposed 30s; and a sub-3s negotiation is floored at 3s rather than used.
//
// PREVENTS: Silently ignoring the peer's proposal, or accepting an illegal 1s/2s value.
//
// RFC requirement: RFC4271-6.2-2 negative -- the locally configured hold time is discarded
// when it is not the smaller of the two, and an out-of-range result is floored rather than
// installed verbatim (internal/component/bgp/reactor/session_negotiate.go:50-62).
func TestRFC4271LocalHoldTimeNotUsedWhenPeerProposesSmaller(t *testing.T) {
	s := newNegotiateSession(90*time.Second, 30*time.Second)
	s.negotiateWith(nil, nil)
	assert.NotEqual(t, 90*time.Second, s.timers.HoldTime(), "local proposal must not win")

	floored := newNegotiateSession(1*time.Second, 2*time.Second)
	floored.negotiateWith(nil, nil)
	assert.Equal(t, 3*time.Second, floored.timers.HoldTime(),
		"an illegal sub-3s negotiation is floored, never installed as-is")
}

// TestRFC4271SeparateFSMPerPeer verifies each configured peer gets its own state machine.
//
// VALIDATES: Two sessions built from two peer settings hold distinct FSMs whose states
// move independently.
//
// PREVENTS: One peer's transition dragging another peer's session with it.
//
// RFC requirement: RFC4271-8.2.1-1 positive -- NewSession constructs a fresh fsm.FSM and
// a fresh timer set for each peer, so per-peer state is not shared
// (internal/component/bgp/reactor/session.go:377-413).
func TestRFC4271SeparateFSMPerPeer(t *testing.T) {
	a := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301))
	b := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.2"), 65001, 65003, 0x01020301))

	require.NotSame(t, a.fsm, b.fsm, "each peer has its own FSM instance")
	require.Equal(t, fsm.StateIdle, a.State())
	require.Equal(t, fsm.StateIdle, b.State())

	require.NoError(t, a.fsm.Event(fsm.EventManualStart))
	assert.NotEqual(t, fsm.StateIdle, a.State(), "peer A moved out of Idle")
	assert.Equal(t, fsm.StateIdle, b.State(), "peer B is unaffected by peer A's transition")
}

// TestRFC4271PerPeerFSMDoesNotShareTimers verifies the per-peer separation covers the
// timers the FSM drives, not just the state variable.
//
// VALIDATES: Setting a different hold time on one peer leaves the other peer's hold time
// unchanged.
//
// PREVENTS: A single shared timer set collapsing two peers into one hold-timer schedule.
//
// RFC requirement: RFC4271-8.2.1-1 negative -- the FSMs are not backed by shared state:
// each session owns its own Timers, so one peer's hold time cannot be read as another's
// (internal/component/bgp/reactor/session.go:396-417).
func TestRFC4271PerPeerFSMDoesNotShareTimers(t *testing.T) {
	sa := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	sa.ReceiveHoldTime = 30 * time.Second
	sb := NewPeerSettings(netip.MustParseAddr("192.0.2.2"), 65001, 65003, 0x01020301)
	sb.ReceiveHoldTime = 180 * time.Second

	a := NewSession(sa)
	b := NewSession(sb)

	require.NotSame(t, a.timers, b.timers)
	assert.Equal(t, 30*time.Second, a.timers.HoldTime())
	assert.Equal(t, 180*time.Second, b.timers.HoldTime())
}

// TestRFC4271HoldTimeConfigurablePerPeer verifies the hold timer is a per-peer setting.
//
// VALIDATES: Two peers configured with different hold times keep them; the default is the
// RFC's suggested 90s.
//
// PREVENTS: A single global hold time applied to every neighbor.
//
// RFC requirement: RFC4271-10-1 positive -- ReceiveHoldTime lives on PeerSettings and is
// installed into that peer's own timers, so each peer carries its own HoldTimer value
// (internal/component/bgp/reactor/peersettings.go:251, session.go:416-417).
func TestRFC4271HoldTimeConfigurablePerPeer(t *testing.T) {
	def := NewPeerSettings(netip.MustParseAddr("192.0.2.9"), 65001, 65002, 0x01020301)
	assert.Equal(t, DefaultReceiveHoldTime, def.ReceiveHoldTime, "RFC 4271 suggested default")

	sa := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	sa.ReceiveHoldTime = 3 * time.Second
	sb := NewPeerSettings(netip.MustParseAddr("192.0.2.2"), 65001, 65003, 0x01020301)
	sb.ReceiveHoldTime = 240 * time.Second

	assert.Equal(t, 3*time.Second, NewSession(sa).timers.HoldTime())
	assert.Equal(t, 240*time.Second, NewSession(sb).timers.HoldTime())
}

// TestRFC4271PerPeerHoldTimeSurvivesNegotiation verifies a per-peer hold time is the value
// actually offered and negotiated, not a global constant.
//
// VALIDATES: Two peers with different configured hold times negotiate different values
// against the same peer proposal.
//
// PREVENTS: A per-peer configuration that is accepted but then ignored at negotiation.
//
// RFC requirement: RFC4271-10-1 negative -- the per-peer value is not decorative: a peer
// configured with a smaller hold time negotiates the smaller value while another peer with
// a larger configuration does not (internal/component/bgp/reactor/session_negotiate.go:47-54).
func TestRFC4271PerPeerHoldTimeSurvivesNegotiation(t *testing.T) {
	short := newNegotiateSession(9*time.Second, 90*time.Second)
	short.negotiateWith(nil, nil)
	long := newNegotiateSession(180*time.Second, 90*time.Second)
	long.negotiateWith(nil, nil)

	require.NotNil(t, short.Negotiated())
	require.NotNil(t, long.Negotiated())
	assert.Equal(t, uint16(9), short.Negotiated().HoldTime)
	assert.Equal(t, uint16(90), long.Negotiated().HoldTime)
}

// TestRFC4271DefaultBGPPortIs179 verifies the default TCP port used to connect and listen.
//
// VALIDATES: DefaultBGPPort is 179 and a peer with no explicit port keys on 179.
//
// PREVENTS: Peering on a non-standard port by default.
//
// RFC requirement: RFC4271-8.2.1-2 positive -- DefaultBGPPort is 179 and is what a peer
// with no configured port uses for its connect/listen key
// (internal/component/bgp/reactor/peersettings.go:33).
func TestRFC4271DefaultBGPPortIs179(t *testing.T) {
	assert.Equal(t, 179, DefaultBGPPort)
	ps := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	assert.Equal(t, uint16(DefaultBGPPort), ps.PeerKey().Port(), "default peer key uses port 179")
}

// TestRFC4271ExplicitPortOverridesDefault verifies the 179 default is a real lookup rather
// than a hardcoded constant everywhere.
//
// VALIDATES: A peer configured on port 1179 keys on 1179, not 179.
//
// PREVENTS: Reading "always 179" as proof that the port is honored at all.
//
// RFC requirement: RFC4271-8.2.1-2 negative -- the port is read from the peer's settings,
// so an explicitly configured non-179 port is used; 179 is the default value of that
// setting, not an unconditional constant
// (internal/component/bgp/reactor/peersettings.go:31-33,232).
func TestRFC4271ExplicitPortOverridesDefault(t *testing.T) {
	ps := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	ps.Port = 1179
	assert.Equal(t, uint16(1179), ps.PeerKey().Port())
	assert.NotEqual(t, uint16(DefaultBGPPort), ps.PeerKey().Port())
}

// TestRFC4271ThirdPartyNextHopCanBeDisabled verifies the operator can force ze's own
// address as the advertised NEXT_HOP, suppressing any third-party next hop.
//
// VALIDATES: NextHopSelf with a valid local address precomputes a self next-hop rewrite;
// NextHopUnchanged leaves the received (possibly third-party) next hop alone.
//
// PREVENTS: Being unable to stop a third-party next hop reaching a peer.
//
// RFC requirement: RFC4271-5.1.3-3 positive -- NextHopSelf makes precomputeNextHop install
// a rewrite to the local address, which is what disables advertisement of a third-party
// NEXT_HOP (internal/component/bgp/reactor/peer_forward_facts.go:153-177), and the
// per-route resolver returns the same local address
// (internal/component/bgp/reactor/peer.go:670-679).
func TestRFC4271ThirdPartyNextHopCanBeDisabled(t *testing.T) {
	s := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	s.LocalAddress = netip.MustParseAddr("198.51.100.7")
	s.NextHopMode = NextHopSelf

	facts := &peerForwardFacts{}
	precomputeNextHop(s, facts)
	require.Equal(t, nhModeSelf4, facts.nhMode, "self rewrite armed")
	assert.Equal(t, [4]byte{198, 51, 100, 7}, facts.nhLegacy)

	unchanged := NewPeerSettings(netip.MustParseAddr("192.0.2.2"), 65001, 65002, 0x01020301)
	unchanged.LocalAddress = netip.MustParseAddr("198.51.100.7")
	unchanged.NextHopMode = NextHopUnchanged
	other := &peerForwardFacts{}
	precomputeNextHop(unchanged, other)
	assert.Equal(t, nhModeNone, other.nhMode, "unchanged mode performs no rewrite")
}

// TestRFC4271ThirdPartyNextHopDisableFailsClosed verifies the disable does not silently
// degrade when it cannot be honored.
//
// VALIDATES: NextHopSelf without a local address arms no rewrite, and the per-route
// resolver reports ErrNextHopSelfNoLocal instead of producing an address.
//
// PREVENTS: A misconfigured next-hop-self quietly advertising a third-party next hop as if
// the disable were in force.
//
// RFC requirement: RFC4271-5.1.3-3 negative -- a next-hop-self that cannot be satisfied is
// refused rather than approximated: precomputeNextHop leaves the mode at none
// (internal/component/bgp/reactor/peer_forward_facts.go:157-161) and resolveNextHop
// returns ErrNextHopSelfNoLocal (internal/component/bgp/reactor/peer.go:670-674).
func TestRFC4271ThirdPartyNextHopDisableFailsClosed(t *testing.T) {
	s := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	s.NextHopMode = NextHopSelf // LocalAddress deliberately unset

	facts := &peerForwardFacts{}
	precomputeNextHop(s, facts)
	assert.Equal(t, nhModeNone, facts.nhMode, "no rewrite armed without a local address")

	p := &Peer{settings: s}
	_, err := p.resolveNextHop(bgptypes.NewNextHopSelf(), family.IPv4Unicast)
	assert.ErrorIs(t, err, ErrNextHopSelfNoLocal)
}

// TestRFC4271OwnASReachabilityChangeAdvertised verifies a change to a destination inside
// ze's own AS is expressed as an UPDATE.
//
// VALIDATES: A locally originated prefix encodes as an announce UPDATE carrying the
// prefix in the NLRI field.
//
// PREVENTS: A local origination that never reaches the wire.
//
// RFC requirement: RFC4271-9.2-9 positive -- a reachable destination within the speaker's
// own AS is advertised by encoding an announce UPDATE whose NLRI field carries the prefix
// (internal/component/bgp/reactor/reactor_wire.go:230-390).
func TestRFC4271OwnASReachabilityChangeAdvertised(t *testing.T) {
	buf := make([]byte, 4096)
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
	}
	n := WriteAnnounceUpdate(buf, 0, route, 65001, true, true, false)
	require.Greater(t, n, message.HeaderLen)

	assert.Equal(t, byte(message.TypeUPDATE), buf[message.MarkerLen+2])
	withdrawnLen := int(buf[message.HeaderLen])<<8 | int(buf[message.HeaderLen+1])
	assert.Equal(t, 0, withdrawnLen, "an announce carries no withdrawals")
	attrLen := int(buf[message.HeaderLen+2])<<8 | int(buf[message.HeaderLen+3])
	nlri := buf[message.HeaderLen+4+attrLen : n]
	assert.Equal(t, []byte{24, 10, 0, 0}, nlri, "the prefix is announced in the NLRI field")
}

// TestRFC4271OwnASUnreachabilityChangeAdvertised verifies the loss of a locally originated
// destination is also expressed as an UPDATE.
//
// VALIDATES: Removing the prefix encodes a withdraw UPDATE whose Withdrawn Routes field
// carries the prefix and whose NLRI field is empty.
//
// PREVENTS: Reading "changes are advertised" as "only additions are advertised".
//
// RFC requirement: RFC4271-9.2-9 negative -- the advertisement is a change notification,
// not an announce-only path: a destination that stops being reachable is encoded into the
// Withdrawn Routes field rather than announced
// (internal/component/bgp/reactor/reactor_wire.go:451-475).
func TestRFC4271OwnASUnreachabilityChangeAdvertised(t *testing.T) {
	buf := make([]byte, 4096)
	n := WriteWithdrawUpdate(buf, 0, netip.MustParsePrefix("10.0.0.0/24"), false)
	require.Greater(t, n, message.HeaderLen)

	assert.Equal(t, byte(message.TypeUPDATE), buf[message.MarkerLen+2])
	withdrawnLen := int(buf[message.HeaderLen])<<8 | int(buf[message.HeaderLen+1])
	require.Equal(t, 4, withdrawnLen, "withdrawn routes field carries the prefix")
	assert.Equal(t, []byte{24, 10, 0, 0}, buf[message.HeaderLen+2:message.HeaderLen+2+withdrawnLen])
	attrLen := int(buf[message.HeaderLen+2+withdrawnLen])<<8 | int(buf[message.HeaderLen+3+withdrawnLen])
	assert.Equal(t, 0, attrLen, "a withdraw carries no path attributes")
}

// TestRFC4271MEDAlterationHappensAtIngress verifies an alteration of a received
// MULTI_EXIT_DISC is expressible on the ingress path, which runs before the route reaches
// the decision process.
//
// VALIDATES: An ingress filter's rewritten payload is what safeIngressFilter returns, and
// the ingress loop installs that payload as the UPDATE before dispatch.
//
// PREVENTS: Altering MED after the route has already been used for best-path selection.
//
// RFC requirement: RFC4271-5.1.4-3 positive -- the only place ze can alter a received MED
// is the ingress filter chain, whose modified payload replaces the WireUpdate before the
// UPDATE is dispatched to the RIB plugin that runs phases 1 and 2
// (internal/component/bgp/reactor/reactor_notify.go:427-466).
func TestRFC4271MEDAlterationHappensAtIngress(t *testing.T) {
	original := []byte{
		0x00, 0x00, 0x00, 0x0b,
		0x40, 0x01, 0x01, 0x00,
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64, // MED = 100
	}
	rewritten := append([]byte(nil), original...)
	rewritten[len(rewritten)-1] = 0x0a // MED = 10

	medFilter := func(_ filterapi.PeerFilterInfo, payload []byte, _ map[string]any) (bool, []byte) {
		require.Equal(t, original, payload, "the filter sees the received UPDATE")
		return true, rewritten
	}
	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001}

	accept, modified := safeIngressFilter(medFilter, src, original, map[string]any{})
	require.True(t, accept)
	require.Equal(t, rewritten, modified,
		"the altered MED is what the ingress loop installs before dispatch")
}

// rfc4271IBGPPeer builds an established iBGP RS fast-path peer.
func rfc4271IBGPPeer(t *testing.T, addr string, routerID uint32, rrClient bool, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	t.Helper()
	s := &PeerSettings{
		Connection:           ConnectionBoth,
		Address:              netip.MustParseAddr(addr),
		LocalAS:              65000,
		PeerAS:               65000,
		RouterID:             routerID,
		RSFastPath:           true,
		RouteReflectorClient: rrClient,
	}
	p := NewPeer(s)
	p.state.Store(int32(PeerStateEstablished))
	p.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
	})
	p.sendCtx.Store(ctx)
	p.sendCtxID = ctxID
	p.refreshForwardFacts()
	return p
}

// TestRFC4271NoIBGPToIBGPRedistribution verifies a route learned from an internal peer is
// not passed on to another internal peer when neither side is a route reflector client.
//
// VALIDATES: With a plain iBGP source and a plain iBGP destination, nothing is dispatched
// to the destination peer.
//
// PREVENTS: An iBGP mesh loop caused by re-advertising internal routes internally.
//
// RFC requirement: RFC4271-9.2-6 positive -- reactorForwardRS skips an internal
// destination for an internally learned route unless the source or the destination is a
// route reflector client (internal/component/bgp/reactor/forward_rs.go:309-313).
func TestRFC4271NoIBGPToIBGPRedistribution(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(81)
	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}
	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(81, 1)

	src := rfc4271IBGPPeer(t, "10.0.0.1", 0x01020301, false, ctx, ctxID)
	src.remoteRouterID.Store(0x0A000001)
	dst := rfc4271IBGPPeer(t, "10.0.0.2", 0x01020302, false, ctx, ctxID)

	var mu sync.Mutex
	var dispatched []fwdItem
	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched = append(dispatched, items...)
		mu.Unlock()
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{
		recentUpdates:   cache,
		attrModHandlers: attrModHandlersWithDefaults(),
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: testPool,
	}

	reactorForwardRS(r, update, 81, netip.MustParseAddr("10.0.0.1"), src)

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	assert.Empty(t, dispatched, "iBGP-learned route must not be redistributed to another iBGP peer")
}

// TestRFC4271IBGPRedistributionAllowedForReflectorClient verifies the split-horizon drop is
// conditional on neither side being a route reflector client.
//
// VALIDATES: With the source marked as a route reflector client, the same iBGP-to-iBGP
// forward is dispatched.
//
// PREVENTS: Reading the split-horizon rule as "iBGP never forwards to iBGP", which would
// break route reflection.
//
// RFC requirement: RFC4271-9.2-6 negative -- the prohibition carries the route reflector
// exception: with a route reflector client on one side the same forward is not skipped
// (internal/component/bgp/reactor/forward_rs.go:309-313).
func TestRFC4271IBGPRedistributionAllowedForReflectorClient(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(82)
	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}
	cache := NewRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(82, 1)

	src := rfc4271IBGPPeer(t, "10.0.0.1", 0x01020301, true, ctx, ctxID)
	src.remoteRouterID.Store(0x0A000001)
	dst := rfc4271IBGPPeer(t, "10.0.0.2", 0x01020302, false, ctx, ctxID)

	var mu sync.Mutex
	var dispatched []fwdItem
	done := make(chan struct{})
	var once sync.Once
	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched = append(dispatched, items...)
		mu.Unlock()
		once.Do(func() { close(done) })
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{
		recentUpdates:   cache,
		attrModHandlers: attrModHandlersWithDefaults(),
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: testPool,
	}

	reactorForwardRS(r, update, 82, netip.MustParseAddr("10.0.0.1"), src)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for the reflected forward")
	}
	mu.Lock()
	defer mu.Unlock()
	assert.NotEmpty(t, dispatched, "a route reflector client's route is reflected to iBGP")
}

// TestRFC4271UpdateErrorReportedAsUpdateMessageError verifies an error detected in a
// received UPDATE is reported with Error Code UPDATE Message Error.
//
// VALIDATES: A duplicate MP_REACH_NLRI tears the session down with a NOTIFICATION whose
// Error Code is 3, and the code is not Cease.
//
// PREVENTS: Reporting an UPDATE error under the wrong Error Code, which a peer cannot
// classify.
//
// RFC requirement: RFC4271-6.3-1 positive -- an UPDATE error is signaled by a NOTIFICATION
// carrying Error Code UPDATE Message Error
// (internal/component/bgp/reactor/session_validation.go:198-211).
// RFC requirement: RFC4271-6.7-1 negative -- Cease is not used for this error: a fatal
// protocol error exists, so the NOTIFICATION carries Error Code 3 and not Error Code 6
// (internal/component/bgp/reactor/session_validation.go:205-207).
func TestRFC4271UpdateErrorReportedAsUpdateMessageError(t *testing.T) {
	session, client, _, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	mpReach := []byte{
		0x00, 0x01,
		0x01,
		0x04,
		0xc0, 0x00, 0x02, 0x01,
		0x00,
		0x08, 0x0a,
	}
	pathAttrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00, 0x80, 0x0e, byte(len(mpReach))}
	pathAttrs = append(pathAttrs, mpReach...)
	pathAttrs = append(pathAttrs, 0x80, 0x0e, byte(len(mpReach)))
	pathAttrs = append(pathAttrs, mpReach...)

	update := []byte{0x00, 0x00, byte(len(pathAttrs) >> 8), byte(len(pathAttrs))}
	update = append(update, pathAttrs...)

	var received []byte
	done := make(chan struct{})
	go func() {
		client.Write(buildUpdateMsg(update)) //nolint:errcheck // test goroutine
		buf := make([]byte, 4096)
		n, _ := client.Read(buf) //nolint:errcheck // read NOTIFICATION
		received = buf[:n]
		close(done)
	}()

	err := session.ReadAndProcess()
	require.Error(t, err)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for NOTIFICATION")
	}

	require.GreaterOrEqual(t, len(received), message.HeaderLen+2)
	hdr, hdrErr := message.ParseHeader(received[:message.HeaderLen])
	require.NoError(t, hdrErr)
	require.Equal(t, message.TypeNOTIFICATION, hdr.Type)
	body := received[message.HeaderLen:]
	assert.Equal(t, byte(message.NotifyUpdateMessage), body[0], "Error Code 3, UPDATE Message Error")
	assert.NotEqual(t, byte(message.NotifyCease), body[0], "a fatal error must not be reported as Cease")
}

// TestRFC4271ConformantUpdateSendsNoUpdateError verifies a well-formed UPDATE raises no
// UPDATE Message Error.
//
// VALIDATES: A valid UPDATE is processed with no error, the session stays Established, and
// nothing is written back on the connection.
//
// PREVENTS: An implementation that satisfies the code-3 obligation by notifying on every
// UPDATE.
//
// RFC requirement: RFC4271-6.3-1 negative -- the code-3 NOTIFICATION is driven by an actual
// UPDATE error: a conformant UPDATE takes the RFC7606ActionNone path and sends nothing
// (internal/component/bgp/reactor/session_validation.go:123-125).
func TestRFC4271ConformantUpdateSendsNoUpdateError(t *testing.T) {
	session, client, _, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
	}
	update := []byte{0x00, 0x00, byte(len(pathAttrs) >> 8), byte(len(pathAttrs))}
	update = append(update, pathAttrs...)
	update = append(update, 0x08, 0x0a)

	written := make(chan struct{})
	go func() {
		client.Write(buildUpdateMsg(update)) //nolint:errcheck // test goroutine
		close(written)
	}()

	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, session.State())

	select {
	case <-written:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout writing UPDATE")
	}

	require.NoError(t, client.SetReadDeadline(time.Now().Add(200*time.Millisecond)))
	buf := make([]byte, 4096)
	n, readErr := client.Read(buf)
	require.Error(t, readErr, "no NOTIFICATION may be sent for a conformant UPDATE; got %d byte(s)", n)
}

// TestRFC4271ConformantOpenSendsNoOpenError verifies a well-formed OPEN raises no OPEN
// Message Error.
//
// VALIDATES: handleOpen accepts a valid OPEN body without error and writes no NOTIFICATION.
//
// PREVENTS: An implementation that satisfies the code-2 obligation by rejecting every OPEN.
//
// Untagged for RFC4271-6.2-3 (recorded {gap} in rfc/short/rfc4271.md): the code-2
// NOTIFICATION is emitted only when an OPEN error is detected, and a conformant OPEN passes
// version, hold-time and capability validation with nothing sent
// (internal/component/bgp/reactor/session_handlers.go:39-116).
func TestRFC4271ConformantOpenSendsNoOpenError(t *testing.T) {
	s, client := newOpenSentSessionWithClient(t)

	// Drain whatever the session writes so handleOpen never blocks on the pipe, and keep
	// every byte so the absence of a NOTIFICATION can be asserted rather than assumed.
	var mu sync.Mutex
	var seen []byte
	go func() {
		buf := make([]byte, 65536)
		for {
			n, err := client.Read(buf)
			if n > 0 {
				mu.Lock()
				seen = append(seen, buf[:n]...)
				mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()

	require.NoError(t, s.handleOpen(validOpenBody()))
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	for off := 0; off+message.HeaderLen <= len(seen); {
		hdr, err := message.ParseHeader(seen[off : off+message.HeaderLen])
		require.NoError(t, err)
		assert.NotEqual(t, message.TypeNOTIFICATION, hdr.Type,
			"a conformant OPEN must not draw a NOTIFICATION")
		off += int(hdr.Length)
	}
}
