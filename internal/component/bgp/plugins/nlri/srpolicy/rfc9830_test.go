// VALIDATES: the RFC 9830 obligations ze's SR Policy ORIGINATOR carries -- the SAFI 73
// NLRI (AFI, bit length, mandatory attributes) and every SR Policy sub-TLV ze writes
// into the Tunnel Encapsulation attribute: the mandated value lengths, the Flags and
// RESERVED octets that must be zero on transmission, the single-instance rule, and the
// MPLS TC/S/TTL bits.
// PREVENTS: a sub-TLV whose declared length does not span its value (which desynchronises
// every sub-TLV after it), a reserved octet carrying leftover data, a second instance of a
// single-instance sub-TLV, and an S bit or TC field bleeding out of an MPLS label.

package srpolicy

import (
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// srpDirty configures every SR Policy sub-TLV ze can emit with a deliberately
// non-zero value in every operator-settable field. A reserved octet that reads zero
// out of THIS attribute is zero because the encoder wrote zero there, not because the
// buffer happened to be blank.
const srpDirty = "distinguisher 4294967295 color 4294967295 endpoint 10.0.0.1 " +
	"preference 3735928559 binding-sid mpls 1048575 srv6-binding-sid fc00::feed " +
	"priority 255 " +
	"segment-list weight 3405691582 segment type-a mpls 1048575 " +
	"segment type-b srv6 fc00::beef endpoint-behavior 65535 32 16 16 64 " +
	"policy-name alpha candidate-path-name primary"

// srpTLV parses the Tunnel Encapsulation attribute ze builds for a config and returns
// its single Tunnel Type TLV.
func srpTLV(t *testing.T, cfg string) attribute.TunnelTLV {
	t.Helper()
	te, err := attribute.ParseTunnelEncap(attr(t, cfg))
	require.NoError(t, err, "the attribute ze builds must parse as a TLV sequence")
	require.Len(t, te.TLVs, 1)
	return te.TLVs[0]
}

// srpSubs returns every sub-TLV of the SR Policy TLV ze builds for a config. The walk
// only succeeds when each declared length spans exactly to the next sub-TLV.
func srpSubs(t *testing.T, cfg string) []attribute.SubTLV {
	t.Helper()
	tlv := srpTLV(t, cfg)
	subs, err := tlv.SubTLVs()
	require.NoError(t, err)
	return subs
}

// srpAll returns the values of every sub-TLV of the given type, in wire order.
func srpAll(subs []attribute.SubTLV, styp uint8) [][]byte {
	var out [][]byte
	for i := range subs {
		if subs[i].Type == styp {
			out = append(out, subs[i].Value)
		}
	}
	return out
}

// srpOne returns the value of the one sub-TLV of the given type, failing when there is
// not exactly one.
func srpOne(t *testing.T, subs []attribute.SubTLV, styp uint8) []byte {
	t.Helper()
	found := srpAll(subs, styp)
	require.Len(t, found, 1, "exactly one sub-TLV of type %d", styp)
	return found[0]
}

// srpSegSubs walks the sub-sub-TLVs of a Segment List sub-TLV value, whose first octet
// is the RESERVED field and whose remainder is a plain RFC 9012 sub-TLV sequence.
func srpSegSubs(t *testing.T, value []byte) []attribute.SubTLV {
	t.Helper()
	require.NotEmpty(t, value)
	inner := attribute.TunnelTLV{Value: value[1:]}
	subs, err := inner.SubTLVs()
	require.NoError(t, err, "each segment sub-TLV length must span exactly to the next")
	return subs
}

// TestRFC9830NLRIAddressFamilies pins the two address families SAFI 73 is defined for
// and the bit length each one puts in the NLRI's first octet.
func TestRFC9830NLRIAddressFamilies(t *testing.T) {
	t.Parallel()

	// RFC requirement: RFC9830-2.1-1 positive -- ze registers SAFI 73 for IPv4 (AFI 1) and IPv6 (AFI 2) and both encode an NLRI
	assert.Equal(t, family.AFIIPv4, IPv4SRPolicy.AFI)
	assert.Equal(t, family.AFIIPv6, IPv6SRPolicy.AFI)
	for _, name := range family.RegisteredFamilyNames() {
		fam, ok := family.LookupFamily(name)
		require.True(t, ok)
		if fam.SAFI != family.SAFISRPolicy {
			continue
		}
		assert.Contains(t, []family.AFI{family.AFIIPv4, family.AFIIPv6}, fam.AFI,
			"%s: SAFI 73 is defined for AFI 1 and AFI 2 only", name)
	}

	v4hex, err := EncodeNLRIHex("ipv4/sr-policy", strings.Fields("distinguisher 7 color 100 endpoint 10.0.0.1"))
	require.NoError(t, err)
	v6hex, err := EncodeNLRIHex("ipv6/sr-policy", strings.Fields("distinguisher 7 color 100 endpoint 2001:db8::1"))
	require.NoError(t, err)
	v4, err := hex.DecodeString(v4hex)
	require.NoError(t, err)
	v6, err := hex.DecodeString(v6hex)
	require.NoError(t, err)

	// RFC requirement: RFC9830-2.1-2 positive -- the NLRI length octet is 96 for AFI 1 and 192 for AFI 2, and the body that follows is the matching 12 or 24 octets
	assert.Equal(t, byte(96), v4[0])
	assert.Equal(t, byte(192), v6[0])
	assert.Len(t, v4, 1+ipv4BodyLen)
	assert.Len(t, v6, 1+ipv6BodyLen)

	// RFC requirement: RFC9830-2.1-1 negative -- a family that is not one of the two SR Policy families is refused, and no SAFI 73 family exists for any other AFI
	_, err = srPolicyAFI("ipv4/unicast")
	require.Error(t, err, "SAFI 73 is what makes a family an SR Policy family")
	_, ok := family.LookupFamily("l2vpn/sr-policy")
	assert.False(t, ok)

	// RFC requirement: RFC9830-2.1-2 negative -- an NLRI carrying the other AFI's length is refused rather than read short: a 12-octet body is not an AFI 2 NLRI, and a length that is not a whole number of octets is refused outright
	_, err = Parse(family.AFIIPv6, v4[1:])
	require.ErrorIs(t, err, ErrSRPolicyTruncated)
	_, err = SplitSRPolicy([]byte{100, 0, 0, 0, 0}, false, nil)
	require.Error(t, err, "a length of 100 bits is not byte-aligned")
}

// TestRFC9830NLRICarriesAllThreeFields pins that an SR Policy NLRI is only ever formed,
// and only ever read, with all three of its fields present.
func TestRFC9830NLRICarriesAllThreeFields(t *testing.T) {
	t.Parallel()

	sp := New(family.AFIIPv4, 7, 100, netip.MustParseAddr("10.0.0.1"))
	wire := sp.Bytes()
	parsed, err := Parse(family.AFIIPv4, wire[1:])
	require.NoError(t, err)

	// RFC requirement: RFC9830-4.2.1-2 positive -- the NLRI includes a Distinguisher, a Color and an Endpoint, each read back from its mandated offset in the 12-octet body
	assert.Equal(t, uint32(7), parsed.Distinguisher())
	assert.Equal(t, uint32(100), parsed.Color())
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), parsed.Endpoint())
	assert.Equal(t, []byte{0, 0, 0, 7}, wire[1:5], "distinguisher at octets 0..3 of the body")
	assert.Equal(t, []byte{0, 0, 0, 100}, wire[5:9], "color at octets 4..7")
	assert.Equal(t, []byte{10, 0, 0, 1}, wire[9:13], "endpoint at octets 8..11")

	// RFC requirement: RFC9830-4.2.1-2 negative -- an NLRI that would be missing one of the three is refused rather than formed or read partially: a config naming only two of them fails, and a body one octet short of holding all three fails
	for _, cfg := range []string{
		"color 100 endpoint 10.0.0.1",
		"distinguisher 7 endpoint 10.0.0.1",
		"distinguisher 7 color 100",
	} {
		_, err := parseConfigRoute(cr(strings.Fields(cfg), "192.0.2.1", false))
		require.ErrorIs(t, err, errSRPolicyMissingFields, "config %q", cfg)
	}
	_, err = Parse(family.AFIIPv4, wire[1:len(wire)-1])
	require.ErrorIs(t, err, ErrSRPolicyTruncated)
}

