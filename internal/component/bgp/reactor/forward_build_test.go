package reactor

import (
	"encoding/binary"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// buildModTestPayload constructs a minimal UPDATE payload for testing:
// withdrawn_len(2) + withdrawn + attr_len(2) + attrs + nlri.
func buildModTestPayload(attrs, nlri []byte) []byte {
	// withdrawn_len = 0
	wdLen := 0
	attrLen := len(attrs)
	total := 2 + wdLen + 2 + attrLen + len(nlri)
	buf := make([]byte, total)
	binary.BigEndian.PutUint16(buf[0:2], uint16(wdLen))
	binary.BigEndian.PutUint16(buf[2:4], uint16(attrLen))
	copy(buf[4:], attrs)
	copy(buf[4+attrLen:], nlri)
	return buf
}

// makeAttr builds a single path attribute: flags + code + len + value.
func makeAttr(flags, code byte, value []byte) []byte {
	attr := make([]byte, 3+len(value))
	attr[0] = flags
	attr[1] = code
	attr[2] = byte(len(value))
	copy(attr[3:], value)
	return attr
}

// VALIDATES: AC-9 — buildModifiedPayload returns nil when mods.Len() == 0.
// PREVENTS: Unnecessary allocation on the zero-mod fast path.
func TestProgressiveBuildNoMods(t *testing.T) {
	attrs := makeAttr(0x40, 1, []byte{0x00}) // ORIGIN=IGP
	payload := buildModTestPayload(attrs, nil)

	var mods registry.ModAccumulator
	result := buildModifiedPayload(payload, &mods, nil)
	assert.Nil(t, result, "no mods should return nil")
}

// VALIDATES: AC-13, AC-14 — OTC added when source has no OTC attr.
// PREVENTS: Progressive build fails to add new attributes.
func TestProgressiveBuildOTCAdd(t *testing.T) {
	// Source: ORIGIN attribute only.
	origin := makeAttr(0x40, 1, []byte{0x00})
	nlri := []byte{24, 10, 0, 0} // 10.0.0.0/24
	payload := buildModTestPayload(origin, nlri)

	// OTC handler: writes 7-byte OTC attribute.
	otcHandler := registry.AttrModHandler(func(src []byte, ops []registry.AttrOp, buf []byte, off int) int {
		if len(src) > 0 {
			copy(buf[off:], src)
			return off + len(src)
		}
		for _, op := range ops {
			if op.Action != registry.AttrModSet || len(op.Buf) != 4 {
				continue
			}
			buf[off] = 0xC0 // flags: Optional + Transitive
			buf[off+1] = 35 // OTC type code
			buf[off+2] = 4  // length
			copy(buf[off+3:], op.Buf)
			return off + 7
		}
		return off
	})

	handlers := map[uint8]registry.AttrModHandler{35: otcHandler}

	asnBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(asnBuf, 65000)

	var mods registry.ModAccumulator
	mods.Op(35, registry.AttrModSet, asnBuf)

	result := buildModifiedPayload(payload, &mods, handlers)
	require.NotNil(t, result, "should produce modified payload")

	// Parse result: should have ORIGIN + OTC + NLRI.
	require.True(t, len(result) > len(payload), "result should be longer (OTC added)")

	// Check withdrawn_len preserved.
	wdLen := binary.BigEndian.Uint16(result[0:2])
	assert.Equal(t, uint16(0), wdLen)

	// Check attr_len updated.
	attrLen := int(binary.BigEndian.Uint16(result[2:4]))
	assert.Equal(t, len(origin)+7, attrLen, "attr_len should include ORIGIN + OTC")

	// Check ORIGIN preserved.
	assert.Equal(t, origin, result[4:4+len(origin)])

	// Check OTC appended after ORIGIN.
	otcStart := 4 + len(origin)
	assert.Equal(t, byte(0xC0), result[otcStart], "OTC flags")
	assert.Equal(t, byte(35), result[otcStart+1], "OTC type")
	assert.Equal(t, byte(4), result[otcStart+2], "OTC length")
	asn := binary.BigEndian.Uint32(result[otcStart+3 : otcStart+7])
	assert.Equal(t, uint32(65000), asn)

	// Check NLRI preserved at end.
	nlriStart := 4 + attrLen
	assert.Equal(t, nlri, result[nlriStart:])
}

// VALIDATES: AC-15 — Replace existing attribute value.
// PREVENTS: Progressive build fails to replace existing attributes.
func TestProgressiveBuildAttrReplace(t *testing.T) {
	// Source: ORIGIN + LOCAL_PREF=100
	origin := makeAttr(0x40, 1, []byte{0x00})
	lpValue := make([]byte, 4)
	binary.BigEndian.PutUint32(lpValue, 100)
	localPref := makeAttr(0x40, 5, lpValue)
	attrs := slices.Concat(origin, localPref)
	payload := buildModTestPayload(attrs, nil)

	// LOCAL_PREF handler: replaces value with op's buf.
	lpHandler := registry.AttrModHandler(func(src []byte, ops []registry.AttrOp, buf []byte, off int) int {
		for _, op := range ops {
			if op.Action != registry.AttrModSet {
				continue
			}
			buf[off] = 0x40 // flags
			buf[off+1] = 5  // LOCAL_PREF code
			buf[off+2] = byte(len(op.Buf))
			copy(buf[off+3:], op.Buf)
			return off + 3 + len(op.Buf)
		}
		// No set op: copy source.
		if len(src) > 0 {
			copy(buf[off:], src)
			return off + len(src)
		}
		return off
	})

	handlers := map[uint8]registry.AttrModHandler{5: lpHandler}

	newLPValue := make([]byte, 4)
	binary.BigEndian.PutUint32(newLPValue, 0)
	var mods registry.ModAccumulator
	mods.Op(5, registry.AttrModSet, newLPValue)

	result := buildModifiedPayload(payload, &mods, handlers)
	require.NotNil(t, result)

	// Same total length (same-size replacement).
	assert.Equal(t, len(payload), len(result))

	// Check LOCAL_PREF value changed to 0.
	attrLen := int(binary.BigEndian.Uint16(result[2:4]))
	lpStart := 4 + len(origin)
	assert.Equal(t, byte(5), result[lpStart+1], "LOCAL_PREF code preserved")
	newLP := binary.BigEndian.Uint32(result[lpStart+3 : lpStart+3+4])
	assert.Equal(t, uint32(0), newLP, "LOCAL_PREF value should be 0")
	_ = attrLen
}

// VALIDATES: AC-16 — Multiple ops on same attr code.
// PREVENTS: Handler receives partial ops.
func TestProgressiveBuildMultiOps(t *testing.T) {
	// Source: ORIGIN + COMMUNITY with one value.
	origin := makeAttr(0x40, 1, []byte{0x00})
	commValue := make([]byte, 4)
	binary.BigEndian.PutUint32(commValue, 0xFFFF0001) // no-export
	community := makeAttr(0xC0, 8, commValue)
	attrs := slices.Concat(origin, community)
	payload := buildModTestPayload(attrs, nil)

	// COMMUNITY handler: tracks how many ops it received.
	var receivedOps int
	commHandler := registry.AttrModHandler(func(src []byte, ops []registry.AttrOp, buf []byte, off int) int {
		receivedOps = len(ops)
		// Just copy source for simplicity.
		if len(src) > 0 {
			copy(buf[off:], src)
			return off + len(src)
		}
		return off
	})

	handlers := map[uint8]registry.AttrModHandler{8: commHandler}

	var mods registry.ModAccumulator
	mods.Op(8, registry.AttrModAdd, []byte{0xFF, 0xFF, 0x00, 0x02})    // add community
	mods.Op(8, registry.AttrModRemove, []byte{0xFF, 0xFF, 0x00, 0x01}) // remove no-export
	mods.Op(8, registry.AttrModAdd, []byte{0xFF, 0xFF, 0x00, 0x03})    // add another

	result := buildModifiedPayload(payload, &mods, handlers)
	require.NotNil(t, result)
	assert.Equal(t, 3, receivedOps, "handler should receive all 3 ops at once")
}

// VALIDATES: AC-18 — Unknown attr code: ops skipped, source copied.
// PREVENTS: Panic or data loss on unregistered handler code.
func TestProgressiveBuildUnknownCode(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	payload := buildModTestPayload(origin, nil)

	var mods registry.ModAccumulator
	mods.Op(99, registry.AttrModSet, []byte{0x01}) // No handler for code 99.

	// No handlers registered.
	result := buildModifiedPayload(payload, &mods, map[uint8]registry.AttrModHandler{})
	require.NotNil(t, result)

	// ORIGIN should still be present.
	attrLen := int(binary.BigEndian.Uint16(result[2:4]))
	assert.Equal(t, len(origin), attrLen, "original attrs preserved")
}

// VALIDATES: Withdrawn section copied verbatim.
// PREVENTS: Withdrawn routes corrupted by attr mods.
func TestProgressiveBuildWithdrawnPreserved(t *testing.T) {
	// Build payload with withdrawn routes.
	withdrawn := []byte{24, 10, 0, 0} // 10.0.0.0/24
	origin := makeAttr(0x40, 1, []byte{0x00})
	attrLen := len(origin)

	total := 2 + len(withdrawn) + 2 + attrLen
	payload := make([]byte, total)
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(withdrawn)))
	copy(payload[2:], withdrawn)
	binary.BigEndian.PutUint16(payload[2+len(withdrawn):], uint16(attrLen))
	copy(payload[2+len(withdrawn)+2:], origin)

	// Add a new OTC attribute to force modification.
	otcHandler := registry.AttrModHandler(func(_ []byte, ops []registry.AttrOp, buf []byte, off int) int {
		buf[off] = 0xC0
		buf[off+1] = 35
		buf[off+2] = 4
		copy(buf[off+3:], ops[0].Buf)
		return off + 7
	})

	var mods registry.ModAccumulator
	asnBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(asnBuf, 65000)
	mods.Op(35, registry.AttrModSet, asnBuf)

	result := buildModifiedPayload(payload, &mods, map[uint8]registry.AttrModHandler{35: otcHandler})
	require.NotNil(t, result)

	// Check withdrawn section preserved.
	wdLen := int(binary.BigEndian.Uint16(result[0:2]))
	assert.Equal(t, len(withdrawn), wdLen)
	assert.Equal(t, withdrawn, result[2:2+wdLen])
}

