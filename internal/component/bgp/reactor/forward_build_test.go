package reactor

import (
	"encoding/binary"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
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

// modTestNLRI is one advertised prefix, 10.0.0.0/24.
//
// A fixture whose modification CREATES an attribute must carry it.
// buildModifiedPayload refuses to create a path attribute on a body that
// advertises no reachable NLRI (advertiseGate, RFC 4271 Sections 4.3 and 6.3),
// so a fixture with no NLRI is a withdraw-only UPDATE claiming to be an
// advertisement, and it proves nothing about the rebuild it drives.
var modTestNLRI = []byte{24, 10, 0, 0}

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

	var mods filterapi.ModAccumulator
	result, _, _ := buildModifiedPayload(payload, &mods, nil, nil, nil)
	assert.Nil(t, result, "no mods should return nil")
}

// VALIDATES: rib-arch-8 AC-1 — ModAccumulator.SetNLRIRewrite replaces the
// announced legacy NLRI section, even with no attribute ops.
// PREVENTS: NLRI rewrite silently ignored on the forward path.
func TestBuildModifiedPayloadNLRIRewrite(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00}) // ORIGIN=IGP
	nlri := []byte{24, 10, 0, 0}              // 10.0.0.0/24
	payload := buildModTestPayload(origin, nlri)

	var mods filterapi.ModAccumulator
	rewritten := []byte{24, 172, 16, 0} // 172.16.0.0/24
	mods.SetNLRIRewrite(rewritten)
	require.True(t, mods.HasModifications(), "NLRI rewrite counts as a modification")
	assert.Equal(t, 0, mods.Len(), "NLRI rewrite is not an attribute op")

	result, _, _ := buildModifiedPayload(payload, &mods, nil, nil, nil)
	require.NotNil(t, result, "NLRI rewrite alone must produce a modified payload")

	wdLen := int(binary.BigEndian.Uint16(result[0:2]))
	attrLen := int(binary.BigEndian.Uint16(result[2+wdLen : 2+wdLen+2]))
	attrStart := 2 + wdLen + 2
	nlriStart := attrStart + attrLen
	assert.Equal(t, origin, result[attrStart:nlriStart], "attributes preserved")
	assert.Equal(t, rewritten, result[nlriStart:], "NLRI section rewritten")
}

// VALIDATES: rib-arch-8 AC-2 — ModAccumulator.SetWithdrawnRewrite replaces the
// withdrawn NLRI section so a rewritten route is withdrawn under the same prefix.
// PREVENTS: adj-rib-out desync (withdrawal referencing the original prefix).
func TestBuildModifiedPayloadWithdrawnRewrite(t *testing.T) {
	// Pure withdrawal: withdrawn_len(2)=4, withdrawn=10.0.0.0/24, attr_len(2)=0.
	withdrawn := []byte{24, 10, 0, 0}
	payload := make([]byte, 2+len(withdrawn)+2)
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(withdrawn)))
	copy(payload[2:], withdrawn)

	var mods filterapi.ModAccumulator
	rewritten := []byte{24, 172, 16, 0} // 172.16.0.0/24
	mods.SetWithdrawnRewrite(rewritten)
	require.True(t, mods.HasModifications())

	result, _, _ := buildModifiedPayload(payload, &mods, nil, nil, nil)
	require.NotNil(t, result, "withdrawn rewrite must produce a modified payload")

	wdLen := int(binary.BigEndian.Uint16(result[0:2]))
	assert.Equal(t, len(rewritten), wdLen, "withdrawn_len reflects the rewrite")
	assert.Equal(t, rewritten, result[2:2+wdLen], "withdrawn section rewritten")
	assert.Equal(t, uint16(0), binary.BigEndian.Uint16(result[2+wdLen:2+wdLen+2]), "attr_len stays 0")
}

