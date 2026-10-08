// Design: docs/architecture/wire/attributes.md -- carrier-aware tunnel attribute validation.
// Related: session_tunnel_encap.go -- receive-side framing and carrier enforcement.
// RFC 9012 and RFC 9830 receive-and-propagate obligations for the Tunnel
// Encapsulation attribute, driven through receive dispatch, the cached export API,
// the socket writer, and a downstream receive consumer.
//
// Related: ../../../core/bgp/attribute/rfc9012_test.go -- the codec half
// Related: ../../../core/bgp/attribute/rfc9830_test.go -- the codec half
//
// Ze carries received tunnel descriptions without acting as an SR Policy
// headend. These tests observe BGP receive decisions and propagation, not SRPM
// candidate-path selection or tunnel dataplane installation.
//
// VALIDATES: a received Tunnel Encapsulation attribute, clean or full of
// fields to ignore and sub-TLVs to tolerate, gets no RFC 7606 action and
// reaches the next peer octet for octet, forwarded raw or rebuilt.
// PREVENTS: a receiver or re-encoder that refuses, strips, masks or reorders
// what RFC 9012 and RFC 9830 say must be ignored and passed along.

package reactor

import (
	"bufio"
	"bytes"
	"net/netip"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// teSub builds a one-octet-length sub-TLV (RFC 9012 Section 2, type below 128).
func teSub(typ byte, value ...byte) []byte {
	return append([]byte{typ, byte(len(value))}, value...)
}

// teLongSub builds a two-octet-length sub-TLV (RFC 9012 Section 2, type 128 or above).
func teLongSub(typ byte, value ...byte) []byte {
	return append([]byte{typ, byte(len(value) >> 8), byte(len(value))}, value...)
}

// teTLV builds a Tunnel TLV: Tunnel Type (2), Length (2), sub-TLVs.
func teTLV(tunnelType uint16, subs ...[]byte) []byte {
	var value []byte
	for _, s := range subs {
		value = append(value, s...)
	}
	head := []byte{byte(tunnelType >> 8), byte(tunnelType), byte(len(value) >> 8), byte(len(value))}
	return append(head, value...)
}

var teSRv6SID = []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

// teSRPolicyValue builds one SR Policy TLV for SAFI 73. x changes ignored
// reserved fields; ignored adds sub-TLVs without SR Policy applicability.
// RFC 9830 Section 2.2: "The content of the SR Policy CP is encoded in the Tunnel
// Encapsulation Attribute defined in [RFC9012] using a Tunnel Type called the
// "SR Policy" type with code point 15.".
func teSRPolicyValue(x byte, ignored bool) []byte {
	typeA := teSub(1, 0x7F&x, x, 0x05, 0xDC, 0x01&x, 255)
	typeB := teSub(13, append(append([]byte{0x10 | (0x6F & x), x}, teSRv6SID...), 0xFF, 0xFF, x, x, 32, 16, 16, 64)...)
	// RFC 9830 Section 2.4.4.2 forbids mixing MPLS and SRv6 in one list.
	mplsList := teLongSub(128, append(append([]byte{x}, teSub(9, x, x, 0, 0, 0, 7)...), typeA...)...)
	srv6List := teLongSub(128, append(append([]byte{x}, teSub(9, x, x, 0, 0, 0, 7)...), typeB...)...)
	srPolicy := [][]byte{
		// Preference, Binding SID, SRv6 Binding SID, Priority (RFC 9830
		// Sections 2.4.1, 2.4.2, 2.4.3, 2.4.6).
		teSub(12, x, x, 0, 0, 0, 100),
		teSub(13, 0x3F&x, x, 0x05, 0xDC, 0x0F&x, x),
		teSub(20, append([]byte{0x1F & x, x}, teSRv6SID...)...),
		teSub(15, 9, x),
		// Candidate Path Name, Policy Name, Segment List (Sections 2.4.7,
		// 2.4.8, 2.4.4).
		teLongSub(129, append([]byte{x}, "primary"...)...),
		teLongSub(130, append([]byte{x}, "alpha"...)...),
		mplsList, srv6List,
	}
	if ignored {
		// RFC 9830 Section 2.3: "If these sub-TLVs are present, a BGP speaker
		// MUST ignore them and MAY remove them from the Tunnel Encapsulation
		// Attribute during propagation."
		srPolicy = append(srPolicy,
			teSub(6, x, x, x, x, 0, 1, 10, 0, 0, 77),
			teSub(6, 0, 0, 0, 0, 0, 1, 10, 0, 0, 78),
			teSub(4, 0x03, 0x0b, 0, 0, 0, 0, 0, 5),
			teSub(9, 1),
			teSub(8, 0x12, 0x34),
			teSub(1, 0xC0|(0x3F&x), 0, 0, 10, 0, 0, 0, 0, 0, 0, x, x),
		)
	}
	return teTLV(15, srPolicy...)
}

// teCarryValue supplies RFC 9012 tunnels with exactly one endpoint each.
// RFC 9012 Section 13: "Within a Tunnel Encapsulation attribute that is carried
// by a BGP UPDATE whose AFI/SAFI is one of those explicitly listed in the first
// paragraph of Section 6, a TLV that does not contain exactly one Tunnel Egress
// Endpoint sub-TLV MUST be treated as if it contained a malformed Tunnel Egress
// Endpoint sub-TLV.".
func teCarryValue(x byte, odd bool) []byte {
	endpoint := teSub(6, x, x, x, x, 0, 1, 10, 0, 0, 77)
	gre := [][]byte{endpoint, teSub(9, 1)}
	if odd {
		gre = append(gre,
			teSub(9, 2),                // Duplicate Embedded Label Handling.
			teSub(8, 0x12, 0x34),       // Well-formed but meaningless for GRE.
			teLongSub(200, 0xde, 0xad), // Unrecognized sub-TLV.
		)
	}
	value := teTLV(2, gre...)
	vxlan := teSub(1, 0xC0|(0x3F&x), 0, 0, 10, 0, 0, 0, 0, 0, 0, x, x)
	value = append(value, teTLV(8, endpoint, vxlan)...)
	value = append(value, teTLV(9, endpoint, vxlan)...)
	udp := teSub(8, 0x12, 0x34)
	if odd {
		udp = teSub(8, 0x12, 0x34, 0x56) // Malformed in an applicable UDP tunnel.
	}
	value = append(value, teTLV(13, endpoint, udp)...)
	if odd {
		value = append(value, teTLV(0xFFFE, endpoint, teSub(99, 1, 2, 3))...)
	}
	return value
}

// teColorExtComm is received Color 42 with nonzero unassigned Flags (Section 4.3).
// Keep RFC 9830's assigned Color-Only bits clear to isolate ignored bits.
var teColorExtComm = []byte{0x03, 0x0b, 0x3f, 0xa5, 0, 0, 0, 42}

func teCarryBody(value []byte) []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN
		0x40, 0x02, 0x06, 2, 1, 0, 0, 0xfd, 0xea, // AS_SEQUENCE 65002 (ASN4).
		0x40, 0x03, 0x04, 192, 0, 2, 1, // NEXT_HOP
		0xC0, byte(attribute.AttrExtCommunity), byte(len(teColorExtComm)),
	}
	attrs = append(attrs, teColorExtComm...)
	attrs = append(attrs, 0xD0, byte(attribute.AttrTunnelEncap), byte(len(value)>>8), byte(len(value)))
	attrs = append(attrs, value...)
	return makeUpdateBody(nil, attrs, []byte{24, 10, 30, 0})
}