// VALIDATES: NLRI section preserved after attr modification.
// PREVENTS: NLRI lost or corrupted during progressive build.
func TestProgressiveBuildNLRIPreserved(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	nlri := []byte{24, 10, 0, 0, 16, 172, 16} // Two prefixes.
	payload := buildModTestPayload(origin, nlri)

	otcHandler := registry.AttrModHandler(func(_ []byte, ops []registry.AttrOp, buf []byte, off int) int {
		buf[off] = 0xC0
		buf[off+1] = 35
		buf[off+2] = 4
		copy(buf[off+3:], ops[0].Buf)
		return off + 7
	})

	var mods registry.ModAccumulator
	asnBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(asnBuf, 65000)
	mods.Op(35, registry.AttrModSet, asnBuf)

	result := buildModifiedPayload(payload, &mods, map[uint8]registry.AttrModHandler{35: otcHandler})
	require.NotNil(t, result)

	// NLRI should be at the end after the expanded attr section.
	newAttrLen := int(binary.BigEndian.Uint16(result[2:4]))
	nlriStart := 4 + newAttrLen
	assert.Equal(t, nlri, result[nlriStart:], "NLRI must be preserved verbatim")
}

// VALIDATES: attr_len backfilled correctly after mods.
// PREVENTS: Wrong attr_len causing parse failures.
func TestProgressiveBuildAttrLenBackfill(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00}) // 4 bytes
	payload := buildModTestPayload(origin, nil)

	// Handler adds 7-byte OTC.
	otcHandler := registry.AttrModHandler(func(_ []byte, _ []registry.AttrOp, buf []byte, off int) int {
		buf[off] = 0xC0
		buf[off+1] = 35
		buf[off+2] = 4
		buf[off+3] = 0
		buf[off+4] = 0
		buf[off+5] = 0xFD
		buf[off+6] = 0xE8
		return off + 7
	})

	var mods registry.ModAccumulator
	mods.Op(35, registry.AttrModSet, []byte{0, 0, 0xFD, 0xE8})

	result := buildModifiedPayload(payload, &mods, map[uint8]registry.AttrModHandler{35: otcHandler})
	require.NotNil(t, result)

	attrLen := int(binary.BigEndian.Uint16(result[2:4]))
	assert.Equal(t, len(origin)+7, attrLen, "attr_len = ORIGIN(4) + OTC(7) = 11")

	// Verify the actual attribute bytes match attr_len.
	actualAttrs := result[4 : 4+attrLen]
	assert.Equal(t, attrLen, len(actualAttrs))
}

