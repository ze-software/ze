// RFC: rfc/short/rfc6793.md — four-octet AS number space (codec obligations)
//
// Requirement-bound tests for RFC 6793. Each test carries an
// "RFC requirement: RFC6793-<sec>-<n> <polarity>" tag consumed by
// scripts/dev/rfc_requirements.py.

package attribute_test

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
)

// nonMappableASN has a non-zero high half, so it cannot be carried in a
// two-octet AS_PATH (RFC 6793 Section 4.2.1 "mappable" definition).
const nonMappableASN uint32 = 4200000001

// asTransWire is the reserved two-octet AS number (RFC 6793 Section 9).
const asTransWire uint16 = 23456

// TestRFC6793EncodeFourOctetToNewSpeaker proves that once the four-octet AS
// capability is negotiated, AS_PATH and AGGREGATOR are laid out with four-octet
// AS numbers by ASPath.WriteToWithContext (aspath.go WriteToWithASN4) and
// Aggregator.WriteToWithContext (simple.go).
//
// RFC requirement: RFC6793-4.1-4 positive -- with a four-octet destination context the
// AS_PATH segment uses a 4-byte stride carrying the real non-mappable AS, and AGGREGATOR
// is the 8-octet form carrying the real four-octet AS.
func TestRFC6793EncodeFourOctetToNewSpeaker(t *testing.T) {
	t.Parallel()
	ctx4 := ctxASN4

	path := &attribute.ASPath{Segments: []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{65001, nonMappableASN}},
	}}
	buf := make([]byte, 64)
	n := path.WriteToWithContext(buf, 0, ctx4, ctx4)
	// type(1) + count(1) + 2*4
	require.Equal(t, 10, n)
	require.Equal(t, byte(attribute.ASSequence), buf[0])
	require.Equal(t, byte(2), buf[1])
	assert.Equal(t, uint32(65001), binary.BigEndian.Uint32(buf[2:6]))
	assert.Equal(t, nonMappableASN, binary.BigEndian.Uint32(buf[6:10]))

	agg := &attribute.Aggregator{ASN: nonMappableASN, Address: netip.MustParseAddr("192.0.2.7")}
	require.Equal(t, 8, agg.LenWithContext(ctx4, ctx4))
	n = agg.WriteToWithContext(buf, 0, ctx4, ctx4)
	require.Equal(t, 8, n)
	assert.Equal(t, nonMappableASN, binary.BigEndian.Uint32(buf[0:4]))
	assert.Equal(t, []byte{192, 0, 2, 7}, buf[4:8])
}

// TestRFC6793EncodeTwoOctetToOldSpeaker proves the four-octet layout is
// conditional on the negotiated capability: toward an OLD speaker the same
// AS_PATH and AGGREGATOR are emitted with two-octet AS numbers, substituting
// AS_TRANS for the non-mappable AS.
//
// RFC requirement: RFC6793-4.1-4 negative -- the four-octet encoding is NOT emitted
// unconditionally: with a non-ASN4 destination context the same ASPath/Aggregator produce
// the two-octet forms, so the four-octet layout is bound to the negotiated capability.
// RFC requirement: RFC6793-4.2.2-1 positive -- toward an OLD speaker the AS path is sent in
// AS_PATH encoded with two-octet AS numbers, the non-mappable AS appearing as AS_TRANS.
func TestRFC6793EncodeTwoOctetToOldSpeaker(t *testing.T) {
	t.Parallel()

	path := &attribute.ASPath{Segments: []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{65001, nonMappableASN}},
	}}
	buf := make([]byte, 64)
	n := path.WriteToWithContext(buf, 0, ctxASN4, ctxASN2)
	// type(1) + count(1) + 2*2
	require.Equal(t, 6, n)
	assert.Equal(t, uint16(65001), binary.BigEndian.Uint16(buf[2:4]))
	assert.Equal(t, asTransWire, binary.BigEndian.Uint16(buf[4:6]))

	agg := &attribute.Aggregator{ASN: nonMappableASN, Address: netip.MustParseAddr("192.0.2.7")}
	require.Equal(t, 6, agg.LenWithContext(ctxASN4, ctxASN2))
	n = agg.WriteToWithContext(buf, 0, ctxASN4, ctxASN2)
	require.Equal(t, 6, n)
	assert.Equal(t, asTransWire, binary.BigEndian.Uint16(buf[0:2]))
}

// TestRFC6793TwoOctetASPathKeepsMappableASNs proves the AS_TRANS substitution in
// the two-octet AS_PATH is scoped to non-mappable AS numbers.
//
// RFC requirement: RFC6793-4.2.2-1 negative -- a mappable AS is NOT replaced by AS_TRANS in
// the two-octet AS_PATH: only an AS above 65535 is substituted, so the two-octet encoding
// preserves every AS it can represent.
func TestRFC6793TwoOctetASPathKeepsMappableASNs(t *testing.T) {
	t.Parallel()
	path := &attribute.ASPath{Segments: []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{65535, 1}},
	}}
	buf := make([]byte, 64)
	n := path.WriteToWithContext(buf, 0, ctxASN4, ctxASN2)
	require.Equal(t, 6, n)
	assert.Equal(t, uint16(65535), binary.BigEndian.Uint16(buf[2:4]))
	assert.Equal(t, uint16(1), binary.BigEndian.Uint16(buf[4:6]))
	assert.NotEqual(t, asTransWire, binary.BigEndian.Uint16(buf[2:4]))
}

