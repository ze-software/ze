package filter_community

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
)

// buildPayload creates an UPDATE body with the given path attributes and no NLRI.
// Format: withdrawn_len(2) + attr_len(2) + attrs.
func buildPayload(attrs []byte) []byte {
	buf := make([]byte, 4+len(attrs))
	binary.BigEndian.PutUint16(buf[2:4], uint16(len(attrs))) //nolint:gosec // test data
	copy(buf[4:], attrs)
	return buf
}

// buildCommunityAttr builds a COMMUNITY attribute (code 8) with the given 4-byte values.
func buildCommunityAttr(values ...uint32) []byte {
	dataLen := len(values) * 4
	attr := make([]byte, 4+dataLen) // flags(1) + code(1) + len(2) + data
	attr[0] = 0xC0                  // Optional Transitive
	attr[1] = byte(attribute.AttrCommunity)
	binary.BigEndian.PutUint16(attr[2:4], uint16(dataLen)) //nolint:gosec // test data
	attr[0] |= 0x10                                        // Extended length (2-byte len)
	for i, v := range values {
		binary.BigEndian.PutUint32(attr[4+i*4:], v)
	}
	return attr
}

// buildOriginAttr builds an ORIGIN attribute (code 1, value IGP=0).
func buildOriginAttr() []byte {
	return []byte{0x40, 0x01, 0x01, 0x00} // Transitive, code 1, len 1, IGP
}

// extractCommunities reads all 4-byte community values from the COMMUNITY attribute
// in a payload. Returns nil if no COMMUNITY attribute found.
func extractCommunities(payload []byte) []uint32 {
	if len(payload) < 4 {
		return nil
	}
	attrLen := int(binary.BigEndian.Uint16(payload[2:4]))
	pos := 4
	end := pos + attrLen
	if end > len(payload) {
		return nil
	}
	for pos < end {
		if pos+2 > end {
			break
		}
		flags := payload[pos]
		code := payload[pos+1]
		pos += 2
		var dataLen int
		if flags&0x10 != 0 {
			if pos+2 > end {
				break
			}
			dataLen = int(binary.BigEndian.Uint16(payload[pos : pos+2]))
			pos += 2
		} else {
			if pos+1 > end {
				break
			}
			dataLen = int(payload[pos])
			pos++
		}
		if pos+dataLen > end {
			break
		}
		if attribute.AttributeCode(code) == attribute.AttrCommunity {
			var result []uint32
			for i := 0; i+4 <= dataLen; i += 4 {
				result = append(result, binary.BigEndian.Uint32(payload[pos+i:]))
			}
			return result
		}
		pos += dataLen
	}
	return nil
}

// TestIngressTagStandard verifies that ingress tagging adds a standard community
// to an UPDATE that already has a COMMUNITY attribute.
//
// VALIDATES: AC-6, AC-15 -- communities added to existing attribute.
// PREVENTS: Tag silently doing nothing.
func TestIngressTagStandard(t *testing.T) {
	existing := buildCommunityAttr(0x0001_0001) // 1:1
	payload := buildPayload(append(buildOriginAttr(), existing...))

	tagWire := make([]byte, 4)
	binary.BigEndian.PutUint32(tagWire, 0x0002_0002) // 2:2

	modified := ingressTagCommunities(payload, attribute.AttrCommunity, [][]byte{tagWire})
	require.NotNil(t, modified, "should return modified payload")

	communities := extractCommunities(modified)
	require.Equal(t, 2, len(communities), "should have 2 communities")
	assert.Equal(t, uint32(0x0001_0001), communities[0])
	assert.Equal(t, uint32(0x0002_0002), communities[1])
}

// TestIngressTagCreatesAttribute verifies that ingress tagging creates a new
// COMMUNITY attribute when none exists in the UPDATE.
//
// VALIDATES: AC-14 -- new COMMUNITY attribute created.
// PREVENTS: Tag failing silently when attribute is absent.
func TestIngressTagCreatesAttribute(t *testing.T) {
	payload := buildPayload(buildOriginAttr()) // No COMMUNITY attr

	tagWire := make([]byte, 4)
	binary.BigEndian.PutUint32(tagWire, 0x0003_0003) // 3:3

	modified := ingressTagCommunities(payload, attribute.AttrCommunity, [][]byte{tagWire})
	require.NotNil(t, modified)

	communities := extractCommunities(modified)
	require.Equal(t, 1, len(communities))
	assert.Equal(t, uint32(0x0003_0003), communities[0])
}

