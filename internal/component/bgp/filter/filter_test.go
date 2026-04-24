package filter

import (
	"encoding/binary"
	"sync"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
)

// testPrefix10 is a test prefix used across filter tests.
const testPrefix10 = "10.0.0.0/24"

// emptyEncCtx is an empty encoding context for tests (no ADD-PATH).
var emptyEncCtx = bgpctx.EncodingContextForASN4(true)

// testEncodingContext creates an encoding context for tests.
func testEncodingContext() bgpctx.ContextID {
	ctx := bgpctx.NewEncodingContext(
		&capability.PeerIdentity{
			LocalASN: 65001,
			PeerASN:  65001,
		},
		&capability.EncodingCaps{
			ASN4: true,
		},
		bgpctx.DirectionRecv,
	)
	id, _ := bgpctx.Registry.Register(ctx)
	return id
}

// buildTestAttributeBytes builds wire bytes for testing.
// Contains: ORIGIN(1), AS_PATH(2), NEXT_HOP(3), MED(4).
func buildTestAttributeBytes() []byte {
	// ORIGIN IGP: 40 01 01 00
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	// AS_PATH [65001]: 40 02 06 02 01 00 00 fd e9
	asPath := []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9}
	// NEXT_HOP 10.0.0.1: 40 03 04 0a 00 00 01
	nextHop := []byte{0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01}
	// MED 100: 80 04 04 00 00 00 64
	med := []byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64}

	result := make([]byte, 0, len(origin)+len(asPath)+len(nextHop)+len(med))
	result = append(result, origin...)
	result = append(result, asPath...)
	result = append(result, nextHop...)
	result = append(result, med...)
	return result
}

// TestAttributeFilterModeAll verifies all attributes included.
//
// VALIDATES: Apply() with FilterModeAll returns all attrs as map.
// PREVENTS: Wrong mode, missing conversion from []Attribute to map.
func TestAttributeFilterModeAll(t *testing.T) {
	ctxID := testEncodingContext()

	attrBytes := buildTestAttributeBytes()
	wire := attribute.NewAttributesWire(attrBytes, ctxID)
	filter := NewFilterAll()

	result, err := filter.Apply(wire)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	// Should have all 4 attributes
	if len(result.Attributes) != 4 {
		t.Errorf("Apply() returned %d attrs, want 4", len(result.Attributes))
	}

	// Check specific codes
	wantCodes := []attribute.AttributeCode{
		attribute.AttrOrigin,
		attribute.AttrASPath,
		attribute.AttrNextHop,
		attribute.AttrMED,
	}
	for _, code := range wantCodes {
		if _, ok := result.Attributes[code]; !ok {
			t.Errorf("Apply() missing code %d (%s)", code, code)
		}
	}
}

// TestAttributeFilterModeNone verifies no attributes included.
//
// VALIDATES: Apply() with FilterModeNone returns empty map.
// PREVENTS: Accidental attribute leakage.
func TestAttributeFilterModeNone(t *testing.T) {
	ctxID := testEncodingContext()

	attrBytes := buildTestAttributeBytes()
	wire := attribute.NewAttributesWire(attrBytes, ctxID)
	filter := NewFilterNone()

	result, err := filter.Apply(wire)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if len(result.Attributes) != 0 {
		t.Errorf("Apply() returned %d attrs, want 0", len(result.Attributes))
	}
}

// TestAttributeFilterModeSelective verifies specific codes included.
//
// VALIDATES: Only requested codes returned.
// PREVENTS: Extra or missing attributes.
func TestAttributeFilterModeSelective(t *testing.T) {
	ctxID := testEncodingContext()

	attrBytes := buildTestAttributeBytes()
	wire := attribute.NewAttributesWire(attrBytes, ctxID)

	// Only request ORIGIN and MED
	filter := NewFilterSelective([]attribute.AttributeCode{
		attribute.AttrOrigin,
		attribute.AttrMED,
	})

	result, err := filter.Apply(wire)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	// Should have exactly 2 attributes
	if len(result.Attributes) != 2 {
		t.Errorf("Apply() returned %d attrs, want 2", len(result.Attributes))
	}

	// Check we got the right ones
	if _, ok := result.Attributes[attribute.AttrOrigin]; !ok {
		t.Error("Apply() missing ORIGIN")
	}
	if _, ok := result.Attributes[attribute.AttrMED]; !ok {
		t.Error("Apply() missing MED")
	}

	// Check we don't have the others
	if _, ok := result.Attributes[attribute.AttrASPath]; ok {
		t.Error("Apply() should not have AS_PATH")
	}
	if _, ok := result.Attributes[attribute.AttrNextHop]; ok {
		t.Error("Apply() should not have NEXT_HOP")
	}
}