// TestRFC9830UpdateCarriesMandatoryAttributes pins that the UPDATE ze builds for a
// SAFI 73 NLRI carries the BGP mandatory attributes rather than the MP_REACH alone.
func TestRFC9830UpdateCarriesMandatoryAttributes(t *testing.T) {
	t.Parallel()

	cmd := "distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 192.0.2.1 preference 100"
	ibgp, _, err := EncodeRoute(cmd, "ipv4/sr-policy", 65000, true, true, false)
	require.NoError(t, err)
	ebgp, _, err := EncodeRoute(cmd, "ipv4/sr-policy", 65000, false, true, false)
	require.NoError(t, err)

	ibgpHex := strings.ToUpper(hex.EncodeToString(ibgp))
	ebgpHex := strings.ToUpper(hex.EncodeToString(ebgp))

	// RFC requirement: RFC9830-2.1-3 positive -- an SR Policy UPDATE carries ORIGIN (code 1, well-known transitive, IGP) and AS_PATH (code 2) alongside MP_REACH_NLRI, on both an iBGP and an eBGP session
	assert.Contains(t, ibgpHex, "40010100", "ORIGIN IGP")
	assert.Contains(t, ebgpHex, "40010100", "ORIGIN IGP")
	assert.Contains(t, ibgpHex, "4002", "AS_PATH")
	assert.Contains(t, ebgpHex, "400206020100", "AS_PATH with the local AS prepended on eBGP")

	// RFC requirement: RFC9830-2.1-3 negative -- LOCAL_PREF, which is well-known DISCRETIONARY, rides only on the iBGP UPDATE, so the mandatory set is chosen per attribute category and is not a fixed blob attached to every SR Policy route
	assert.Contains(t, ibgpHex, "40050400000064", "LOCAL_PREF 100 on iBGP")
	assert.NotContains(t, ebgpHex, "40050400000064")
}