// VALIDATES: AC-13, AC-14 — OTC added when source has no OTC attr.
// PREVENTS: Progressive build fails to add new attributes.
func TestProgressiveBuildOTCAdd(t *testing.T) {
	// Source: ORIGIN attribute only.
	origin := makeAttr(0x40, 1, []byte{0x00})
	nlri := []byte{24, 10, 0, 0} // 10.0.0.0/24
	payload := buildModTestPayload(origin, nlri)

	// OTC handler: plans a 7-byte OTC attribute.
	otcHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		if p.Source() != nil {
			p.KeepAll()
			return
		}
		for i, op := range p.Ops() {
			if op.Action != filterapi.AttrModSet || len(op.Buf) != 4 {
				continue
			}
			p.Op(i)
			p.Emit(0xC0, 35) // flags: Optional + Transitive; OTC type code
			return
		}
		p.Drop()
	})

	handlers := map[uint8]filterapi.AttrModHandler{35: otcHandler}

	asnBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(asnBuf, 65000)

	var mods filterapi.ModAccumulator
	mods.Op(35, filterapi.AttrModSet, asnBuf)

	result, _, _ := buildModifiedPayload(payload, &mods, handlers, nil, nil)
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
	lpHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		for i, op := range p.Ops() {
			if op.Action != filterapi.AttrModSet {
				continue
			}
			p.Op(i)
			p.Emit(0x40, 5) // flags; LOCAL_PREF code
			return
		}
		// No set op: keep the source.
		if p.Source() != nil {
			p.KeepAll()
			return
		}
		p.Drop()
	})

	handlers := map[uint8]filterapi.AttrModHandler{5: lpHandler}

	newLPValue := make([]byte, 4)
	binary.BigEndian.PutUint32(newLPValue, 0)
	var mods filterapi.ModAccumulator
	mods.Op(5, filterapi.AttrModSet, newLPValue)

	result, _, _ := buildModifiedPayload(payload, &mods, handlers, nil, nil)
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
	commHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		receivedOps = len(p.Ops())
		// Just keep the source for simplicity.
		if p.Source() != nil {
			p.KeepAll()
			return
		}
		p.Drop()
	})

	handlers := map[uint8]filterapi.AttrModHandler{8: commHandler}

	var mods filterapi.ModAccumulator
	mods.Op(8, filterapi.AttrModAdd, []byte{0xFF, 0xFF, 0x00, 0x02})    // add community
	mods.Op(8, filterapi.AttrModRemove, []byte{0xFF, 0xFF, 0x00, 0x01}) // remove no-export
	mods.Op(8, filterapi.AttrModAdd, []byte{0xFF, 0xFF, 0x00, 0x03})    // add another

	result, _, _ := buildModifiedPayload(payload, &mods, handlers, nil, nil)
	require.NotNil(t, result)
	assert.Equal(t, 3, receivedOps, "handler should receive all 3 ops at once")
}

// VALIDATES: an unregistered handler code SUPPRESSES the route.
// PREVENTS: emitting a route whose policy did not take effect. For RFC 9234
// (OTC, code 35) a missing handler would send the route without the attribute
// Section 5 requires, so skip-and-copy is an RFC violation, not a safe fallback.
//
// test-relax: SUPERSEDES AC-18 ("ops skipped, source copied"). Thomas ruled
// 2026-08-01 that correctness governs and there is one correct way forward. The
// panic half of AC-18's concern is still met by safeAttrModHandler's recover;
// only its skip-and-forward conclusion is reversed. See modifyFailure.failed.
func TestProgressiveBuildUnknownCode(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	payload := buildModTestPayload(origin, modTestNLRI)

	var mods filterapi.ModAccumulator
	mods.Op(99, filterapi.AttrModSet, []byte{0x01}) // No handler for code 99.

	// No handlers registered.
	result, _, fail := buildModifiedPayload(payload, &mods, map[uint8]filterapi.AttrModHandler{}, nil, nil)
	assert.Equal(t, modifyFailureNoHandler, fail, "an unapplied op must name itself")
	assert.True(t, fail.failed(), "the caller must suppress")
	assert.Nil(t, result, "no payload is handed out")
	_ = origin
}

