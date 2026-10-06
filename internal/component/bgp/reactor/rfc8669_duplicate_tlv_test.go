// Design: docs/architecture/wire/attributes.md -- BGP_PREFIX_SID (Code 40), the repeated-TLV discard
// Related: rfc8669_duplicate_tlv.go -- discardRepeatedPrefixSIDTLVs, the ingest step under test
// Related: session_validation.go -- publishBase, the receive step every UPDATE leaves through

package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// srv6L3ServiceTLV is one SRv6 L3 Service TLV (Prefix-SID TLV type 5): Reserved,
// then an SRv6 SID Information Sub-TLV (type 1, length 21) holding the SID,
// Service SID Flags 00, Endpoint Behavior 0x0013 (End.DT4) and Reserved.
func srv6L3ServiceTLV(sid string) string {
	return "05" + "0019" + "00" + "01" + "0015" + "00" + sid + "00" + "0013" + "00"
}

// srv6L2ServiceTLV is the same TLV under type 6, the SRv6 L2 Service TLV.
func srv6L2ServiceTLV(sid string) string {
	return "06" + srv6L3ServiceTLV(sid)[2:]
}

// labelIndexTLV is one Label-Index TLV (type 1, length 7): Reserved, Flags
// 0000, then the 4-octet label index given in hex.
func labelIndexTLV(index string) string {
	return "01" + "0007" + "00" + "0000" + index
}

// originatorSRGBTLV is one Originator SRGB TLV (type 3, length 8): Flags 0000,
// then one SRGB range of base 800000 (0x0c3500) and size 4096 (0x001000).
const originatorSRGBTLV = "03" + "0008" + "0000" + "0c3500" + "001000"

// unknownTLV is a TLV of type 9, which no specification Ze knows defines.
const unknownTLV = "09" + "0002" + "aabb"

const (
	firstSID  = "20010db8000000010000000000000000"
	secondSID = "20010db8000000020000000000000000"
)

// rfc8669DuplicateSettings is an internal session, so RFC 8669 Section 4 (an
// EBGP receiver discards the whole attribute) cannot hide the TLV-level rule.
func rfc8669DuplicateSettings(rsClient bool) *PeerSettings {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020301)
	settings.ReceiveHoldTime = 90 * time.Second
	settings.RSClient = rsClient
	return settings
}

// rfc8669PrefixSIDAttr is the Prefix-SID attribute holding the TLVs in
// prefixSID (hex), under the one-octet length header when the value fits it
// and the Extended Length header (flags 0xD0) when extended is set.
func rfc8669PrefixSIDAttr(prefixSID string, extended bool) string {
	octets := len(prefixSID) / 2
	if extended {
		return "d028" + hex.EncodeToString([]byte{byte(octets >> 8), byte(octets)}) + prefixSID
	}
	return "c028" + hex.EncodeToString([]byte{byte(octets)}) + prefixSID
}

// safiUnicast is the SAFI of IPv4 unicast in an MP_REACH_NLRI fixture.
const safiUnicast = "01"

// communityAttr is a COMMUNITIES attribute (code 8) holding 65001:1, the
// attribute the tests place after code 40 to see the tail survive the rebuild.
const communityAttr = "c008" + "04" + "fde90001"

// rfc8669DuplicateReceive builds an IPv4 UPDATE of SAFI safi (labeled unicast
// or unicast, its routes in MP_REACH_NLRI) whose attributes end in attribute40
// then after (both hex), and drives it through enforceRFC7606, the real
// receive entry, which ends in publishBase. It returns the received body and
// what enforceRFC7606 returned.
func rfc8669DuplicateReceive(t *testing.T, safi, attribute40, after string, rsClient bool) ([]byte, *wireu.WireUpdate, message.RFC7606Action, error) {
	t.Helper()
	nlri := "18" + "0a0102"
	if safi == safiLabeled {
		nlri = "30" + "000641" + "0a0102"
	}
	// The same body subsequently enters an ASN4 EBGP session from AS 65002.
	// Keep its path and on-link next hop valid beyond the RFC 7606-only step.
	mp := "0001" + safi + "04c0000201" + "00" + nlri
	attrs := "40010100" + "40020602010000fdea" +
		"800e" + hex.EncodeToString([]byte{byte(len(mp) / 2)}) + mp + attribute40 + after
	attrOctets := len(attrs) / 2
	received, err := hex.DecodeString("0000" +
		hex.EncodeToString([]byte{byte(attrOctets >> 8), byte(attrOctets)}) + attrs)
	require.NoError(t, err, "fixture hex")
	original := append([]byte{}, received...)
	s := NewSession(rfc8669DuplicateSettings(rsClient))
	s.negotiated = &capability.Negotiated{ASN4: true}
	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(received, 0))
	return original, wu, action, err
}