// TestRFC9830SinglePolicyTLVPerAttribute pins that everything an SR Policy candidate
// path signals rides inside ONE Tunnel Type 15 TLV.
func TestRFC9830SinglePolicyTLVPerAttribute(t *testing.T) {
	t.Parallel()

	// RFC requirement: RFC9830-2.2-2 positive -- however many sub-TLVs or segment lists a candidate path configures, ze builds exactly one SR Policy TLV in the Tunnel Encapsulation attribute
	for _, cfg := range []string{
		"distinguisher 0 color 100 endpoint 10.0.0.1",
		"distinguisher 0 color 100 endpoint 10.0.0.1 preference 100",
		srpDirty,
		"distinguisher 0 color 100 endpoint 10.0.0.1 preference 100 " +
			"segment-list weight 1 segment type-a mpls 16001 " +
			"segment-list weight 2 segment type-a mpls 16002",
	} {
		te, err := attribute.ParseTunnelEncap(attr(t, cfg))
		require.NoError(t, err)
		require.Len(t, te.TLVs, 1, "one SR Policy TLV: %s", cfg)
		assert.Equal(t, tunnelTypeSRPolicyCP, te.TLVs[0].TunnelType)
	}
}

// TestRFC9830TunnelTypeIsSRPolicy pins that the sub-TLVs are wrapped in a Tunnel Type
// TLV whose type is 15, on an attribute that is itself attached to every SR Policy route.
func TestRFC9830TunnelTypeIsSRPolicy(t *testing.T) {
	t.Parallel()

	// RFC requirement: RFC9830-4.2.1-6 positive -- every SR Policy route ze builds carries the Tunnel Encapsulation attribute (code 23, optional transitive), including one configured with no sub-TLV content at all
	for _, cfg := range []string{"distinguisher 0 color 100 endpoint 10.0.0.1", srpDirty} {
		pr, err := parseConfigRoute(cr(strings.Fields(cfg), "192.0.2.1", false))
		require.NoError(t, err)
		require.Len(t, pr.Attrs, 1)
		assert.Equal(t, attrCodeTunnelEncap, pr.Attrs[0].Code)
		assert.Equal(t, attrFlagOptTransFlag, pr.Attrs[0].Flags)
	}

	value := attr(t, srpDirty)

	// RFC requirement: RFC9830-4.2.1-7 positive -- the attribute's single TLV declares Tunnel Type 15 in its first two octets and its length covers exactly the sub-TLVs that follow
	assert.Equal(t, []byte{0x00, 0x0F}, value[0:2])
	te, err := attribute.ParseTunnelEncap(value)
	require.NoError(t, err)
	require.Len(t, te.TLVs, 1)
	assert.Equal(t, uint16(15), te.TLVs[0].TunnelType)
	assert.Len(t, te.TLVs[0].Value, len(value)-4)

	// RFC requirement: RFC9830-4.2.1-7 negative -- the sub-TLV payload on its own does not read as an SR Policy TLV, so the type-15 header is load-bearing rather than incidental: an encoder that omitted it would be caught
	bare, err := attribute.ParseTunnelEncap(value[4:])
	if err == nil {
		require.NotEmpty(t, bare.TLVs)
		assert.NotEqual(t, uint16(15), bare.TLVs[0].TunnelType)
	}
}

