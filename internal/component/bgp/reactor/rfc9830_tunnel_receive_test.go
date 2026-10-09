// Design: docs/architecture/wire/attributes.md -- carrier-aware tunnel validation.
// Related: rfc9012_tunnel_encap_carry_test.go -- reserved-field and propagation fixtures.
// Related: session_tunnel_encap.go -- receive enforcement under test.
// RFC 9012 Sections 6 and 13 -- see rfc/short/rfc9012.md.
// RFC 9830 Sections 2.1, 2.2 and 2.3 -- see rfc/short/rfc9830.md.

package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// teSRPolicyBody carries a policy in MP_REACH, never in the inline IPv4 NLRI.
// RFC 9830 Section 2.1: "The NLRI containing an SR Policy CP is carried in a BGP
// UPDATE message [RFC4271] using BGP multiprotocol extensions [RFC4760] with an
// AFI of 1 or 2 (IPv4 or IPv6) and with a SAFI of 73."
// MP value offsets: AFI[0:2], SAFI[2], NHLen[3], NH[4:8], reserved[8],
// NLRI bit length[9], distinguisher[10:14], color[14:18], endpoint[18:].
func teSRPolicyBody(value []byte, afi byte) []byte {
	mp := []byte{0, afi, 73, 4, 192, 0, 2, 1, 0, 96, 0, 0, 0, 7, 0, 0, 0, 42}
	if afi == 2 {
		mp[9] = 192
		mp = append(mp, teSRv6SID...)
	} else {
		mp = append(mp, 10, 0, 0, 77)
	}
	attrs := []byte{0x80, byte(attribute.AttrMPReachNLRI), byte(len(mp))}
	attrs = append(attrs, mp...)
	// RFC 9830 Section 4.2.1: "The SR Policy update MUST have either the
	// NO_ADVERTISE community, at least one Route Target extended community
	// in IPv4-address format, or both."
	// IPv4 RT 192.0.2.2:0 permits propagation; NO_ADVERTISE would forbid it.
	attrs = append(attrs,
		0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xea,
		0xC0, byte(attribute.AttrExtCommunity), 8, 1, 2, 192, 0, 2, 2, 0, 0,
		0xD0, byte(attribute.AttrTunnelEncap), byte(len(value)>>8), byte(len(value)))
	attrs = append(attrs, value...)
	return makeUpdateBody(nil, attrs, nil)
}

// teValidationSession matches the four-octet EBGP path in the tunnel fixtures.
func teValidationSession() *Session {
	s := rfc7311EBGPSession()
	s.negotiated = &capability.Negotiated{ASN4: true}
	return s
}

// teRequireSRPolicyReceipt reads the accepted carrier and the applicable value,
// so opaque propagation on an unrelated family cannot satisfy the receipt oracle.
// RFC 9830 Section 2.2: "The use of the SR Policy Tunnel Type is applicable only
// for the AFI/SAFI pairs of (1/73, 2/73).".
func teRequireSRPolicyReceipt(t *testing.T, before, after, value []byte) {
	t.Helper()
	beforeAttrs := rfc8669PathAttrs(t, before)
	afterAttrs := rfc8669PathAttrs(t, after)
	_, _, wantMP, found := attribute.AttrFind(beforeAttrs, attribute.AttrMPReachNLRI)
	require.True(t, found)
	_, _, gotMP, found := attribute.AttrFind(afterAttrs, attribute.AttrMPReachNLRI)
	require.True(t, found)
	require.Equal(t, wantMP, gotMP, "receive must keep the policy NLRI and its next hop")
	require.Equal(t, byte(73), gotMP[2])
	count, got := countAttrCode(afterAttrs, uint8(attribute.AttrTunnelEncap))
	require.Equal(t, 1, count)
	require.Equal(t, value, got)
	// RFC 9830 Sections 2.2 and 2.4.1.
	parsed, err := attribute.ParseTunnelEncap(got)
	require.NoError(t, err)
	require.Len(t, parsed.TLVs, 1)
	require.Equal(t, uint16(15), parsed.TLVs[0].TunnelType)
	// RFC 9830 Section 2.4.1. Applicable information remains readable.
	preference, found := parsed.TLVs[0].Preference()
	require.True(t, found)
	require.Equal(t, uint32(100), preference)
}