// VALIDATES: Handler panic is caught, source attr preserved.
// PREVENTS: Panic in handler crashes forward path.
func TestProgressiveBuildHandlerPanic(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	localPref := makeAttr(0x40, 5, []byte{0, 0, 0, 100})
	attrs := slices.Concat(origin, localPref)
	payload := buildModTestPayload(attrs, nil)

	panicHandler := registry.AttrModHandler(func(_ []byte, _ []registry.AttrOp, _ []byte, _ int) int {
		panic("test panic in handler")
	})

	handlers := map[uint8]registry.AttrModHandler{5: panicHandler}

	var mods registry.ModAccumulator
	mods.Op(5, registry.AttrModSet, []byte{0, 0, 0, 0})

	result := buildModifiedPayload(payload, &mods, handlers)
	require.NotNil(t, result)

	// LOCAL_PREF should be copied unchanged (panic recovery).
	attrLen := int(binary.BigEndian.Uint16(result[2:4]))
	assert.Equal(t, len(attrs), attrLen, "attrs unchanged after panic")
}

// VALIDATES: Extended-length attribute parsing in progressive build.
// PREVENTS: Silent corruption when payload contains extended-length attrs (flags & 0x10).
func TestProgressiveBuildExtendedLengthAttr(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00}) // 4 bytes, standard header

	// Build an extended-length attribute: flags=0xD0 (Optional+Transitive+ExtLength),
	// code=16 (Extended Communities), 2-byte length, 8-byte value.
	extValue := []byte{0x00, 0x02, 0x00, 0x01, 0x0A, 0x00, 0x00, 0x01}
	extAttr := make([]byte, 4+len(extValue))
	extAttr[0] = 0xD0 // flags: Optional+Transitive+ExtendedLength
	extAttr[1] = 16   // Extended Communities
	binary.BigEndian.PutUint16(extAttr[2:4], uint16(len(extValue)))
	copy(extAttr[4:], extValue)

	attrs := slices.Concat(origin, extAttr)
	payload := buildModTestPayload(attrs, nil)

	// Add OTC via handler (new attribute, not touching extended-length one).
	otcHandler := registry.AttrModHandler(func(_ []byte, ops []registry.AttrOp, buf []byte, off int) int {
		buf[off] = 0xC0
		buf[off+1] = 35
		buf[off+2] = 4
		copy(buf[off+3:], ops[0].Buf)
		return off + 7
	})

	var mods registry.ModAccumulator
	asnBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(asnBuf, 65000)
	mods.Op(35, registry.AttrModSet, asnBuf)

	result := buildModifiedPayload(payload, &mods, map[uint8]registry.AttrModHandler{35: otcHandler})
	require.NotNil(t, result)

	// Check attr_len = ORIGIN(4) + ExtComm(12) + OTC(7) = 23.
	attrLen := int(binary.BigEndian.Uint16(result[2:4]))
	assert.Equal(t, len(origin)+len(extAttr)+7, attrLen, "attr_len should include all attrs")

	// Verify extended-length attribute preserved verbatim.
	assert.Equal(t, origin, result[4:4+len(origin)], "ORIGIN preserved")
	extStart := 4 + len(origin)
	assert.Equal(t, extAttr, result[extStart:extStart+len(extAttr)], "extended-length attr preserved verbatim")
}

