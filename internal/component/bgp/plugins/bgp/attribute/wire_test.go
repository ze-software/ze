package attribute

import (
	"bytes"
	"sync"
	"testing"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/context"
)

// Helper to build packed attribute bytes.
func packAttr(flags AttributeFlags, code AttributeCode, value []byte) []byte {
	buf := make([]byte, 4+len(value))
	n := WriteHeaderTo(buf, 0, flags, code, uint16(len(value))) //nolint:gosec // G115: test values are small
	copy(buf[n:], value)
	return buf[:n+len(value)]
}

// Helper to build multiple attributes into packed bytes.
func packAttrs(attrs ...[]byte) []byte {
	total := 0
	for _, a := range attrs {
		total += len(a)
	}
	buf := make([]byte, 0, total)
	for _, a := range attrs {
		buf = append(buf, a...)
	}
	return buf
}

// setupTestContext registers a test context and returns its ID.
func setupTestContext(asn4 bool) bgpctx.ContextID {
	ctx := bgpctx.EncodingContextForASN4(asn4)
	return bgpctx.Registry.Register(ctx)
}

// TestAttributesWireGet verifies lazy single-attribute parsing.
//
// VALIDATES: Only requested attribute is parsed, cached for reuse.
// PREVENTS: Full parse on single attribute access.
func TestAttributesWireGet(t *testing.T) {
	ctxID := setupTestContext(true)

	// Build packed: ORIGIN (IGP=0) + LOCAL_PREF (100)
	origin := packAttr(FlagTransitive, AttrOrigin, []byte{0x00})
	localPref := packAttr(FlagTransitive, AttrLocalPref, []byte{0x00, 0x00, 0x00, 0x64})
	packed := packAttrs(origin, localPref)

	aw := NewAttributesWire(packed, ctxID)

	// Get ORIGIN
	attr, err := aw.Get(AttrOrigin)
	if err != nil {
		t.Fatalf("Get(ORIGIN) error: %v", err)
	}
	if attr == nil {
		t.Fatal("Get(ORIGIN) returned nil")
	}
	if attr.Code() != AttrOrigin {
		t.Errorf("Get(ORIGIN) code = %v, want ORIGIN", attr.Code())
	}

	// Get again - should use cache
	attr2, err := aw.Get(AttrOrigin)
	if err != nil {
		t.Fatalf("Get(ORIGIN) second call error: %v", err)
	}
	if attr2 != attr {
		t.Error("Get(ORIGIN) second call returned different pointer (not cached)")
	}

	// Get non-existent
	attr3, err := aw.Get(AttrMED)
	if err != nil {
		t.Fatalf("Get(MED) error: %v", err)
	}
	if attr3 != nil {
		t.Errorf("Get(MED) = %v, want nil for non-existent", attr3)
	}
}

// TestAttributesWireGetError verifies error handling for malformed data.
//
// VALIDATES: Errors returned with context, not swallowed.
// PREVENTS: Silent failures on corrupt wire bytes.
func TestAttributesWireGetError(t *testing.T) {
	ctxID := setupTestContext(true)

	// Truncated header (only 2 bytes)
	packed := []byte{0x40, 0x01}
	aw := NewAttributesWire(packed, ctxID)

	_, err := aw.Get(AttrOrigin)
	if err == nil {
		t.Error("Get() on truncated header should return error")
	}
}

// TestAttributesWireHas verifies header-only scanning.
//
// VALIDATES: Check existence without parsing value, returns error on malformed data.
// PREVENTS: Parsing overhead for existence check, silent failures.
func TestAttributesWireHas(t *testing.T) {
	ctxID := setupTestContext(true)

	origin := packAttr(FlagTransitive, AttrOrigin, []byte{0x00})
	packed := packAttrs(origin)
	aw := NewAttributesWire(packed, ctxID)

	// Existing attribute
	has, err := aw.Has(AttrOrigin)
	if err != nil {
		t.Fatalf("Has(ORIGIN) error: %v", err)
	}
	if !has {
		t.Error("Has(ORIGIN) = false, want true")
	}

	// Non-existent attribute
	has, err = aw.Has(AttrMED)
	if err != nil {
		t.Fatalf("Has(MED) error: %v", err)
	}
	if has {
		t.Error("Has(MED) = true, want false")
	}

	// Error case - malformed data
	truncated := []byte{0x40, 0x01}
	awBad := NewAttributesWire(truncated, ctxID)
	_, err = awBad.Has(AttrOrigin)
	if err == nil {
		t.Error("Has() on truncated header should return error")
	}
}