// TestTunnelAnnouncementUnaffectedByPolicyWithdrawal receives a GRE announcement
// beside an unrelated policy withdrawal through processMessage. The consumer
// must receive the announcement's tunnel attribute and the distinct withdrawal,
// rather than synthesized withdrawals for both routes.
func TestTunnelAnnouncementUnaffectedByPolicyWithdrawal(t *testing.T) {
	unicast := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	policy := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}
	s := prefixReceiveSession(t, unicast, PrefixCountInstalled, 100, false, policy)
	// Connection establishment supplies this interface snapshot in production.
	s.nextHopScope.Store(&receiveNextHopScope{
		addresses: []netip.Prefix{netip.MustParsePrefix("192.0.2.2/24")},
		local:     netip.MustParseAddr("192.0.2.2"),
		remote:    s.settings.Address,
		direct:    true,
	})
	value := teTLV(2, teSub(6, 0, 0, 0, 0, 0, 1, 10, 0, 0, 77))
	attrs := rfc8669PathAttrs(t, teCarryBody(value))
	attrs = append(attrs, 0x40, 5, 4, 0, 0, 0, 100) // Mandatory iBGP LOCAL_PREF.
	withdrawn := []byte{0, 1, 73, 96, 0, 0, 0, 7, 0, 0, 0, 42, 10, 0, 0, 88}
	attrs = append(attrs, 0x80, byte(attribute.AttrMPUnreachNLRI), byte(len(withdrawn)))
	attrs = append(attrs, withdrawn...)
	announced := []byte{24, 10, 30, 0}
	body := makeUpdateBody(nil, attrs, announced)
	calls := 0
	s.onMessageReceived = func(_ netip.Addr, typ msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		calls++
		require.Equal(t, msgtype.TypeUPDATE, typ)
		delivered := wu.Payload()
		require.Equal(t, uint16(0), binary.BigEndian.Uint16(delivered[:2]),
			"the unicast announcement must not become a legacy withdrawal")
		deliveredAttrs := rfc8669PathAttrs(t, delivered)
		_, _, tunnel, found := attribute.AttrFind(deliveredAttrs, attribute.AttrTunnelEncap)
		require.True(t, found, "the announcement must retain its tunnel attribute")
		require.Equal(t, value, tunnel)
		_, _, withdrawal, found := attribute.AttrFind(deliveredAttrs, attribute.AttrMPUnreachNLRI)
		require.True(t, found, "the unrelated policy withdrawal must remain distinct")
		require.Equal(t, withdrawn, withdrawal)
		require.Equal(t, announced, delivered[4+len(deliveredAttrs):])
		return false
	}
	header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(body))}
	err, kept := s.processMessage(&header, body, BufHandle{ID: noPoolBufID, Buf: body})
	require.NoError(t, err)
	require.False(t, kept)
	require.Equal(t, 1, calls, "one intact mixed UPDATE must reach the consumer")
}