// VALIDATES: buildModifiedPayload returns nil on malformed payloads.
// PREVENTS: Panic on truncated or corrupt input.
func TestProgressiveBuildMalformedPayload(t *testing.T) {
	var mods registry.ModAccumulator
	mods.Op(35, registry.AttrModSet, []byte{0, 0, 0xFD, 0xE8})
	handlers := map[uint8]registry.AttrModHandler{}

	tests := []struct {
		name    string
		payload []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"too_short", []byte{0x00, 0x01}},
		{"truncated_withdrawn", []byte{0x00, 0x10, 0x00, 0x00}}, // withdrawn_len=16 but only 4 bytes total
		{"truncated_attrs", []byte{0x00, 0x00, 0x00, 0x10}},     // attr_len=16 but only 4 bytes total
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildModifiedPayload(tt.payload, &mods, handlers)
			assert.Nil(t, result, "malformed payload should return nil")
		})
	}
}

// VALIDATES: Handler panic during new attribute creation (src=nil).
// PREVENTS: Panic in handler for new attribute crashes forward path.
func TestProgressiveBuildNewAttrHandlerPanic(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	payload := buildModTestPayload(origin, nil)

	panicHandler := registry.AttrModHandler(func(_ []byte, _ []registry.AttrOp, _ []byte, _ int) int {
		panic("test panic creating new attr")
	})

	var mods registry.ModAccumulator
	mods.Op(35, registry.AttrModSet, []byte{0, 0, 0xFD, 0xE8})

	result := buildModifiedPayload(payload, &mods, map[uint8]registry.AttrModHandler{35: panicHandler})
	require.NotNil(t, result)

	// ORIGIN preserved, new attr not written (panic skipped it).
	attrLen := int(binary.BigEndian.Uint16(result[2:4]))
	assert.Equal(t, len(origin), attrLen, "only ORIGIN preserved after new-attr handler panic")
}