// TestRFC9830PreferenceSubTLV pins the Preference sub-TLV ze writes: one instance, a
// value length of 6, and zero Flags and RESERVED octets.
func TestRFC9830PreferenceSubTLV(t *testing.T) {
	t.Parallel()

	subs := srpSubs(t, srpDirty)
	value := srpOne(t, subs, subTLVPreference)

	// RFC requirement: RFC9830-2.4.1-3 positive -- the Preference sub-TLV value is 6 octets: Flags(1) + RESERVED(1) + Preference(4)
	require.Len(t, value, 6)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, value[2:6], "the preference occupies the last four octets")

	// RFC requirement: RFC9830-2.4.1-4 positive -- the Flags octet is zero on transmission
	assert.Equal(t, byte(0), value[0])
	// RFC requirement: RFC9830-2.4.1-6 positive -- the RESERVED octet is zero on transmission
	assert.Equal(t, byte(0), value[1])

	// RFC requirement: RFC9830-2.4.1-4 negative -- the preference itself is 0xDEADBEEF in the same six octets, so the zero Flags octet is a property of that field and not of an all-zero sub-TLV
	// RFC requirement: RFC9830-2.4.1-6 negative -- the same six octets are not blank, so the zero RESERVED octet is likewise a written zero rather than an untouched buffer
	assert.NotEqual(t, make([]byte, 6), value)

	// RFC requirement: RFC9830-2.4.1-2 positive -- a config naming the preference twice still yields exactly one Preference sub-TLV
	repeated := srpSubs(t, "distinguisher 0 color 100 endpoint 10.0.0.1 preference 100 preference 200")
	assert.Len(t, srpAll(repeated, subTLVPreference), 1)

	// RFC requirement: RFC9830-2.4.1-2 negative -- a config that names no preference emits no Preference sub-TLV, so "at most one" is a real count and not an unconditional single emission
	none := srpSubs(t, "distinguisher 0 color 100 endpoint 10.0.0.1 priority 3")
	assert.Empty(t, srpAll(none, subTLVPreference))
}

// TestRFC9830BindingSIDSubTLV pins the MPLS Binding SID sub-TLV lengths, its RESERVED
// octet, and the TC/S/TTL bits of the label stack entry.
func TestRFC9830BindingSIDSubTLV(t *testing.T) {
	t.Parallel()

	withBSID := srpOne(t, srpSubs(t, srpDirty), subTLVBindingSID)
	noBSID := srpOne(t, srpSubs(t, "distinguisher 0 color 100 endpoint 10.0.0.1 binding-sid null"), subTLVBindingSID)

	// RFC requirement: RFC9830-2.4.2-4 positive -- the value is 6 octets when an SR-MPLS BSID is present: Flags(1) + RESERVED(1) + the 4-octet label stack entry
	require.Len(t, withBSID, 6)
	// RFC requirement: RFC9830-2.4.2-4 negative -- the value is 2 octets when no BSID is present, so the length tracks the content rather than being a constant 6
	require.Len(t, noBSID, 2)

	// RFC requirement: RFC9830-2.4.2-7 positive -- the RESERVED octet is zero on transmission, in both the 6-octet and the 2-octet form
	assert.Equal(t, byte(0), withBSID[1])
	assert.Equal(t, byte(0), noBSID[1])

	// RFC requirement: RFC9830-2.4.2-9 positive -- the TC bits, the S bit and the TTL octet of the label stack entry are all zero on transmission
	assert.Equal(t, byte(0), withBSID[4]&0x0E, "TC")
	assert.Equal(t, byte(0), withBSID[4]&0x01, "S")
	assert.Equal(t, byte(0), withBSID[5], "TTL")

	// RFC requirement: RFC9830-2.4.2-9 negative -- the label is 0xFFFFF, so every bit of the 20-bit label field is set while TC, S and TTL stay zero: the zeroes come from the field layout and not from a label small enough to leave the low octets blank
	label := uint32(withBSID[2])<<12 | uint32(withBSID[3])<<4 | uint32(withBSID[4])>>4
	assert.Equal(t, uint32(0xFFFFF), label)
	// RFC requirement: RFC9830-2.4.2-7 negative -- the same six octets carry that non-zero label, so the zero RESERVED octet is a written zero rather than an untouched buffer
	assert.NotEqual(t, make([]byte, 6), withBSID)

	// RFC requirement: RFC9830-2.4.2-2 positive -- a config naming the binding SID twice still yields exactly one Binding SID sub-TLV
	repeated := srpSubs(t, "distinguisher 0 color 100 endpoint 10.0.0.1 binding-sid mpls 24000 binding-sid null")
	assert.Len(t, srpAll(repeated, subTLVBindingSID), 1)

	// RFC requirement: RFC9830-2.4.2-2 negative -- a config naming no binding SID emits none, so the single instance is counted rather than always written
	assert.Empty(t, srpAll(srpSubs(t, "distinguisher 0 color 100 endpoint 10.0.0.1 priority 3"), subTLVBindingSID))
}