// teForwardedAttrs receives an EBGP UPDATE through processMessage and publishes
// it through the reactor's normal cache/dispatch callback. ForwardUpdate exports
// that message to an IBGP peer; a second session consumes the socket output.
// The rebuild case installs an actual egress policy setting MED, rather than
// constructing a forward item or calling the payload builder from the test.
// RFC 9012 Section 13: "If the route carrying the Tunnel Encapsulation attribute
// is propagated with the attribute, the unrecognized TLV MUST remain in the attribute.".
func teForwardedAttrs(t *testing.T, body []byte, rebuild bool, expectedTunnel ...[]byte) []byte {
	t.Helper()
	before, err := wire.ParseUpdateSections(body)
	require.NoError(t, err)
	var families []family.Family
	if len(before.NLRI(body))+len(before.Withdrawn(body)) > 0 {
		families = append(families, family.IPv4Unicast)
	}
	for _, code := range []attribute.AttributeCode{attribute.AttrMPReachNLRI, attribute.AttrMPUnreachNLRI} {
		_, _, mp, found := attribute.AttrFind(before.Attrs(body), code)
		if !found {
			continue
		}
		require.GreaterOrEqual(t, len(mp), 3)
		fam := family.Family{AFI: family.AFI(uint16(mp[0])<<8 | uint16(mp[1])), SAFI: family.SAFI(mp[2])}
		if !slices.Contains(families, fam) {
			families = append(families, fam)
		}
	}
	source, _ := tePropagationPeer(t, "192.0.2.1", 65002, families...)
	destination, conn := tePropagationPeer(t, "192.0.2.2", 65001, families...)
	cache := newRecentUpdateCache(16)
	t.Cleanup(cache.Stop)
	cache.RegisterConsumer("tunnel-forwarder")
	r := &Reactor{
		clock:  source.clock,
		config: &Config{LocalAS: 65001},
		peers: map[netip.AddrPort]*Peer{
			source.Settings().PeerKey():      source,
			destination.Settings().PeerKey(): destination,
		},
		recentUpdates:   cache,
		attrModHandlers: attrModHandlersWithDefaults(),
	}
	if rebuild {
		r.orderedEgressSteps = orderedEgressStepsFromFuncs(func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
			mods.Op(uint8(attribute.AttrMED), filterapi.AttrModSet, []byte{0, 0, 0, 50})
			return true
		})
	}
	r.fwdPool = newFwdPool(fwdBatchHandler, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: destination.Settings().PeerKey()}, 4096)
	t.Cleanup(r.fwdPool.Stop)
	var id uint64
	r.setMessageReceiver(&testDeliveryReceiver{
		consumerCount: 1,
		onReceived:    func(_ plugin.PeerInfo, msg bgptypes.RawMessage) { id = msg.MessageID },
	})
	source.session.onMessageReceived = r.notifyMessageReceiver
	header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(body))}
	// RFC 9012 Section 13; RFC 9830 Sections 2.2, 2.3 and 2.4.
	err, kept := source.session.processMessage(&header, body, BufHandle{ID: noPoolBufID, Buf: body})
	require.NoError(t, err)
	require.True(t, kept)
	require.NotZero(t, id, "the received route must reach dispatch and its forward cache")
	sel, err := selector.Parse(destination.Settings().Address.String())
	require.NoError(t, err)
	require.NoError(t, (&reactorAPIAdapter{r: r}).ForwardUpdate(sel, id, "tunnel-forwarder", plugin.ProcessSender("tunnel-forwarder")))
	forwardSocketBarrier(t, r)
	bodies := aigpSocketBodies(t, conn)
	require.Len(t, bodies, 1, "exactly one announcement must reach the downstream peer")
	sent := bodies[0]
	after, err := wire.ParseUpdateSections(sent)
	require.NoError(t, err)
	require.Equal(t, before.NLRI(body), after.NLRI(sent), "export must retain the announced legacy NLRI")
	require.Equal(t, before.Withdrawn(body), after.Withdrawn(sent), "export must not synthesize a withdrawal")
	attrs := after.Attrs(sent)
	for _, code := range []attribute.AttributeCode{
		attribute.AttrOrigin, attribute.AttrASPath, attribute.AttrNextHop,
		attribute.AttrTunnelEncap, attribute.AttrExtCommunity, attribute.AttrMPReachNLRI,
	} {
		wantCount, want := countAttrCode(before.Attrs(body), uint8(code))
		if code == attribute.AttrTunnelEncap {
			if len(expectedTunnel) != 0 {
				require.Len(t, expectedTunnel, 1, "one explicit receive-rewrite expectation")
				want = expectedTunnel[0]
			}
		}
		gotCount, got := countAttrCode(attrs, uint8(code))
		require.Equal(t, wantCount, gotCount, "attribute %d count", code)
		require.Equal(t, want, got, "attribute %d must survive actual export unchanged", code)
	}
	// Both rails add mandatory IBGP LOCAL_PREF; only the policy rail adds MED.
	count, localPref := countAttrCode(attrs, uint8(attribute.AttrLocalPref))
	require.Equal(t, 1, count)
	require.Equal(t, []byte{0, 0, 0, 100}, localPref)
	if rebuild {
		count, med := countAttrCode(attrs, uint8(attribute.AttrMED))
		require.Equal(t, 1, count)
		require.Equal(t, []byte{0, 0, 0, 50}, med, "the egress policy must actually rebuild")
	}
	downstream, _ := tePropagationPeer(t, "192.0.2.9", 65001, families...)
	received := 0
	downstream.session.onMessageReceived = func(_ netip.Addr, typ msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		received++
		require.Equal(t, msgtype.TypeUPDATE, typ)
		require.Equal(t, sent, wu.Payload(), "downstream must dispatch the announcement, not a withdrawal")
		return false
	}
	header.Length = uint16(message.HeaderLen + len(sent))
	err, kept = downstream.session.processMessage(&header, sent, BufHandle{ID: noPoolBufID, Buf: sent})
	require.NoError(t, err)
	require.False(t, kept)
	require.Equal(t, 1, received)
	return attrs
}

