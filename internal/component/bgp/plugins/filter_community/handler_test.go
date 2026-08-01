package filter_community

import (
	"bytes"
	"encoding/binary"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/metrics"
)

// buildFullAttr builds a full community-family attribute (flags+code+extlen+data)
// for handler tests. Matches the format that buildModifiedPayload passes to
// AttrModHandlers.
func buildFullAttr(code byte, data []byte) []byte {
	attr := make([]byte, 4+len(data))
	attr[0] = 0xC0 | 0x10 // Optional Transitive + Extended Length
	attr[1] = code
	binary.BigEndian.PutUint16(attr[2:4], uint16(len(data))) //nolint:gosec // test data
	copy(attr[4:], data)
	return attr
}

// buildFullCommunityAttr builds a full COMMUNITY attribute (flags+code+extlen+data) for handler tests.
// Matches the format that buildModifiedPayload passes to AttrModHandlers.
func buildFullCommunityAttr(data []byte) []byte {
	return buildFullAttr(byte(attribute.AttrCommunity), data)
}

// planAttr drives an AttrModHandler over a synthetic one-attribute section and
// returns the finished edit set together with the slot the handler planned.
//
// A handler no longer writes bytes: it describes the output, and the edit set is
// what materializes it. A test therefore reads the outcome through the same
// accessors the rebuild uses, rather than through a returned offset.
func planAttr(h filterapi.AttrModHandler, code uint8, srcAttr []byte, ops []filterapi.AttrOp) (*filterapi.EditSet, filterapi.SlotID) {
	mods := &filterapi.ModAccumulator{}
	edit := mods.EditSet()
	edit.Begin()

	var src []byte
	srcOff, srcLen, valOff, valLen := 0, 0, 0, 0
	if len(srcAttr) > 0 {
		hdr := 3
		if srcAttr[0]&0x10 != 0 {
			hdr = 4
		}
		src = srcAttr
		srcLen = len(srcAttr)
		valOff, valLen = hdr, len(srcAttr)-hdr
	}

	p := edit.Attr(code, src, srcOff, srcLen, valOff, valLen, ops, 0)
	h(p)
	id := edit.Commit(p)
	return edit, id
}

// planHandlerBytes runs a handler over a synthetic one-attribute section and
// returns the bytes it plans. ok is false when the plan dropped or refused.
//
// srcAttr is passed to SlotWrite as the attribute section because a KeepAll plan
// emits the source attribute verbatim, and the writer reads those bytes from the
// section rather than from the plan.
func planHandlerBytes(h filterapi.AttrModHandler, code uint8, srcAttr []byte, ops []filterapi.AttrOp) (out []byte, ok bool) {
	edit, id := planAttr(h, code, srcAttr, ops)
	if edit.SlotFailed(id) || edit.SlotDropped(id) {
		return nil, false
	}
	n := edit.SlotSize(id)
	buf := make([]byte, n)
	if got := edit.SlotWrite(id, srcAttr, ops, buf, 0); got != n {
		return nil, false
	}
	return buf, true
}

// handlerForWidth maps a community value width to the handler and attribute code
// that own it. ok is false for a width no community family uses.
func handlerForWidth(width int) (filterapi.AttrModHandler, byte, bool) {
	switch width {
	case 4:
		return communityAttrModHandler, byte(attribute.AttrCommunity), true
	case 8:
		return extCommunityAttrModHandler, byte(attribute.AttrExtCommunity), true
	case 12:
		return largeCommunityAttrModHandler, byte(attribute.AttrLargeCommunity), true
	}
	return nil, 0, false
}