// TestRFC9830SRv6BindingSIDSubTLV pins the SRv6 Binding SID sub-TLV (type 20) ze writes.
func TestRFC9830SRv6BindingSIDSubTLV(t *testing.T) {
	t.Parallel()

	subs := srpSubs(t, srpDirty)
	value := srpOne(t, subs, subTLVSRv6BindingSID)

	// RFC requirement: RFC9830-2.4.3-3 positive -- the value is 18 octets: Flags(1) + RESERVED(1) + the 16-octet SRv6 SID, ze attaching no SRv6 Endpoint Behavior and SID Structure that would make it 26
	require.Len(t, value, 18)

	// RFC requirement: RFC9830-2.4.3-9 positive -- the 16-octet SRv6 SID is always present and the Endpoint Behavior and SID Structure is never appended, so the encoding never carries the structure without a SID
	assert.Equal(t, byte(0xFC), value[2], "the SID ze was configured with")
	assert.Equal(t, byte(0xED), value[17])

	// RFC requirement: RFC9830-2.4.3-4 positive -- the unassigned bits of the Flags octet are zero on transmission (ze signals neither S, I nor B here, so the whole octet is zero)
	assert.Equal(t, byte(0), value[0])
	// RFC requirement: RFC9830-2.4.3-6 positive -- the RESERVED octet is zero on transmission
	assert.Equal(t, byte(0), value[1])

	// RFC requirement: RFC9830-2.4.3-4 negative -- the SID octets in the same value are non-zero, so the zero Flags octet is a written zero and not an all-zero sub-TLV
	// RFC requirement: RFC9830-2.4.3-6 negative -- the same value is not blank, so the zero RESERVED octet is likewise written rather than untouched
	assert.NotEqual(t, make([]byte, 18), value)
}

// TestRFC9830PrioritySubTLV pins the Priority sub-TLV length, field order and RESERVED
// octet.
func TestRFC9830PrioritySubTLV(t *testing.T) {
	t.Parallel()

	value := srpOne(t, srpSubs(t, srpDirty), subTLVPriority)

	// RFC requirement: RFC9830-2.4.6-4 positive -- the Priority sub-TLV value is 2 octets: Priority(1) + RESERVED(1)
	require.Len(t, value, 2)
	assert.Equal(t, byte(255), value[0], "the configured priority is the FIRST octet")

	// RFC requirement: RFC9830-2.4.6-5 positive -- the RESERVED octet is zero on transmission
	assert.Equal(t, byte(0), value[1])
	// RFC requirement: RFC9830-2.4.6-5 negative -- the priority octet beside it is 255, so the zero RESERVED octet is written by the encoder rather than being an untouched two-octet buffer, and the two fields are not transposed
	assert.NotEqual(t, []byte{0x00, 0x00}, value)

	// RFC requirement: RFC9830-2.4.6-3 positive -- a config naming the priority twice still yields exactly one Priority sub-TLV
	repeated := srpSubs(t, "distinguisher 0 color 100 endpoint 10.0.0.1 priority 3 priority 9")
	assert.Len(t, srpAll(repeated, subTLVPriority), 1)
	// RFC requirement: RFC9830-2.4.6-3 negative -- a config naming no priority emits none, so the single instance is counted rather than always written
	assert.Empty(t, srpAll(srpSubs(t, "distinguisher 0 color 100 endpoint 10.0.0.1 preference 3"), subTLVPriority))
}