// TestRFC9830InapplicableSubTLVsIgnoredOnReceipt compares policies with and
// without inapplicable sub-TLVs on both actual SAFI 73 carriers. Duplicated and
// malformed-length endpoints are deliberately invalid for RFC 9012 unicast,
// making the accepted verdict discriminate the SR Policy ignore rule.
// RFC 9830 Section 2.3: "If these sub-TLVs are present, a BGP speaker MUST ignore
// them and MAY remove them from the Tunnel Encapsulation Attribute during propagation."
// RFC requirement: RFC9830-2.3-1 positive -- endpoint and color sub-TLVs on AFI 1/73 and 2/73 leave the receive verdict, policy NLRI and Preference unchanged.
// RFC requirement: RFC9830-2.3-1 negative -- duplicate and malformed-length endpoint sub-TLVs do not invoke the unicast removal rule on the SR Policy carrier.
// RFC requirement: RFC9830-2.3-3 positive -- well-framed VXLAN, Embedded Label Handling and UDP-port sub-TLVs leave the received SAFI 73 policy intact and reach the peer unchanged.
// RFC requirement: RFC9830-2.3-3 negative -- adding those inapplicable sub-TLVs changes neither the no-action verdict nor the MP_REACH bytes during unchanged or policy-rebuilt cached forwarding.
// MUTATION: applying the RFC 9012 endpoint count rule to SAFI 73 withdraws the dirty policy.
func TestRFC9830InapplicableSubTLVsIgnoredOnReceipt(t *testing.T) {
	clean := teSRPolicyValue(0, false)
	endpoint := teSub(6, 0, 0, 0, 0, 0, 1, 10, 0, 0, 77)
	cases := []struct {
		name  string
		value []byte
	}{
		{"clean", clean},
		{"one-endpoint", teTLV(15, clean[4:], endpoint)},
		{"duplicate-endpoints", teTLV(15, clean[4:], endpoint, endpoint)},
		{"short-endpoint", teTLV(15, clean[4:], teSub(6, 0))},
		{"color", teTLV(15, clean[4:], teSub(4, 0x03, 0x0b, 0, 0, 0, 0, 0, 5))},
		{"embedded-label", teTLV(15, clean[4:], teSub(9, 1))},
		{"udp-port", teTLV(15, clean[4:], teSub(8, 0x12, 0x34))},
		{"vxlan", teTLV(15, clean[4:], teSub(1, 0xC0, 0, 0, 10, 0, 0, 0, 0, 0, 0, 0, 0))},
		{"all-inapplicable", teTLV(15, teSRPolicyValue(0, true)[4:], teSub(6, 0))},
	}
	for _, afi := range []byte{1, 2} {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				// RFC 9830 Sections 2.2 and 2.3.
				value := tc.value
				// RFC 9830 Section 2.1.
				body := teSRPolicyBody(value, afi)
				s := teValidationSession()
				// RFC 9830 Sections 2.2 and 2.3.
				wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
				require.NoError(t, err)
				require.Equal(t, message.RFC7606ActionNone, action)
				// RFC 9830 Sections 2.1, 2.2 and 2.4.1.
				teRequireSRPolicyReceipt(t, body, wu.Payload(), value)
				for _, rebuild := range []bool{false, true} {
					attrs := teForwardedAttrs(t, wu.Payload(), rebuild)
					count, sent := countAttrCode(attrs, uint8(attribute.AttrTunnelEncap))
					require.Equal(t, 1, count)
					require.Equal(t, value, sent)
					_, _, sentMP, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
					require.True(t, found)
					_, _, wantMP, found := attribute.AttrFind(rfc8669PathAttrs(t, body), attribute.AttrMPReachNLRI)
					require.True(t, found)
					require.Equal(t, wantMP, sentMP)
				}
			})
		}
	}
}

// TestRFC9830TunnelTypeReceiveVerdicts pairs accepted policies with forbidden
// tunnel types and repeated SR Policy TLVs, using both SAFI 73 address families.
// RFC 9830 Section 2.2: "A Tunnel Encapsulation Attribute MUST NOT contain more
// than one TLV of type "SR Policy"; such updates MUST be considered malformed
// and handled by the "treat-as-withdraw" strategy [RFC7606]."
// RFC requirement: RFC9830-2.2-1 positive -- a single type 15 TLV is accepted on AFI 1/73 and 2/73.
// RFC requirement: RFC9830-2.2-1 negative -- VXLAN and unknown tunnel types, alone or beside type 15, cause exactly treat-as-withdraw.
// RFC requirement: RFC9830-2.2-3 positive -- one type 15 TLV has no RFC 7606 action.
// RFC requirement: RFC9830-2.2-3 negative -- two type 15 TLVs cause exactly treat-as-withdraw, not acceptance or session reset.
// MUTATION: removing applyTunnelEncap accepts the wrong-type and duplicate cases.
func TestRFC9830TunnelTypeReceiveVerdicts(t *testing.T) {
	// RFC 9830 Section 2.2.
	policy := teSRPolicyValue(0, false)
	for _, afi := range []byte{1, 2} {
		for _, tc := range []struct {
			name  string
			value []byte
			want  message.RFC7606Action
		}{
			{"single-policy", policy, message.RFC7606ActionNone},
			{"vxlan", teTLV(8, teSub(6, 0, 0, 0, 0, 0, 0)), message.RFC7606ActionTreatAsWithdraw},
			{"unknown", teTLV(0xFFFE, teSub(99, 1)), message.RFC7606ActionTreatAsWithdraw},
			{"policy-and-vxlan", append(append([]byte{}, policy...), teTLV(8, teSub(99, 1))...), message.RFC7606ActionTreatAsWithdraw},
			{"duplicate-policy", append(append([]byte{}, policy...), policy...), message.RFC7606ActionTreatAsWithdraw},
		} {
			t.Run(tc.name, func(t *testing.T) {
				// RFC 9830 Sections 2.1 and 2.2.
				body := teSRPolicyBody(tc.value, afi)
				s := teValidationSession()
				// RFC 9830 Section 2.2.
				wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
				require.NoError(t, err, "treat-as-withdraw must not reset the session")
				require.Equal(t, tc.want, action)
				require.Equal(t, body, wu.Payload(), "withdrawal synthesis needs the original policy NLRI")
			})
		}
	}
}