// tePropagationPeer supplies the established-session transport used by the
// existing reactor socket fixtures, negotiating only the UPDATE's actual families.
func tePropagationPeer(t *testing.T, address string, peerAS uint32, families ...family.Family) (*Peer, *recordingConn) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr(address), 65001, peerAS, 0x01020301)
	settings.LocalAddress = netip.MustParseAddr("192.0.2.254")
	settings.NextHopMode = NextHopUnchanged
	// The propagation fixture's peers belong to one SR domain. An EBGP source
	// must opt in; otherwise RFC 8669 Section 4 correctly discards Prefix-SID.
	settings.AcceptSRv6PrefixSID = true
	settings.ProcessBindings = []ProcessBinding{{PluginName: "tunnel-forwarder", SendAll: true}}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	caps := []capability.Capability{&capability.ASN4{ASN: 65001}}
	negotiatedFamilies := make(map[family.Family]bool, len(families))
	for _, fam := range families {
		caps = append(caps, &capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI})
		negotiatedFamilies[fam] = true
	}
	s := NewSession(settings)
	t.Cleanup(s.timers.StopAll)
	remoteCaps := append([]capability.Capability{&capability.ASN4{ASN: peerAS}}, caps[1:]...)
	s.negotiated = capability.Negotiate(caps, remoteCaps, capability.PeerIdentity{
		LocalASN: settings.LocalAS, PeerASN: settings.PeerAS, Internal: settings.IsIBGP(),
	})
	ctx := bgpctx.FromNegotiatedRecv(s.negotiated)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	s.recvCtxID, s.sendCtxID = ctxID, ctxID
	s.SetSourceID(peer.SourceID())
	for _, event := range []fsm.Event{fsm.EventManualStart, fsm.EventTCPConnectionConfirmed, fsm.EventBGPOpen, fsm.EventKeepaliveMsg} {
		require.NoError(t, s.fsm.Event(event))
	}
	conn := &recordingConn{}
	s.conn = conn
	s.bufWriter = bufio.NewWriterSize(conn, 4096)
	s.nextHopScope.Store(&receiveNextHopScope{
		addresses: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
		local:     settings.LocalAddress, remote: settings.Address, direct: true,
	})
	peer.session = s
	peer.negotiated.Store(&NegotiatedCapabilities{ASN4: true, families: negotiatedFamilies})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID, peer.recvCtxID = ctxID, ctxID
	peer.refreshForwardFacts()
	return peer, conn
}

// TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong drives a received
// Tunnel Encapsulation attribute through receive validation, processMessage,
// dispatch and ForwardUpdate to the socket and a downstream receive consumer.
// The second rail uses an egress MED policy to force payload rebuilding.
// Method: isolated unknown, duplicate, meaningless and malformed-value controls
// have exactly one valid endpoint. Malformed UDP is tested under MPLS-in-UDP;
// well-formed UDP under GRE isolates meaninglessness. Combined reserved-field
// cases also cover VXLAN and NVGRE. Endpoint-invalid TLVs are tested separately
// because Section 13 requires their removal. No encapsulation-header creation
// or SRPM semantic behavior is asserted.
//
// RFC requirement: RFC9012-13-3 positive -- an attribute holding a TLV of unrecognized tunnel type 0xFFFE gets no RFC 7606 action and is kept.
// RFC requirement: RFC9012-13-3 negative -- export rebuilding must not misclassify or lose the framed unknown TLV.
// RFC requirement: RFC9012-13-5 positive -- the unrecognized-type TLV reaches the peer's wire unchanged when the route is forwarded as received.
// RFC requirement: RFC9012-13-5 negative -- rebuilding the UPDATE for an egress MED policy does not drop or alter it.
// RFC requirement: RFC9012-13-8 positive -- a GRE TLV holding duplicate Embedded Label Handling sub-TLVs gets no RFC 7606 action and is kept.
// RFC requirement: RFC9012-13-8 negative -- the rebuilt UPDATE still carries that TLV whole.
// RFC requirement: RFC9012-13-9 positive -- both Embedded Label Handling occurrences reach the wire unchanged.
// RFC requirement: RFC9012-13-9 negative -- the rebuilt UPDATE still carries both occurrences.
// RFC requirement: RFC9012-13-10 positive -- a TLV with an unrecognized sub-TLV gets the same verdict and handling as the clean TLV without it.
// RFC requirement: RFC9012-13-10 negative -- the dirty and clean runs differ only in the octets of the attribute itself.
// RFC requirement: RFC9012-13-11 positive -- the unrecognized sub-TLV reaches the wire unchanged.
// RFC requirement: RFC9012-13-11 negative -- the rebuilt UPDATE still carries it.
// RFC requirement: RFC9012-13-12 positive -- a UDP Destination Port sub-TLV of Length 3 is handled as an unrecognized one: no action, carried.
// RFC requirement: RFC9012-13-12 negative -- the dirty and clean runs reach the same verdict.
// RFC requirement: RFC9012-13-18 positive -- the TLV holding the meaningless sub-TLV gets no RFC 7606 action.
// RFC requirement: RFC9012-13-18 negative -- the rebuilt UPDATE still carries it.
// RFC requirement: RFC9012-13-19 positive -- the meaningless sub-TLV reaches the wire unchanged.
// RFC requirement: RFC9012-13-19 negative -- the rebuilt UPDATE still carries it.
// RFC requirement: RFC9012-3.1-3 positive -- non-zero Tunnel Egress Endpoint Reserved octets reach the wire unchanged.
// RFC requirement: RFC9012-3.1-3 negative -- the rebuilt UPDATE does not zero them.
// RFC requirement: RFC9012-3.2.1-2 positive -- the VXLAN and NVGRE R bits reach the downstream consumer unchanged.
// RFC requirement: RFC9012-3.2.1-2 negative -- the rebuilt UPDATE does not mask them.
// RFC requirement: RFC9012-3.5-3 positive -- the Embedded Label Handling sub-TLV reaches the wire.
// RFC requirement: RFC9012-3.5-3 negative -- the rebuilt UPDATE does not strip it.
// RFC requirement: RFC9012-4.3-2 positive -- the complete Color Extended Community, including nonzero Flags 0x3fa5, reaches the wire unchanged.
// RFC requirement: RFC9012-4.3-2 negative -- rebuilding the UPDATE does not clear or change the received Flags field.
// RFC requirement: RFC9012-3.1-2 positive -- zero endpoint Reserved fields are accepted and delivered unchanged through receive dispatch and downstream receipt.
// RFC requirement: RFC9012-3.1-2 negative -- nonzero endpoint Reserved fields change neither ActionNone nor successful dispatch and downstream receipt.
func TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong(t *testing.T) {
	endpoint := teSub(6, 0, 0, 0, 0, 0, 1, 10, 0, 0, 77)
	cases := []struct {
		name  string
		value []byte
	}{
		{"clean", teCarryValue(0, false)},
		{"dirty", teCarryValue(0xFF, true)},
		{"known-tunnel", teTLV(2, endpoint)},
		{"unknown-tunnel", teTLV(0xFFFE, endpoint)},
		{"unknown-subtlv", teTLV(2, endpoint, teLongSub(200, 0xde, 0xad))},
		{"single-embedded-label", teTLV(2, endpoint, teSub(9, 1))},
		{"duplicate-embedded-label", teTLV(2, endpoint, teSub(9, 1), teSub(9, 2))},
		{"meaningless-udp-in-gre", teTLV(2, endpoint, teSub(8, 0x12, 0x34))},
		{"applicable-udp-absent", teTLV(13, endpoint)},
		{"applicable-udp-valid", teTLV(13, endpoint, teSub(8, 0x12, 0x34))},
		{"applicable-udp-malformed-length", teTLV(13, endpoint, teSub(8, 0x12, 0x34, 0x56))},
		{"applicable-udp-malformed-zero", teTLV(13, endpoint, teSub(8, 0, 0))},
	}
	var verdicts []message.RFC7606Action
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// RFC 9012 Sections 6 and 13.
			value := tc.value
			s := teValidationSession()
			// RFC 9012 Section 13.
			wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(teCarryBody(value), 0))
			require.NoError(t, err, "the attribute must not reset the session")
			require.Equal(t, message.RFC7606ActionNone, action, "the attribute must not be treated as malformed")
			verdicts = append(verdicts, action)
			received := wu.Payload()
			count, kept := countAttrCode(rfc8669PathAttrs(t, received), uint8(attribute.AttrTunnelEncap))
			require.Equal(t, 1, count)
			require.True(t, bytes.Equal(value, kept), "receive changed the attribute: got %x want %x", kept, value)
			for _, rebuild := range []bool{false, true} {
				attrs := teForwardedAttrs(t, received, rebuild)
				count, sent := countAttrCode(attrs, uint8(attribute.AttrTunnelEncap))
				require.Equal(t, 1, count, "rebuild=%v: the attribute must be passed along", rebuild)
				require.True(t, bytes.Equal(value, sent), "rebuild=%v: got %x want %x", rebuild, sent, value)
				count, ext := countAttrCode(attrs, uint8(attribute.AttrExtCommunity))
				require.Equal(t, 1, count, "rebuild=%v: the Color Extended Community must be passed along", rebuild)
				require.True(t, bytes.Equal(teColorExtComm, ext), "rebuild=%v: got %x want %x", rebuild, ext, teColorExtComm)
			}
		})
	}
	require.Len(t, verdicts, len(cases))
	for _, action := range verdicts {
		require.Equal(t, verdicts[0], action, "each isolated tolerated input must have the clean verdict")
	}
}