// TestRFC9830SegmentListSubTLV pins the Segment List RESERVED octet and the Weight
// sub-TLV inside it.
func TestRFC9830SegmentListSubTLV(t *testing.T) {
	t.Parallel()

	lists := srpAll(srpSubs(t, srpDirty), subTLVSegmentList)
	require.Len(t, lists, 1)
	value := lists[0]

	// RFC requirement: RFC9830-2.4.4-4 positive -- the Segment List RESERVED octet, which precedes the sub-TLVs, is zero on transmission
	assert.Equal(t, byte(0), value[0])

	inner := srpSegSubs(t, value)
	// RFC requirement: RFC9830-2.4.4-4 negative -- the sub-TLVs that follow that octet are non-empty and non-zero, so the zero RESERVED octet is written by the encoder and is not the head of a blank segment list
	require.NotEmpty(t, inner)
	assert.NotEqual(t, make([]byte, len(value)), value)

	weights := srpAll(inner, segSubTLVWeight)
	// RFC requirement: RFC9830-2.4.4.1-2 positive -- the Segment List carries exactly one Weight sub-TLV
	require.Len(t, weights, 1)
	weight := weights[0]

	// RFC requirement: RFC9830-2.4.4.1-3 positive -- the Weight sub-TLV value is 6 octets: Flags(1) + RESERVED(1) + Weight(4), and the walk of the segment list reaches its end, so the declared length spans exactly the value
	require.Len(t, weight, 6)
	assert.Equal(t, []byte{0xCA, 0xFE, 0xBA, 0xBE}, weight[2:6])

	// RFC requirement: RFC9830-2.4.4.1-4 positive -- the Weight Flags octet is zero on transmission
	assert.Equal(t, byte(0), weight[0])
	// RFC requirement: RFC9830-2.4.4.1-6 positive -- the Weight RESERVED octet is zero on transmission
	assert.Equal(t, byte(0), weight[1])
	// RFC requirement: RFC9830-2.4.4.1-4 negative -- the weight in the same six octets is 0xCAFEBABE, so the zero Flags octet is a written zero rather than an all-zero sub-TLV
	// RFC requirement: RFC9830-2.4.4.1-6 negative -- the same six octets are not blank, so the zero RESERVED octet is likewise written rather than untouched
	assert.NotEqual(t, make([]byte, 6), weight)

	// RFC requirement: RFC9830-2.4.4.1-3 negative -- a Weight sub-TLV is 8 octets on the wire (type + length + the 6-octet value), so a length field carrying the total rather than the value length would leave the segment list walk short
	assert.Len(t, value, 1+8+8+28, "RESERVED + weight + type A + type B with the endpoint behavior")

	// RFC requirement: RFC9830-2.4.4.1-2 negative -- a second `weight` token inside one segment list is not consumed as a second Weight sub-TLV: the segment list ends and the stray token is refused
	_, err := parseConfigRoute(cr(strings.Fields(
		"distinguisher 0 color 100 endpoint 10.0.0.1 segment-list weight 1 weight 2 segment type-a mpls 16001"),
		"192.0.2.1", false))
	require.Error(t, err)
}

// TestRFC9830SegmentTypeASubTLV pins the Type A (SR-MPLS) segment sub-TLV.
func TestRFC9830SegmentTypeASubTLV(t *testing.T) {
	t.Parallel()

	inner := srpSegSubs(t, srpAll(srpSubs(t, srpDirty), subTLVSegmentList)[0])
	typeA := srpAll(inner, segSubTLVTypeA)
	require.Len(t, typeA, 1)
	value := typeA[0]

	// RFC requirement: RFC9830-2.4.4.2.1-1 positive -- the Type A segment sub-TLV value is 6 octets: Flags(1) + RESERVED(1) + the 4-octet label stack entry
	require.Len(t, value, 6)

	// RFC requirement: RFC9830-2.4.4.2.1-2 positive -- the RESERVED octet is zero on transmission
	assert.Equal(t, byte(0), value[1])
	// RFC requirement: RFC9830-2.4.4.2.1-4 positive -- the S bit of the label stack entry is zero on transmission
	assert.Equal(t, byte(0), value[4]&0x01)

	label := uint32(value[2])<<12 | uint32(value[3])<<4 | uint32(value[4])>>4
	// RFC requirement: RFC9830-2.4.4.2.1-4 negative -- the label is 0xFFFFF, so every bit of the label field is set while the S bit stays clear: the zero S bit comes from the shift, not from a label that leaves the low octets blank
	assert.Equal(t, uint32(0xFFFFF), label)
	// RFC requirement: RFC9830-2.4.4.2.1-2 negative -- the same six octets carry that non-zero label, so the zero RESERVED octet is written by the encoder rather than being an untouched buffer
	assert.NotEqual(t, make([]byte, 6), value)

	// RFC requirement: RFC9830-2.4.4.2.1-1 negative -- a Type A sub-TLV is 8 octets on the wire; a value length of 8 (the total) instead of 6 would run past the Type B sub-TLV that follows and the walk above would not have reached the end of the segment list
	assert.Len(t, inner, 3, "weight, type A, type B")

	// RFC requirement: RFC9830-2.4.4.2.3-1 positive -- the Type A Flags octet has no assigned bit set: ze signals neither V nor B on a Type A segment, so every unassigned bit is zero on transmission
	assert.Equal(t, byte(0), value[0])
}