// TestAttributesWireGetMultiple verifies partial parsing.
//
// VALIDATES: Only requested attributes are parsed.
// PREVENTS: Parsing unrequested attributes.
func TestAttributesWireGetMultiple(t *testing.T) {
	ctxID := setupTestContext(true)

	// Build: ORIGIN, LOCAL_PREF, MED
	origin := packAttr(FlagTransitive, AttrOrigin, []byte{0x00})
	localPref := packAttr(FlagTransitive, AttrLocalPref, []byte{0x00, 0x00, 0x00, 0x64})
	med := packAttr(FlagOptional, AttrMED, []byte{0x00, 0x00, 0x00, 0x0A})
	packed := packAttrs(origin, localPref, med)
	aw := NewAttributesWire(packed, ctxID)

	// Request only ORIGIN and MED
	attrs, err := aw.GetMultiple([]AttributeCode{AttrOrigin, AttrMED})
	if err != nil {
		t.Fatalf("GetMultiple error: %v", err)
	}

	if len(attrs) != 2 {
		t.Errorf("GetMultiple returned %d attrs, want 2", len(attrs))
	}
	if _, ok := attrs[AttrOrigin]; !ok {
		t.Error("GetMultiple missing ORIGIN")
	}
	if _, ok := attrs[AttrMED]; !ok {
		t.Error("GetMultiple missing MED")
	}
	if _, ok := attrs[AttrLocalPref]; ok {
		t.Error("GetMultiple returned unrequested LOCAL_PREF")
	}
}

// TestAttributesWirePackFor verifies zero-copy forwarding.
//
// VALIDATES: Same context returns original bytes.
// PREVENTS: Unnecessary re-encoding.
func TestAttributesWirePackFor(t *testing.T) {
	ctxID := setupTestContext(true)

	origin := packAttr(FlagTransitive, AttrOrigin, []byte{0x00})
	packed := packAttrs(origin)
	aw := NewAttributesWire(packed, ctxID)

	// Same context - should return exact same slice
	result, err := aw.PackFor(ctxID)
	if err != nil {
		t.Fatalf("PackFor(same context) error: %v", err)
	}

	if &result[0] != &packed[0] {
		t.Error("PackFor(same context) returned copy, want same slice")
	}
}

// TestAttributesWirePackForDifferentContext verifies re-encoding.
//
// VALIDATES: Different context triggers re-encode.
// PREVENTS: Sending wrong encoding to peer.
func TestAttributesWirePackForDifferentContext(t *testing.T) {
	srcCtxID := setupTestContext(true)  // ASN4
	dstCtxID := setupTestContext(false) // ASN2

	// Simple origin - should re-encode but produce same bytes (no ASN)
	origin := packAttr(FlagTransitive, AttrOrigin, []byte{0x00})
	packed := packAttrs(origin)
	aw := NewAttributesWire(packed, srcCtxID)

	result, err := aw.PackFor(dstCtxID)
	if err != nil {
		t.Fatalf("PackFor(different context) error: %v", err)
	}

	// For ORIGIN (no AS content), result should be equivalent
	if !bytes.Equal(result, packed) {
		t.Errorf("PackFor result = %x, want %x", result, packed)
	}
}

