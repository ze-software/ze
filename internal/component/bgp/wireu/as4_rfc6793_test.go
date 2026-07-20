// RFC: rfc/short/rfc6793.md — AS4_PATH / AS4_AGGREGATOR egress construction
//
// Requirement-bound tests for RFC 6793 over the two wireu egress paths
// (TranscodeASPath, RewriteASPath) and the shared rule in aspath_as4.go.

package wireu

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
)

// rfc6793NonMappable and rfc6793NonMappableB cannot be represented in two
// octets, so a two-octet AS_PATH must substitute AS_TRANS for them.
const (
	rfc6793NonMappable  uint32 = 4200000001
	rfc6793NonMappableB uint32 = 4200000002
	rfc6793ASTrans      uint32 = 23456
)

// rawAS4PathAttr wraps arbitrary (possibly malformed) bytes in an AS4_PATH
// attribute header, bypassing the encoder so malformed values can be injected.
func rawAS4PathAttr(value []byte) []byte {
	buf := make([]byte, 3+len(value))
	attribute.WriteHeaderTo(buf, 0,
		attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAS4Path, uint16(len(value))) //nolint:gosec // test data
	copy(buf[3:], value)
	return buf
}

// flattenAS4 returns the (type, ASNs) shape of an AS4_PATH for assertions.
func flattenAS4(p *attribute.AS4Path) []attribute.ASPathSegment {
	if p == nil {
		return nil
	}
	return p.Segments
}

// TestRFC6793TranscodeEmitsAS4PathForNonMappable drives TranscodeASPath 4->2
// with a non-mappable AS in the path. as4PathForPath (aspath_as4.go) owns the
// rule and returns the AS4_PATH that writeAS4PathAttr then emits.
//
// RFC requirement: RFC6793-4.2.2-2 positive -- sending to an OLD speaker a path containing a
// non-mappable four-octet AS, the AS_PATH carries AS_TRANS and an AS4_PATH carrying the real
// four-octet AS numbers is also sent.
// RFC requirement: RFC6793-4.2.2-3 negative -- the AS4_PATH suppression is conditional: a path
// that is NOT composed of mappable AS numbers only does get an AS4_PATH.
func TestRFC6793TranscodeEmitsAS4PathForNonMappable(t *testing.T) {
	attrs := concatAttrs(
		buildOriginAttr(),
		buildASPathAttr([]attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, rfc6793NonMappable}},
		}, true),
	)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+256)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	result := dst[:n]

	asPath := parseASPathFromPayload(t, result, false)
	require.Len(t, asPath.Segments, 1)
	assert.Equal(t, []uint32{65001, rfc6793ASTrans}, asPath.Segments[0].ASNs,
		"two-octet AS_PATH substitutes AS_TRANS for the non-mappable AS")

	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4, "AS4_PATH MUST be sent when the path holds a non-mappable AS")
	require.Len(t, as4.Segments, 1)
	assert.Equal(t, []uint32{65001, rfc6793NonMappable}, as4.Segments[0].ASNs)
}

// TestRFC6793TranscodeOmitsAS4PathWhenAllMappable drives the same path with
// only mappable AS numbers.
//
// RFC requirement: RFC6793-4.2.2-3 positive -- when all of the AS path information is composed
// of mappable four-octet AS numbers only, no AS4_PATH attribute is sent to the OLD speaker.
// RFC requirement: RFC6793-4.2.2-2 negative -- the AS4_PATH is not emitted unconditionally for
// every OLD-speaker UPDATE: with every AS mappable the attribute is absent.
func TestRFC6793TranscodeOmitsAS4PathWhenAllMappable(t *testing.T) {
	attrs := concatAttrs(
		buildOriginAttr(),
		buildASPathAttr([]attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65535}},
		}, true),
	)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+256)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	result := dst[:n]

	asPath := parseASPathFromPayload(t, result, false)
	require.Len(t, asPath.Segments, 1)
	assert.Equal(t, []uint32{65001, 65535}, asPath.Segments[0].ASNs)

	assert.Nil(t, parseAS4PathFromPayload(t, result),
		"AS4_PATH MUST NOT be sent when every AS is mappable")
}