// TestRFC9012EndpointInvalidTLVsRemovedOnUnicast checks the Section 13 exception
// rather than requiring an impossible endpoint-invalid TLV to survive. A valid
// sibling stays, while no surviving TLV causes exactly treat-as-withdraw.
// RFC 9012 Section 13: "Within a Tunnel Encapsulation attribute that is carried
// by a BGP UPDATE whose AFI/SAFI is one of those explicitly listed in the first
// paragraph of Section 6, a TLV that does not contain exactly one Tunnel Egress
// Endpoint sub-TLV MUST be treated as if it contained a malformed Tunnel Egress
// Endpoint sub-TLV."
// RFC requirement: RFC9012-13-15 positive -- a unicast TLV with one well-framed endpoint survives unchanged.
// RFC requirement: RFC9012-13-15 negative -- missing and duplicate endpoints remove only the offending TLV, or cause treat-as-withdraw if none survive.
// RFC requirement: RFC9012-13-14 positive -- malformed-length endpoint TLVs are removed before raw or rebuilt forwarding while a valid sibling remains.
// RFC requirement: RFC9012-13-14 negative -- valid endpoint TLVs and unrelated route bytes are not removed with a malformed sibling.
// MUTATION: bypassing applyTunnelEncap leaves the invalid TLVs in the socket output.
// RFC requirement: RFC9012-3.1-4 negative -- short-ipv4 removes its entire TLV around a valid sibling, or selects exact TreatAsWithdraw without a survivor.
// RFC requirement: RFC9012-3.1-5 negative -- short-ipv6 removes its entire TLV around a valid sibling, or selects exact TreatAsWithdraw without a survivor.
// RFC requirement: RFC9012-3.1-7 negative -- long-next-hop removes its entire AFI-zero TLV around a valid sibling, or selects exact TreatAsWithdraw without a survivor.
// RFC requirement: RFC9012-13-13 positive -- structurally malformed endpoints cause whole-TLV removal rather than sub-TLV-only removal.
// RFC requirement: RFC9012-13-13 negative -- the valid sibling TLV survives structural endpoint removal and remains in downstream output.
func TestRFC9012EndpointInvalidTLVsRemovedOnUnicast(t *testing.T) {
	endpoint := teSub(6, 0xFF, 0xFF, 0xFF, 0xFF, 0, 1, 10, 0, 0, 77)
	valid := teTLV(2, endpoint, teSub(9, 1))
	for _, tc := range []struct {
		name string
		bad  []byte
	}{
		{"missing", teTLV(8, teSub(1, 0, 0, 0, 10, 0, 0, 0, 0, 0, 0, 0, 0))},
		{"duplicate", teTLV(2, endpoint, endpoint)},
		{"short-endpoint", teTLV(2, teSub(6, 0))},
		{"short-ipv4", teTLV(2, teSub(6, 0, 0, 0, 0, 0, 1, 10, 0, 0))},
		{"short-ipv6", teTLV(2, teSub(6, 0, 0, 0, 0, 0, 2, 0))},
		{"long-next-hop", teTLV(2, teSub(6, 0, 0, 0, 0, 0, 0, 0))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, sibling := range []bool{false, true} {
				value := append([]byte{}, tc.bad...)
				want := message.RFC7606ActionTreatAsWithdraw
				if sibling {
					value = append(value, valid...)
					value = append(value, tc.bad...)
					want = message.RFC7606ActionNone
				}
				body := teCarryBody(value)
				original := append([]byte{}, body...)
				input := wireu.NewWireUpdate(body, 0)
				input.SetSourceID(99)
				s := teValidationSession()
				// RFC 9012 Section 13.
				wu, action, err := s.enforceRFC7606(input)
				require.NoError(t, err)
				require.Equal(t, want, action)
				require.Equal(t, original, input.Payload(), "diagnostic source bytes stay immutable")
				require.Equal(t, input.SourceID(), wu.SourceID())
				require.Equal(t, input.SourceCtxID(), wu.SourceCtxID())
				if sibling {
					require.Equal(t, teCarryBody(valid), wu.Payload(), "only the offending TLVs are removed")
					for _, rebuild := range []bool{false, true} {
						attrs := teForwardedAttrs(t, wu.Payload(), rebuild)
						count, sent := countAttrCode(attrs, uint8(attribute.AttrTunnelEncap))
						require.Equal(t, 1, count)
						require.Equal(t, valid, sent)
					}
				}
			}
		})
	}
}