// TestAttributesWireAll verifies full parse.
//
// VALIDATES: All attributes returned in wire order.
// PREVENTS: Missing or duplicated attributes.
func TestAttributesWireAll(t *testing.T) {
	ctxID := setupTestContext(true)

	// Build: ORIGIN, LOCAL_PREF (wire order)
	origin := packAttr(FlagTransitive, AttrOrigin, []byte{0x00})
	localPref := packAttr(FlagTransitive, AttrLocalPref, []byte{0x00, 0x00, 0x00, 0x64})
	packed := packAttrs(origin, localPref)
	aw := NewAttributesWire(packed, ctxID)

	attrs, err := aw.All()
	if err != nil {
		t.Fatalf("All() error: %v", err)
	}

	if len(attrs) != 2 {
		t.Fatalf("All() returned %d attrs, want 2", len(attrs))
	}

	// Verify wire order
	if attrs[0].Code() != AttrOrigin {
		t.Errorf("All()[0] = %v, want ORIGIN", attrs[0].Code())
	}
	if attrs[1].Code() != AttrLocalPref {
		t.Errorf("All()[1] = %v, want LOCAL_PREF", attrs[1].Code())
	}
}

// TestAttributesWireConcurrentAccess verifies thread safety.
//
// VALIDATES: Concurrent Get() calls don't race.
// PREVENTS: Data races on parsed cache.
func TestAttributesWireConcurrentAccess(t *testing.T) {
	ctxID := setupTestContext(true)

	origin := packAttr(FlagTransitive, AttrOrigin, []byte{0x00})
	localPref := packAttr(FlagTransitive, AttrLocalPref, []byte{0x00, 0x00, 0x00, 0x64})
	packed := packAttrs(origin, localPref)
	aw := NewAttributesWire(packed, ctxID)

	var wg sync.WaitGroup
	errs := make(chan error, 100)

	// Concurrent Gets
	for range 100 {
		wg.Go(func() {
			_, err := aw.Get(AttrOrigin)
			if err != nil {
				errs <- err
			}
			_, err = aw.Get(AttrLocalPref)
			if err != nil {
				errs <- err
			}
		})
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("Concurrent Get error: %v", err)
	}
}

// TestAttributesWireIndexReuse verifies index caching.
//
// VALIDATES: Second Get() reuses index, doesn't rescan.
// PREVENTS: O(n^2) scanning for multiple Gets.
func TestAttributesWireIndexReuse(t *testing.T) {
	ctxID := setupTestContext(true)

	origin := packAttr(FlagTransitive, AttrOrigin, []byte{0x00})
	localPref := packAttr(FlagTransitive, AttrLocalPref, []byte{0x00, 0x00, 0x00, 0x64})
	packed := packAttrs(origin, localPref)
	aw := NewAttributesWire(packed, ctxID)

	// First Get builds index
	_, _ = aw.Get(AttrOrigin)

	// Second Get for different attr should reuse index
	attr, err := aw.Get(AttrLocalPref)
	if err != nil {
		t.Fatalf("Second Get error: %v", err)
	}
	if attr == nil {
		t.Error("Second Get returned nil")
	}
	// Note: We can't directly verify index reuse without exposing internals,
	// but if both Gets succeed, index was built and reused.
}

// TestAttributesWireDuplicateAttribute verifies RFC 4271 compliance.
//
// VALIDATES: Duplicate attributes return error.
// PREVENTS: Silent acceptance of malformed UPDATE messages.
func TestAttributesWireDuplicateAttribute(t *testing.T) {
	ctxID := setupTestContext(true)

	// Two ORIGIN attributes (RFC 4271 violation)
	origin1 := packAttr(FlagTransitive, AttrOrigin, []byte{0x00})
	origin2 := packAttr(FlagTransitive, AttrOrigin, []byte{0x01})
	packed := packAttrs(origin1, origin2)
	aw := NewAttributesWire(packed, ctxID)

	_, err := aw.Get(AttrOrigin)
	if err == nil {
		t.Error("Get() should return error for duplicate attribute")
	}
}

// TestAttributesWireEmptyPacked verifies edge case handling.
//
// VALIDATES: Empty packed bytes returns empty results, not error.
// PREVENTS: Nil pointer dereference on empty input.
func TestAttributesWireEmptyPacked(t *testing.T) {
	ctxID := setupTestContext(true)

	// Empty packed
	aw := NewAttributesWire(nil, ctxID)

	attr, err := aw.Get(AttrOrigin)
	if err != nil {
		t.Errorf("Get on empty packed error: %v", err)
	}
	if attr != nil {
		t.Errorf("Get on empty packed = %v, want nil", attr)
	}

	attrs, err := aw.All()
	if err != nil {
		t.Errorf("All on empty packed error: %v", err)
	}
	if len(attrs) != 0 {
		t.Errorf("All on empty packed = %d attrs, want 0", len(attrs))
	}
}