// VALIDATES: Withdrawn section copied verbatim.
// PREVENTS: Withdrawn routes corrupted by attr mods.
//
// The modification REPLACES the ORIGIN the source carries rather than adding a
// new OTC attribute, which is what it did before 2026-08-04. This body withdraws
// and advertises nothing, so creating an attribute on it is the RFC 4271 Section
// 6.3 Missing-Well-known-Attribute shape that advertiseGate now refuses: the
// fixture was asserting the withdrawn bytes survive a rebuild that must never
// happen. Replacing a PRESENT attribute drives the same rebuild and asserts the
// same bytes.
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

	// Rewrite the ORIGIN the source already carries, to force a modification.
	originHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		p.Op(0)
		p.Emit(0x40, 1)
	})

	var mods filterapi.ModAccumulator
	mods.Op(1, filterapi.AttrModSet, []byte{0x02}) // ORIGIN=INCOMPLETE

	result, _, _ := buildModifiedPayload(payload, &mods, map[uint8]filterapi.AttrModHandler{1: originHandler}, nil, nil)
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

	otcHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		p.Op(0)
		p.Emit(0xC0, 35)
	})

	var mods filterapi.ModAccumulator
	asnBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(asnBuf, 65000)
	mods.Op(35, filterapi.AttrModSet, asnBuf)

	result, _, _ := buildModifiedPayload(payload, &mods, map[uint8]filterapi.AttrModHandler{35: otcHandler}, nil, nil)
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
	payload := buildModTestPayload(origin, modTestNLRI)

	// Handler adds 7-byte OTC.
	otcHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		p.New([]byte{0, 0, 0xFD, 0xE8})
		p.Emit(0xC0, 35)
	})

	var mods filterapi.ModAccumulator
	mods.Op(35, filterapi.AttrModSet, []byte{0, 0, 0xFD, 0xE8})

	result, _, _ := buildModifiedPayload(payload, &mods, map[uint8]filterapi.AttrModHandler{35: otcHandler}, nil, nil)
	require.NotNil(t, result)

	attrLen := int(binary.BigEndian.Uint16(result[2:4]))
	assert.Equal(t, len(origin)+7, attrLen, "attr_len = ORIGIN(4) + OTC(7) = 11")

	// Verify the actual attribute bytes match attr_len.
	actualAttrs := result[4 : 4+attrLen]
	assert.Equal(t, attrLen, len(actualAttrs))
}

// VALIDATES: a handler panic is caught AND the route is suppressed.
// PREVENTS: a panic crashing the forward path, and separately, a recovered panic
// silently forwarding a route whose modification never happened.
//
// test-relax: SUPERSEDES the previous "attrs unchanged after panic" expectation.
// Recovery copies the source through, which produced a valid payload and a
// success verdict, so the route went out unmodified. Thomas ruled 2026-08-01
// that correctness governs. The panic-containment half is unchanged.
func TestProgressiveBuildHandlerPanic(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	localPref := makeAttr(0x40, 5, []byte{0, 0, 0, 100})
	attrs := slices.Concat(origin, localPref)
	payload := buildModTestPayload(attrs, nil)

	panicHandler := filterapi.AttrModHandler(func(_ *filterapi.AttrPlan) {
		panic("test panic in handler")
	})

	handlers := map[uint8]filterapi.AttrModHandler{5: panicHandler}

	var mods filterapi.ModAccumulator
	mods.Op(5, filterapi.AttrModSet, []byte{0, 0, 0, 0})

	result, _, fail := buildModifiedPayload(payload, &mods, handlers, nil, nil)

	assert.Equal(t, modifyFailureHandlerFault, fail, "a recovered panic must name itself")
	assert.True(t, fail.failed(), "the caller must suppress")
	assert.Nil(t, result, "no payload is handed out after a handler panic")
	assert.NotEmpty(t, attrs, "guard: the fixture built attributes")
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
	payload := buildModTestPayload(attrs, modTestNLRI)

	// Add OTC via handler (new attribute, not touching extended-length one).
	otcHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		p.Op(0)
		p.Emit(0xC0, 35)
	})

	var mods filterapi.ModAccumulator
	asnBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(asnBuf, 65000)
	mods.Op(35, filterapi.AttrModSet, asnBuf)

	result, _, _ := buildModifiedPayload(payload, &mods, map[uint8]filterapi.AttrModHandler{35: otcHandler}, nil, nil)
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
	var mods filterapi.ModAccumulator
	mods.Op(35, filterapi.AttrModSet, []byte{0, 0, 0xFD, 0xE8})
	handlers := map[uint8]filterapi.AttrModHandler{}

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
			result, _, _ := buildModifiedPayload(tt.payload, &mods, handlers, nil, nil)
			assert.Nil(t, result, "malformed payload should return nil")
		})
	}
}

