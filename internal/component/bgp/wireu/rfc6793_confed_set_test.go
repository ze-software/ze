// VALIDATES: RFC 6793 Section 4.2.2 for the AS_CONFED_SET half: when the AS path
// information holds an AS_CONFED_SET (type 4), the AS4_PATH that TranscodeASPath
// constructs for an OLD speaker leaves it out, as it does an AS_CONFED_SEQUENCE.
// PREVENTS: a confederation set leaking into AS4_PATH, which the sibling unit
// TestRFC6793ConstructedAS4PathExcludesConfed cannot see: it holds only a
// confederation sequence.

package wireu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// rfc6793TranscodeToOld builds an UPDATE whose four-octet AS_PATH holds segments,
// transcodes it for an OLD speaker, and returns the AS4_PATH and AS_PATH it wrote.
func rfc6793TranscodeToOld(t *testing.T, segments []attribute.ASPathSegment) (as4 *attribute.AS4Path, asPath *attribute.ASPath) {
	t.Helper()
	payload := buildPayload(nil, concatAttrs(buildOriginAttr(), buildASPathAttr(segments, true)), nil)

	dst := make([]byte, len(payload)+256)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	result := dst[:n]

	as4 = parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4, "a non-mappable AS in the path obliges an AS4_PATH")
	return as4, parseASPathFromPayload(t, result)
}

// TestRFC6793ConstructedAS4PathExcludesConfedSet holds an AS_CONFED_SET and an
// AS_CONFED_SEQUENCE before an AS_SEQUENCE carrying a non-mappable AS.
func TestRFC6793ConstructedAS4PathExcludesConfedSet(t *testing.T) {
	// RFC requirement: RFC6793-4.2.2-4 positive -- with an AS_CONFED_SET {64514, 64515} and an AS_CONFED_SEQUENCE [64512] in the AS path information, the constructed AS4_PATH holds neither: it is exactly one AS_SEQUENCE [rfc6793NonMappable, 65001], while the two-octet AS_PATH still carries all three segments
	as4, asPath := rfc6793TranscodeToOld(t, []attribute.ASPathSegment{
		{Type: attribute.ASConfedSet, ASNs: []uint32{64514, 64515}},
		{Type: attribute.ASConfedSequence, ASNs: []uint32{64512}},
		{Type: attribute.ASSequence, ASNs: []uint32{rfc6793NonMappable, 65001}},
	})

	require.Len(t, as4.Segments, 1, "only the ordinary segment reaches AS4_PATH")
	assert.Equal(t, attribute.ASSequence, as4.Segments[0].Type)
	assert.Equal(t, []uint32{rfc6793NonMappable, 65001}, as4.Segments[0].ASNs)

	require.Len(t, asPath.Segments, 3, "AS_PATH keeps the confederation segments")
	assert.Equal(t, attribute.ASConfedSet, asPath.Segments[0].Type)
	assert.Equal(t, attribute.ASConfedSequence, asPath.Segments[1].Type)
}

// TestRFC6793ConstructedAS4PathKeepsAnOrdinarySet holds an ordinary AS_SET in the
// same position the confederation set took above.
func TestRFC6793ConstructedAS4PathKeepsAnOrdinarySet(t *testing.T) {
	// RFC requirement: RFC6793-4.2.2-4 negative -- the exclusion is scoped to the two confederation types: an ordinary AS_SET {64514, 64515} in the same position is carried into the AS4_PATH, ahead of the AS_SEQUENCE [rfc6793NonMappable, 65001]
	as4, _ := rfc6793TranscodeToOld(t, []attribute.ASPathSegment{
		{Type: attribute.ASSet, ASNs: []uint32{64514, 64515}},
		{Type: attribute.ASSequence, ASNs: []uint32{rfc6793NonMappable, 65001}},
	})

	require.Len(t, as4.Segments, 2, "an ordinary set is AS path information AS4_PATH carries")
	assert.Equal(t, attribute.ASSet, as4.Segments[0].Type)
	assert.Equal(t, []uint32{64514, 64515}, as4.Segments[0].ASNs)
	assert.Equal(t, attribute.ASSequence, as4.Segments[1].Type)
}