// rfc8669PublishedPrefixSID returns the value of the one Prefix-SID attribute
// in the attribute section attrs.
func rfc8669PublishedPrefixSID(t *testing.T, attrs []byte) string {
	t.Helper()
	count, value := countAttrCode(attrs, uint8(attribute.AttrPrefixSID))
	require.Equal(t, 1, count, "exactly one Prefix-SID attribute")
	return hex.EncodeToString(value)
}

// TestRFC8669DuplicateServiceTLVDiscardedOnReceive receives an UPDATE whose
// Prefix-SID repeats a TLV that is single-occurrence on its family and reads
// the attribute Ze publishes and the one it writes to a peer's socket.
// Method: each shape on an internal session and on a route-server client
// session. On labeled unicast: two SRv6 L3 Service TLVs (the second with
// another SID), two SRv6 L2 Service TLVs, two Label-Index TLVs with an unknown
// TLV between them, two L3 Service TLVs then an unknown TLV after the repeat
// (the walk must keep copying after its first discard), two L3 Service TLVs
// with a COMMUNITIES attribute after code 40 (the rebuild must keep the
// attributes behind the Prefix-SID), and two L3 Service TLVs then an unknown
// one under an Extended Length header (the value ends in octets unlike the
// ones before it, so a header width read one short shows). On IPv4 unicast:
// two L3 Service TLVs, which RFC 9252 Section 7 limits on every family. Each
// must get no error and no RFC 7606 action, the published and the forwarded
// attribute must be the received TLVs less every repeat, in received order,
// under the peer's header width, and the published attributes after code 40
// must be the received ones, octet for octet.
//
// RFC requirement: RFC8669-6-3 positive -- a received Prefix-SID repeating the SRv6 L3 Service or SRv6 L2 Service TLV on any family, or the Label-Index TLV on labeled unicast, gets no error and no RFC 7606 action, and the attribute Ze publishes and forwards to a peer keeps the first TLV of that type and every other TLV in received order, with the repeat removed and the attributes after it unchanged.
func TestRFC8669DuplicateServiceTLVDiscardedOnReceive(t *testing.T) {
	l3Twice := srv6L3ServiceTLV(firstSID) + srv6L3ServiceTLV(secondSID)
	tests := []struct {
		name     string
		safi     string
		received string
		want     string
		after    string
		extended bool
	}{
		{"L3 Service twice", safiLabeled, l3Twice, srv6L3ServiceTLV(firstSID), "", false},
		{"L2 Service twice", safiLabeled, srv6L2ServiceTLV(firstSID) + srv6L2ServiceTLV(secondSID), srv6L2ServiceTLV(firstSID), "", false},
		{
			"Label-Index twice, unknown TLV between", safiLabeled,
			labelIndexTLV("00000064") + unknownTLV + labelIndexTLV("000000c8"),
			labelIndexTLV("00000064") + unknownTLV,
			"", false,
		},
		{"L3 Service twice, unknown TLV after", safiLabeled, l3Twice + unknownTLV, srv6L3ServiceTLV(firstSID) + unknownTLV, "", false},
		{"L3 Service twice, attribute after", safiLabeled, l3Twice, srv6L3ServiceTLV(firstSID), communityAttr, false},
		{"L3 Service twice, extended length", safiLabeled, l3Twice + unknownTLV, srv6L3ServiceTLV(firstSID) + unknownTLV, communityAttr, true},
		{"L3 Service twice, IPv4 unicast", safiUnicast, l3Twice, srv6L3ServiceTLV(firstSID), "", false},
	}
	for _, tt := range tests {
		for _, rsClient := range []bool{false, true} {
			name := tt.name
			if rsClient {
				name += ", route-server client"
			}
			t.Run(name, func(t *testing.T) {
				_, wu, action, err := rfc8669DuplicateReceive(t, tt.safi, rfc8669PrefixSIDAttr(tt.received, tt.extended), tt.after, rsClient)
				require.NoError(t, err, "a repeated TLV is discarded, never an error")
				require.Equal(t, message.RFC7606ActionNone, action, "the UPDATE continues to be processed")

				retained := rfc8669PathAttrs(t, wu.Payload())
				require.Equal(t, tt.want, rfc8669PublishedPrefixSID(t, retained),
					"the published Prefix-SID keeps the first TLV of each single-occurrence type")
				_, flags, _, found := attribute.AttrFind(retained, attribute.AttrPrefixSID)
				require.True(t, found)
				require.Equal(t, tt.extended, flags.IsExtLength(), "the attribute keeps the peer's header width")
				wantTail := rfc8669PrefixSIDAttr(tt.want, tt.extended) + tt.after
				require.Equal(t, wantTail, hex.EncodeToString(retained[len(retained)-len(wantTail)/2:]),
					"the rebuilt Prefix-SID is followed by the received attributes, octet for octet")

				sent := teForwardedAttrs(t, wu.Payload(), false)
				require.Equal(t, tt.want, rfc8669PublishedPrefixSID(t, sent),
					"the repeat is discarded before the route is passed along")
			})
		}
	}
}