// TestAttributeFilterNilWire verifies nil AttrsWire handling.
//
// VALIDATES: Apply() returns empty result when wire is nil.
// PREVENTS: Nil pointer panic.
func TestAttributeFilterNilWire(t *testing.T) {
	filter := NewFilterAll()

	result, err := filter.Apply(nil)
	if err != nil {
		t.Fatalf("Apply(nil) error = %v", err)
	}

	if len(result.Attributes) != 0 {
		t.Errorf("Apply(nil) returned %d attrs, want 0", len(result.Attributes))
	}
}

// TestAttributeFilterEmptyResult verifies empty result handling.
//
// VALIDATES: Requested attr not present -> nil map, not empty map.
// PREVENTS: Empty "attributes": {} in output.
func TestAttributeFilterEmptyResult(t *testing.T) {
	ctxID := testEncodingContext()

	attrBytes := buildTestAttributeBytes()
	wire := attribute.NewAttributesWire(attrBytes, ctxID)

	// Request attribute that doesn't exist
	filter := NewFilterSelective([]attribute.AttributeCode{
		attribute.AttrCommunity, // Not in test attrs
	})

	result, err := filter.Apply(wire)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	// Should return nil or empty map - key is no "attributes" key in JSON
	if len(result.Attributes) != 0 {
		t.Errorf("Apply() returned %d attrs, want 0", len(result.Attributes))
	}
}

// TestAttributeFilterConcurrent verifies thread safety.
//
// VALIDATES: Multiple goroutines can call Apply() safely.
// PREVENTS: Race conditions (relies on AttributesWire internal mutex).
func TestAttributeFilterConcurrent(t *testing.T) {
	ctxID := testEncodingContext()

	attrBytes := buildTestAttributeBytes()
	wire := attribute.NewAttributesWire(attrBytes, ctxID)
	filter := NewFilterAll()

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for range 10 {
		wg.Go(func() {
			result, err := filter.Apply(wire)
			if err != nil {
				errors <- err
				return
			}
			if len(result.Attributes) != 4 {
				errors <- err
			}
		})
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent Apply() error = %v", err)
		}
	}
}