// removeViaHandler reproduces, through the migrated handler, the contract the
// old removeValues helper carried: apply ONE Remove operation of arbitrary arity
// to a list of wire values, and report both the retained bytes and whether the
// operation was well-formed.
//
// removeValues computed the retained list itself. The handler now names each
// retained run as a fragment over bytes already on the wire, so the retained list
// is read back out of the attribute the plan emits. The refusal signal is
// wholeValues, which is the guard the handler consults before it interprets a
// Remove buffer at all: a buffer that is not a whole number of values removes
// nothing, so the emitted value equals the input.
func removeViaHandler(t *testing.T, data []byte, width int, toRemove []byte) (got []byte, ok bool) {
	t.Helper()
	h, code, known := handlerForWidth(width)
	require.True(t, known, "unsupported community value width %d", width)

	ops := []filterapi.AttrOp{{Code: code, Action: filterapi.AttrModRemove, Buf: toRemove}}
	out, emitted := planHandlerBytes(h, code, buildFullAttr(code, data), ops)
	ok = wholeValues(toRemove, width)
	if !emitted {
		// Every value was removed: the attribute leaves the UPDATE, so nothing
		// is retained.
		return []byte{}, ok
	}
	// These handlers always emit the 4-byte extended-length header.
	return out[4:], ok
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
	ops := []filterapi.AttrOp{{Code: byte(attribute.AttrCommunity), Action: filterapi.AttrModAdd, Buf: addBuf}}

	out, ok := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), src, ops)
	require.True(t, ok, "the plan must emit the attribute")

	// Should have: flags(1) + code(1) + extlen(2) + data(8) = 12 bytes
	require.Len(t, out, 12)
	assert.Equal(t, byte(0xC0|0x10), out[0])
	assert.Equal(t, byte(attribute.AttrCommunity), out[1])
	dataLen := int(binary.BigEndian.Uint16(out[2:4]))
	assert.Equal(t, 8, dataLen)

	// Verify both communities present with correct wire encoding.
	assert.Equal(t, uint32(0x0001_0001), binary.BigEndian.Uint32(out[4:8]))
	assert.Equal(t, uint32(0x0002_0002), binary.BigEndian.Uint32(out[8:12]))
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
	ops := []filterapi.AttrOp{{Code: byte(attribute.AttrCommunity), Action: filterapi.AttrModRemove, Buf: rmBuf}}

	out, ok := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), src, ops)
	require.True(t, ok, "the plan must emit the attribute")

	require.Len(t, out, 8)
	dataLen := int(binary.BigEndian.Uint16(out[2:4]))
	assert.Equal(t, 4, dataLen)
	assert.Equal(t, uint32(0x0002_0002), binary.BigEndian.Uint32(out[4:8]))
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
	ops := []filterapi.AttrOp{{Code: byte(attribute.AttrCommunity), Action: filterapi.AttrModRemove, Buf: rmBuf}}

	out, ok := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), src, ops)

	assert.False(t, ok, "all removed: the plan drops the attribute")
	assert.Empty(t, out, "all removed: attribute omitted")
}

// TestCommunityAttrModHandlerCreateAbsent verifies that AttrModAdd with nil src
// creates a new attribute from the op's wire bytes.
//
// VALIDATES: Egress tagging when source UPDATE has no COMMUNITY attribute.
// PREVENTS: Tag silently failing when attribute is absent.
func TestCommunityAttrModHandlerCreateAbsent(t *testing.T) {
	addBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(addBuf, 0x0003_0003)
	ops := []filterapi.AttrOp{{Code: byte(attribute.AttrCommunity), Action: filterapi.AttrModAdd, Buf: addBuf}}

	// nil src = attribute absent.
	out, ok := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), nil, ops)
	require.True(t, ok, "the plan must emit the attribute")

	require.Len(t, out, 8)
	assert.Equal(t, byte(0xC0|0x10), out[0])
	assert.Equal(t, byte(attribute.AttrCommunity), out[1])
	assert.Equal(t, uint32(0x0003_0003), binary.BigEndian.Uint32(out[4:8]))
}