// TestRFC8669SingleOccurrencePrefixSIDKeptWhole receives Prefix-SID
// attributes that repeat no TLV single-occurrence on their family and checks
// nothing changes.
// Method: on labeled unicast, one of each TLV Ze recognizes (Label-Index,
// Originator SRGB, SRv6 L3 and L2 Service) beside an unknown one, an unknown
// TLV twice, and an Originator SRGB TLV twice, which RFC 8669 does not limit to
// one occurrence. On IPv4 unicast, two Label-Index TLVs with an unknown TLV
// between them: RFC 8669 Section 3.1 has the TLV ignored off labeled unicast,
// so there it is an unrecognized TLV and propagated unmodified. The published
// body must be the received one octet for octet, and the forwarded attribute
// must hold every TLV.
//
// RFC requirement: RFC8669-6-3 negative -- a received Prefix-SID holding each recognized TLV once, an unknown TLV twice, or an Originator SRGB TLV twice on labeled unicast, or a Label-Index TLV twice on IPv4 unicast, is published octet-equal to the received body and forwarded with every TLV, so only a repeat of a type single-occurrence on that family is discarded.
func TestRFC8669SingleOccurrencePrefixSIDKeptWhole(t *testing.T) {
	tests := []struct {
		name     string
		safi     string
		received string
	}{
		{"each type once", safiLabeled, labelIndexTLV("00000064") + originatorSRGBTLV + srv6L3ServiceTLV(firstSID) + srv6L2ServiceTLV(secondSID) + unknownTLV},
		{"unknown TLV twice", safiLabeled, srv6L3ServiceTLV(firstSID) + unknownTLV + unknownTLV},
		{"Originator SRGB twice", safiLabeled, labelIndexTLV("00000064") + originatorSRGBTLV + originatorSRGBTLV},
		{"Label-Index twice, IPv4 unicast", safiUnicast, labelIndexTLV("00000064") + unknownTLV + labelIndexTLV("000000c8")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			received, wu, action, err := rfc8669DuplicateReceive(t, tt.safi, rfc8669PrefixSIDAttr(tt.received, false), "", false)
			require.NoError(t, err)
			require.Equal(t, message.RFC7606ActionNone, action)
			require.Equal(t, received, wu.Payload(), "the published body must be the received one, octet for octet")
			sent := teForwardedAttrs(t, wu.Payload(), false)
			require.Equal(t, tt.received, rfc8669PublishedPrefixSID(t, sent), "every TLV is passed along")
		})
	}
}

// TestRFC8669DuplicateMalformedServiceTLVStillWithdrawn receives two SRv6 L3
// Service TLVs of which the second is malformed.
// Method: the second TLV's SID Information Sub-TLV declares 20 octets, below
// the 21 RFC 9252 Section 3.2 requires. The repeat would be discarded if it
// were well formed, but the RFC 7606 walk validates it first, so the UPDATE
// must still be treat-as-withdraw. Untagged: it pins the order of the two
// receive steps, and the RFC9252-3.4-1 verdict belongs to the validator's own
// tests.
func TestRFC8669DuplicateMalformedServiceTLVStillWithdrawn(t *testing.T) {
	malformed := "05" + "0018" + "00" + "01" + "0014" + "00" + secondSID + "00" + "0013"
	_, _, action, err := rfc8669DuplicateReceive(t, safiLabeled, rfc8669PrefixSIDAttr(srv6L3ServiceTLV(firstSID)+malformed, false), "", false)
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionTreatAsWithdraw, action, "a malformed SRv6 Service TLV is treat-as-withdraw, repeat or not")
}

// TestDiscardRepeatedPrefixSIDTLVsAllocatesNothingWithoutRepeat calls the
// ingest step on an UPDATE whose Prefix-SID repeats nothing.
// Method: the step must return the WireUpdate it was given and allocate
// nothing, so an UPDATE without a repeat pays only the read.
func TestDiscardRepeatedPrefixSIDTLVsAllocatesNothingWithoutRepeat(t *testing.T) {
	received, _, _, err := rfc8669DuplicateReceive(t, safiLabeled, rfc8669PrefixSIDAttr(labelIndexTLV("00000064")+srv6L3ServiceTLV(firstSID)+unknownTLV+unknownTLV, false), "", false)
	require.NoError(t, err)
	wu := wireu.NewWireUpdate(received, 0)
	peer := netip.MustParseAddr("192.0.2.1")
	require.Same(t, wu, discardRepeatedPrefixSIDTLVs(wu, peer), "no repeat, no new WireUpdate")
	allocs := testing.AllocsPerRun(100, func() { discardRepeatedPrefixSIDTLVs(wu, peer) })
	require.Zero(t, allocs, "no repeat, no allocation")
}