// VALIDATES: attr_len overflow returns nil.
// PREVENTS: Bogus attr_len written on overflow.
func TestProgressiveBuildAttrLenOverflow(t *testing.T) {
	// Build a large payload near the attr_len limit.
	bigValue := make([]byte, 65500)
	bigAttr := make([]byte, 4+len(bigValue))
	bigAttr[0] = 0xD0 // Extended length
	bigAttr[1] = 99
	binary.BigEndian.PutUint16(bigAttr[2:4], uint16(len(bigValue)))
	copy(bigAttr[4:], bigValue)

	payload := buildModTestPayload(bigAttr, nil)

	// Handler that writes 100 bytes (will push past 65535).
	bigHandler := registry.AttrModHandler(func(_ []byte, _ []registry.AttrOp, buf []byte, off int) int {
		n := 100
		if off+n > len(buf) {
			return off
		}
		for i := range n {
			buf[off+i] = 0xFF
		}
		return off + n
	})

	var mods registry.ModAccumulator
	mods.Op(200, registry.AttrModSet, []byte{0x01})

	result := buildModifiedPayload(payload, &mods, map[uint8]registry.AttrModHandler{200: bigHandler})
	assert.Nil(t, result, "should return nil on attr_len overflow")
}

// VALIDATES: Handler returning invalid offset is caught and source preserved.
// PREVENTS: Buffer corruption from buggy handler returning offset < input or > buf length.
func TestProgressiveBuildInvalidHandlerOffset(t *testing.T) {
	t.Run("offset_below_input", func(t *testing.T) {
		origin := makeAttr(0x40, 1, []byte{0x00})
		localPref := makeAttr(0x40, 5, []byte{0, 0, 0, 100})
		attrs := slices.Concat(origin, localPref)
		payload := buildModTestPayload(attrs, nil)

		// Handler returns off-1 (invalid: below input offset).
		badHandler := registry.AttrModHandler(func(_ []byte, _ []registry.AttrOp, _ []byte, off int) int {
			return off - 1
		})

		var mods registry.ModAccumulator
		mods.Op(5, registry.AttrModSet, []byte{0, 0, 0, 0})

		result := buildModifiedPayload(payload, &mods, map[uint8]registry.AttrModHandler{5: badHandler})
		require.NotNil(t, result, "should fall back to source copy, not abandon")

		// LOCAL_PREF should be preserved unchanged (fallback to safeCopy).
		attrLen := int(binary.BigEndian.Uint16(result[2:4]))
		assert.Equal(t, len(attrs), attrLen, "attrs unchanged after invalid offset")
	})

	t.Run("offset_beyond_buffer", func(t *testing.T) {
		origin := makeAttr(0x40, 1, []byte{0x00})
		localPref := makeAttr(0x40, 5, []byte{0, 0, 0, 100})
		attrs := slices.Concat(origin, localPref)
		payload := buildModTestPayload(attrs, nil)

		// Handler returns len(buf)+1 (invalid: beyond buffer).
		badHandler := registry.AttrModHandler(func(_ []byte, _ []registry.AttrOp, buf []byte, _ int) int {
			return len(buf) + 1
		})

		var mods registry.ModAccumulator
		mods.Op(5, registry.AttrModSet, []byte{0, 0, 0, 0})

		result := buildModifiedPayload(payload, &mods, map[uint8]registry.AttrModHandler{5: badHandler})
		require.NotNil(t, result, "should fall back to source copy, not abandon")

		attrLen := int(binary.BigEndian.Uint16(result[2:4]))
		assert.Equal(t, len(attrs), attrLen, "attrs unchanged after invalid offset")
	})

	t.Run("invalid_offset_new_attr", func(t *testing.T) {
		origin := makeAttr(0x40, 1, []byte{0x00})
		payload := buildModTestPayload(origin, nil)

		// Handler for new attr returns negative offset.
		badHandler := registry.AttrModHandler(func(_ []byte, _ []registry.AttrOp, _ []byte, off int) int {
			return off - 1
		})

		var mods registry.ModAccumulator
		mods.Op(35, registry.AttrModSet, []byte{0, 0, 0xFD, 0xE8})

		result := buildModifiedPayload(payload, &mods, map[uint8]registry.AttrModHandler{35: badHandler})
		require.NotNil(t, result)

		// New attr skipped, only ORIGIN in output.
		attrLen := int(binary.BigEndian.Uint16(result[2:4]))
		assert.Equal(t, len(origin), attrLen, "new attr skipped on invalid offset")
	})
}