// TestCommunityAttrModHandlerBoundsCheck verifies that the write returns off
// unchanged when the output buffer is too small.
//
// The bound moved with the contract: a handler no longer touches the output
// buffer, so the check now sits in SlotWrite, which is the only code that does.
// The guarantee is the same one the handler used to carry -- a short buffer is
// left untouched and the offset does not advance.
//
// VALIDATES: Finding 3 -- bounds check before writing to buf.
// PREVENTS: Panic from writing past buffer end.
func TestCommunityAttrModHandlerBoundsCheck(t *testing.T) {
	addBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(addBuf, 0x0001_0001)
	ops := []filterapi.AttrOp{{Code: byte(attribute.AttrCommunity), Action: filterapi.AttrModAdd, Buf: addBuf}}

	edit, id := planAttr(communityAttrModHandler, byte(attribute.AttrCommunity), nil, ops)
	require.Equal(t, 8, edit.SlotSize(id), "the plan needs header(4) + data(4)")

	tinyBuf := make([]byte, 4) // Too small for header(4) + data(4)
	off := edit.SlotWrite(id, nil, ops, tinyBuf, 0)
	assert.Equal(t, 0, off, "should return off unchanged when buffer too small")
	assert.Equal(t, make([]byte, 4), tinyBuf, "a short buffer must not be written at all")
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
	ops := []filterapi.AttrOp{{Code: byte(attribute.AttrLargeCommunity), Action: filterapi.AttrModAdd, Buf: addBuf}}

	out, ok := planHandlerBytes(largeCommunityAttrModHandler, byte(attribute.AttrLargeCommunity), nil, ops)
	require.True(t, ok, "the plan must emit the attribute")

	require.Len(t, out, 16) // flags(1) + code(1) + extlen(2) + data(12)
	assert.Equal(t, byte(attribute.AttrLargeCommunity), out[1])
	dataLen := int(binary.BigEndian.Uint16(out[2:4]))
	assert.Equal(t, 12, dataLen)
	// Verify actual wire values.
	assert.Equal(t, uint32(65000), binary.BigEndian.Uint32(out[4:8]))
	assert.Equal(t, uint32(1), binary.BigEndian.Uint32(out[8:12]))
	assert.Equal(t, uint32(2), binary.BigEndian.Uint32(out[12:16]))
}

// TestExtCommunityAttrModHandler verifies Add for extended communities
// (code 16, 8-byte values).
//
// VALIDATES: Extended community wire manipulation in egress path.
// PREVENTS: Wrong value size breaking extended community encoding.
func TestExtCommunityAttrModHandler(t *testing.T) {
	addBuf := []byte{0x00, 0x02, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64} // target:65000:100
	ops := []filterapi.AttrOp{{Code: byte(attribute.AttrExtCommunity), Action: filterapi.AttrModAdd, Buf: addBuf}}

	out, ok := planHandlerBytes(extCommunityAttrModHandler, byte(attribute.AttrExtCommunity), nil, ops)
	require.True(t, ok, "the plan must emit the attribute")

	require.Len(t, out, 12) // flags(1) + code(1) + extlen(2) + data(8)
	assert.Equal(t, byte(attribute.AttrExtCommunity), out[1])
	dataLen := int(binary.BigEndian.Uint16(out[2:4]))
	assert.Equal(t, 8, dataLen)
	// Verify actual wire bytes.
	assert.Equal(t, addBuf, out[4:12])
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
	ops := []filterapi.AttrOp{{Code: byte(attribute.AttrCommunity), Action: filterapi.AttrModSet, Buf: setBuf}}

	out, ok := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), src, ops)
	require.True(t, ok, "the plan must emit the attribute")

	require.Len(t, out, 8) // header(4) + one community(4)
	assert.Equal(t, uint32(0x0003_0003), binary.BigEndian.Uint32(out[4:8]))
	// Old values should NOT be present.
	dataLen := int(binary.BigEndian.Uint16(out[2:4]))
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
	ops := []filterapi.AttrOp{{Code: byte(attribute.AttrCommunity), Action: filterapi.AttrModAdd, Buf: addBuf}}

	out, ok := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), src, ops)
	require.True(t, ok, "the plan must emit the attribute")

	// Output should be extended-length (4-byte header) with both communities.
	require.Len(t, out, 12)
	assert.Equal(t, byte(0xC0|0x10), out[0], "output should use extended length")
	assert.Equal(t, uint32(0x0001_0001), binary.BigEndian.Uint32(out[4:8]))
	assert.Equal(t, uint32(0x0002_0002), binary.BigEndian.Uint32(out[8:12]))
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

	var mods filterapi.ModAccumulator
	applyEgressFilter(defs, fc, &mods)

	ops := mods.Ops()
	require.Equal(t, 2, len(ops), "should have 2 ops (1 strip + 1 tag)")

	// Strip comes first (strip-before-tag ordering).
	assert.Equal(t, byte(attribute.AttrCommunity), ops[0].Code)
	assert.Equal(t, filterapi.AttrModRemove, ops[0].Action)
	assert.Equal(t, stripWire, ops[0].Buf)

	// Tag second.
	assert.Equal(t, byte(attribute.AttrCommunity), ops[1].Code)
	assert.Equal(t, filterapi.AttrModAdd, ops[1].Action)
	assert.Equal(t, tagWire, ops[1].Buf)
}