// TestIngressStripStandard verifies that ingress stripping removes matching
// community values from the COMMUNITY attribute.
//
// VALIDATES: AC-9 -- matching communities removed from received UPDATE.
// PREVENTS: Strip silently leaving values in place.
func TestIngressStripStandard(t *testing.T) {
	// Payload with communities 1:1 and 2:2
	existing := buildCommunityAttr(0x0001_0001, 0x0002_0002)
	payload := buildPayload(append(buildOriginAttr(), existing...))

	// Strip 1:1
	stripWire := make([]byte, 4)
	binary.BigEndian.PutUint32(stripWire, 0x0001_0001)

	modified := ingressStripCommunities(payload, attribute.AttrCommunity, 4, [][]byte{stripWire})
	require.NotNil(t, modified)

	communities := extractCommunities(modified)
	require.Equal(t, 1, len(communities))
	assert.Equal(t, uint32(0x0002_0002), communities[0])
}

// TestStripRemovesEntireAttribute verifies that stripping all values from a
// COMMUNITY attribute removes the attribute entirely.
//
// VALIDATES: AC-16 -- empty attribute removed, not left as zero-length.
// PREVENTS: Malformed zero-length attribute in wire output.
func TestStripRemovesEntireAttribute(t *testing.T) {
	existing := buildCommunityAttr(0x0001_0001)
	payload := buildPayload(append(buildOriginAttr(), existing...))

	stripWire := make([]byte, 4)
	binary.BigEndian.PutUint32(stripWire, 0x0001_0001)

	modified := ingressStripCommunities(payload, attribute.AttrCommunity, 4, [][]byte{stripWire})
	require.NotNil(t, modified)

	// Should have no COMMUNITY attribute at all.
	communities := extractCommunities(modified)
	assert.Nil(t, communities, "all communities stripped, attribute should be absent")
}

// TestStripNoMatch verifies that stripping with no matching values returns nil
// (no modification needed).
//
// VALIDATES: AC-13 -- payload unchanged when no match.
// PREVENTS: Unnecessary payload copies.
func TestStripNoMatch(t *testing.T) {
	existing := buildCommunityAttr(0x0001_0001)
	payload := buildPayload(append(buildOriginAttr(), existing...))

	stripWire := make([]byte, 4)
	binary.BigEndian.PutUint32(stripWire, 0x9999_9999) // Not present

	modified := ingressStripCommunities(payload, attribute.AttrCommunity, 4, [][]byte{stripWire})
	assert.Nil(t, modified, "no match should return nil (no modification)")
}

// TestStripBeforeTag verifies that within the ingress filter, strip runs before tag.
//
// VALIDATES: AC-18 -- strip before tag ordering.
// PREVENTS: Tag adding a value that strip then removes.
func TestStripBeforeTag(t *testing.T) {
	// Start with community 1:1
	existing := buildCommunityAttr(0x0001_0001)
	payload := buildPayload(append(buildOriginAttr(), existing...))

	stripWire := make([]byte, 4)
	binary.BigEndian.PutUint32(stripWire, 0x0001_0001) // Strip 1:1

	tagWire := make([]byte, 4)
	binary.BigEndian.PutUint32(tagWire, 0x0002_0002) // Tag 2:2

	defs := communityDefs{
		"strip-me": &communityDef{typ: communityTypeStandard, wireValues: [][]byte{stripWire}},
		"add-me":   &communityDef{typ: communityTypeStandard, wireValues: [][]byte{tagWire}},
	}
	fc := filterConfig{
		ingressStrip: []string{"strip-me"},
		ingressTag:   []string{"add-me"},
	}

	modified := applyIngressFilter(payload, defs, fc)
	require.NotNil(t, modified)

	communities := extractCommunities(modified)
	// 1:1 stripped, 2:2 tagged. Result should be only 2:2.
	require.Equal(t, 1, len(communities))
	assert.Equal(t, uint32(0x0002_0002), communities[0])
}

