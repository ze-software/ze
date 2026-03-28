package filter_community

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// buildFullCommunityAttr builds a full COMMUNITY attribute (flags+code+extlen+data) for handler tests.
// Matches the format that buildModifiedPayload passes to AttrModHandlers.
func buildFullCommunityAttr(data []byte) []byte {
	attr := make([]byte, 4+len(data))
	attr[0] = 0xC0 | 0x10 // Optional Transitive + Extended Length
	attr[1] = byte(attribute.AttrCommunity)
	binary.BigEndian.PutUint16(attr[2:4], uint16(len(data))) //nolint:gosec // test data
	copy(attr[4:], data)
	return attr
}

// buildCommunityValues builds raw community value bytes (4 bytes each).
func buildCommunityValues(values ...uint32) []byte {
	buf := make([]byte, len(values)*4)
	for i, v := range values {
		binary.BigEndian.PutUint32(buf[i*4:], v)
	}
	return buf
}

// TestCommunityAttrModHandlerAdd verifies that AttrModAdd appends community
// wire bytes to an existing COMMUNITY attribute.
//
// VALIDATES: Egress tagging via ModAccumulator progressive build.
// PREVENTS: Tag values silently dropped during egress build.
func TestCommunityAttrModHandlerAdd(t *testing.T) {
	// Full attribute: COMMUNITY with value 1:1
	src := buildFullCommunityAttr(buildCommunityValues(0x0001_0001))

	// Op: add community 2:2
	addBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(addBuf, 0x0002_0002)
	ops := []registry.AttrOp{{Code: byte(attribute.AttrCommunity), Action: registry.AttrModAdd, Buf: addBuf}}

	buf := make([]byte, 256)
	off := communityAttrModHandler(src, ops, buf, 0)

	// Should have: flags(1) + code(1) + extlen(2) + data(8) = 12 bytes
	require.Equal(t, 12, off)
	assert.Equal(t, byte(0xC0|0x10), buf[0])
	assert.Equal(t, byte(attribute.AttrCommunity), buf[1])
	dataLen := int(binary.BigEndian.Uint16(buf[2:4]))
	assert.Equal(t, 8, dataLen)

	// Verify both communities present with correct wire encoding.
	assert.Equal(t, uint32(0x0001_0001), binary.BigEndian.Uint32(buf[4:8]))
	assert.Equal(t, uint32(0x0002_0002), binary.BigEndian.Uint32(buf[8:12]))
}

// TestCommunityAttrModHandlerRemove verifies that AttrModRemove removes matching
// community wire bytes from an existing attribute.
//
// VALIDATES: Egress stripping via ModAccumulator progressive build.
// PREVENTS: Strip leaving values in the egress wire output.
func TestCommunityAttrModHandlerRemove(t *testing.T) {
	// Full attribute: COMMUNITY with values 1:1 and 2:2
	src := buildFullCommunityAttr(buildCommunityValues(0x0001_0001, 0x0002_0002))

	// Op: remove 1:1
	rmBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(rmBuf, 0x0001_0001)
	ops := []registry.AttrOp{{Code: byte(attribute.AttrCommunity), Action: registry.AttrModRemove, Buf: rmBuf}}

	buf := make([]byte, 256)
	off := communityAttrModHandler(src, ops, buf, 0)

	require.Equal(t, 8, off)
	dataLen := int(binary.BigEndian.Uint16(buf[2:4]))
	assert.Equal(t, 4, dataLen)
	assert.Equal(t, uint32(0x0002_0002), binary.BigEndian.Uint32(buf[4:8]))
}

// TestCommunityAttrModHandlerRemoveAll verifies that removing all values
// omits the attribute entirely (returns offset unchanged).
//
// VALIDATES: Empty attribute not written to wire.
// PREVENTS: Malformed zero-length attribute in egress output.
func TestCommunityAttrModHandlerRemoveAll(t *testing.T) {
	src := buildFullCommunityAttr(buildCommunityValues(0x0001_0001))

	rmBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(rmBuf, 0x0001_0001)
	ops := []registry.AttrOp{{Code: byte(attribute.AttrCommunity), Action: registry.AttrModRemove, Buf: rmBuf}}

	buf := make([]byte, 256)
	off := communityAttrModHandler(src, ops, buf, 0)

	assert.Equal(t, 0, off, "all removed: attribute omitted")
}