// TestRemoveValuesMultiValueBuffer verifies that a Remove buffer holding SEVERAL
// whole wire values removes every one of them.
//
// VALIDATES: spec-fixit-rs-community-strip-arity AC-1/AC-2 -- the route-server
// strip path (reactor_api_forward.go:635, forward_rs.go:342) hands this function
// the concatenated output of wireu.StripControlCommunities, which accumulates
// EVERY matching control community into one slice.
// PREVENTS: the leak this spec exists for. Before the fix the size guard saw
// 8 bytes against a 4-byte value width, returned the list untouched, and both
// control communities reached the route-server client.
//
// The removeValues helper this test drove no longer exists: the handler plans
// the retained runs as fragments instead of computing a new list. removeViaHandler
// puts the same question to the handler and reads the retained values back out of
// the attribute it emits.
func TestRemoveValuesMultiValueBuffer(t *testing.T) {
	data := buildCommunityValues(0x0000_FDE9, 0x0001_0001, 0x0000_FDEA, 0x0002_0002)
	toRemove := buildCommunityValues(0x0000_FDE9, 0x0000_FDEA)

	got, ok := removeViaHandler(t, data, 4, toRemove)

	require.True(t, ok, "a whole number of values is a valid buffer")
	assert.Equal(t, buildCommunityValues(0x0001_0001, 0x0002_0002), got,
		"both control communities removed, both ordinary ones kept in order")
}

// TestRemoveValuesSingleUnchanged pins the single-value case byte-for-byte.
//
// VALIDATES: spec-fixit-rs-community-strip-arity A-2 -- widening the contract
// must not disturb the producers that already split, namely the text-delta path
// (reactor/filter_delta.go:221-224) and the plugin's own egress filter
// (egress.go:28-30).
// PREVENTS: a regression in configured `community { egress strip NAME }`, which
// is the only community stripping with existing functional coverage.
func TestRemoveValuesSingleUnchanged(t *testing.T) {
	data := buildCommunityValues(0x0001_0001, 0x0002_0002, 0x0003_0003)
	toRemove := buildCommunityValues(0x0002_0002)

	got, ok := removeViaHandler(t, data, 4, toRemove)

	require.True(t, ok)
	assert.Equal(t, buildCommunityValues(0x0001_0001, 0x0003_0003), got)
}

// TestRemoveValuesNonMultipleRefusedLoudly verifies that a buffer length which is
// not a whole multiple of the value width is REPORTED, not silently swallowed.
//
// VALIDATES: spec-fixit-rs-community-strip-arity AC-5 and
// ai/rules/fail-closed-guards.md -- a guard must fail closed or say something.
// PREVENTS: the second half of this defect. The original guard returned the data
// unchanged with the comment "caller bug, silently preserve data", so the
// route-server contract violation was invisible from the day it was introduced.
// Asserting on the RETURNED signal rather than only on the log keeps this test
// independent of logging configuration.
func TestRemoveValuesNonMultipleRefusedLoudly(t *testing.T) {
	data := buildCommunityValues(0x0001_0001, 0x0002_0002)
	toRemove := []byte{0x00, 0x01, 0x00, 0x02, 0xFF} // 5 bytes, width 4

	got, ok := removeViaHandler(t, data, 4, toRemove)

	assert.False(t, ok, "a non-multiple length is a caller-contract violation")
	assert.Equal(t, data, got, "data is preserved unchanged when the op is refused")
}