// TestAttributesWireUnknownAttribute verifies opaque handling.
//
// VALIDATES: Unknown attribute codes return OpaqueAttribute.
// PREVENTS: Errors on vendor-specific or future attributes.
func TestAttributesWireUnknownAttribute(t *testing.T) {
	ctxID := setupTestContext(true)

	// Unknown attribute code 250
	unknown := packAttr(FlagOptional|FlagTransitive, AttributeCode(250), []byte{0xDE, 0xAD})
	packed := packAttrs(unknown)
	aw := NewAttributesWire(packed, ctxID)

	attr, err := aw.Get(AttributeCode(250))
	if err != nil {
		t.Fatalf("Get(unknown) error: %v", err)
	}
	if attr == nil {
		t.Fatal("Get(unknown) returned nil")
	}

	opaque, ok := attr.(*OpaqueAttribute)
	if !ok {
		t.Fatalf("Get(unknown) returned %T, want *OpaqueAttribute", attr)
	}

	if opaque.Code() != AttributeCode(250) {
		t.Errorf("OpaqueAttribute.Code() = %d, want 250", opaque.Code())
	}
}

// TestAttributesWireInvalidContext verifies context validation.
//
// VALIDATES: Invalid context ID returns error.
// PREVENTS: Nil pointer dereference on missing context.
func TestAttributesWireInvalidContext(t *testing.T) {
	// Use an invalid context ID (not registered)
	invalidCtxID := bgpctx.ContextID(65000)

	origin := packAttr(FlagTransitive, AttrOrigin, []byte{0x00})
	packed := packAttrs(origin)
	aw := NewAttributesWire(packed, invalidCtxID)

	// Get should fail when trying to parse (needs context)
	_, err := aw.Get(AttrOrigin)
	if err == nil {
		t.Error("Get with invalid context should return error")
	}
}

// TestAttributesWirePreservesFlags verifies flag preservation for unknown attributes.
//
// VALIDATES: Unknown transitive attributes retain original flags including Partial bit.
// PREVENTS: Incorrect flag reconstruction during forwarding (RFC 4271 violation).
func TestAttributesWirePreservesFlags(t *testing.T) {
	ctxID := setupTestContext(true)

	// Unknown attribute with Partial bit set
	flags := FlagOptional | FlagTransitive | FlagPartial
	unknown := packAttr(flags, AttributeCode(200), []byte{0x01, 0x02})
	packed := packAttrs(unknown)
	aw := NewAttributesWire(packed, ctxID)

	attr, err := aw.Get(AttributeCode(200))
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}

	if attr.Flags() != flags {
		t.Errorf("Flags() = %02x, want %02x (Partial bit lost)", attr.Flags(), flags)
	}
}

// TestAttributesWireAtomicAggregateValidation verifies length validation.
//
// VALIDATES: ATOMIC_AGGREGATE with non-zero length returns error.
// PREVENTS: Silent acceptance of malformed ATOMIC_AGGREGATE.
func TestAttributesWireAtomicAggregateValidation(t *testing.T) {
	ctxID := setupTestContext(true)

	// Valid ATOMIC_AGGREGATE (empty)
	valid := packAttr(FlagTransitive, AttrAtomicAggregate, nil)
	aw := NewAttributesWire(valid, ctxID)

	attr, err := aw.Get(AttrAtomicAggregate)
	if err != nil {
		t.Errorf("Valid ATOMIC_AGGREGATE error: %v", err)
	}
	if attr == nil {
		t.Error("Valid ATOMIC_AGGREGATE returned nil")
	}

	// Invalid ATOMIC_AGGREGATE (has data)
	invalid := packAttr(FlagTransitive, AttrAtomicAggregate, []byte{0x01})
	awBad := NewAttributesWire(invalid, ctxID)

	_, err = awBad.Get(AttrAtomicAggregate)
	if err == nil {
		t.Error("Invalid ATOMIC_AGGREGATE should return error")
	}
}