// TestRFC6793NoAS4PathBetweenNewSpeakers drives RewriteASPath toward a peer that
// negotiated the four-octet capability with a non-mappable local AS to prepend.
//
// With the four-octet capability negotiated on both sides the outgoing UPDATE carries
// neither AS4_PATH nor AS4_AGGREGATOR; the real four-octet AS numbers ride in AS_PATH
// and AGGREGATOR themselves.
//
// This proves only that ze does not ORIGINATE either attribute toward a NEW peer: the
// input built below carries neither, so nothing here exercises the forwarding path.
// RFC6793-4.1-6 also forbids CARRYING a received one, which ze does (see the {gap} on
// RFC6793-4.1-6 in rfc/short/rfc6793.md), so this test carries no requirement tag.
func TestRFC6793NoAS4PathBetweenNewSpeakers(t *testing.T) {
	aggAddr := netip.MustParseAddr("192.0.2.9")
	attrs := concatAttrs(
		buildOriginAttr(),
		buildASPathAttr([]attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{rfc6793NonMappableB}},
		}, true),
		buildAggregatorAttr(rfc6793NonMappable, aggAddr, true),
	)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+256)
	n, err := RewriteASPath(dst, payload, rfc6793NonMappable, true, true)
	require.NoError(t, err)
	result := dst[:n]

	asPath := parseASPathFromPayload(t, result, true)
	require.Len(t, asPath.Segments, 1)
	assert.Equal(t, []uint32{rfc6793NonMappable, rfc6793NonMappableB}, asPath.Segments[0].ASNs)

	assert.Nil(t, parseAS4PathFromPayload(t, result),
		"AS4_PATH MUST NOT be carried between NEW BGP speakers")
	_, _, found := parseAS4AggregatorFromPayload(t, result)
	assert.False(t, found, "AS4_AGGREGATOR MUST NOT be carried between NEW BGP speakers")

	aggASN, aggLen, ok := parseAggregatorFromPayload(t, result)
	require.True(t, ok)
	assert.Equal(t, 8, aggLen)
	assert.Equal(t, rfc6793NonMappable, aggASN)
}

// TestRFC6793AS4AggregatorForNonMappableAggregator drives the AGGREGATOR
// transcoding branch of TranscodeASPath (aspath_transcode.go) with a
// non-mappable aggregating AS.
//
// RFC requirement: RFC6793-4.2.2-5 positive -- when the aggregating AS is non-mappable the
// speaker sends AS4_AGGREGATOR with the real four-octet AS and sets the AS number field of the
// existing AGGREGATOR to AS_TRANS.
// RFC requirement: RFC6793-4.2.2-6 negative -- the AS4_AGGREGATOR suppression is conditional:
// a non-mappable aggregating AS does produce the attribute.
func TestRFC6793AS4AggregatorForNonMappableAggregator(t *testing.T) {
	aggAddr := netip.MustParseAddr("10.1.2.3")
	attrs := concatAttrs(
		buildOriginAttr(),
		buildASPathAttr([]attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		}, true),
		buildAggregatorAttr(rfc6793NonMappable, aggAddr, true),
	)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+256)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	result := dst[:n]

	aggASN, aggLen, ok := parseAggregatorFromPayload(t, result)
	require.True(t, ok, "AGGREGATOR must still be present")
	assert.Equal(t, 6, aggLen, "two-octet AGGREGATOR toward an OLD speaker")
	assert.Equal(t, rfc6793ASTrans, aggASN, "AGGREGATOR AS field set to AS_TRANS")

	as4ASN, as4Addr, found := parseAS4AggregatorFromPayload(t, result)
	require.True(t, found, "AS4_AGGREGATOR MUST be used for a non-mappable aggregating AS")
	assert.Equal(t, rfc6793NonMappable, as4ASN)
	assert.Equal(t, aggAddr, as4Addr)
}