// VALIDATES: Buffer overflow during verbatim copy triggers graceful nil return.
// PREVENTS: Panic when handler expansion leaves no room for subsequent attrs.
func TestProgressiveBuildBufferOverflow(t *testing.T) {
	// Build a payload where ORIGIN is first, LOCAL_PREF is second.
	// The handler for ORIGIN will expand it to fill the buffer,
	// leaving no room for the LOCAL_PREF verbatim copy.
	origin := makeAttr(0x40, 1, []byte{0x00})            // 4 bytes
	localPref := makeAttr(0x40, 5, []byte{0, 0, 0, 100}) // 7 bytes
	attrs := slices.Concat(origin, localPref)
	nlri := []byte{24, 10, 0, 0}
	payload := buildModTestPayload(attrs, nlri)

	// Buffer size = len(payload) + 256. payload = 2+2+11+4 = 19. buf = 275.
	// After withdrawn(2) + attr_len_skip(2) = off is 4.
	// Handler writes enough to leave < 7 bytes for LOCAL_PREF.
	// Need to write: 275 - 4 - 7 + 1 = 265 bytes (leaves 6, LOCAL_PREF needs 7).
	bigHandler := registry.AttrModHandler(func(_ []byte, _ []registry.AttrOp, buf []byte, off int) int {
		n := len(buf) - off - 6 // Leave exactly 6 bytes (LOCAL_PREF needs 7)
		if n <= 0 || off+n > len(buf) {
			return off
		}
		for i := range n {
			buf[off+i] = 0xAA
		}
		return off + n
	})

	var mods registry.ModAccumulator
	mods.Op(1, registry.AttrModSet, []byte{0x00}) // Replace ORIGIN

	result := buildModifiedPayload(payload, &mods, map[uint8]registry.AttrModHandler{1: bigHandler})
	// Handler fills buffer leaving 6 bytes. LOCAL_PREF (7 bytes) won't fit.
	assert.Nil(t, result, "should return nil when buffer overflows during verbatim copy")
}

