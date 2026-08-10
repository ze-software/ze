package filter_community

import (
	"bytes"
	"encoding/binary"
	"io"
	"log/slog"
	"runtime"
	"strconv"
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
// strip path (both `wireu.StripControlCommunities` call sites, in
// reactor/reactor_api_forward.go and reactor/forward_rs.go) hands this function
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
// (the per-value split in reactor/filter_delta.go `textDeltaToModOps`) and the
// plugin's own egress filter (`applyEgressFilter` in egress.go).
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
// ai/rules/evidence.md -- a guard must fail closed or say something.
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

	// Assert the KEY=VALUE pairs, never the bare value. `slog` writes an RFC 3339
	// timestamp on every line, so `Contains(out, "3")` matched whatever the
	// handler said -- deleting the buffer-length attribute left the test green.
	out := logged.String()
	assert.Contains(t, out, "level=WARN", "the refusal must be reported, not swallowed")
	assert.Contains(t, out, "buffer-length=3", "the message names the offending buffer length")
	assert.Contains(t, out, "attribute-code=8", "the message names the attribute it refused")
	assert.Contains(t, out, "value-size=4", "the message names the width the buffer failed to fill")

	require.True(t, emitted, "the plan must emit the attribute")
	require.Len(t, attr, 8, "the well-formed sibling op still applied")
	assert.Equal(t, uint32(0x0002_0002), binary.BigEndian.Uint32(attr[4:8]),
		"only the value named by the VALID op was removed")
}

// TestRemoveValuesAllWidths proves the fix is width-independent.
//
// VALIDATES: spec-fixit-rs-community-strip-arity AC-7 and A-4 -- widths 4, 8 and
// 12 share genericCommunityHandler with only valueSize differing (the three
// registered handlers in handler.go), so the arity rule must hold for all three.
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
			// Two families of values, each one built so that comparing a PART of
			// a value cannot tell its members apart:
			//   tail(id) differs from its siblings in the LAST byte only,
			//   head(id) differs from its siblings in the FIRST byte only.
			// A comparison narrowed to the leading bytes matches every tail
			// value, and one narrowed to the trailing bytes matches every head
			// value, so either shape removes values it was never given and the
			// assertions below fail. Values whose bytes are all one fill byte
			// cannot discriminate that: at widths 8 and 12 a key truncated to 4
			// bytes still told them apart, and the truncation survived.
			tail := func(id byte) []byte {
				b := bytes.Repeat([]byte{0xA5}, tc.width)
				b[tc.width-1] = id
				return b
			}
			head := func(id byte) []byte {
				b := bytes.Repeat([]byte{0x5A}, tc.width)
				b[0] = id
				return b
			}
			join := func(vals ...[]byte) []byte { return bytes.Join(vals, nil) }

			// Six values, interleaved, and a three-value Remove that names a
			// non-contiguous subset of them. More than the two the old case
			// carried, so a removal that stops after the first match, or one
			// that coalesces the wrong retained runs, is visible here.
			data := join(tail(1), head(1), tail(2), head(2), tail(3), head(3))
			toRemove := join(tail(1), tail(3), head(2))

			got, ok := removeViaHandler(t, data, tc.width, toRemove)

			require.True(t, ok)
			assert.Equal(t, join(head(1), tail(2), head(3)), got,
				"exactly the three named values removed, the rest kept in order at width %d", tc.width)

			// A value one byte short of the width is not a whole value.
			short, ok := removeViaHandler(t, data, tc.width, tail(1)[:tc.width-1])
			assert.False(t, ok, "a non-multiple length is refused at width %d", tc.width)
			assert.Equal(t, data, short)

			// A near miss: same width, differs from tail(1) in its last byte
			// alone. Nothing may be removed.
			miss, ok := removeViaHandler(t, data, tc.width, tail(9))
			require.True(t, ok)
			assert.Equal(t, data, miss,
				"a value the list does not carry removes nothing at width %d", tc.width)
		})
	}
}