// TestRFC6793NoAS4AggregatorForMappableAggregator drives the same branch with a
// mappable aggregating AS.
//
// RFC requirement: RFC6793-4.2.2-6 positive -- when the aggregating AS is mappable the
// AS4_AGGREGATOR attribute is not sent and the two-octet AGGREGATOR carries the real AS.
// RFC requirement: RFC6793-4.2.2-5 negative -- the AS_TRANS substitution is scoped to
// non-mappable aggregating AS numbers: a mappable one is left intact in AGGREGATOR.
func TestRFC6793NoAS4AggregatorForMappableAggregator(t *testing.T) {
	aggAddr := netip.MustParseAddr("10.1.2.3")
	attrs := concatAttrs(
		buildOriginAttr(),
		buildASPathAttr([]attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		}, true),
		buildAggregatorAttr(65010, aggAddr, true),
	)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+256)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	result := dst[:n]

	aggASN, aggLen, ok := parseAggregatorFromPayload(t, result)
	require.True(t, ok)
	assert.Equal(t, 6, aggLen)
	assert.Equal(t, uint32(65010), aggASN)
	assert.NotEqual(t, rfc6793ASTrans, aggASN)

	_, _, found := parseAS4AggregatorFromPayload(t, result)
	assert.False(t, found, "AS4_AGGREGATOR MUST NOT be sent for a mappable aggregating AS")
}

// TestRFC6793ConstructedAS4PathExcludesConfed drives TranscodeASPath 4->2 with a
// path holding both a confederation segment and an ordinary segment.
//
// RFC requirement: RFC6793-4.2.2-4 positive -- the AS_CONFED_SEQUENCE segment present in the
// AS path information is excluded from the AS4_PATH attribute being constructed.
// RFC requirement: RFC6793-4.2.2-4 negative -- the exclusion is scoped to confederation
// segments: the ordinary AS_SEQUENCE from the same path IS carried in the AS4_PATH, and the
// confederation segment still rides in the two-octet AS_PATH.
func TestRFC6793ConstructedAS4PathExcludesConfed(t *testing.T) {
	attrs := concatAttrs(
		buildOriginAttr(),
		buildASPathAttr([]attribute.ASPathSegment{
			{Type: attribute.ASConfedSequence, ASNs: []uint32{64512, 64513}},
			{Type: attribute.ASSequence, ASNs: []uint32{rfc6793NonMappable, 65001}},
		}, true),
	)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+256)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	result := dst[:n]

	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4)
	for _, seg := range flattenAS4(as4) {
		assert.NotEqual(t, attribute.ASConfedSequence, seg.Type)
		assert.NotEqual(t, attribute.ASConfedSet, seg.Type)
	}
	require.Len(t, as4.Segments, 1)
	assert.Equal(t, attribute.ASSequence, as4.Segments[0].Type)
	assert.Equal(t, []uint32{rfc6793NonMappable, 65001}, as4.Segments[0].ASNs)

	asPath := parseASPathFromPayload(t, result, false)
	require.Len(t, asPath.Segments, 2)
	assert.Equal(t, attribute.ASConfedSequence, asPath.Segments[0].Type,
		"the confederation segment stays in AS_PATH, only AS4_PATH excludes it")
}

// TestRFC6793MalformedAS4PathDiscarded feeds RewriteASPath an UPDATE from an OLD
// speaker whose AS4_PATH is malformed (odd length). rewritePrependASPathFull
// (aspath_rewrite.go) parses it, drops it on error, and keeps going.
//
// RFC requirement: RFC6793-6-4 positive -- a malformed AS4_PATH received from an OLD speaker
// is discarded and the UPDATE continues to be processed: the rewrite succeeds, the AS_PATH is
// prepended, and the malformed octets never reach the emitted AS4_PATH.
func TestRFC6793MalformedAS4PathDiscarded(t *testing.T) {
	// Odd-length AS4_PATH value: malformed per RFC 6793 Section 6.
	malformed := rawAS4PathAttr([]byte{0x02, 0x01, 0x00, 0x00, 0xFD})
	attrs := concatAttrs(
		buildOriginAttr(),
		buildASPathAttr([]attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		}, false),
		malformed,
	)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+256)
	// Non-mappable local AS forces the slow path that parses the received AS4_PATH.
	n, err := RewriteASPath(dst, payload, rfc6793NonMappable, false, false)
	require.NoError(t, err, "UPDATE processing continues despite the malformed AS4_PATH")
	result := dst[:n]

	asPath := parseASPathFromPayload(t, result, false)
	require.Len(t, asPath.Segments, 1)
	assert.Equal(t, []uint32{rfc6793ASTrans, 65001}, asPath.Segments[0].ASNs)

	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4, "the locally constructed AS4_PATH replaces the discarded one")
	require.Len(t, as4.Segments, 1)
	assert.Equal(t, []uint32{rfc6793NonMappable, 65001}, as4.Segments[0].ASNs,
		"the discarded AS4_PATH contributed nothing")
}

