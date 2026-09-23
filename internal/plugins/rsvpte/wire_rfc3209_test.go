// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RFC 3209 wire-format proofs
// Related: wire_test.go -- codec round trips; frr.go -- SESSION_ATTRIBUTE codec
//
// Each test here carries an RFC 3209 requirement tag and asserts the bytes the
// codec writes or the error the decoder returns, so a stub that writes nothing
// or accepts anything fails it.
package rsvpte

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// subobjectLengths walks the subobjects of one encoded ERO or RRO object body
// and returns each Length octet. The walk trusts the Length it reads, which is
// what a receiver does, so a wrong Length would surface as a wrong count.
func subobjectLengths(t *testing.T, body []byte) []int {
	t.Helper()
	var lengths []int
	for off := 0; off < len(body); {
		require.Less(t, off+1, len(body), "subobject header inside the body")
		l := int(body[off+1])
		require.Positive(t, l, "a zero Length would never advance")
		lengths = append(lengths, l)
		off += l
	}
	return lengths
}

// objectBodies splits an encoded message into (Class-Num, body) pairs in wire
// order, using the object headers alone.
func objectBodies(t *testing.T, raw []byte) []objectHeader {
	t.Helper()
	var objs []objectHeader
	for off := rsvpHdrLen; off < len(raw); {
		hdr, err := decodeObjectHeader(raw[off:])
		require.NoError(t, err)
		objs = append(objs, hdr)
		off += int(hdr.Length)
	}
	return objs
}

// RFC requirement: RFC3209-4.2.1-1 positive — encodeLabelRequest writes the two reserved octets of a C-Type 1 LABEL_REQUEST as zero over a buffer that held 0xFF, with the L3PID in the last two octets.
func TestRFC3209LabelRequestReservedZeroOnSend(t *testing.T) {
	buf := make([]byte, 16)
	for i := range buf {
		buf[i] = 0xFF
	}
	n := encodeLabelRequest(buf, labelRequest{L3PID: 0x86DD})
	require.Equal(t, 8, n)
	assert.Equal(t, CTypeGeneric, buf[3], "C-Type 1: no label range")
	assert.Equal(t, []byte{0, 0}, buf[4:6], "reserved octets are zero on transmission")
	assert.Equal(t, uint16(0x86DD), binary.BigEndian.Uint16(buf[6:8]))
}

// RFC requirement: RFC3209-4.2.1-1 negative — a LABEL_REQUEST whose reserved octets are non-zero on receipt is not refused: decodeLabelRequest returns the L3PID and no error.
func TestRFC3209LabelRequestReservedIgnoredOnReceipt(t *testing.T) {
	body := []byte{0xFF, 0xFF, 0x08, 0x00}
	lr, err := decodeLabelRequest(body)
	require.NoError(t, err, "reserved octets are ignored on receipt")
	assert.Equal(t, labelRequest{L3PID: 0x0800}, lr)
}

// RFC requirement: RFC3209-4.3.3-1 positive — every EXPLICIT_ROUTE subobject encodeERO writes has a Length of 8 (IPv4 prefix) or 20 (IPv6 prefix), each at least 4 and a multiple of 4.
func TestRFC3209EROSubobjectLengthOnSend(t *testing.T) {
	hops := []eroHop{
		{Address: netip.MustParsePrefix("10.0.0.5/32")},
		{Loose: true, Address: netip.MustParsePrefix("2001:db8::1/128")},
		{Address: netip.MustParsePrefix("10.0.0.9/32")},
	}
	buf := make([]byte, maxRSVPMessage)
	n := encodeERO(buf, hops)
	lengths := subobjectLengths(t, buf[objHdrLen:n])
	assert.Equal(t, []int{8, 20, 8}, lengths)
	for _, l := range lengths {
		assert.GreaterOrEqual(t, l, 4)
		assert.Zero(t, l%4, "Length %d is a multiple of 4", l)
	}
}