// TestRFC9830SegmentTypeBSubTLV pins the Type B (SRv6) segment sub-TLV, with and
// without the SRv6 Endpoint Behavior and SID Structure.
func TestRFC9830SegmentTypeBSubTLV(t *testing.T) {
	t.Parallel()

	withEB := srpAll(srpSegSubs(t, srpAll(srpSubs(t, srpDirty), subTLVSegmentList)[0]), segSubTLVTypeBSID)
	require.Len(t, withEB, 1)
	value := withEB[0]

	bare := srpAll(srpSegSubs(t, srpAll(srpSubs(t,
		"distinguisher 0 color 100 endpoint 10.0.0.1 segment-list weight 1 segment type-b srv6 fc00::beef"),
		subTLVSegmentList)[0]), segSubTLVTypeBSID)
	require.Len(t, bare, 1)

	// RFC requirement: RFC9830-2.4.4.2.2-1 positive -- the value is 26 octets when the SRv6 Endpoint Behavior and SID Structure is present
	require.Len(t, value, 26)
	// RFC requirement: RFC9830-2.4.4.2.2-1 negative -- the value is 18 octets when it is absent, so the length tracks the content rather than being a constant 26
	require.Len(t, bare[0], 18)

	// RFC requirement: RFC9830-2.4.4.2.2-2 positive -- the Type B RESERVED octet is zero on transmission, with and without the endpoint behavior
	assert.Equal(t, byte(0), value[1])
	assert.Equal(t, byte(0), bare[0][1])
	// RFC requirement: RFC9830-2.4.4.2.2-2 negative -- the SRv6 SID in the same value is non-zero, so the zero RESERVED octet is written rather than an untouched buffer
	assert.NotEqual(t, make([]byte, 26), value)
	assert.Equal(t, byte(0xFC), value[2])

	// RFC requirement: RFC9830-2.4.4.2.3-1 positive -- the only Flags bit ze sets on a Type B segment is the assigned B-Flag (bit 3), so every unassigned bit is zero on transmission
	assert.Equal(t, byte(0x10), value[0])
	assert.Equal(t, byte(0), value[0]&^byte(0x10))
	// RFC requirement: RFC9830-2.4.4.2.3-1 negative -- that Flags octet is NOT zero, so "the unassigned bits are zero" is a statement about which bits the encoder sets and not about an all-zero flags byte; without the endpoint behavior the same field is zero
	assert.NotEqual(t, byte(0), value[0])
	assert.Equal(t, byte(0), bare[0][0])

	// RFC requirement: RFC9830-2.4.4.2.4-2 positive -- the two Reserved octets of the SRv6 Endpoint Behavior and SID Structure are zero on transmission
	assert.Equal(t, []byte{0x00, 0x00}, value[20:22])
	// RFC requirement: RFC9830-2.4.4.2.4-2 negative -- the endpoint behavior before them is 0xFFFF and the SID structure after them is 32/16/16/64, so those two zero octets are written by the encoder and sit at the right offset
	assert.Equal(t, []byte{0xFF, 0xFF}, value[18:20])
	assert.Equal(t, []byte{32, 16, 16, 64}, value[22:26])

	// RFC requirement: RFC9830-2.4.4.2.2-4 positive -- the endpoint behavior is only ever encoded after a 16-octet SRv6 SID, which occupies the value's octets 2..17
	assert.Equal(t, byte(0xEF), value[17])

	// RFC requirement: RFC9830-2.4.4.2.2-4 negative -- an `endpoint-behavior` that is not preceded by a `type-b srv6 <sid>` segment is refused, so no encoding can carry the structure without a SID
	_, _, err := parseSegmentList(strings.Fields("weight 1 segment endpoint-behavior 65 32 16 16 0"))
	require.Error(t, err)
	_, err = parseConfigRoute(cr(strings.Fields(
		"distinguisher 0 color 100 endpoint 10.0.0.1 segment-list weight 1 segment endpoint-behavior 65 32 16 16 0"),
		"192.0.2.1", false))
	require.Error(t, err)
}