// VALIDATES: a panic while creating a NEW attribute is caught AND suppresses.
// PREVENTS: the crash, and separately the RFC 9234 hole -- code 35 is OTC, so
// forwarding after a failed add sends a route without the attribute Section 5
// requires.
//
// test-relax: SUPERSEDES "only ORIGIN preserved after new-attr handler panic".
// Thomas ruled 2026-08-01 that correctness governs. Panic containment is
// unchanged; only the forward-anyway conclusion is reversed.
func TestProgressiveBuildNewAttrHandlerPanic(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	payload := buildModTestPayload(origin, modTestNLRI)

	panicHandler := filterapi.AttrModHandler(func(_ *filterapi.AttrPlan) {
		panic("test panic creating new attr")
	})

	var mods filterapi.ModAccumulator
	mods.Op(35, filterapi.AttrModSet, []byte{0, 0, 0xFD, 0xE8})

	result, _, fail := buildModifiedPayload(payload, &mods, map[uint8]filterapi.AttrModHandler{35: panicHandler}, nil, nil)
	assert.Equal(t, modifyFailureHandlerFault, fail, "a recovered panic must name itself")
	assert.True(t, fail.failed(), "the caller must suppress")
	assert.Nil(t, result, "no payload is handed out")
	assert.NotEmpty(t, origin, "guard: the fixture built an ORIGIN attribute")
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

	// Handler that plans 100 value bytes (will push past 65535).
	bigHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		p.Op(0)
		p.Emit(0xC0, p.Code())
	})

	var mods filterapi.ModAccumulator
	mods.Op(200, filterapi.AttrModSet, make([]byte, 100))

	result, _, _ := buildModifiedPayload(payload, &mods, map[uint8]filterapi.AttrModHandler{200: bigHandler}, nil, nil)
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

		// A handler that refuses. Under the exactly-sized rebuild a handler no
		// longer returns an offset, so "an offset below the input" has become
		// "the plan could not be produced"; the fault class and the verdict are
		// unchanged.
		badHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
			p.Fail()
		})

		var mods filterapi.ModAccumulator
		mods.Op(5, filterapi.AttrModSet, []byte{0, 0, 0, 0})

		// test-relax: SUPERSEDES "should fall back to source copy, not abandon".
		// An out-of-range offset means the handler's operations never landed, so
		// the route must not go out. Thomas ruled 2026-08-01 that correctness
		// governs.
		result, _, fail := buildModifiedPayload(payload, &mods, map[uint8]filterapi.AttrModHandler{5: badHandler}, nil, nil)
		assert.True(t, fail.failed(), "an invalid handler offset must suppress")
		assert.Nil(t, result, "no payload is handed out")
		assert.NotEmpty(t, attrs, "guard: the fixture built attributes")
	})

	t.Run("offset_beyond_buffer", func(t *testing.T) {
		origin := makeAttr(0x40, 1, []byte{0x00})
		localPref := makeAttr(0x40, 5, []byte{0, 0, 0, 100})
		attrs := slices.Concat(origin, localPref)
		payload := buildModTestPayload(attrs, nil)

		// A handler naming bytes beyond the source value. This is the direct
		// successor of "an offset beyond the buffer": the plan bounds every
		// fragment against the source at construction, so the same fault is
		// caught before a buffer exists rather than after one was written.
		badHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
			p.Keep(0, 1<<20)
			p.Emit(0x40, 5)
		})

		var mods filterapi.ModAccumulator
		mods.Op(5, filterapi.AttrModSet, []byte{0, 0, 0, 0})

		// test-relax: SUPERSEDES "should fall back to source copy, not abandon".
		// An out-of-range offset means the handler's operations never landed, so
		// the route must not go out. Thomas ruled 2026-08-01 that correctness
		// governs.
		result, _, fail := buildModifiedPayload(payload, &mods, map[uint8]filterapi.AttrModHandler{5: badHandler}, nil, nil)
		assert.True(t, fail.failed(), "an invalid handler offset must suppress")
		assert.Nil(t, result, "no payload is handed out")
		assert.NotEmpty(t, attrs, "guard: the fixture built attributes")
	})

	t.Run("invalid_offset_new_attr", func(t *testing.T) {
		origin := makeAttr(0x40, 1, []byte{0x00})
		payload := buildModTestPayload(origin, modTestNLRI)

		// A handler that refuses while creating a NEW attribute.
		badHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
			p.Fail()
		})

		var mods filterapi.ModAccumulator
		mods.Op(35, filterapi.AttrModSet, []byte{0, 0, 0xFD, 0xE8})

		// test-relax: same reversal -- the attribute the policy asked to add was
		// never written, so forwarding emits a route missing it. For code 35
		// that is the RFC 9234 OTC attribute.
		result, _, fail := buildModifiedPayload(payload, &mods, map[uint8]filterapi.AttrModHandler{35: badHandler}, nil, nil)
		assert.True(t, fail.failed(), "an invalid new-attr offset must suppress")
		assert.Nil(t, result, "no payload is handed out")
		assert.NotEmpty(t, origin, "guard: the fixture built an ORIGIN attribute")
	})
}