// RFC 9012 Section 4.3: "No flags are defined in this document; this field MUST
// be set to zero by the originator and ignored by the receiver; the value MUST
// NOT be changed when propagating this extended community."
// The zero-flags control traverses the same exporter as the nonzero-flags case,
// so a mutation that clears received flags must leave this control unchanged.
func TestRFC9012ZeroColorFlagsExportControl(t *testing.T) {
	endpoint := teSub(6, 0, 0, 0, 0, 0, 1, 10, 0, 0, 77)
	body := teCarryBody(teTLV(2, endpoint))
	count, color := countAttrCode(attrSection(body), uint8(attribute.AttrExtCommunity))
	require.Equal(t, 1, count)
	require.Len(t, color, 8)
	color[2], color[3] = 0, 0
	for _, rebuild := range []bool{false, true} {
		t.Run(strconv.FormatBool(rebuild), func(t *testing.T) {
			// RFC 9012 Section 4.3.
			attrs := teForwardedAttrs(t, body, rebuild)
			count, sent := countAttrCode(attrs, uint8(attribute.AttrExtCommunity))
			require.Equal(t, 1, count)
			require.Equal(t, []byte{0x03, 0x0b, 0, 0, 0, 0, 0, 42}, sent)
		})
	}
}

// TestRFC9830ReservedFieldsIgnoredOnReceipt checks BGP receipt and propagation
// without claiming the absent SRPM interprets BSIDs, weights or segments.
// Method: both SAFI 73 AFIs, a zero control, a combined dirty control and each
// changed octet in isolation, through validation, receive dispatch and export to
// a downstream consumer. Only ignored bits vary: S/I/B of SRv6 Binding SID,
// V/B of Type B and Type A TC/TTL retain their meaningful values.
// The transmission tests are in plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go.
//
// RFC requirement: RFC9830-2.4.1-5 positive -- Preference Flags 0xFF: no RFC 7606 action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.1-5 negative -- the verdict equals the zero-Flags run, so the field is not acted on.
// RFC requirement: RFC9830-2.4.1-7 positive -- Preference RESERVED 0xFF: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.1-7 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.2-8 positive -- Binding SID RESERVED 0xFF: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.2-8 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.2-10 positive -- Binding SID TC, S and TTL all ones: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.2-10 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.3-5 positive -- SRv6 Binding SID unassigned Flags bits set: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.3-5 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.3-7 positive -- SRv6 Binding SID RESERVED 0xFF: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.3-7 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.4-5 positive -- Segment List RESERVED 0xFF: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.4-5 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.4.1-5 positive -- Weight Flags 0xFF: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.4.1-5 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.4.1-7 positive -- Weight RESERVED 0xFF: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.4.1-7 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.4.2.1-3 positive -- Type A RESERVED 0xFF: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.4.2.1-3 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.4.2.1-5 positive -- Type A label stack entry S bit set: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.4.2.1-5 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.4.2.2-3 positive -- Type B RESERVED 0xFF: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.4.2.2-3 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.4.2.3-2 positive -- unassigned Segment Flags bits set on Type A and Type B: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.4.2.3-2 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.4.2.3-3 positive -- a B-Flag on a Type A segment: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.4.2.3-3 negative -- the verdict equals the run without it.
// RFC requirement: RFC9830-2.4.4.2.4-3 positive -- SRv6 Endpoint Behavior and SID Structure Reserved octets 0xFFFF: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.4.2.4-3 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.6-6 positive -- Priority RESERVED 0xFF: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.6-6 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.7-7 positive -- Candidate Path Name RESERVED 0xFF: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.7-7 negative -- the verdict equals the zero run.
// RFC requirement: RFC9830-2.4.8-7 positive -- Policy Name RESERVED 0xFF: no action, forwarded unchanged.
// RFC requirement: RFC9830-2.4.8-7 negative -- the verdict equals the zero run.
func TestRFC9830ReservedFieldsIgnoredOnReceipt(t *testing.T) {
	// RFC 9830 Sections 2.2 and 2.4.
	clean, dirty := teSRPolicyValue(0, false), teSRPolicyValue(0xFF, false)
	require.Len(t, dirty, len(clean))
	values := [][]byte{clean, dirty}
	names := []string{"clean", "all-ignored-fields"}
	for offset := range clean {
		if clean[offset] != dirty[offset] {
			value := append([]byte{}, clean...)
			value[offset] = dirty[offset]
			values = append(values, value)
			names = append(names, "ignored-octet-"+strconv.Itoa(offset))
		}
	}
	// RFC 9830 Sections 2.4.3 and 2.4.4.2.4: assigned S/I/B stay fixed while
	// only unassigned flags or one endpoint-behavior reserved octet varies.
	for _, tc := range []struct {
		name                        string
		flags, reserved0, reserved1 byte
	}{
		{"clean", 0xE0, 0, 0},
		{"unassigned-flags", 0xFF, 0, 0},
		{"reserved-0", 0xE0, 0xFF, 0},
		{"reserved-1", 0xE0, 0, 0xFF},
	} {
		bsid := append([]byte{tc.flags, 0}, teSRv6SID...)
		bsid = append(bsid, 0xFF, 0xFF, tc.reserved0, tc.reserved1, 32, 16, 16, 64)
		values = append(values, teTLV(15, teSub(12, 0, 0, 0, 0, 0, 100), teSub(20, bsid...)))
		names = append(names, "srv6-bsid-structure-"+tc.name)
	}
	for _, afi := range []byte{1, 2} {
		t.Run("afi-"+strconv.Itoa(int(afi)), func(t *testing.T) {
			var verdicts []message.RFC7606Action
			for index, value := range values {
				t.Run(names[index], func(t *testing.T) {
					// RFC 9830 Sections 2.1 and 4.2.1.
					body := teSRPolicyBody(value, afi)
					s := teValidationSession()
					// RFC 9830 Sections 2.2 and 2.4.
					wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
					require.NoError(t, err)
					require.Equal(t, message.RFC7606ActionNone, action, "ignored SR Policy fields changed the receive action")
					verdicts = append(verdicts, action)
					// RFC 9830 Sections 2.1, 2.2 and 2.4.1.
					teRequireSRPolicyReceipt(t, body, wu.Payload(), value)
					for _, rebuild := range []bool{false, true} {
						// RFC 9830 Section 4.2.3.
						attrs := teForwardedAttrs(t, wu.Payload(), rebuild)
						count, sent := countAttrCode(attrs, uint8(attribute.AttrTunnelEncap))
						require.Equal(t, 1, count)
						require.Equal(t, value, sent)
						_, _, expectedMP, found := attribute.AttrFind(rfc8669PathAttrs(t, body), attribute.AttrMPReachNLRI)
						require.True(t, found)
						_, _, sentMP, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
						require.True(t, found)
						require.Equal(t, expectedMP, sentMP, "forwarding must preserve the SAFI 73 carrier")
					}
				})
			}
			require.Len(t, verdicts, len(values))
			for _, action := range verdicts {
				require.Equal(t, verdicts[0], action)
			}
		})
	}
}