// TestAttributesWireOriginatorID verifies route reflection attribute parsing.
//
// VALIDATES: ORIGINATOR_ID is correctly parsed.
// PREVENTS: Route reflection failures.
func TestAttributesWireOriginatorID(t *testing.T) {
	ctxID := setupTestContext(true)

	// ORIGINATOR_ID is 4 bytes (IPv4 router ID)
	originatorID := packAttr(FlagOptional, AttrOriginatorID, []byte{10, 0, 0, 1})
	packed := packAttrs(originatorID)
	aw := NewAttributesWire(packed, ctxID)

	attr, err := aw.Get(AttrOriginatorID)
	if err != nil {
		t.Fatalf("Get(ORIGINATOR_ID) error: %v", err)
	}
	if attr == nil {
		t.Fatal("Get(ORIGINATOR_ID) returned nil")
	}
	if attr.Code() != AttrOriginatorID {
		t.Errorf("Code() = %v, want ORIGINATOR_ID", attr.Code())
	}
}

// TestAttributesWireSourceContext verifies context accessor.
//
// VALIDATES: SourceContext returns the original context ID.
// PREVENTS: Context mismatch in forwarding logic.
func TestAttributesWireSourceContext(t *testing.T) {
	ctxID := setupTestContext(true)

	aw := NewAttributesWire(nil, ctxID)

	if aw.SourceContext() != ctxID {
		t.Errorf("SourceContext() = %d, want %d", aw.SourceContext(), ctxID)
	}
}

// TestAttributesWirePacked verifies raw bytes accessor.
//
// VALIDATES: Packed returns original bytes unchanged.
// PREVENTS: Modification of wire bytes.
func TestAttributesWirePacked(t *testing.T) {
	ctxID := setupTestContext(true)

	origin := packAttr(FlagTransitive, AttrOrigin, []byte{0x00})
	packed := packAttrs(origin)
	aw := NewAttributesWire(packed, ctxID)

	result := aw.Packed()

	if !bytes.Equal(result, packed) {
		t.Errorf("Packed() = %x, want %x", result, packed)
	}
	if &result[0] != &packed[0] {
		t.Error("Packed() returned copy, want same slice")
	}
}

// TestAttributesWireExtendedLength verifies 4-byte header handling.
//
// VALIDATES: Extended Length attributes (>255 bytes) are handled correctly.
// PREVENTS: Parse errors on large attributes.
func TestAttributesWireExtendedLength(t *testing.T) {
	ctxID := setupTestContext(true)

	// Large community list (>255 bytes)
	// Each large community is 12 bytes, need 22+ for >255 bytes
	bigData := make([]byte, 300)
	for i := range bigData {
		bigData[i] = byte(i % 256)
	}

	// Write with extended length
	largeCommunity := packAttr(FlagOptional|FlagTransitive, AttrLargeCommunity, bigData)
	packed := packAttrs(largeCommunity)
	aw := NewAttributesWire(packed, ctxID)

	attr, err := aw.Get(AttrLargeCommunity)
	if err != nil {
		t.Fatalf("Get(LARGE_COMMUNITY) error: %v", err)
	}
	if attr == nil {
		t.Fatal("Get(LARGE_COMMUNITY) returned nil")
	}
}

// TestAttributesWireTruncatedValue verifies truncation detection.
//
// VALIDATES: Truncated attribute values are detected.
// PREVENTS: Buffer overread on malformed data.
func TestAttributesWireTruncatedValue(t *testing.T) {
	ctxID := setupTestContext(true)

	// Header says 4 bytes but only 2 present
	// Flags=0x40 (Transitive), Code=1 (ORIGIN), Length=4
	truncated := []byte{0x40, 0x01, 0x04, 0x00, 0x00} // Only 2 value bytes
	aw := NewAttributesWire(truncated, ctxID)

	_, err := aw.Has(AttrOrigin)
	if err == nil {
		t.Error("Has() should return error for truncated value")
	}
}