// TestCommunityAttrModHandlerCreateAbsent verifies that AttrModAdd with nil src
// creates a new attribute from the op's wire bytes.
//
// VALIDATES: Egress tagging when source UPDATE has no COMMUNITY attribute.
// PREVENTS: Tag silently failing when attribute is absent.
func TestCommunityAttrModHandlerCreateAbsent(t *testing.T) {
	addBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(addBuf, 0x0003_0003)
	ops := []registry.AttrOp{{Code: byte(attribute.AttrCommunity), Action: registry.AttrModAdd, Buf: addBuf}}

	buf := make([]byte, 256)
	off := communityAttrModHandler(nil, ops, buf, 0) // nil src = attribute absent

	require.Equal(t, 8, off)
	assert.Equal(t, byte(0xC0|0x10), buf[0])
	assert.Equal(t, byte(attribute.AttrCommunity), buf[1])
	assert.Equal(t, uint32(0x0003_0003), binary.BigEndian.Uint32(buf[4:8]))
}

// TestCommunityAttrModHandlerBoundsCheck verifies that the handler returns off
// unchanged when the output buffer is too small.
//
// VALIDATES: Finding 3 -- bounds check before writing to buf.
// PREVENTS: Panic from writing past buffer end.
func TestCommunityAttrModHandlerBoundsCheck(t *testing.T) {
	addBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(addBuf, 0x0001_0001)
	ops := []registry.AttrOp{{Code: byte(attribute.AttrCommunity), Action: registry.AttrModAdd, Buf: addBuf}}

	tinyBuf := make([]byte, 4) // Too small for header(4) + data(4)
	off := communityAttrModHandler(nil, ops, tinyBuf, 0)
	assert.Equal(t, 0, off, "should return off unchanged when buffer too small")
}

// TestLargeCommunityAttrModHandler verifies Add for large communities
// (code 32, 12-byte values).
//
// VALIDATES: Large community wire manipulation in egress path.
// PREVENTS: Wrong value size breaking large community encoding.
func TestLargeCommunityAttrModHandler(t *testing.T) {
	addBuf := make([]byte, 12)
	binary.BigEndian.PutUint32(addBuf[0:4], 65000)
	binary.BigEndian.PutUint32(addBuf[4:8], 1)
	binary.BigEndian.PutUint32(addBuf[8:12], 2)
	ops := []registry.AttrOp{{Code: byte(attribute.AttrLargeCommunity), Action: registry.AttrModAdd, Buf: addBuf}}

	buf := make([]byte, 256)
	off := largeCommunityAttrModHandler(nil, ops, buf, 0)

	require.Equal(t, 16, off) // flags(1) + code(1) + extlen(2) + data(12)
	assert.Equal(t, byte(attribute.AttrLargeCommunity), buf[1])
	dataLen := int(binary.BigEndian.Uint16(buf[2:4]))
	assert.Equal(t, 12, dataLen)
	// Verify actual wire values.
	assert.Equal(t, uint32(65000), binary.BigEndian.Uint32(buf[4:8]))
	assert.Equal(t, uint32(1), binary.BigEndian.Uint32(buf[8:12]))
	assert.Equal(t, uint32(2), binary.BigEndian.Uint32(buf[12:16]))
}

// TestExtCommunityAttrModHandler verifies Add for extended communities
// (code 16, 8-byte values).
//
// VALIDATES: Extended community wire manipulation in egress path.
// PREVENTS: Wrong value size breaking extended community encoding.
func TestExtCommunityAttrModHandler(t *testing.T) {
	addBuf := []byte{0x00, 0x02, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64} // target:65000:100
	ops := []registry.AttrOp{{Code: byte(attribute.AttrExtCommunity), Action: registry.AttrModAdd, Buf: addBuf}}

	buf := make([]byte, 256)
	off := extCommunityAttrModHandler(nil, ops, buf, 0)

	require.Equal(t, 12, off) // flags(1) + code(1) + extlen(2) + data(8)
	assert.Equal(t, byte(attribute.AttrExtCommunity), buf[1])
	dataLen := int(binary.BigEndian.Uint16(buf[2:4]))
	assert.Equal(t, 8, dataLen)
	// Verify actual wire bytes.
	assert.Equal(t, addBuf, buf[4:12])
}