// TestRFC9012EndpointAddressRegistryPropagation observes whole-TLV removal at
// the downstream wire consumer, on both raw and MED-rebuilt forwarding rails.
// RFC 9012 Section 3.1: "The IP address in the sub-TLV's Address subfield lies
// within a block listed in the relevant Special-Purpose IP Address registry
// [RFC6890] with either a \"destination\" attribute value or a \"forwardable\"
// attribute value of \"false\"."
// RFC requirement: RFC9012-13-14 positive -- IANA-prohibited IPv4 and IPv6 endpoint TLVs are removed before original-input receive dispatch and raw/rebuilt downstream propagation.
// RFC requirement: RFC9012-13-14 negative -- valid sibling bytes, private/ULA/benchmark and more-specific allowed endpoints survive; AFI0, unknown AFI and SR Policy semantics remain unchanged.
// MUTATION: accepting every correctly sized endpoint retains the forbidden TLV.
// RFC requirement: RFC9012-13-13 positive -- original-input registry violations remove the whole offending endpoint TLV before raw and rebuilt downstream propagation.
// RFC requirement: RFC9012-13-13 negative -- valid marked siblings and allowed endpoint controls survive original-input receive and export unchanged.
func TestRFC9012EndpointAddressRegistryPropagation(t *testing.T) {
	valid := teTLV(2, teRegistryEndpoint("10.0.0.77"), teSub(99, 7))
	for _, address := range teRegistryForbiddenAddresses {
		t.Run(address, func(t *testing.T) {
			bad := teTLV(8, teRegistryEndpoint(address), teSub(99, 8))
			mixed := append(bytes.Clone(bad), valid...)
			mixed = append(mixed, bad...)
			body := teCarryBody(mixed)
			original := bytes.Clone(body)
			for _, rebuild := range []bool{false, true} {
				// RFC 9012 Sections 3.1 and 13: receive the original input,
				// not a pre-sanitized UPDATE, before forwarding its cache ID.
				attrs := teForwardedAttrs(t, body, rebuild, valid)
				count, sent := countAttrCode(attrs, uint8(attribute.AttrTunnelEncap))
				if count != 1 || !bytes.Equal(sent, valid) {
					t.Fatalf("rebuild=%v: retained tunnel %x, want %x", rebuild, sent, valid)
				}
				if !bytes.Equal(body, original) {
					t.Fatal("receive changed the original diagnostic buffer")
				}
			}
		})
	}
	for _, address := range []string{
		"10.0.0.77", "172.16.0.1", "192.168.0.1", "100.64.0.1", "198.18.0.1",
		"fd00::1", "2001:2::1", "100::1", "64:ff9b:1::1", "5f00::1",
		"192.0.0.1", "192.0.0.9", "192.0.0.10", "2001::1", "2001:1::1",
		"2001:1::2", "2001:1::3", "2001:3::1", "2001:4:112::1",
		"2001:20::1", "2001:30::1", "192.88.99.1", "2001:10::1",
		"8.8.8.8", "2606:4700::1111",
	} {
		t.Run("allowed-"+address, func(t *testing.T) {
			value := teTLV(2, teRegistryEndpoint(address), teSub(99, 7))
			for _, rebuild := range []bool{false, true} {
				// RFC 9012 Sections 3.1 and 13.
				teForwardedAttrs(t, teCarryBody(value), rebuild)
			}
		})
	}
	for _, endpoint := range [][]byte{
		teSub(6, 0, 0, 0, 0, 0, 0),
		teSub(6, 0, 0, 0, 0, 0xff, 0xff, 192, 0, 2, 1),
	} {
		for _, rebuild := range []bool{false, true} {
			// RFC 9012 Section 3.1: AFI0 does not classify NEXT_HOP;
			// an unknown AFI remains opaque even with documentation bytes.
			teForwardedAttrs(t, teCarryBody(teTLV(2, endpoint)), rebuild)
		}
	}
	for _, afi := range []byte{1, 2} {
		value := teTLV(15, teSRPolicyValue(0, false)[4:],
			teRegistryEndpoint("192.0.2.1"), teRegistryEndpoint("2001:db8::1"))
		for _, rebuild := range []bool{false, true} {
			// RFC 9830 Section 2.3: SR Policy ignores endpoint sub-TLVs.
			teForwardedAttrs(t, teSRPolicyBody(value, afi), rebuild)
		}
	}
}

// These samples are independent wire expectations, not the production dataset.
// The fixed IANA XML snapshots own the complete special-purpose prefix list.
var teRegistryForbiddenAddresses = []string{
	"0.0.0.0", "0.1.2.3", "127.0.0.1", "169.254.1.1",
	"192.0.0.8", "192.0.0.11", "192.0.0.170", "192.0.0.171",
	"192.0.2.1", "198.51.100.1", "203.0.113.1", "240.0.0.1", "255.255.255.255",
	"::", "::1", "::ffff:10.0.0.77", "100:0:0:1::1", "2001:1::4",
	"2001:db8::1", "3fff::1", "fe80::1",
}

// teRegistryEndpoint preserves AFI2 mapped IPv4 as sixteen octets.
// RFC 9012 Section 3.1: the address follows Reserved[0:4] and AFI[4:6].
func teRegistryEndpoint(address string) []byte {
	ip := netip.MustParseAddr(address)
	afi := byte(2)
	if ip.Is4() {
		afi = 1
	}
	return teSub(6, append([]byte{0, 0, 0, 0, 0, afi}, ip.AsSlice()...)...)
}