// TestGenericCommunityHandlerWarnsOnNonMultiple verifies the handler both LOGS the
// refusal and keeps applying the attribute's remaining operations.
//
// VALIDATES: spec-fixit-rs-community-strip-arity AC-5, both halves.
// PREVENTS: a refusal that is silent (the original defect) or one that discards
// the sibling operations along with the bad one.
func TestGenericCommunityHandlerWarnsOnNonMultiple(t *testing.T) {
	var logged bytes.Buffer
	saved := logger
	logger = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelWarn}))
	}
	t.Cleanup(func() { logger = saved })

	src := buildFullCommunityAttr(buildCommunityValues(0x0001_0001, 0x0002_0002))
	ops := []filterapi.AttrOp{
		{Code: byte(attribute.AttrCommunity), Action: filterapi.AttrModRemove, Buf: []byte{0x00, 0x01, 0x00}},
		{Code: byte(attribute.AttrCommunity), Action: filterapi.AttrModRemove, Buf: buildCommunityValues(0x0001_0001)},
	}

	attr, emitted := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), src, ops)

	out := logged.String()
	assert.Contains(t, out, "level=WARN", "the refusal must be reported, not swallowed")
	assert.Contains(t, out, "3", "the message names the offending buffer length")

	require.True(t, emitted, "the plan must emit the attribute")
	require.Len(t, attr, 8, "the well-formed sibling op still applied")
	assert.Equal(t, uint32(0x0002_0002), binary.BigEndian.Uint32(attr[4:8]),
		"only the value named by the VALID op was removed")
}

// TestRemoveValuesAllWidths proves the fix is width-independent.
//
// VALIDATES: spec-fixit-rs-community-strip-arity AC-7 and A-4 -- widths 4, 8 and
// 12 share genericCommunityHandler with only valueSize differing
// (handler.go:19-31), so the arity rule must hold for all three.
// PREVENTS: a fix that repairs COMMUNITY while leaving EXTENDED_COMMUNITY and
// LARGE_COMMUNITY silently dropping multi-value Remove buffers.
func TestRemoveValuesAllWidths(t *testing.T) {
	for _, tc := range []struct {
		name  string
		width int
	}{
		{"community", 4},
		{"extended-community", 8},
		{"large-community", 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			val := func(fill byte) []byte {
				b := make([]byte, tc.width)
				for i := range b {
					b[i] = fill
				}
				return b
			}
			data := append(append(append([]byte{}, val(1)...), val(2)...), val(3)...)
			toRemove := append(append([]byte{}, val(1)...), val(3)...)

			got, ok := removeViaHandler(t, data, tc.width, toRemove)

			require.True(t, ok)
			assert.Equal(t, val(2), got, "both listed values removed at width %d", tc.width)

			short, ok := removeViaHandler(t, data, tc.width, val(1)[:tc.width-1])
			assert.False(t, ok, "a non-multiple length is refused at width %d", tc.width)
			assert.Equal(t, data, short)
		})
	}
}

// TestRemoveValuesMultiRemovingEverythingOmitsAttribute verifies that a
// multi-value Remove which empties the list still omits the attribute.
//
// VALIDATES: spec-fixit-rs-community-strip-arity AC-4 -- the existing
// "omit entirely when nothing remains" behavior at handler.go:93-95 must
// survive the arity change.
// PREVENTS: a zero-length COMMUNITY attribute on the wire, which is malformed.
func TestRemoveValuesMultiRemovingEverythingOmitsAttribute(t *testing.T) {
	src := buildFullCommunityAttr(buildCommunityValues(0x0000_FDE9, 0x0000_FDEA))
	ops := []filterapi.AttrOp{{
		Code:   byte(attribute.AttrCommunity),
		Action: filterapi.AttrModRemove,
		Buf:    buildCommunityValues(0x0000_FDE9, 0x0000_FDEA),
	}}

	out, ok := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), src, ops)

	assert.False(t, ok, "every value removed: the plan drops the attribute")
	assert.Empty(t, out, "every value removed: attribute omitted entirely")
}