// VALIDATES: Successful modification on large payload (>4096, non-pool path).
// PREVENTS: Regression in the direct allocation path.
func TestProgressiveBuildLargePayload(t *testing.T) {
	// Build a large attribute (>4000 bytes) to exceed pool buffer size.
	bigValue := make([]byte, 4000)
	for i := range bigValue {
		bigValue[i] = byte(i % 256)
	}
	bigAttr := make([]byte, 4+len(bigValue))
	bigAttr[0] = 0xD0 // Extended length
	bigAttr[1] = 99   // Private code
	binary.BigEndian.PutUint16(bigAttr[2:4], uint16(len(bigValue)))
	copy(bigAttr[4:], bigValue)

	origin := makeAttr(0x40, 1, []byte{0x00})
	attrs := slices.Concat(origin, bigAttr)
	nlri := []byte{24, 10, 0, 0}
	payload := buildModTestPayload(attrs, nlri)

	// Add a small OTC attribute.
	otcHandler := registry.AttrModHandler(func(_ []byte, ops []registry.AttrOp, buf []byte, off int) int {
		if off+7 > len(buf) {
			return off
		}
		buf[off] = 0xC0
		buf[off+1] = 35
		buf[off+2] = 4
		copy(buf[off+3:], ops[0].Buf)
		return off + 7
	})

	var mods registry.ModAccumulator
	asnBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(asnBuf, 65000)
	mods.Op(35, registry.AttrModSet, asnBuf)

	result := buildModifiedPayload(payload, &mods, map[uint8]registry.AttrModHandler{35: otcHandler})
	require.NotNil(t, result, "large payload should produce non-nil result")

	// Verify structure.
	attrLen := int(binary.BigEndian.Uint16(result[2:4]))
	assert.Equal(t, len(origin)+len(bigAttr)+7, attrLen, "attr_len = ORIGIN + bigAttr + OTC")

	// Verify NLRI preserved at end.
	nlriStart := 4 + attrLen
	assert.Equal(t, nlri, result[nlriStart:], "NLRI preserved")
}

// VALIDATES: Progressive build produces byte-identical output to insertOTCInPayload for OTC addition.
// PREVENTS: Regression during v1-to-v2 migration.
func TestProgressiveBuildMatchesInsertOTC(t *testing.T) {
	// Build a payload with ORIGIN + AS_PATH + NLRI (no OTC).
	origin := makeAttr(0x40, 1, []byte{0x00})
	asPath := makeAttr(0x40, 2, []byte{0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9}) // AS_PATH=[65001]
	attrs := slices.Concat(origin, asPath)
	nlri := []byte{24, 10, 0, 0}
	payload := buildModTestPayload(attrs, nlri)

	localASN := uint32(65000)

	// V1 path: insertOTCInPayload (direct payload modification).
	v1Result := insertOTCInPayloadForTest(payload, localASN)
	require.NotNil(t, v1Result, "v1 should produce result")

	// V2 path: buildModifiedPayload with otcAttrModHandler.
	otcHandler := registry.AttrModHandler(func(src []byte, ops []registry.AttrOp, buf []byte, off int) int {
		if len(src) > 0 {
			if off+len(src) > len(buf) {
				return off
			}
			copy(buf[off:], src)
			return off + len(src)
		}
		for _, op := range ops {
			if op.Action != registry.AttrModSet || len(op.Buf) != 4 {
				continue
			}
			if off+7 > len(buf) {
				return off
			}
			buf[off] = 0xC0
			buf[off+1] = 35
			buf[off+2] = 4
			copy(buf[off+3:], op.Buf)
			return off + 7
		}
		return off
	})

	asnBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(asnBuf, localASN)
	var mods registry.ModAccumulator
	mods.Op(35, registry.AttrModSet, asnBuf)

	v2Result := buildModifiedPayload(payload, &mods, map[uint8]registry.AttrModHandler{35: otcHandler})
	require.NotNil(t, v2Result, "v2 should produce result")

	assert.Equal(t, v1Result, v2Result, "v1 and v2 must produce byte-identical output")
}

// insertOTCInPayloadForTest is a copy of the v1 OTC insertion logic for comparison testing.
func insertOTCInPayloadForTest(payload []byte, otcASN uint32) []byte {
	if len(payload) < 4 {
		return nil
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(payload) < attrOffset+2 {
		return nil
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	attrEnd := attrOffset + 2 + attrLen
	if len(payload) < attrEnd {
		return nil
	}
	// OTC: flags=0xC0, type=35, len=4, ASN
	otcWire := [7]byte{0xC0, 35, 4, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(otcWire[3:], otcASN)
	newAttrLen := attrLen + 7
	if newAttrLen > 65535 {
		return nil
	}
	result := make([]byte, len(payload)+7)
	copy(result, payload[:attrOffset])
	binary.BigEndian.PutUint16(result[attrOffset:], uint16(newAttrLen))
	copy(result[attrOffset+2:], payload[attrOffset+2:attrEnd])
	copy(result[attrEnd:], otcWire[:])
	copy(result[attrEnd+7:], payload[attrEnd:])
	return result
}
