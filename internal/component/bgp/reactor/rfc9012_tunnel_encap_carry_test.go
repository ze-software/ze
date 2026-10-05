// Design: docs/architecture/wire/attributes.md -- carrier-aware tunnel attribute validation.
// Related: session_tunnel_encap.go -- receive-side framing and carrier enforcement.
// RFC 9012 and RFC 9830 receive-and-propagate obligations for the Tunnel
// Encapsulation attribute, driven through the path a peer's UPDATE takes: the
// receive validator (enforceRFC7606), then the forwarding rail to the socket.
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
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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
// "SR Policy" type with code point 15."
func teSRPolicyValue(x byte, ignored bool) []byte {
	typeA := teSub(1, 0x7F&x, x, 0x05, 0xDC, 0x0F&x, x)
	typeB := teSub(13, append(append([]byte{0x10, x}, teSRv6SID...), 0xFF, 0xFF, x, x, 32, 16, 16, 64)...)
	segmentList := teLongSub(128, append(append([]byte{x}, teSub(9, x, x, 0, 0, 0, 7)...), append(typeA, typeB...)...)...)
	srPolicy := [][]byte{
		// Preference, Binding SID, SRv6 Binding SID, Priority (RFC 9830
		// Sections 2.4.1, 2.4.2, 2.4.3, 2.4.6).
		teSub(12, x, x, 0, 0, 0, 100),
		teSub(13, x, x, 0x05, 0xDC, 0x0F&x, x),
		teSub(20, append([]byte{x, x}, teSRv6SID...)...),
		teSub(15, 9, x),
		// Candidate Path Name, Policy Name, Segment List (Sections 2.4.7,
		// 2.4.8, 2.4.4).
		teLongSub(129, append([]byte{x}, "primary"...)...),
		teLongSub(130, append([]byte{x}, "alpha"...)...),
		segmentList,
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
			teSub(8, 0x12, 0x34, 0x56),
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
// Endpoint sub-TLV."
func teCarryValue(x byte, odd bool) []byte {
	endpoint := teSub(6, x, x, x, x, 0, 1, 10, 0, 0, 77)
	gre := [][]byte{endpoint, teSub(9, 1)}
	if odd {
		gre = append(gre,
			teSub(9, 2),                // Duplicate Embedded Label Handling.
			teSub(8, 0x12, 0x34, 0x56), // Malformed and meaningless for GRE.
			teLongSub(200, 0xde, 0xad), // Unrecognized sub-TLV.
		)
	}
	value := teTLV(2, gre...)
	vxlan := teSub(1, 0xC0|(0x3F&x), 0, 0, 10, 0, 0, 0, 0, 0, 0, x, x)
	value = append(value, teTLV(8, endpoint, vxlan)...)
	if odd {
		value = append(value, teTLV(0xFFFE, endpoint, teSub(99, 1, 2, 3))...)
	}
	return value
}

// teColorExtComm is a Color Extended Community (RFC 9012 Section 4.3), color 42.
var teColorExtComm = []byte{0x03, 0x0b, 0, 0, 0, 0, 0, 42}

func teCarryBody(value []byte) []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 192, 0, 2, 1, // NEXT_HOP
		0xC0, byte(attribute.AttrExtCommunity), byte(len(teColorExtComm)),
	}
	attrs = append(attrs, teColorExtComm...)
	attrs = append(attrs, 0xD0, byte(attribute.AttrTunnelEncap), byte(len(value)>>8), byte(len(value)))
	attrs = append(attrs, value...)
	return makeUpdateBody(nil, attrs, []byte{24, 10, 30, 0})
}

// teForwardedAttrs forwards body to a fresh peer, optionally rebuilt with
// next-hop-self and a new MED, and returns the path attributes on the wire.
func teForwardedAttrs(t *testing.T, body []byte, rebuild bool) []byte {
	t.Helper()
	out := body
	if rebuild {
		var mods filterapi.ModAccumulator
		mods.Op(uint8(attribute.AttrNextHop), filterapi.AttrModSet, []byte{192, 0, 2, 9})
		mods.Op(uint8(attribute.AttrMED), filterapi.AttrModSet, []byte{0, 0, 0, 50})
		rebuilt, _, failure := buildModifiedPayload(body, &mods, attrModHandlersWithDefaults(), nil, nil)
		require.Equal(t, modifyFailureNone, failure)
		require.NotNil(t, rebuilt)
		out = rebuilt
	}
	peer, conn := newAnnouncePeer(t, "192.0.2.2")
	fwdBatchHandler(fwdKey{}, []fwdItem{{peer: peer, rawBodies: [][]byte{out}, sourceMessageID: 1}})
	sent := conn.written()
	require.Greater(t, len(sent), message.HeaderLen, "the route must reach the socket")
	return rfc8669PathAttrs(t, sent[message.HeaderLen:])
}

// TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong drives a received
// Tunnel Encapsulation attribute through enforceRFC7606 and then to a peer's
// socket, both forwarded as received and rebuilt for next-hop-self with a new
// MED, and compares the attribute octets at every step.
// Method: both IPv4-unicast variants have exactly one endpoint in every TLV.
// The dirty variant sets ignored reserved bits and adds unknown and malformed
// sub-TLVs, a UDP port meaningless for GRE, a duplicate Embedded Label Handling
// sub-TLV, and an unknown tunnel type. Each verdict must be no action, and both
// forwarding rails must preserve every octet. Endpoint-invalid TLVs are tested
// separately because Section 13 requires their removal on this carrier.
//
// RFC requirement: RFC9012-13-3 positive -- an attribute holding a TLV of unrecognized tunnel type 0xFFFE gets no RFC 7606 action and is kept.
// RFC requirement: RFC9012-13-3 negative -- the same attribute re-encoded for next-hop-self is still carried whole, the unrecognized TLV included, never dropped as malformed.
// RFC requirement: RFC9012-13-5 positive -- the unrecognized-type TLV reaches the peer's wire unchanged when the route is forwarded as received.
// RFC requirement: RFC9012-13-5 negative -- rebuilding the UPDATE for next-hop-self and a new MED does not drop or alter it.
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
// RFC requirement: RFC9012-13-16 positive -- a UDP Destination Port sub-TLV inside a GRE TLV changes neither verdict nor handling.
// RFC requirement: RFC9012-13-16 negative -- the dirty and clean runs reach the same verdict.
// RFC requirement: RFC9012-13-18 positive -- the TLV holding the meaningless sub-TLV gets no RFC 7606 action.
// RFC requirement: RFC9012-13-18 negative -- the rebuilt UPDATE still carries it.
// RFC requirement: RFC9012-13-19 positive -- the meaningless sub-TLV reaches the wire unchanged.
// RFC requirement: RFC9012-13-19 negative -- the rebuilt UPDATE still carries it.
// RFC requirement: RFC9012-3.1-3 positive -- non-zero Tunnel Egress Endpoint Reserved octets reach the wire unchanged.
// RFC requirement: RFC9012-3.1-3 negative -- the rebuilt UPDATE does not zero them.
// RFC requirement: RFC9012-3.2.1-2 positive -- the VXLAN R bits reach the wire unchanged.
// RFC requirement: RFC9012-3.2.1-2 negative -- the rebuilt UPDATE does not mask them.
// RFC requirement: RFC9012-3.5-3 positive -- the Embedded Label Handling sub-TLV reaches the wire.
// RFC requirement: RFC9012-3.5-3 negative -- the rebuilt UPDATE does not strip it.
// RFC requirement: RFC9012-4.3-2 positive -- the Color Extended Community value reaches the wire unchanged.
// RFC requirement: RFC9012-4.3-2 negative -- the rebuilt UPDATE does not change it.
func TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong(t *testing.T) {
	var verdicts []message.RFC7606Action
	for _, tc := range []struct {
		name string
		x    byte
		odd  bool
	}{
		{"clean", 0x00, false},
		{"dirty", 0xFF, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// RFC 9012 Sections 6 and 13.
			value := teCarryValue(tc.x, tc.odd)
			s := rfc7311EBGPSession()
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
	require.Len(t, verdicts, 2)
	require.Equal(t, verdicts[0], verdicts[1], "fields to ignore and sub-TLVs to tolerate must not change the verdict")
}

// TestRFC9830ReservedFieldsIgnoredOnReceipt proves the receipt half of every
// SR Policy "MUST be ignored on receipt" field: Ze reaches the same verdict for
// an attribute whose Flags, RESERVED and reserved bits are all ones as for the
// one where they are zero, and passes the ones along untouched rather than
// zeroing them. The transmission half of each row is proven in
// internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go.
// Method: AFI 1/SAFI 73 and AFI 2/SAFI 73 UPDATEs, through enforceRFC7606 and
// the forwarding rail, with exact MP_REACH and Preference readback. A malformed
// carrier control is covered by TestRFC9830TunnelTypeReceiveVerdicts.
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
// RFC requirement: RFC9830-2.4.4.2.3-2 positive -- unassigned Segment Flags bits set on Type A: no action, forwarded unchanged.
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
	for _, afi := range []byte{1, 2} {
		var verdicts []message.RFC7606Action
		for _, x := range []byte{0x00, 0xFF} {
			// RFC 9830 Sections 2.2 and 2.4.
			value := teSRPolicyValue(x, false)
			// RFC 9830 Section 2.1.
			body := teSRPolicyBody(value, afi)
			s := rfc7311EBGPSession()
			// RFC 9830 Sections 2.2 and 2.4.
			wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
			require.NoError(t, err)
			require.Equal(t, message.RFC7606ActionNone, action)
			verdicts = append(verdicts, action)
			// RFC 9830 Sections 2.1, 2.2 and 2.4.1.
			teRequireSRPolicyReceipt(t, body, wu.Payload(), value)
			for _, rebuild := range []bool{false, true} {
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
		}
		require.Equal(t, verdicts[0], verdicts[1])
	}
}