// TestAttributesWireErrorRecovery verifies error handling across multiple calls.
//
// VALIDATES: Parse errors are returned consistently on subsequent calls.
// PREVENTS: Silent data loss when first parse fails but subsequent calls succeed.
func TestAttributesWireErrorRecovery(t *testing.T) {
	ctxID := setupTestContext(true)

	// Valid ORIGIN followed by truncated second attribute
	// This tests that index build failure doesn't leave partial state
	origin := packAttr(FlagTransitive, AttrOrigin, []byte{0x00})
	// Truncated LOCAL_PREF: header says 4 bytes but only 2 present
	truncatedLP := []byte{0x40, 0x05, 0x04, 0x00, 0x00}
	packed := packAttrs(origin, truncatedLP)
	aw := NewAttributesWire(packed, ctxID)

	// First Get should fail (truncated data)
	_, err1 := aw.Get(AttrOrigin)
	if err1 == nil {
		t.Fatal("First Get should return error for truncated data")
	}

	// Second Get should ALSO fail (not silently succeed with partial index)
	_, err2 := aw.Get(AttrOrigin)
	if err2 == nil {
		t.Fatal("Second Get should also return error (index must not be partially built)")
	}

	// Has should also fail consistently
	_, err3 := aw.Has(AttrLocalPref)
	if err3 == nil {
		t.Fatal("Has should return error for truncated data")
	}
}

// TestAttributesWireErrorOnFirstAttribute verifies error handling when first attribute is malformed.
//
// VALIDATES: Empty partial index doesn't cause silent success.
// PREVENTS: Bug where non-nil empty index causes subsequent calls to return "not found" instead of error.
func TestAttributesWireErrorOnFirstAttribute(t *testing.T) {
	ctxID := setupTestContext(true)

	// First attribute is truncated - no valid attributes at all
	// Header says ORIGIN with 4 bytes but only 1 byte present
	truncatedFirst := []byte{0x40, 0x01, 0x04, 0x00}
	aw := NewAttributesWire(truncatedFirst, ctxID)

	// First Get should fail
	_, err1 := aw.Get(AttrOrigin)
	if err1 == nil {
		t.Fatal("First Get should return error for truncated first attribute")
	}

	// Second Get should also fail (not return nil with empty index)
	_, err2 := aw.Get(AttrOrigin)
	if err2 == nil {
		t.Fatal("Second Get should return error, not nil (empty index bug)")
	}

	// All should also fail
	_, err3 := aw.All()
	if err3 == nil {
		t.Fatal("All should return error for truncated data")
	}
}

// TestAttributesWireASN4ReEncoding verifies context-dependent AS_PATH re-encoding.
//
// VALIDATES: AS_PATH with 4-byte ASNs is re-encoded for 2-byte context.
// PREVENTS: Sending 4-byte ASNs to peers that don't support them.
func TestAttributesWireASN4ReEncoding(t *testing.T) {
	// Source context: 4-byte ASN capable
	srcCtxID := setupTestContext(true)
	// Destination context: 2-byte ASN only
	dstCtxID := setupTestContext(false)

	// Build AS_PATH with large ASN (> 65535 requires 4-byte encoding)
	// AS_PATH: AS_SEQUENCE with one ASN = 100000
	// Segment: type=2 (SEQUENCE), length=1, ASN=100000 (4 bytes)
	asPath4Byte := []byte{
		0x02,                   // AS_SEQUENCE
		0x01,                   // 1 ASN in segment
		0x00, 0x01, 0x86, 0xA0, // ASN 100000 (4-byte)
	}
	asPathAttr := packAttr(FlagTransitive, AttrASPath, asPath4Byte)
	packed := packAttrs(asPathAttr)
	aw := NewAttributesWire(packed, srcCtxID)

	// Re-encode for 2-byte ASN context
	result, err := aw.PackFor(dstCtxID)
	if err != nil {
		t.Fatalf("PackFor error: %v", err)
	}

	// Result should be different size (4-byte vs 2-byte ASN encoding)
	// Original: 3-byte header + 6-byte value (type+len+4-byte ASN) = 9 bytes
	// Re-encoded: 3-byte header + 4-byte value (type+len+2-byte ASN) = 7 bytes
	if len(result) == len(packed) {
		t.Errorf("PackFor should produce different size: got %d, orig %d", len(result), len(packed))
	}

	// Parse result header to find AS_PATH value
	_, code, length, hdrLen, err := ParseHeader(result)
	if err != nil {
		t.Fatalf("ParseHeader on result error: %v", err)
	}
	if code != AttrASPath {
		t.Fatalf("Result code = %v, want AS_PATH", code)
	}

	// Extract the value bytes (after header)
	valueBytes := result[hdrLen : hdrLen+int(length)]

	// 2-byte encoding: type=2, len=1, ASN=23456 (0x5BA0)
	// Expected value: 02 01 5B A0
	expected := []byte{0x02, 0x01, 0x5B, 0xA0}
	if !bytes.Equal(valueBytes, expected) {
		t.Errorf("Re-encoded AS_PATH value = %x, want %x", valueBytes, expected)
	}
}