// TestRemoveValuesMultiRemovingEverythingOmitsAttribute verifies that a
// multi-value Remove which empties the list still omits the attribute.
//
// VALIDATES: spec-fixit-rs-community-strip-arity AC-4 -- the existing
// "omit entirely when nothing remains" behavior (the `p.ValueLen() == 0` drop
// at the end of genericCommunityHandler) must
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

// TestGenericCommunityHandlerRefusesMalformedSourceValue drives the handler with
// a SOURCE attribute whose value is not a whole number of wire values, and
// requires the plan to be refused rather than normalized.
//
// VALIDATES: ai/rules/evidence.md, "test the shape that should be
// rejected". The retained-run loop steps by valueSize, so a trailing partial
// value fell off the end and the handler re-emitted a well-formed attribute the
// peer never sent. That is fabrication on the wire, and it was silent.
// PREVENTS: a future producer reaching this handler with a malformed value and
// having the attribute quietly rounded down instead of the route being
// suppressed. No producer can do it today: a peer-sourced COMMUNITY of length
// 4k+3 is classified treat-as-withdraw by validateCommunityAttr
// (internal/component/bgp/message/rfc7606.go, registered in the attrValidators
// table and reached from Session.enforceRFC7606), and the withdrawal that
// replaces the UPDATE carries no COMMUNITY at all; locally originated routes get
// their value from writeCommunitiesAttr (reactor/reactor_wire.go), which writes
// 4 bytes per community. The guard exists so that a NEW producer says so rather
// than being absorbed.
func TestGenericCommunityHandlerRefusesMalformedSourceValue(t *testing.T) {
	silenced := logger
	var logged bytes.Buffer
	logger = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelWarn}))
	}
	t.Cleanup(func() { logger = silenced })

	for _, tc := range []struct {
		name  string
		width int
	}{
		{"community", 4},
		{"extended-community", 8},
		{"large-community", 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logged.Reset()
			h, code, known := handlerForWidth(tc.width)
			require.True(t, known)

			// Two whole values plus a trailing partial one.
			value := bytes.Repeat([]byte{0x11}, 2*tc.width+tc.width-1)
			ops := []filterapi.AttrOp{{
				Code:   code,
				Action: filterapi.AttrModRemove,
				Buf:    bytes.Repeat([]byte{0x11}, tc.width),
			}}

			edit, id := planAttr(h, code, buildFullAttr(code, value), ops)

			assert.True(t, edit.SlotFailed(id),
				"a source value of %d bytes is not whole values of %d: refuse, do not round down",
				len(value), tc.width)
			assert.False(t, edit.SlotDropped(id), "the plan must refuse, not silently drop the attribute")
			assert.Contains(t, logged.String(), "level=WARN", "the refusal must be reported")
			assert.Contains(t, logged.String(), "value-length="+strconv.Itoa(len(value)),
				"the message names the malformed source length")
		})
	}
}

// TestGenericCommunityHandlerEmptyRemoveIsSilentNoOp pins what an EMPTY Remove
// buffer does: it names no value, so it removes none, and it is not a
// contract violation.
//
// VALIDATES: spec-fixit-rs-community-strip-arity, the carried-forward finding
// "empty toRemove is untested". Zero is a whole number of values
// (`len(nil) % 4 == 0`), so wholeValues admits it and the attribute is
// forwarded unchanged. No producer emits one today -- reactor/filter_delta.go,
// egress.go and wireu.StripControlCommunities all gate on a non-empty set --
// which is exactly why the behavior needs pinning rather than assuming.
// PREVENTS: a future arity guard that reclassifies an empty buffer as
// malformed, which would fire the warning and the refusal counter at any
// producer that ever sends one, and make the operator hunt a defect that is
// not there.
func TestGenericCommunityHandlerEmptyRemoveIsSilentNoOp(t *testing.T) {
	reg := &countingRegistry{}
	filterapi.SetMetricsRegistry(reg)
	t.Cleanup(func() { filterapi.SetMetricsRegistry(nil) })

	var logged bytes.Buffer
	saved := logger
	logger = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelWarn}))
	}
	t.Cleanup(func() { logger = saved })

	data := buildCommunityValues(0x0001_0001, 0x0002_0002)
	src := buildFullCommunityAttr(data)

	for _, tc := range []struct {
		name string
		buf  []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ops := []filterapi.AttrOp{{
				Code:   byte(attribute.AttrCommunity),
				Action: filterapi.AttrModRemove,
				Buf:    tc.buf,
			}}

			out, emitted := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), src, ops)

			require.True(t, emitted, "an empty Remove leaves the attribute in the UPDATE")
			require.Len(t, out, 12, "header(4) + both values(8)")
			assert.Equal(t, data, out[4:], "an empty Remove names no value, so it removes none")
		})
	}

	assert.Empty(t, logged.String(), "an empty Remove is not a contract violation: nothing is logged")
	assert.NotContains(t, reg.vec.seen, "community", "an empty Remove must not count as a refusal")
}