// TestTagExistingCommunityNoDeDup verifies that tagging a community that already
// exists in the UPDATE does NOT deduplicate.
//
// VALIDATES: AC-17 -- no dedup on tag.
// PREVENTS: Silent deduplication changing operator intent.
func TestTagExistingCommunityNoDeDup(t *testing.T) {
	existing := buildCommunityAttr(0x0001_0001)
	payload := buildPayload(append(buildOriginAttr(), existing...))

	tagWire := make([]byte, 4)
	binary.BigEndian.PutUint32(tagWire, 0x0001_0001) // Same as existing

	modified := ingressTagCommunities(payload, attribute.AttrCommunity, [][]byte{tagWire})
	require.NotNil(t, modified)

	communities := extractCommunities(modified)
	// Should have 1:1 twice (no dedup).
	require.Equal(t, 2, len(communities))
	assert.Equal(t, uint32(0x0001_0001), communities[0])
	assert.Equal(t, uint32(0x0001_0001), communities[1])
}

// buildLargeCommunityAttr builds a LARGE_COMMUNITY attribute (code 32) with 12-byte values.
func buildLargeCommunityAttr(values ...[3]uint32) []byte {
	dataLen := len(values) * 12
	attr := make([]byte, 4+dataLen) // flags(1)+code(1)+extlen(2)+data
	attr[0] = 0xC0 | 0x10           // Optional Transitive + Extended Length
	attr[1] = byte(attribute.AttrLargeCommunity)
	binary.BigEndian.PutUint16(attr[2:4], uint16(dataLen)) //nolint:gosec // test data
	for i, v := range values {
		off := 4 + i*12
		binary.BigEndian.PutUint32(attr[off:], v[0])
		binary.BigEndian.PutUint32(attr[off+4:], v[1])
		binary.BigEndian.PutUint32(attr[off+8:], v[2])
	}
	return attr
}

// TestIngressTagLargeCommunity verifies ingress tagging with 12-byte large communities.
//
// VALIDATES: Large community tag creates correct attribute with code 32.
// PREVENTS: Value size mismatch breaking large community encoding.
func TestIngressTagLargeCommunity(t *testing.T) {
	payload := buildPayload(buildOriginAttr()) // No large community attr

	tagWire := make([]byte, 12)
	binary.BigEndian.PutUint32(tagWire[0:4], 65000)
	binary.BigEndian.PutUint32(tagWire[4:8], 1)
	binary.BigEndian.PutUint32(tagWire[8:12], 2)

	modified := ingressTagCommunities(payload, attribute.AttrLargeCommunity, [][]byte{tagWire})
	require.NotNil(t, modified)

	// Verify large community attribute was created.
	_, _, dataStart, dataEnd, found := findAttribute(modified, attribute.AttrLargeCommunity)
	require.True(t, found, "large community attribute should be created")
	assert.Equal(t, 12, dataEnd-dataStart, "should have 12 bytes (1 large community)")
}

// TestIngressStripLargeCommunity verifies ingress stripping of large communities.
//
// VALIDATES: Large community strip removes correct 12-byte value.
// PREVENTS: Value size mismatch causing incorrect removal.
func TestIngressStripLargeCommunity(t *testing.T) {
	lc1 := [3]uint32{65000, 1, 1}
	lc2 := [3]uint32{65000, 2, 2}
	existing := buildLargeCommunityAttr(lc1, lc2)
	payload := buildPayload(append(buildOriginAttr(), existing...))

	stripWire := make([]byte, 12)
	binary.BigEndian.PutUint32(stripWire[0:4], 65000)
	binary.BigEndian.PutUint32(stripWire[4:8], 1)
	binary.BigEndian.PutUint32(stripWire[8:12], 1)

	modified := ingressStripCommunities(payload, attribute.AttrLargeCommunity, 12, [][]byte{stripWire})
	require.NotNil(t, modified)

	// Should have 1 large community remaining (65000:2:2).
	_, _, dataStart, dataEnd, found := findAttribute(modified, attribute.AttrLargeCommunity)
	require.True(t, found)
	assert.Equal(t, 12, dataEnd-dataStart, "should have 12 bytes (1 large community remaining)")
}