// TestRFC6793DecodeFourOctetWhenNegotiated proves the decoders read AS_PATH and
// AGGREGATOR as four-octet entities when the capability is negotiated
// (ParseASPath fourByte=true, ParseAggregator fourByteAS=true).
//
// RFC requirement: RFC6793-4.1-5 positive -- with the capability negotiated the receiver reads
// AS_PATH ASNs on a 4-byte stride and accepts the 8-octet AGGREGATOR, recovering the real
// four-octet AS numbers.
func TestRFC6793DecodeFourOctetWhenNegotiated(t *testing.T) {
	t.Parallel()
	// AS_SEQUENCE, 1 ASN, 4200000001.
	data := []byte{0x02, 0x01, 0xFA, 0x56, 0xEA, 0x01}
	path, err := attribute.ParseASPath(data, true)
	require.NoError(t, err)
	require.Len(t, path.Segments, 1)
	require.Len(t, path.Segments[0].ASNs, 1)
	assert.Equal(t, nonMappableASN, path.Segments[0].ASNs[0])

	aggData := []byte{0xFA, 0x56, 0xEA, 0x01, 10, 0, 0, 1}
	agg, err := attribute.ParseAggregator(aggData, true)
	require.NoError(t, err)
	assert.Equal(t, nonMappableASN, agg.ASN)
}

// TestRFC6793DecodeTwoOctetWhenNotNegotiated proves the four-octet assumption is
// bound to the negotiation rather than applied unconditionally: without the
// capability the same bytes decode on a two-octet stride and the 8-octet
// AGGREGATOR is rejected as the wrong length.
//
// RFC requirement: RFC6793-4.1-5 negative -- without the capability the receiver does NOT
// assume four-octet entities: the identical AS_PATH bytes decode as two two-octet ASNs and
// an 8-octet AGGREGATOR is rejected with ErrInvalidLength (the 6-octet form is required).
func TestRFC6793DecodeTwoOctetWhenNotNegotiated(t *testing.T) {
	t.Parallel()
	// Same bytes as the four-octet case, but a 2-ASN two-octet sequence.
	data := []byte{0x02, 0x02, 0xFA, 0x56, 0xEA, 0x01}
	path, err := attribute.ParseASPath(data, false)
	require.NoError(t, err)
	require.Len(t, path.Segments, 1)
	require.Len(t, path.Segments[0].ASNs, 2)
	assert.Equal(t, uint32(0xFA56), path.Segments[0].ASNs[0])
	assert.Equal(t, uint32(0xEA01), path.Segments[0].ASNs[1])

	aggData := []byte{0xFA, 0x56, 0xEA, 0x01, 10, 0, 0, 1}
	_, err = attribute.ParseAggregator(aggData, false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, attribute.ErrInvalidLength))

	agg, err := attribute.ParseAggregator([]byte{0x5B, 0xA0, 10, 0, 0, 1}, false)
	require.NoError(t, err)
	assert.Equal(t, uint32(asTransWire), agg.ASN)
}

// TestRFC6793AS4PathWellFormedAccepted drives ParseAS4Path (as4.go) with the
// shapes RFC 6793 Section 6 declares well formed.
//
// RFC requirement: RFC6793-6-1 positive -- an AS4_PATH whose length is a multiple of two,
// at least 6 octets, with non-zero segment lengths consistent with the attribute length and
// defined segment types, parses into the four-octet AS numbers it carries.
func TestRFC6793AS4PathWellFormedAccepted(t *testing.T) {
	t.Parallel()
	data := []byte{
		0x02, 0x02, // AS_SEQUENCE, 2 ASNs
		0xFA, 0x56, 0xEA, 0x01,
		0x00, 0x00, 0xFD, 0xE9,
		0x01, 0x01, // AS_SET, 1 ASN
		0x00, 0x00, 0xFD, 0xEA,
	}
	path, err := attribute.ParseAS4Path(data)
	require.NoError(t, err)
	require.Len(t, path.Segments, 2)
	assert.Equal(t, attribute.ASSequence, path.Segments[0].Type)
	assert.Equal(t, []uint32{nonMappableASN, 65001}, path.Segments[0].ASNs)
	assert.Equal(t, attribute.ASSet, path.Segments[1].Type)
	assert.Equal(t, []uint32{65002}, path.Segments[1].ASNs)
}