// TestRFC9830NameSubTLVs pins the two symbolic-name sub-TLVs: one instance each, the
// RESERVED octet, and the whole name after it.
func TestRFC9830NameSubTLVs(t *testing.T) {
	t.Parallel()

	subs := srpSubs(t, srpDirty)

	for _, tc := range []struct {
		styp uint8
		want string
	}{
		{subTLVPolicyName, "alpha"},
		{subTLVCandidatePathNam, "primary"},
	} {
		value := srpOne(t, subs, tc.styp)
		// RFC requirement: RFC9830-2.4.7-6 positive -- the SR Policy Candidate Path Name sub-TLV RESERVED octet is zero on transmission and the whole name follows it
		// RFC requirement: RFC9830-2.4.8-6 positive -- the SR Policy Name sub-TLV RESERVED octet is likewise zero on transmission, with the whole name after it
		require.Len(t, value, 1+len(tc.want))
		assert.Equal(t, byte(0), value[0])
		assert.Equal(t, tc.want, string(value[1:]), "the name is not truncated by the reserved octet")
	}

	// RFC requirement: RFC9830-2.4.7-6 negative -- the name octets beside it are the ASCII of the configured name, so the zero RESERVED octet is written by the encoder rather than being an all-zero value
	// RFC requirement: RFC9830-2.4.8-6 negative -- the same holds for the SR Policy Name sub-TLV
	assert.NotEqual(t, make([]byte, 6), srpOne(t, subs, subTLVPolicyName))

	// RFC requirement: RFC9830-2.4.7-5 positive -- naming the candidate path twice yields exactly one Candidate Path Name sub-TLV
	// RFC requirement: RFC9830-2.4.8-5 positive -- naming the policy twice yields exactly one SR Policy Name sub-TLV
	repeated := srpSubs(t, "distinguisher 0 color 100 endpoint 10.0.0.1 "+
		"policy-name one policy-name two candidate-path-name a candidate-path-name b")
	assert.Len(t, srpAll(repeated, subTLVPolicyName), 1)
	assert.Len(t, srpAll(repeated, subTLVCandidatePathNam), 1)

	// RFC requirement: RFC9830-2.4.7-5 negative -- an unnamed candidate path emits no Candidate Path Name sub-TLV, so the single instance is counted rather than always written
	// RFC requirement: RFC9830-2.4.8-5 negative -- an unnamed policy likewise emits no SR Policy Name sub-TLV
	none := srpSubs(t, "distinguisher 0 color 100 endpoint 10.0.0.1 preference 1")
	assert.Empty(t, srpAll(none, subTLVPolicyName))
	assert.Empty(t, srpAll(none, subTLVCandidatePathNam))
}

// TestRFC9830PriorityLengthIsValueLength pins that the Priority sub-TLV's length octet
// counts its value and not the whole sub-TLV.
func TestRFC9830PriorityLengthIsValueLength(t *testing.T) {
	t.Parallel()

	// RFC requirement: RFC9830-2.4.6-4 negative -- the length octet carries the VALUE length: a Priority sub-TLV occupies 4 octets on the wire, so a length of 4 would swallow whatever follows it. Here the TLV payload is exactly the one 4-octet sub-TLV, and a length of anything but 2 would leave the walk short or long
	only := attr(t, "distinguisher 0 color 100 endpoint 10.0.0.1 priority 255")
	assert.Equal(t, []byte{subTLVPriority, 2, 255, 0}, only[4:],
		"the whole TLV payload is one 4-octet Priority sub-TLV declaring a 2-octet value")
	assert.Len(t, srpAll(srpSubs(t, "distinguisher 0 color 100 endpoint 10.0.0.1 priority 255"), subTLVPriority), 1)
}