// TestRFC9012TunnelFramingReceiveVerdicts ensures ignored sub-TLV content does
// not imply blanket acceptance of unparseable TLV boundaries.
// RFC 9012 Section 13: "The final octet of a TLV MUST also be the final octet of
// its final sub-TLV. If this is not the case, the TLV MUST be considered to be
// malformed, and the "Treat-as-withdraw" procedure of [RFC7606] is applied."
// RFC requirement: RFC9012-13-1 positive -- a sub-TLV sequence ending exactly at the tunnel boundary is accepted.
// RFC requirement: RFC9012-13-1 negative -- a sub-TLV value crossing the tunnel boundary causes treat-as-withdraw.
// RFC requirement: RFC9012-13-2 positive -- truncated outer headers, outer values, sub-TLV headers and sub-TLV values cause exactly treat-as-withdraw.
// RFC requirement: RFC9012-13-2 negative -- a framed unknown sub-TLV remains accepted beside a valid endpoint.
// MUTATION: omitting the sub-TLV boundary check accepts truncated values beside a valid endpoint.
func TestRFC9012TunnelFramingReceiveVerdicts(t *testing.T) {
	// A valid endpoint prevents an unrelated missing-endpoint rejection.
	endpoint := teSub(6, 0, 0, 0, 0, 0, 0)
	for _, tc := range []struct {
		name  string
		value []byte
		want  message.RFC7606Action
	}{
		{"framed-unknown", teTLV(2, endpoint, teSub(99, 1)), message.RFC7606ActionNone},
		{"empty-attribute", nil, message.RFC7606ActionTreatAsWithdraw},
		{"short-tunnel-header", []byte{0, 2, 0}, message.RFC7606ActionTreatAsWithdraw},
		{"short-tunnel-value", []byte{0, 2, 0, 8, 6, 6}, message.RFC7606ActionTreatAsWithdraw},
		{"short-sub-header", teTLV(2, endpoint, []byte{99}), message.RFC7606ActionTreatAsWithdraw},
		{"short-long-sub-header", teTLV(2, endpoint, []byte{200, 0}), message.RFC7606ActionTreatAsWithdraw},
		{"short-sub-value", teTLV(2, endpoint, []byte{99, 2, 1}), message.RFC7606ActionTreatAsWithdraw},
		{"short-long-sub-value", teTLV(2, endpoint, []byte{200, 0, 2, 1}), message.RFC7606ActionTreatAsWithdraw},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := teValidationSession()
			// RFC 9012 Section 13.
			_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(teCarryBody(tc.value), 0))
			require.NoError(t, err)
			require.Equal(t, tc.want, action)
		})
	}
}