// RFC requirement: RFC3209-4.3.3-1 negative — an EXPLICIT_ROUTE subobject whose Length is 3 (below the minimum of 4) is refused by decodeERO with errShortERO.
func TestRFC3209EROSubobjectLengthShortRefused(t *testing.T) {
	// One well-formed IPv4 subobject followed by a Length 3 subobject.
	body := []byte{
		EROSubIPv4Prefix, 8, 10, 0, 0, 5, 32, 0,
		EROSubIPv4Prefix, 3, 10,
	}
	hops, err := decodeERO(body)
	require.ErrorIs(t, err, errShortERO)
	assert.Len(t, hops, 1, "the subobjects before the bad one are returned with the error")
}

// RFC requirement: RFC3209-4.4.1-1 positive — every RECORD_ROUTE subobject encodeRRO writes has a Length of 8 (IPv4, Label) or 20 (IPv6), each at least 4 and a multiple of 4.
func TestRFC3209RROSubobjectLengthOnSend(t *testing.T) {
	entries := []rroEntry{
		{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.5")},
		{Type: RROSubLabel, Label: 1001},
		{Type: RROSubIPv6, Address: netip.MustParseAddr("2001:db8::5")},
	}
	buf := make([]byte, maxRSVPMessage)
	n := encodeRRO(buf, entries)
	lengths := subobjectLengths(t, buf[objHdrLen:n])
	assert.Equal(t, []int{8, 8, 20}, lengths)
	for _, l := range lengths {
		assert.GreaterOrEqual(t, l, 4)
		assert.Zero(t, l%4, "Length %d is a multiple of 4", l)
	}
}

// RFC requirement: RFC3209-4.4.1-1 negative — a RECORD_ROUTE subobject whose Length is 3 (below the minimum of 4) is refused by decodeRRO with errShortRRO.
func TestRFC3209RROSubobjectLengthShortRefused(t *testing.T) {
	body := []byte{
		RROSubIPv4, 8, 10, 0, 0, 5, 32, 0,
		RROSubIPv4, 3, 10,
	}
	entries, err := decodeRRO(body)
	require.ErrorIs(t, err, errShortRRO)
	assert.Len(t, entries, 1)
}

// RFC requirement: RFC3209-4.7.3-1 positive — encodeSessionAttr writes a Length that is a multiple of 4 and at least 8 for Session Names of 0 to 5 octets, and the header Length equals the bytes written.
func TestRFC3209SessionAttributeLengthOnSend(t *testing.T) {
	for _, name := range []string{"", "a", "ab", "abc", "abcd", "abcde"} {
		buf := make([]byte, 64)
		n := encodeSessionAttr(buf, sessionAttribute{SetupPrio: 7, HoldPrio: 7, Name: name})
		hdr, err := decodeObjectHeader(buf)
		require.NoError(t, err)
		assert.Equal(t, n, int(hdr.Length), "name %q", name)
		assert.GreaterOrEqual(t, int(hdr.Length), 8, "name %q", name)
		assert.Zero(t, int(hdr.Length)%4, "name %q: Length %d is a multiple of 4", name, hdr.Length)
		assert.Equal(t, uint8(len(name)), buf[7], "Name Length octet")
	}
}

// RFC requirement: RFC3209-4.7.3-1 negative — a SESSION_ATTRIBUTE whose body is shorter than the four fixed octets (object Length below 8) is refused by decodeSessionAttr with errShortObject.
func TestRFC3209SessionAttributeLengthShortRefused(t *testing.T) {
	_, err := decodeSessionAttr([]byte{7, 7, 0}, CTypeSessionAttr)
	require.ErrorIs(t, err, errShortObject)
	_, err = decodeSessionAttr(make([]byte, 12+3), CTypeSessionAttrRA)
	require.ErrorIs(t, err, errShortObject, "C-Type 1 adds 12 octets of affinities before the same four")
}

// TestRFC3209ObjectsAcceptedInAnyOrder distinguishes the three freely ordered
// PATH objects (RFC 3209 Section 3) from the ordered RESV descriptor suffix
// (RFC 2205 Section 3.1.4 and RFC 3209 Section 3.2).
func TestRFC3209ObjectsAcceptedInAnyOrder(t *testing.T) {
	rsb := &resvStateBlock{
		Session:  sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 42, ExtTunnelID: 0x0a000001},
		FlowSpec: FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		Label:    labelObject{Label: 16050},
		Style:    StyleSharedExplicit,
		RRO:      []rroEntry{{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.9")}},
	}
	filter := senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 7}
	hop := rsvpHop{NextHop: netip.MustParseAddr("10.0.0.5")}
	tv := timeValues{RefreshPeriod: refreshMillis(DefaultRefreshPeriod)}
	ero := []eroHop{
		{Address: netip.MustParsePrefix("10.0.0.5/32")},
		{Loose: true, Address: netip.MustParsePrefix("10.0.0.9/32")},
	}
	request := labelRequest{L3PID: 0x0800}
	attr := sessionAttribute{SetupPrio: 3, HoldPrio: 5, Flags: 0x02, Name: "permuted"}
	tspec := rsb.FlowSpec
	tspec.Service = serviceGeneral
	pathPrefix := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, rsb.Session) },
		func(b []byte) int { return encodeRSVPHop(b, hop) },
		func(b []byte) int { return encodeTimeValues(b, tv) },
	}
	pathObjects := []objEncoder{
		func(b []byte) int { return encodeERO(b, ero) },
		func(b []byte) int { return encodeLabelRequest(b, request) },
		func(b []byte) int { return encodeSessionAttr(b, attr) },
	}
	permutations := [][3]int{
		{0, 1, 2}, {0, 2, 1}, {1, 0, 2},
		{1, 2, 0}, {2, 0, 1}, {2, 1, 0},
	}
	for _, order := range permutations {
		t.Run(fmt.Sprintf("PATH/%d%d%d", order[0], order[1], order[2]), func(t *testing.T) {
			// RFC requirement: RFC3209-3-2 positive — all six relative permutations of EXPLICIT_ROUTE, LABEL_REQUEST and SESSION_ATTRIBUTE decode in a valid PATH with their values and sender descriptor intact.
			// MUTATION: reject LABEL_REQUEST after SESSION_ATTRIBUTE, or
			// discard EXPLICIT_ROUTE when LABEL_REQUEST has been decoded.
			encoders := append([]objEncoder(nil), pathPrefix...)
			for _, index := range order {
				encoders = append(encoders, pathObjects[index])
			}
			encoders = append(encoders,
				func(b []byte) int { return encodeSenderTemplate(b, filter) },
				func(b []byte) int { return encodeFlowSpec(b, ClassSenderTSpec, tspec) },
			)
			msg, err := DecodeMessage(encodeMessage(MsgTypePath, defaultIPTTL, encoders))
			require.NoError(t, err)
			assert.False(t, msg.HasUnknownObject)
			assert.True(t, msg.HasSession)
			assert.True(t, msg.HasHop)
			assert.True(t, msg.HasTimeValues)
			assert.True(t, msg.HasERO)
			assert.True(t, msg.HasLabelRequest)
			assert.True(t, msg.HasSessionAttr)
			assert.True(t, msg.HasSenderTemplate)
			assert.True(t, msg.HasSenderTSpec)
			assert.Equal(t, rsb.Session, msg.Session)
			assert.Equal(t, hop, msg.Hop)
			assert.Equal(t, tv, msg.TimeValues)
			assert.Equal(t, ero, msg.ERO)
			assert.Equal(t, request, msg.LabelRequest)
			assert.Equal(t, attr, msg.SessionAttr)
			assert.Equal(t, filter, msg.SenderTemplate)
			assert.Equal(t, tspec, msg.SenderTSpec)
		})
	}

	canonical := buildResv(rsb, filter, DefaultRefreshPeriod, hop.NextHop)
	objs := objectBodies(t, canonical)
	classes := []uint8{ClassSession, ClassRSVPHop, ClassTimeValues, ClassStyle, ClassFlowSpec, ClassFilterSpec, ClassLabel, ClassRecordRoute}
	require.Len(t, objs, len(classes))
	chunks := make([][]byte, len(objs))
	off := rsvpHdrLen
	for i, obj := range objs {
		require.Equal(t, classes[i], obj.ClassNum)
		end := off + int(obj.Length)
		chunks[i] = canonical[off:end]
		off = end
	}
	require.Equal(t, len(canonical), off)
	// Complete four-byte-aligned objects can be permuted without changing
	// their one's-complement checksum. Every case retains every object.
	reorder := func(order [8]int) []byte {
		raw := make([]byte, 0, len(canonical))
		raw = append(raw, canonical[:rsvpHdrLen]...)
		for _, index := range order {
			raw = append(raw, chunks[index]...)
		}
		return raw
	}
	want, err := DecodeMessage(canonical)
	require.NoError(t, err)
	require.Len(t, want.FlowDescriptors, 1)
	require.Len(t, want.FlowDescriptors[0].Filters, 1)
	for _, order := range permutations {
		t.Run(fmt.Sprintf("RESV-prefix/%d%d%d", order[0], order[1], order[2]), func(t *testing.T) {
			// RFC requirement: RFC2205-4-4 positive — all six SESSION/HOP/TIME_VALUES prefix orders are accepted with STYLE and the ordered descriptor suffix last; SESSION, STYLE, FLOWSPEC, filter, LABEL and RRO remain unchanged.
			// MUTATION: require SESSION before RSVP_HOP in
			// checkObjectPlacement, or drop the decoded filter's RRO.
			raw := reorder([8]int{order[0], order[1], order[2], 3, 4, 5, 6, 7})
			got, err := DecodeMessage(raw)
			require.NoError(t, err, "the RESV prefix order is unrestricted")
			require.Len(t, got.FlowDescriptors, 1)
			require.Len(t, got.FlowDescriptors[0].Filters, 1)
			assert.Equal(t, want.Session, got.Session)
			assert.Equal(t, want.Style, got.Style)
			assert.Equal(t, want.FlowDescriptors[0].FlowSpec, got.FlowDescriptors[0].FlowSpec)
			assert.Equal(t, want.FlowDescriptors[0].Filters[0].Filter, got.FlowDescriptors[0].Filters[0].Filter)
			assert.Equal(t, want.FlowDescriptors[0].Filters[0].Label, got.FlowDescriptors[0].Filters[0].Label)
			assert.Equal(t, want.FlowDescriptors[0].Filters[0].RRO, got.FlowDescriptors[0].Filters[0].RRO)
			assert.Equal(t, rsb.RRO, got.FlowDescriptors[0].Filters[0].RRO)
			assert.Equal(t, uint32(16050), got.FlowDescriptors[0].Filters[0].Label.Label)
		})
	}
	for _, tc := range []struct {
		name  string
		order [8]int
	}{
		{"flow-before-STYLE", [8]int{0, 1, 2, 4, 3, 5, 6, 7}},
		{"prefix-after-STYLE", [8]int{0, 1, 3, 2, 4, 5, 6, 7}},
		{"filter-before-FLOWSPEC", [8]int{0, 1, 2, 3, 5, 4, 6, 7}},
		{"LABEL-before-filter", [8]int{0, 1, 2, 3, 4, 6, 5, 7}},
		{"RRO-before-filter", [8]int{0, 1, 2, 3, 4, 7, 5, 6}},
		{"RRO-before-LABEL", [8]int{0, 1, 2, 3, 4, 5, 7, 6}},
		{"reversed", [8]int{7, 6, 5, 4, 3, 2, 1, 0}},
	} {
		t.Run("RESV-illegal/"+tc.name, func(t *testing.T) {
			// RFC requirement: RFC2205-4-4 negative — a RESV retaining every object but violating the STYLE/flow-descriptor suffix or FILTER_SPEC/LABEL/RRO order is rejected.
			// MUTATION: accept non-descriptor objects after STYLE in
			// checkObjectPlacement; prefix-after-STYLE must then fail.
			raw := reorder(tc.order)
			require.NotEqual(t, canonical, raw)
			_, err := DecodeMessage(raw)
			require.Error(t, err, "RESV descriptors are not freely reorderable")
		})
	}
}