// TestRFC6793AS4PathMalformedRejected drives every malformed condition listed in
// RFC 6793 Section 6 through ParseAS4Path.
//
// RFC requirement: RFC6793-6-1 negative -- an AS4_PATH whose length is not a multiple of two,
// is too small to carry one AS number, has a zero or inconsistent segment length, or carries
// an undefined segment type is rejected as malformed rather than parsed.
func TestRFC6793AS4PathMalformedRejected(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"odd length", []byte{0x02, 0x01, 0x00, 0x00, 0xFD}, attribute.ErrInvalidLength},
		{"too small for one AS number", []byte{0x02, 0x01, 0x00, 0x00}, attribute.ErrShortData},
		{"zero segment length", []byte{0x02, 0x00}, attribute.ErrInvalidLength},
		{"segment length overruns attribute", []byte{0x02, 0x03, 0x00, 0x00, 0xFD, 0xE9}, attribute.ErrShortData},
		{"undefined segment type", []byte{0x05, 0x01, 0x00, 0x00, 0xFD, 0xE9}, attribute.ErrMalformedValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := attribute.ParseAS4Path(tc.data)
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.want), "got %v, want %v", err, tc.want)
		})
	}
}

// TestRFC6793AS4AggregatorLengthEight drives ParseAS4Aggregator (as4.go) at and
// away from the single legal length.
//
// RFC requirement: RFC6793-6-2 positive -- an AS4_AGGREGATOR of exactly 8 octets is well
// formed and yields the four-octet AS and the aggregator's IPv4 identifier.
// RFC requirement: RFC6793-6-2 negative -- an AS4_AGGREGATOR whose length is not 8 (0, 6, 7,
// 9, 12 octets) is malformed and rejected, including the 6-octet RFC 4271 AGGREGATOR shape.
func TestRFC6793AS4AggregatorLengthEight(t *testing.T) {
	t.Parallel()
	good := []byte{0xFA, 0x56, 0xEA, 0x01, 10, 0, 0, 1}
	agg, err := attribute.ParseAS4Aggregator(good)
	require.NoError(t, err)
	assert.Equal(t, nonMappableASN, agg.ASN)
	assert.Equal(t, "10.0.0.1", agg.Address.String())
	assert.Equal(t, 8, agg.Len())

	for _, bad := range [][]byte{
		{},
		{0xFA, 0x56, 0xEA, 0x01, 10, 0},
		{0xFA, 0x56, 0xEA, 0x01, 10, 0, 0},
		{0xFA, 0x56, 0xEA, 0x01, 10, 0, 0, 1, 0},
		{0xFA, 0x56, 0xEA, 0x01, 10, 0, 0, 1, 0, 0, 0, 0},
	} {
		_, err := attribute.ParseAS4Aggregator(bad)
		require.Error(t, err, "length %d must be malformed", len(bad))
		assert.True(t, errors.Is(err, attribute.ErrInvalidLength))
	}
}

// TestRFC6793AS4PathWireExcludesConfed proves the AS4_PATH encoder never puts a
// confederation segment on the wire: AS4Path.WriteTo and AS4Path.Len both skip
// AS_CONFED_SEQUENCE and AS_CONFED_SET (as4.go), so the emitted attribute length
// matches the emitted bytes.
//
// RFC requirement: RFC6793-3-1 positive -- an AS4Path holding AS_CONFED_SEQUENCE and
// AS_CONFED_SET segments encodes without them, and Len() agrees with the bytes written so
// the attribute length stays consistent.
// RFC requirement: RFC6793-3-1 negative -- the exclusion is scoped to the confederation
// types: the AS_SEQUENCE and AS_SET segments in the same path ARE carried, so the encoder
// does not simply drop segments.
func TestRFC6793AS4PathWireExcludesConfed(t *testing.T) {
	t.Parallel()
	path := &attribute.AS4Path{Segments: []attribute.ASPathSegment{
		{Type: attribute.ASConfedSequence, ASNs: []uint32{64512, 64513}},
		{Type: attribute.ASSequence, ASNs: []uint32{nonMappableASN}},
		{Type: attribute.ASConfedSet, ASNs: []uint32{64514}},
		{Type: attribute.ASSet, ASNs: []uint32{65001}},
	}}

	buf := make([]byte, 256)
	n := path.WriteTo(buf, 0)
	require.Equal(t, path.Len(), n, "Len must match the bytes actually written")

	parsed, err := attribute.ParseAS4Path(buf[:n])
	require.NoError(t, err)
	require.Len(t, parsed.Segments, 2)
	assert.Equal(t, attribute.ASSequence, parsed.Segments[0].Type)
	assert.Equal(t, []uint32{nonMappableASN}, parsed.Segments[0].ASNs)
	assert.Equal(t, attribute.ASSet, parsed.Segments[1].Type)
	assert.Equal(t, []uint32{65001}, parsed.Segments[1].ASNs)
	for _, seg := range parsed.Segments {
		assert.NotEqual(t, attribute.ASConfedSequence, seg.Type)
		assert.NotEqual(t, attribute.ASConfedSet, seg.Type)
	}
}