// VALIDATES: An expansion that cannot fit an UPDATE triggers a graceful nil return.
// PREVENTS: Panic or truncation when a handler expands an attribute past what a
// message can carry.
//
// test-relax: the MECHANISM this test drives changed and the assertion did not.
// "The handler filled the slack buffer and left no room for the next verbatim
// copy" is no longer reachable: the buffer is sized from the plan, so it always
// has exactly the room the plan needs. The surviving overflow is an expansion
// past the RFC 8654 body ceiling, which is what this now drives.
func TestProgressiveBuildBufferOverflow(t *testing.T) {
	// Build a payload where ORIGIN is first, LOCAL_PREF is second. The handler
	// for ORIGIN expands it past anything an UPDATE body can carry.
	origin := makeAttr(0x40, 1, []byte{0x00})            // 4 bytes
	localPref := makeAttr(0x40, 5, []byte{0, 0, 0, 100}) // 7 bytes
	attrs := slices.Concat(origin, localPref)
	nlri := []byte{24, 10, 0, 0}
	payload := buildModTestPayload(attrs, nlri)

	// 65500 value bytes plus LOCAL_PREF, the withdrawn and NLRI sections and the
	// two length fields lands past the 65516-octet ceiling.
	bigHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		p.Op(0)
		p.Emit(0x40, 1)
	})

	var mods filterapi.ModAccumulator
	mods.Op(1, filterapi.AttrModSet, make([]byte, 65500)) // Replace ORIGIN

	result, _, _ := buildModifiedPayload(payload, &mods, map[uint8]filterapi.AttrModHandler{1: bigHandler}, nil, nil)
	assert.Nil(t, result, "should return nil when the expansion cannot fit an UPDATE body")
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
	otcHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		p.Op(0)
		p.Emit(0xC0, 35)
	})

	var mods filterapi.ModAccumulator
	asnBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(asnBuf, 65000)
	mods.Op(35, filterapi.AttrModSet, asnBuf)

	result, _, _ := buildModifiedPayload(payload, &mods, map[uint8]filterapi.AttrModHandler{35: otcHandler}, nil, nil)
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

	// Direct path: insertOTCInPayload (direct payload modification).
	v1Result := insertOTCInPayloadForTest(payload, localASN)
	require.NotNil(t, v1Result, "v1 should produce result")

	// Mod-handler path: buildModifiedPayload with otcAttrModHandler.
	otcHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		if p.Source() != nil {
			p.KeepAll()
			return
		}
		for i, op := range p.Ops() {
			if op.Action != filterapi.AttrModSet || len(op.Buf) != 4 {
				continue
			}
			p.Op(i)
			p.Emit(0xC0, 35)
			return
		}
		p.Drop()
	})

	asnBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(asnBuf, localASN)
	var mods filterapi.ModAccumulator
	mods.Op(35, filterapi.AttrModSet, asnBuf)

	v2Result, _, _ := buildModifiedPayload(payload, &mods, map[uint8]filterapi.AttrModHandler{35: otcHandler}, nil, nil)
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