// TestCommunityAttrModHandlerSet verifies that AttrModSet replaces all data.
//
// VALIDATES: Set overrides existing values and prior Add/Remove ops.
// PREVENTS: Set silently appending instead of replacing.
func TestCommunityAttrModHandlerSet(t *testing.T) {
	// Existing: communities 1:1 and 2:2
	src := buildFullCommunityAttr(buildCommunityValues(0x0001_0001, 0x0002_0002))

	// Set to 3:3 only (replaces everything).
	setBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(setBuf, 0x0003_0003)
	ops := []registry.AttrOp{{Code: byte(attribute.AttrCommunity), Action: registry.AttrModSet, Buf: setBuf}}

	buf := make([]byte, 256)
	off := communityAttrModHandler(src, ops, buf, 0)

	require.Equal(t, 8, off) // header(4) + one community(4)
	assert.Equal(t, uint32(0x0003_0003), binary.BigEndian.Uint32(buf[4:8]))
	// Old values should NOT be present.
	dataLen := int(binary.BigEndian.Uint16(buf[2:4]))
	assert.Equal(t, 4, dataLen, "Set should produce exactly 1 community")
}

// TestCommunityAttrModHandlerNonExtendedSrc verifies that the handler correctly
// parses a source attribute with non-extended (1-byte) length format.
//
// VALIDATES: extractAttrValue handles both extended and non-extended attrs.
// PREVENTS: Non-extended attrs from real peers causing parse errors.
func TestCommunityAttrModHandlerNonExtendedSrc(t *testing.T) {
	// Non-extended attribute: flags=0xC0 (no 0x10), code=8, len=4 (1 byte), data=1:1
	src := make([]byte, 7) // flags(1) + code(1) + len(1) + data(4) = 7
	src[0] = 0xC0          // Optional Transitive, NO extended length
	src[1] = byte(attribute.AttrCommunity)
	src[2] = 4 // 1-byte length = 4
	binary.BigEndian.PutUint32(src[3:7], 0x0001_0001)

	// Add community 2:2
	addBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(addBuf, 0x0002_0002)
	ops := []registry.AttrOp{{Code: byte(attribute.AttrCommunity), Action: registry.AttrModAdd, Buf: addBuf}}

	buf := make([]byte, 256)
	off := communityAttrModHandler(src, ops, buf, 0)

	// Output should be extended-length (4-byte header) with both communities.
	require.Equal(t, 12, off)
	assert.Equal(t, byte(0xC0|0x10), buf[0], "output should use extended length")
	assert.Equal(t, uint32(0x0001_0001), binary.BigEndian.Uint32(buf[4:8]))
	assert.Equal(t, uint32(0x0002_0002), binary.BigEndian.Uint32(buf[8:12]))
}

// TestApplyEgressFilterOps verifies that applyEgressFilter accumulates the
// correct ops in the ModAccumulator.
//
// VALIDATES: Egress strip/tag translated to correct AttrOp code/action/buf.
// PREVENTS: Ops silently dropped or using wrong action constants.
func TestApplyEgressFilterOps(t *testing.T) {
	stripWire := make([]byte, 4)
	binary.BigEndian.PutUint32(stripWire, 0x0001_0001) // Strip 1:1

	tagWire := make([]byte, 4)
	binary.BigEndian.PutUint32(tagWire, 0x0002_0002) // Tag 2:2

	defs := communityDefs{
		"strip-it": &communityDef{typ: communityTypeStandard, wireValues: [][]byte{stripWire}},
		"tag-it":   &communityDef{typ: communityTypeStandard, wireValues: [][]byte{tagWire}},
	}
	fc := filterConfig{
		egressStrip: []string{"strip-it"},
		egressTag:   []string{"tag-it"},
	}

	var mods registry.ModAccumulator
	applyEgressFilter(defs, fc, &mods)

	ops := mods.Ops()
	require.Equal(t, 2, len(ops), "should have 2 ops (1 strip + 1 tag)")

	// Strip comes first (strip-before-tag ordering).
	assert.Equal(t, byte(attribute.AttrCommunity), ops[0].Code)
	assert.Equal(t, registry.AttrModRemove, ops[0].Action)
	assert.Equal(t, stripWire, ops[0].Buf)

	// Tag second.
	assert.Equal(t, byte(attribute.AttrCommunity), ops[1].Code)
	assert.Equal(t, registry.AttrModAdd, ops[1].Action)
	assert.Equal(t, tagWire, ops[1].Buf)
}