// removalOps wraps buf as a single AttrModRemove operation on COMMUNITY.
func removalOps(buf []byte) []filterapi.AttrOp {
	return []filterapi.AttrOp{
		{Code: byte(attribute.AttrCommunity), Action: filterapi.AttrModRemove, Buf: buf},
	}
}

// repeatedCommunities builds n community values starting at base, each distinct.
func repeatedCommunities(base uint32, n int) []byte {
	values := make([]uint32, n)
	for i := range values {
		values[i] = base + uint32(i)
	}
	return buildCommunityValues(values...)
}

// TestRemovalSetIndexesOnlyAboveThreshold pins the representation newRemovalSet
// picks, on both sides of the boundary and on each operand independently.
//
// VALIDATES: the peer-reachable quadratic on the route-server forward path is
// answered from a map once BOTH operands are large.
// PREVENTS: the guard becoming decorative. Deleting the index branch, or
// thresholding on the removal count alone instead of on the minimum, changes the
// representation and fails here. An assertion on the retained values alone would
// not: the two representations agree on every answer, which is the whole point,
// so only the representation itself can witness the fix.
func TestRemovalSetIndexesOnlyAboveThreshold(t *testing.T) {
	const size = 4
	big := removalIndexThreshold + 1

	tests := []struct {
		name       string
		sourceVals int
		removeVals int
		wantIndex  bool
	}{
		{"both at the threshold", removalIndexThreshold, removalIndexThreshold, false},
		{"both one above", big, big, true},
		{"source one above, removals at it", big, removalIndexThreshold, false},
		{"removals one above, source at it", removalIndexThreshold, big, false},
		{"large removals against a two-value attribute", 2, 16383, false},
		{"large source against a two-value removal", 16383, 2, false},
		{"the measured attack shape", 16383, 16383, true},
		{"no removal operations at all", big, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ops []filterapi.AttrOp
			if tt.removeVals > 0 {
				ops = removalOps(repeatedCommunities(0x0100_0000, tt.removeVals))
			}
			set := newRemovalSet(ops, size, tt.sourceVals)
			assert.Equal(t, tt.wantIndex, set.indexed(),
				"min(%d source, %d removal) values against a threshold of %d",
				tt.sourceVals, tt.removeVals, removalIndexThreshold)
		})
	}
}