// countingCounter and countingRegistry are the smallest metrics.Registry that
// lets this package observe whether the handler reached the recorder in
// filterapi. Only CounterVec is real; everything else is a no-op.
type countingCounter struct{ n int }

func (c *countingCounter) Inc()          { c.n++ }
func (c *countingCounter) Add(_ float64) { c.n++ }

type countingCounterVec struct{ seen map[string]*countingCounter }

func (v *countingCounterVec) With(labelValues ...string) metrics.Counter {
	if v.seen == nil {
		v.seen = map[string]*countingCounter{}
	}
	key := strings.Join(labelValues, "|")
	c, ok := v.seen[key]
	if !ok {
		c = &countingCounter{}
		v.seen[key] = c
	}
	return c
}

func (v *countingCounterVec) Delete(_ ...string) bool { return false }

type countingRegistry struct{ vec *countingCounterVec }

func (r *countingRegistry) Counter(_, _ string) metrics.Counter { return &countingCounter{} }
func (r *countingRegistry) Gauge(_, _ string) metrics.Gauge {
	return metrics.NopRegistry{}.Gauge("", "")
}

func (r *countingRegistry) CounterVec(_, _ string, _ []string) metrics.CounterVec {
	if r.vec == nil {
		r.vec = &countingCounterVec{}
	}
	return r.vec
}

func (r *countingRegistry) GaugeVec(_, _ string, _ []string) metrics.GaugeVec {
	return metrics.NopRegistry{}.GaugeVec("", "", nil)
}

func (r *countingRegistry) Histogram(_, _ string, _ []float64) metrics.Histogram {
	return metrics.NopRegistry{}.Histogram("", "", nil)
}

func (r *countingRegistry) HistogramVec(_, _ string, _ []float64, _ []string) metrics.HistogramVec {
	return metrics.NopRegistry{}.HistogramVec("", "", nil, nil)
}

// TestGenericCommunityHandlerCountsRefusals verifies the handler reaches the
// filterapi refusal counter, and only on a refusal.
//
// VALIDATES: spec-fixit-rs-community-strip-arity R-2 -- "the loud refusal fires
// in production for a producer the enumeration missed" needs to be MEASURABLE,
// not only greppable. Its mitigation is that one line identifies the producer;
// the counter is what makes anyone look.
// PREVENTS: a recorder that is correct in isolation but never called, which is
// the same dead-coverage shape as the untested strip path this spec started
// from. Asserting the label too pins the counter to the right attribute.
func TestGenericCommunityHandlerCountsRefusals(t *testing.T) {
	reg := &countingRegistry{}
	filterapi.SetMetricsRegistry(reg)
	t.Cleanup(func() { filterapi.SetMetricsRegistry(nil) })

	silenced := logger
	logger = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	t.Cleanup(func() { logger = silenced })

	src := buildFullCommunityAttr(buildCommunityValues(0x0001_0001, 0x0002_0002))

	// A well-formed Remove must NOT count.
	good := []filterapi.AttrOp{{
		Code:   byte(attribute.AttrCommunity),
		Action: filterapi.AttrModRemove,
		Buf:    buildCommunityValues(0x0001_0001),
	}}
	planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), src, good)
	// Assert on the ABSENCE of the series, not on its value: SetMetricsRegistry
	// creates the vector eagerly, so reg.vec is already non-nil here and only a
	// With() call -- i.e. an actual refusal -- creates the per-label counter.
	assert.NotContains(t, reg.vec.seen, "community", "a valid Remove must not count as a refusal")

	// A non-multiple buffer must count once, under the community label.
	bad := []filterapi.AttrOp{{
		Code:   byte(attribute.AttrCommunity),
		Action: filterapi.AttrModRemove,
		Buf:    []byte{0x00, 0x01, 0x00},
	}}
	planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), src, bad)

	require.NotNil(t, reg.vec, "the refusal must have created the counter vector")
	assert.Equal(t, 1, reg.vec.seen["community"].n)
}