// TestFilterIsEmpty verifies IsEmpty method.
//
// VALIDATES: IsEmpty returns true for none/empty selective.
// PREVENTS: Wrong empty detection.
func TestFilterIsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		filter AttributeFilter
		want   bool
	}{
		{"all", NewFilterAll(), false},
		{"none", NewFilterNone(), true},
		{"selective with codes", NewFilterSelective([]attribute.AttributeCode{attribute.AttrOrigin}), false},
		{"selective empty", NewFilterSelective(nil), true},
		{"selective empty slice", NewFilterSelective([]attribute.AttributeCode{}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIncludesO1Lookup verifies O(1) lookup via codeSet.
//
// VALIDATES: Includes uses map lookup, not linear search.
// PREVENTS: O(n) performance regression.
func TestIncludesO1Lookup(t *testing.T) {
	// Create filter with many codes
	codes := make([]attribute.AttributeCode, 100)
	for i := range codes {
		codes[i] = attribute.AttributeCode(i + 1) //nolint:gosec // test data
	}
	filter := NewFilterSelective(codes)

	// Verify codeSet is populated
	if len(filter.codeSet) != 100 {
		t.Errorf("codeSet len = %d, want 100", len(filter.codeSet))
	}

	// Verify Includes works correctly
	if !filter.Includes(attribute.AttributeCode(50)) {
		t.Error("Includes(50) = false, want true")
	}
	if filter.Includes(attribute.AttributeCode(200)) {
		t.Error("Includes(200) = true, want false")
	}
}

// TestApplyToUpdateIPv4 verifies NLRI extraction from IPv4 UPDATE.
//
// VALIDATES: IPv4 prefixes extracted from body structure.
// PREVENTS: Missing NLRI in FilterResult.
func TestApplyToUpdateIPv4(t *testing.T) {
	ctxID := testEncodingContext()

	// Build UPDATE with IPv4 NLRI: withdrawn=0, attrs with NEXT_HOP, NLRI=10.0.0.0/24
	// Withdrawn len (2) + Attr len (2) + NEXT_HOP attr + NLRI
	nextHopAttr := []byte{0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01} // 10.0.0.1
	nlri := []byte{24, 10, 0, 0}                                    // 10.0.0.0/24

	body := make([]byte, 4+len(nextHopAttr)+len(nlri))
	// withdrawn len = 0
	body[0], body[1] = 0, 0
	// attr len
	body[2], body[3] = 0, byte(len(nextHopAttr))
	copy(body[4:], nextHopAttr)
	copy(body[4+len(nextHopAttr):], nlri)

	// Create WireUpdate to extract attributes
	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	wire, err := wireUpdate.Attrs()
	if err != nil {
		t.Fatalf("Attrs() error = %v", err)
	}

	filter := NewFilterAll()
	result, err := filter.ApplyToUpdate(wire, body, NewNLRIFilterAll())
	if err != nil {
		t.Fatalf("ApplyToUpdate() error = %v", err)
	}

	// Check announced via FamilyNLRI
	announced := result.AnnouncedByFamily(emptyEncCtx)
	if len(announced) != 1 {
		t.Fatalf("AnnouncedByFamily len = %d, want 1", len(announced))
	}
	if len(announced[0].NLRIs) != 1 {
		t.Errorf("NLRIs len = %d, want 1", len(announced[0].NLRIs))
	} else if announced[0].NLRIs[0].String() != testPrefix10 {
		t.Errorf("NLRI = %s, want %s", announced[0].NLRIs[0], testPrefix10)
	}

	// Check next-hop (IPv4)
	if announced[0].NextHop.String() != "10.0.0.1" {
		t.Errorf("NextHop = %s, want 10.0.0.1", announced[0].NextHop)
	}

	// Check withdrawn is empty
	withdrawn := result.WithdrawnByFamily(emptyEncCtx)
	if len(withdrawn) != 0 {
		t.Errorf("WithdrawnByFamily len = %d, want 0", len(withdrawn))
	}
}

// TestApplyToUpdateWithdrawn verifies withdrawn prefix extraction.
//
// VALIDATES: IPv4 withdrawn prefixes extracted from body.
// PREVENTS: Missing withdrawn in FilterResult.
func TestApplyToUpdateWithdrawn(t *testing.T) {
	// Build UPDATE with withdrawn only: 10.0.0.0/24 withdrawn
	withdrawnBytes := []byte{24, 10, 0, 0} // 10.0.0.0/24
	body := make([]byte, 4+len(withdrawnBytes))
	// withdrawn len
	body[0], body[1] = 0, byte(len(withdrawnBytes))
	copy(body[2:], withdrawnBytes)
	// attr len = 0
	body[2+len(withdrawnBytes)], body[3+len(withdrawnBytes)] = 0, 0

	filter := NewFilterAll()
	result, err := filter.ApplyToUpdate(nil, body, NewNLRIFilterAll())
	if err != nil {
		t.Fatalf("ApplyToUpdate() error = %v", err)
	}

	// Check withdrawn via FamilyNLRI
	withdrawn := result.WithdrawnByFamily(emptyEncCtx)
	if len(withdrawn) != 1 {
		t.Fatalf("WithdrawnByFamily len = %d, want 1", len(withdrawn))
	}
	if len(withdrawn[0].NLRIs) != 1 {
		t.Errorf("NLRIs len = %d, want 1", len(withdrawn[0].NLRIs))
	} else if withdrawn[0].NLRIs[0].String() != testPrefix10 {
		t.Errorf("NLRI = %s, want %s", withdrawn[0].NLRIs[0], testPrefix10)
	}

	// Check announced is empty
	announced := result.AnnouncedByFamily(emptyEncCtx)
	if len(announced) != 0 {
		t.Errorf("AnnouncedByFamily len = %d, want 0", len(announced))
	}
}

// TestApplyToUpdateFilterNone verifies FilterModeNone skips attributes.
//
// VALIDATES: No attributes parsed when mode is None.
// PREVENTS: Wasted parsing for none filter.
func TestApplyToUpdateFilterNone(t *testing.T) {
	ctxID := testEncodingContext()
	attrBytes := buildTestAttributeBytes()
	wire := attribute.NewAttributesWire(attrBytes, ctxID)

	// Build minimal body
	body := make([]byte, 4+len(attrBytes))
	body[2], body[3] = 0, byte(len(attrBytes))
	copy(body[4:], attrBytes)

	filter := NewFilterNone()
	result, err := filter.ApplyToUpdate(wire, body, NewNLRIFilterAll())
	if err != nil {
		t.Fatalf("ApplyToUpdate() error = %v", err)
	}

	// Attributes should be nil/empty
	if len(result.Attributes) != 0 {
		t.Errorf("Attributes len = %d, want 0", len(result.Attributes))
	}
}

// TestApplyToUpdateSelectiveFilter verifies selective attribute parsing.
//
// VALIDATES: Only requested attributes parsed via wire.GetMultiple.
// PREVENTS: Over-parsing when only specific attrs needed.
func TestApplyToUpdateSelectiveFilter(t *testing.T) {
	ctxID := testEncodingContext()
	attrBytes := buildTestAttributeBytes() // ORIGIN, AS_PATH, NEXT_HOP, MED
	wire := attribute.NewAttributesWire(attrBytes, ctxID)

	// Build minimal body
	body := make([]byte, 4+len(attrBytes))
	body[2], body[3] = 0, byte(len(attrBytes))
	copy(body[4:], attrBytes)

	// Only request ORIGIN
	filter := NewFilterSelective([]attribute.AttributeCode{attribute.AttrOrigin})
	result, err := filter.ApplyToUpdate(wire, body, NewNLRIFilterAll())
	if err != nil {
		t.Fatalf("ApplyToUpdate() error = %v", err)
	}

	// Should only have ORIGIN
	if len(result.Attributes) != 1 {
		t.Errorf("Attributes len = %d, want 1", len(result.Attributes))
	}
	if _, ok := result.Attributes[attribute.AttrOrigin]; !ok {
		t.Error("Attributes should have ORIGIN")
	}
}

// buildTestUpdateBody builds an UPDATE body with attributes and NLRI.
// Format: withdrawn_len(2) + withdrawn + attr_len(2) + attrs + nlri.
func buildTestUpdateBody() []byte {
	// Attributes: ORIGIN IGP + NEXT_HOP 10.0.0.1
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP 10.0.0.1
	}
	// NLRI: 192.168.1.0/24
	nlri := []byte{24, 192, 168, 1}

	buf := make([]byte, 4+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(buf[0:2], 0)                  // withdrawn len
	binary.BigEndian.PutUint16(buf[2:4], uint16(len(attrs))) //nolint:gosec // test data
	copy(buf[4:], attrs)
	copy(buf[4+len(attrs):], nlri)
	return buf
}

// TestApplyToUpdate verifies combined attribute filtering and NLRI extraction.
//
// VALIDATES: ApplyToUpdate returns both filtered attrs and NLRI.
// PREVENTS: Missing NLRI when using filter.
func TestApplyToUpdate(t *testing.T) {
	ctxID := testEncodingContext()
	body := buildTestUpdateBody()
	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	wire, err := wireUpdate.Attrs()
	if err != nil {
		t.Fatalf("Attrs() error = %v", err)
	}

	filter := NewFilterSelective([]attribute.AttributeCode{attribute.AttrOrigin})
	result, err := filter.ApplyToUpdate(wire, body, NewNLRIFilterAll())
	if err != nil {
		t.Fatalf("ApplyToUpdate() error = %v", err)
	}

	// Check filtered attributes
	if len(result.Attributes) != 1 {
		t.Errorf("len(Attributes) = %d, want 1", len(result.Attributes))
	}
	if _, ok := result.Attributes[attribute.AttrOrigin]; !ok {
		t.Error("missing ORIGIN in result")
	}
	if _, ok := result.Attributes[attribute.AttrNextHop]; ok {
		t.Error("NEXT_HOP should be filtered out")
	}

	// Check NLRI via FamilyNLRI
	announced := result.AnnouncedByFamily(emptyEncCtx)
	if len(announced) != 1 {
		t.Fatalf("len(AnnouncedByFamily) = %d, want 1", len(announced))
	}
	if len(announced[0].NLRIs) != 1 {
		t.Errorf("len(NLRIs) = %d, want 1", len(announced[0].NLRIs))
	} else if announced[0].NLRIs[0].String() != "192.168.1.0/24" {
		t.Errorf("NLRI = %s, want 192.168.1.0/24", announced[0].NLRIs[0])
	}
}

// TestAnnouncedByFamilyNilCtx verifies nil ctx doesn't panic.
//
// VALIDATES: Nil ctx treated as no ADD-PATH.
// PREVENTS: Nil pointer dereference crash.
func TestAnnouncedByFamilyNilCtx(t *testing.T) {
	// Create a FilterResult with MP_REACH data
	mpReach := buildTestMPReachIPv4()
	result := FilterResult{
		MPReach: []wireu.MPReachWire{mpReach},
	}

	// Should not panic with nil ctx
	announced := result.AnnouncedByFamily(nil)

	// Should still return results
	if len(announced) != 1 {
		t.Fatalf("len(AnnouncedByFamily(nil)) = %d, want 1", len(announced))
	}
}

// TestWithdrawnByFamilyNilCtx verifies nil ctx doesn't panic.
//
// VALIDATES: Nil ctx treated as no ADD-PATH.
// PREVENTS: Nil pointer dereference crash.
func TestWithdrawnByFamilyNilCtx(t *testing.T) {
	// Create a FilterResult with MP_UNREACH data
	mpUnreach := buildTestMPUnreachIPv4()
	result := FilterResult{
		MPUnreach: []wireu.MPUnreachWire{mpUnreach},
	}

	// Should not panic with nil ctx
	withdrawn := result.WithdrawnByFamily(nil)

	// Should still return results
	if len(withdrawn) != 1 {
		t.Fatalf("len(WithdrawnByFamily(nil)) = %d, want 1", len(withdrawn))
	}
}

// buildTestMPReachIPv4 builds MP_REACH_NLRI wire bytes for IPv4 unicast.
// Returns wire bytes for 192.168.1.0/24 with next-hop 10.0.0.1.
func buildTestMPReachIPv4() wireu.MPReachWire {
	data := make([]byte, 0, 32)
	data = append(data, 0x00, 0x01, 0x01, 0x04) // AFI: IPv4, SAFI: unicast, NH length: 4

	// Next-hop: 10.0.0.1
	nh := [4]byte{10, 0, 0, 1}
	data = append(data, nh[:]...)

	// Reserved + NLRI: 192.168.1.0/24
	data = append(data, 0x00, 24, 192, 168, 1)

	return wireu.MPReachWire(data)
}

// buildTestMPUnreachIPv4 builds MP_UNREACH_NLRI wire bytes for IPv4 unicast.
// Returns wire bytes for 192.168.1.0/24.
func buildTestMPUnreachIPv4() wireu.MPUnreachWire {
	// AFI: IPv4, SAFI: unicast, NLRI: 192.168.1.0/24
	data := make([]byte, 0, 16)
	data = append(data, 0x00, 0x01, 0x01, 24, 192, 168, 1)

	return wireu.MPUnreachWire(data)
}

// TODO: TestFormatMessageBothPaths disabled pending new API format implementation
// Will be replaced with tests for new format:
//   peer X update announce <attrs> <afi> <safi> next-hop <ip> nlri <prefixes>...