// TestRFC6793WellFormedAS4PathNotDiscarded is the counterpart: a well-formed
// AS4_PATH from an OLD speaker is kept and the local AS is prepended to it.
//
// RFC requirement: RFC6793-6-4 negative -- the discard is scoped to malformed attributes: a
// well-formed AS4_PATH received from an OLD speaker is NOT discarded, its four-octet AS
// numbers survive into the AS4_PATH sent onward.
func TestRFC6793WellFormedAS4PathNotDiscarded(t *testing.T) {
	received := buildAS4PathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{rfc6793NonMappableB, 65001}},
	})
	attrs := concatAttrs(
		buildOriginAttr(),
		buildASPathAttr([]attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{rfc6793ASTrans, 65001}},
		}, false),
		received,
	)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+256)
	n, err := RewriteASPath(dst, payload, rfc6793NonMappable, false, false)
	require.NoError(t, err)
	result := dst[:n]

	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4)
	require.Len(t, as4.Segments, 1)
	assert.Equal(t, []uint32{rfc6793NonMappable, rfc6793NonMappableB, 65001},
		as4.Segments[0].ASNs)
}

// TestRFC6793ReceivedConfedInAS4PathDiscarded feeds RewriteASPath an AS4_PATH
// from an OLD speaker that illegally carries an AS_CONFED_SEQUENCE segment.
//
// RFC requirement: RFC6793-6-3 positive -- AS_CONFED_SEQUENCE / AS_CONFED_SET path segments
// received in an AS4_PATH are discarded, the attribute length is adjusted to match the
// remaining segments, and the UPDATE continues to be processed.
// RFC requirement: RFC6793-6-3 negative -- the discard is scoped to the confederation segment
// types: the AS_SEQUENCE segment carried alongside them in the same received AS4_PATH is
// retained rather than dropped with them.
func TestRFC6793ReceivedConfedInAS4PathDiscarded(t *testing.T) {
	// Encode the confed-bearing AS4_PATH by hand: the encoder itself drops confed.
	received := rawAS4PathAttr([]byte{
		0x03, 0x02, // AS_CONFED_SEQUENCE, 2 ASNs
		0x00, 0x00, 0xFC, 0x00, // 64512
		0x00, 0x00, 0xFC, 0x01, // 64513
		0x02, 0x01, // AS_SEQUENCE, 1 ASN
		0xFA, 0x56, 0xEA, 0x02, // 4200000002
	})
	attrs := concatAttrs(
		buildOriginAttr(),
		buildASPathAttr([]attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{rfc6793ASTrans}},
		}, false),
		received,
	)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+256)
	n, err := RewriteASPath(dst, payload, rfc6793NonMappable, false, false)
	require.NoError(t, err, "UPDATE processing continues")
	result := dst[:n]

	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4)
	for _, seg := range as4.Segments {
		assert.NotEqual(t, attribute.ASConfedSequence, seg.Type)
		assert.NotEqual(t, attribute.ASConfedSet, seg.Type)
		assert.NotContains(t, seg.ASNs, uint32(64512))
		assert.NotContains(t, seg.ASNs, uint32(64513))
	}

	var got []uint32
	for _, seg := range as4.Segments {
		got = append(got, seg.ASNs...)
	}
	assert.Equal(t, []uint32{rfc6793NonMappable, rfc6793NonMappableB}, got,
		"the non-confederation segment survives")
}