// TestTunnelEndpointRemovalComposesWithAttributeRepairs checks both attribute
// header sizes with a preceding AIGP discard and following duplicate MED. The
// received source buffer, source identity, NLRI and retained attributes must
// survive the three successive receive rewrites and the socket forwarding rail.
func TestTunnelEndpointRemovalComposesWithAttributeRepairs(t *testing.T) {
	valid := teTLV(2, teSub(6, 0, 0, 0, 0, 0, 0))
	bad := teTLV(8, teSub(99, 1))
	value := append(append([]byte{}, bad...), valid...)
	for _, extended := range []bool{false, true} {
		attrs := []byte{
			0x40, 1, 1, 0,
			0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xea,
			0x40, 3, 4, 192, 0, 2, 1,
		}
		attrs = append(attrs, rfc7311AIGPAttrs(0xC0)[14:]...)
		if extended {
			attrs = append(attrs, 0xD0, byte(attribute.AttrTunnelEncap), 0, byte(len(value)))
		} else {
			attrs = append(attrs, 0xC0, byte(attribute.AttrTunnelEncap), byte(len(value)))
		}
		attrs = append(attrs, value...)
		attrs = append(attrs,
			0x80, byte(attribute.AttrMED), 4, 0, 0, 0, 10,
			0x80, byte(attribute.AttrMED), 4, 0, 0, 0, 20)
		nlri := []byte{24, 10, 30, 0}
		body := makeUpdateBody(nil, attrs, nlri)
		original := append([]byte{}, body...)
		input := wireu.NewWireUpdate(body, 0)
		input.SetSourceID(1234)
		s := teValidationSession()
		// RFC 9012 Section 13, RFC 7606 Section 3(g), RFC 7311 Section 3.2.
		wu, action, err := s.enforceRFC7606(input)
		require.NoError(t, err)
		require.Equal(t, message.RFC7606ActionAttributeDiscard, action)
		require.Equal(t, original, input.Payload())
		require.Equal(t, input.SourceID(), wu.SourceID())
		require.Equal(t, input.SourceCtxID(), wu.SourceCtxID())
		require.Equal(t, nlri, wu.Payload()[len(wu.Payload())-len(nlri):])
		for _, got := range [][]byte{
			rfc8669PathAttrs(t, wu.Payload()),
			teForwardedAttrs(t, wu.Payload(), false),
		} {
			count, tunnel := countAttrCode(got, uint8(attribute.AttrTunnelEncap))
			require.Equal(t, 1, count)
			require.Equal(t, valid, tunnel)
			count, med := countAttrCode(got, uint8(attribute.AttrMED))
			require.Equal(t, 1, count)
			require.Equal(t, []byte{0, 0, 0, 10}, med)
			count, _ = countAttrCode(got, uint8(attribute.AttrAIGP))
			require.Zero(t, count)
			count, marker := countAttrCode(got, uint8(attribute.AttrTombstone))
			require.Equal(t, 1, count)
			require.GreaterOrEqual(t, len(marker), 2)
			require.Equal(t, byte(attribute.AttrAIGP), marker[0])
		}
	}
}

// TestTunnelEndpointLengthsAccepted checks each recognized endpoint length and
// an unknown address family, preventing length checks from rejecting all TLVs.
// RFC 9012 Section 3.1: "In the context of this specification, if the Address
// Family subfield has any value other than IPv4, IPv6, or the special value 0,
// the Tunnel Egress Endpoint sub-TLV is considered "unrecognized" (see Section 13).".
// RFC requirement: RFC9012-3.1-4 positive -- an AFI-1 endpoint with exactly four address octets is accepted unchanged.
// RFC requirement: RFC9012-3.1-5 positive -- an AFI-2 endpoint with exactly sixteen address octets is accepted unchanged.
// RFC requirement: RFC9012-3.1-7 positive -- AFI zero with exactly six value octets and no Address subfield is accepted unchanged.
func TestTunnelEndpointLengthsAccepted(t *testing.T) {
	for _, endpoint := range [][]byte{
		{0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 1, 10, 0, 0, 77},
		{0, 0, 0, 0, 0, 2, 0xfd, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		{0, 0, 0, 0, 0xff, 0xfe, 1},
	} {
		value := teTLV(2, teSub(6, endpoint...))
		body := teCarryBody(value)
		s := teValidationSession()
		// RFC 9012 Sections 3.1 and 13.
		wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
		require.NoError(t, err)
		require.Equal(t, message.RFC7606ActionNone, action)
		require.Equal(t, body, wu.Payload())
	}
}

// TestMalformedTunnelWithoutReachableNLRIResets preserves the existing RFC 7606
// no-reachable-NLRI escalation when the newly enforced tunnel check finds an
// error after the common validator has completed.
// RFC 7606 Section 5.2: "For this reason, if any path attribute errors are
// encountered in such an UPDATE message and if any encountered error specifies
// an error-handling approach other than "attribute discard", then the
// "session reset" approach MUST be used.".
func TestMalformedTunnelWithoutReachableNLRIResets(t *testing.T) {
	s := teValidationSession()
	attrs := []byte{0xC0, byte(attribute.AttrTunnelEncap), 3, 0, 2, 0}
	// RFC 9012 Section 13 and RFC 7606 Section 5.2.
	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(makeUpdateBody(nil, attrs, nil), 0))
	require.Error(t, err)
	require.Equal(t, message.RFC7606ActionSessionReset, action)
}