// TestBuildWithdrawalPayload_IPv4 verifies conversion of IPv4 unicast announce to withdrawal.
//
// VALIDATES: AC-2: EBGP non-LLGR peer receives withdrawal (IPv4 path).
// PREVENTS: Stale routes staying in non-LLGR peer's RIB.
func TestBuildWithdrawalPayload_IPv4(t *testing.T) {
	// Build an announce: no withdrawn, some attrs, NLRI = 10.0.0.0/24.
	nlri := []byte{24, 10, 0, 0} // /24, 10.0.0.0
	attrs := makeAttr(0x40, 1, []byte{0})
	payload := buildModTestPayload(attrs, nlri)

	result, _ := buildWithdrawalPayload(payload, nil)
	require.NotNil(t, result, "should produce withdrawal")

	// Withdrawn length should be len(nlri).
	wdLen := binary.BigEndian.Uint16(result[0:2])
	assert.Equal(t, uint16(len(nlri)), wdLen, "withdrawn routes should contain the NLRI")

	// Withdrawn bytes should match original NLRI.
	assert.Equal(t, nlri, result[2:2+wdLen], "withdrawn routes should be the original NLRI")

	// Attr len should be 0.
	attrLen := binary.BigEndian.Uint16(result[2+wdLen : 4+wdLen])
	assert.Equal(t, uint16(0), attrLen, "no attributes in withdrawal")
}

// TestBuildWithdrawalPayload_MPReach verifies conversion of MP_REACH announce to MP_UNREACH withdrawal.
//
// VALIDATES: AC-2: EBGP non-LLGR peer receives withdrawal (MP path, e.g., IPv6).
// PREVENTS: Non-IPv4 stale routes staying in non-LLGR peer's RIB.
func TestBuildWithdrawalPayload_MPReach(t *testing.T) {
	// Build MP_REACH_NLRI: AFI=2(IPv6), SAFI=1(unicast), NH_Len=16, NH=::1, Reserved=0, NLRI.
	nh := make([]byte, 16)
	nh[15] = 1                                         // ::1
	mpNLRI := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0, 0} // /48, 2001:db8::
	mpReachVal := make([]byte, 0, 3+1+len(nh)+1+len(mpNLRI))
	mpReachVal = append(mpReachVal, 0, 2, 1, byte(len(nh))) // AFI=2, SAFI=1, NH_Len
	mpReachVal = append(mpReachVal, nh...)
	mpReachVal = append(mpReachVal, 0) // reserved
	mpReachVal = append(mpReachVal, mpNLRI...)

	mpReachAttr := makeAttr(0x80, 14, mpReachVal) // Optional, code 14
	payload := buildModTestPayload(mpReachAttr, nil)

	result, _ := buildWithdrawalPayload(payload, nil)
	require.NotNil(t, result, "should produce MP withdrawal")

	// Parse result: withdrawn_len=0, then attr section with MP_UNREACH.
	wdLen := binary.BigEndian.Uint16(result[0:2])
	assert.Equal(t, uint16(0), wdLen, "no legacy withdrawn routes for MP")

	attrLen := binary.BigEndian.Uint16(result[2:4])
	require.Greater(t, attrLen, uint16(0), "should have MP_UNREACH attribute")

	// Parse the MP_UNREACH attribute.
	attrData := result[4 : 4+attrLen]
	require.GreaterOrEqual(t, len(attrData), 3, "attr header minimum")
	assert.Equal(t, byte(15), attrData[1], "should be MP_UNREACH_NLRI (code 15)")

	// Extract value: AFI(2) + SAFI(1) + NLRI.
	var valStart int
	if attrData[0]&0x10 != 0 {
		valStart = 4
	} else {
		valStart = 3
	}
	val := attrData[valStart:]
	require.GreaterOrEqual(t, len(val), 3, "AFI+SAFI minimum")
	assert.Equal(t, []byte{0, 2}, val[0:2], "AFI should be IPv6")
	assert.Equal(t, byte(1), val[2], "SAFI should be unicast")
	assert.Equal(t, mpNLRI, val[3:], "NLRI should match original")
}

// TestBuildWithdrawalPayload_Nil verifies nil/malformed input handling.
//
// VALIDATES: Defensive: malformed payload returns nil.
// PREVENTS: Panic on truncated or empty payloads.
func TestBuildWithdrawalPayload_Nil(t *testing.T) {
	r, _ := buildWithdrawalPayload(nil, nil)
	assert.Nil(t, r)
	r, _ = buildWithdrawalPayload([]byte{0}, nil)
	assert.Nil(t, r)
	r, _ = buildWithdrawalPayload([]byte{0, 0, 0}, nil) // too short for attr_len
	assert.Nil(t, r)
}