// TestRemovalSetAnswersAgreeAcrossRepresentations proves the optimization is not
// a behavior change: the map and the scan return the same answer for every
// value, including values absent from the set.
//
// VALIDATES: indexing preserves membership semantics exactly.
// PREVENTS: an index built from the wrong operand, or one that skips a malformed
// buffer differently from removedByAny.
func TestRemovalSetAnswersAgreeAcrossRepresentations(t *testing.T) {
	const size = 4
	removals := repeatedCommunities(0x0100_0000, removalIndexThreshold+1)

	indexed := newRemovalSet(removalOps(removals), size, removalIndexThreshold+1)
	require.True(t, indexed.indexed(), "the large case must index, or this test compares one thing to itself")
	scanned := newRemovalSet(removalOps(removals), size, 1)
	require.False(t, scanned.indexed(), "the small case must scan")

	for i := 0; i+size <= len(removals); i += size {
		want := removals[i : i+size]
		assert.True(t, indexed.has(want), "indexed set must contain value %d", i/size)
		assert.Equal(t, scanned.has(want), indexed.has(want), "representations disagree on value %d", i/size)
	}

	for _, absent := range []uint32{0x0200_0000, 0x0000_0001, 0xFFFF_FFFF} {
		v := buildCommunityValues(absent)
		assert.False(t, indexed.has(v), "indexed set must not contain %#08x", absent)
		assert.Equal(t, scanned.has(v), indexed.has(v), "representations disagree on absent %#08x", absent)
	}
}

// TestRemovalSetDeduplicatesIndexEntries pins the property that made an earlier
// candidate fix worse than the defect: a duplicate-heavy removal buffer must not
// cost one map entry per duplicate.
//
// VALIDATES: a duplicate-heavy removal buffer yields one index ENTRY per
// distinct value, and the collapsed value is still removed.
// PREVENTS: a representation that carries one entry per buffer value, which a
// slice or a multiset index would, and a deduplicating build that loses the
// value it collapsed.
// CANNOT SEE: bytes. An entry count is a property of a Go map, so this assertion
// stayed green under the pre-fix build, which allocated 234,864 bytes at this
// test's 4096 values to store its single entry. That is what the byte ceiling in
// TestRemovalSetIndexAllocatesByDistinctValues is for, and that test measures the
// same shape at 16383 values, where the pre-fix build cost 939,312.
//
// Those last two digits have now been wrong twice, in opposite directions, in
// this one comment. 939,312 is the DUPLICATE-HEAVY shape under the pre-fix build
// (hint plus unconditional insert). 939,264 is the ALL-DISTINCT shape with the
// hint restored, which is a different test's counterfactual. They differ by 48
// bytes. Read the table above `newRemovalSet` before changing either.
func TestRemovalSetDeduplicatesIndexEntries(t *testing.T) {
	const size = 4
	const count = 4096

	values := make([]uint32, count)
	for i := range values {
		values[i] = 0x0100_0001 // every entry identical
	}
	set := newRemovalSet(removalOps(buildCommunityValues(values...)), size, count)

	require.True(t, set.indexed(), "4096 values must index")
	assert.Len(t, set.index, 1, "%d identical values must collapse to one entry", count)
	assert.True(t, set.has(buildCommunityValues(0x0100_0001)), "the one distinct value is still removed")
}

// maxDistinctIndexBytes bounds what one removalSet index may allocate for a
// removal buffer holding ONE distinct value. A map with a single entry needs a
// header, one group and one four-byte key. The bound is loose enough that a
// runtime map-layout change does not fail it, and two orders of magnitude below
// what a raw-count size hint costs at the same input.
const maxDistinctIndexBytes = 8192

// TestRemovalSetIndexAllocatesByDistinctValues measures what the index COSTS,
// which the entry count above cannot see.
//
// VALIDATES: the bytes newRemovalSet allocates track distinct values, not the
// raw value count of the removal buffer.
// PREVENTS: sizing the map from the undeduplicated count, and inserting each
// repeat. A size hint of 16383 allocates the whole table before the first
// insert: one entry, and 939,312 bytes measured here -- 93% of the megabyte that
// got the earlier candidate fix reverted, reachable from one peer repeating 0:0.
// This is the 16.5x regression and the up-to-a-megabyte growth the reviewers
// measured against a set built without deduplication, and an entry count cannot
// see either one (ai/rules/evidence.md: entries stored is not bytes allocated).
func TestRemovalSetIndexAllocatesByDistinctValues(t *testing.T) {
	const size = 4
	const count = 16383 // the RFC 4271 attribute ceiling, 65532 octets

	values := make([]uint32, count)
	for i := range values {
		values[i] = 0 // 0:0, which StripControlCommunities matches on high == 0
	}
	ops := removalOps(buildCommunityValues(values...))

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	set := newRemovalSet(ops, size, count)
	runtime.ReadMemStats(&after)

	used := after.TotalAlloc - before.TotalAlloc
	t.Logf("%d identical values indexed in %d bytes (ceiling %d)", count, used, maxDistinctIndexBytes)

	require.True(t, set.indexed(), "16383 values must index, or this measures the scan")
	assert.Len(t, set.index, 1, "%d identical values must collapse to one entry", count)
	assert.LessOrEqual(t, used, uint64(maxDistinctIndexBytes),
		"one distinct value cost %d bytes: the index is sized by the raw value count, "+
			"so a peer repeating one control community pays for a table it never fills", used)
}