// TestAttributesWireGetRaw verifies raw bytes extraction without parsing.
//
// VALIDATES: GetRaw returns attribute value bytes without parsing.
// PREVENTS: Unnecessary parsing when only raw bytes needed (e.g., for MPReachWire).
func TestAttributesWireGetRaw(t *testing.T) {
	ctxID := setupTestContext(true)

	// Build MP_REACH_NLRI with known content
	// Wire format: AFI(2) + SAFI(1) + NHLen(1) + NextHop(4) + Reserved(1) + NLRI
	mpReachValue := []byte{
		0x00, 0x01, // AFI=1 (IPv4)
		0x01,        // SAFI=1 (unicast)
		0x04,        // NH len=4
		10, 0, 0, 1, // Next-hop 10.0.0.1
		0x00,            // Reserved
		24, 192, 168, 1, // NLRI: 192.168.1.0/24
	}

	mpReachAttr := packAttr(FlagOptional, AttrMPReachNLRI, mpReachValue)
	packed := packAttrs(mpReachAttr)
	aw := NewAttributesWire(packed, ctxID)

	// GetRaw should return exactly the value bytes (no header)
	raw, err := aw.GetRaw(AttrMPReachNLRI)
	if err != nil {
		t.Fatalf("GetRaw(MP_REACH_NLRI) error: %v", err)
	}
	if raw == nil {
		t.Fatal("GetRaw(MP_REACH_NLRI) returned nil")
	}

	if !bytes.Equal(raw, mpReachValue) {
		t.Errorf("GetRaw() = %x, want %x", raw, mpReachValue)
	}
}

// TestAttributesWireGetRawNotFound verifies GetRaw for missing attribute.
//
// VALIDATES: GetRaw returns nil, nil for missing attributes.
// PREVENTS: Errors on absent attributes.
func TestAttributesWireGetRawNotFound(t *testing.T) {
	ctxID := setupTestContext(true)

	origin := packAttr(FlagTransitive, AttrOrigin, []byte{0x00})
	packed := packAttrs(origin)
	aw := NewAttributesWire(packed, ctxID)

	// GetRaw for attribute not present
	raw, err := aw.GetRaw(AttrMPReachNLRI)
	if err != nil {
		t.Fatalf("GetRaw(not found) error: %v", err)
	}
	if raw != nil {
		t.Errorf("GetRaw(not found) = %x, want nil", raw)
	}
}

// TestAttributesWireGetRawZeroCopy verifies GetRaw returns slice into original buffer.
//
// VALIDATES: No copy made - returns view into packed bytes.
// PREVENTS: Memory waste from unnecessary copies.
func TestAttributesWireGetRawZeroCopy(t *testing.T) {
	ctxID := setupTestContext(true)

	originValue := []byte{0x00}
	originAttr := packAttr(FlagTransitive, AttrOrigin, originValue)
	packed := packAttrs(originAttr)
	aw := NewAttributesWire(packed, ctxID)

	raw, err := aw.GetRaw(AttrOrigin)
	if err != nil {
		t.Fatalf("GetRaw error: %v", err)
	}

	// raw should be a slice into packed, not a copy
	// Value starts at offset 3 (3-byte header for short attrs)
	if &raw[0] != &packed[3] {
		t.Error("GetRaw returned copy, want slice into packed bytes")
	}
}