// maxAllDistinctIndexBytes bounds what one removalSet index may allocate for the
// shape the dropped size hint made WORSE: 16383 DISTINCT four-byte values, the
// RFC 4271 attribute ceiling. Here the buffer length IS the distinct count, so a
// hint would have been exactly right-sized, and the hintless map instead grows
// geometrically and discards each intermediate table.
//
// Measured by the test below on the shipped code: 1,812,792 bytes on a plain
// build, 2,072,584 under -race. Three mebibytes is 1.52x the worse of the two.
// That is loose enough for the race detector's own overhead and for a runtime
// map-layout change, and tight enough to catch what would matter: one further
// doubling of the table costs about the final table again, near four megabytes,
// and rebuilding the set per source value costs orders of magnitude more.
const maxAllDistinctIndexBytes = 3 << 20

// TestRemovalSetIndexBoundsAllDistinctValues bounds the half of the allocation
// trade that got worse, so it cannot grow further in silence.
//
// VALIDATES: an all-distinct removal buffer at the attribute ceiling allocates
// within maxAllDistinctIndexBytes.
// PREVENTS: the accepted trade widening unwatched. Dropping the size hint cut
// the duplicate-heavy shape from 873,744 bytes to 272 and raised this one from
// 939,264 to 1,812,792, 1.93x. That was accepted to remove a peer-controlled CPU
// quadratic, not to leave peak peer-controlled memory unmeasured, and this is the
// only test that can see the raised half.
func TestRemovalSetIndexBoundsAllDistinctValues(t *testing.T) {
	const size = 4
	const count = 16383 // the RFC 4271 attribute ceiling, 65532 octets

	// 0:0 through 0:16382. Every value distinct, and every one a form
	// StripControlCommunities matches, so this is a shape a peer can send.
	ops := removalOps(repeatedCommunities(0, count))

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	set := newRemovalSet(ops, size, count)
	runtime.ReadMemStats(&after)

	used := after.TotalAlloc - before.TotalAlloc
	t.Logf("%d distinct values indexed in %d bytes (ceiling %d)", count, used, maxAllDistinctIndexBytes)

	require.True(t, set.indexed(), "16383 values must index, or this measures the scan")
	assert.Len(t, set.index, count, "%d distinct values must each be stored", count)
	assert.LessOrEqual(t, used, uint64(maxAllDistinctIndexBytes),
		"%d distinct values cost %d bytes: the shape the size hint used to cover has grown", count, used)
}

// TestRemovalSetIgnoresMalformedBufferForThreshold proves a refused operation
// contributes nothing to the representation decision.
//
// VALIDATES: a buffer that is not a whole number of values is excluded from the
// count, matching removedByAny which skips it when answering.
// PREVENTS: a malformed buffer pushing the handler onto the indexed path and
// then contributing no entries, which would allocate a map nothing reads.
func TestRemovalSetIgnoresMalformedBufferForThreshold(t *testing.T) {
	const size = 4
	malformed := make([]byte, (removalIndexThreshold+1)*size+1) // one trailing byte

	set := newRemovalSet(removalOps(malformed), size, 16383)
	assert.False(t, set.indexed(), "a refused buffer contributes no values to the threshold")
}
