package api

import (
	"encoding/binary"
	"testing"

	bgpctx "github.com/exa-networks/zebgp/pkg/bgp/context"
)

// TestWireUpdate_Derived verifies derived accessors return correct slices.
//
// VALIDATES: Withdrawn(), Attrs(), NLRI() return correct byte slices from UPDATE payload
// PREVENTS: Off-by-one errors in offset calculations.
func TestWireUpdate_Derived(t *testing.T) {
	// Build UPDATE payload:
	// WithdrawnLen(2) + Withdrawn(variable) + AttrLen(2) + Attrs(variable) + NLRI(variable)
	//
	// Test case: 2 bytes withdrawn, 3 bytes attrs, 4 bytes NLRI
	withdrawn := []byte{0x10, 0x0a} // /16 prefix 10.x.x.x (partial)
	attrs := []byte{0x40, 0x01, 0x01}
	nlri := []byte{0x18, 0xc0, 0xa8, 0x01} // /24 prefix 192.168.1.x

	payload := make([]byte, 2+len(withdrawn)+2+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(withdrawn))) //nolint:gosec // G115: test data
	copy(payload[2:], withdrawn)
	offset := 2 + len(withdrawn)
	binary.BigEndian.PutUint16(payload[offset:], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[offset+2:], attrs)
	copy(payload[offset+2+len(attrs):], nlri)

	wu := NewWireUpdate(payload, 0)

	// Test Withdrawn()
	gotWithdrawn := wu.Withdrawn()
	if len(gotWithdrawn) != len(withdrawn) {
		t.Errorf("Withdrawn() len = %d, want %d", len(gotWithdrawn), len(withdrawn))
	}
	for i := range withdrawn {
		if gotWithdrawn[i] != withdrawn[i] {
			t.Errorf("Withdrawn()[%d] = %02x, want %02x", i, gotWithdrawn[i], withdrawn[i])
		}
	}

	// Test NLRI()
	gotNLRI := wu.NLRI()
	if len(gotNLRI) != len(nlri) {
		t.Errorf("NLRI() len = %d, want %d", len(gotNLRI), len(nlri))
	}
	for i := range nlri {
		if gotNLRI[i] != nlri[i] {
			t.Errorf("NLRI()[%d] = %02x, want %02x", i, gotNLRI[i], nlri[i])
		}
	}

	// Test Attrs() returns non-nil AttributesWire
	gotAttrs := wu.Attrs()
	if gotAttrs == nil {
		t.Fatal("Attrs() returned nil, want *AttributesWire")
	}
	packed := gotAttrs.Packed()
	if len(packed) != len(attrs) {
		t.Errorf("Attrs().Packed() len = %d, want %d", len(packed), len(attrs))
	}
}

// TestWireUpdate_Empty verifies empty sections return nil.
//
// VALIDATES: Empty withdrawn/attrs/NLRI return nil, not empty slice
// PREVENTS: Nil pointer dereference on empty UPDATE.
func TestWireUpdate_Empty(t *testing.T) {
	// Empty UPDATE: WithdrawnLen=0, AttrLen=0, no NLRI
	payload := []byte{0x00, 0x00, 0x00, 0x00}

	wu := NewWireUpdate(payload, 0)

	if wu.Withdrawn() != nil {
		t.Errorf("Withdrawn() = %v, want nil", wu.Withdrawn())
	}
	if wu.Attrs() != nil {
		t.Errorf("Attrs() = %v, want nil", wu.Attrs())
	}
	if wu.NLRI() != nil {
		t.Errorf("NLRI() = %v, want nil", wu.NLRI())
	}
}

// TestWireUpdate_Malformed verifies truncated data returns nil.
//
// VALIDATES: Short/truncated payloads return nil gracefully
// PREVENTS: Panic on malformed UPDATE.
func TestWireUpdate_Malformed(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"too_short_1", []byte{0x00}},
		{"withdrawn_truncated", []byte{0x00, 0x05, 0x01}}, // claims 5 bytes withdrawn, only 1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wu := NewWireUpdate(tt.payload, 0)
			if wu.Withdrawn() != nil {
				t.Errorf("Withdrawn() = %v, want nil", wu.Withdrawn())
			}
			if wu.Attrs() != nil {
				t.Errorf("Attrs() = %v, want nil", wu.Attrs())
			}
			if wu.NLRI() != nil {
				t.Errorf("NLRI() = %v, want nil", wu.NLRI())
			}
		})
	}
}

// TestWireUpdate_MPReach verifies MP_REACH_NLRI extraction.
//
// VALIDATES: MPReach() extracts attribute code 14 from path attributes
// PREVENTS: Wrong attribute extraction for MP-BGP.
func TestWireUpdate_MPReach(t *testing.T) {
	// Build MP_REACH_NLRI attribute (code 14)
	// Format: flags(1) + code(1) + len(1 or 2) + value
	// Value: AFI(2) + SAFI(1) + NHLen(1) + NH + Reserved(1) + NLRI
	//
	// IPv6 unicast: AFI=2, SAFI=1, NH=16 bytes
	mpReachValue := []byte{
		0x00, 0x02, // AFI = 2 (IPv6)
		0x01,                                                                                           // SAFI = 1 (unicast)
		0x10,                                                                                           // NH length = 16
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // NH
		0x00,                                                 // Reserved
		0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00, // /64 prefix 2001:db8:1::
	}

	// Attribute header: Optional+Transitive (0x80), code 14, extended length
	attrs := []byte{0x90, 0x0e, 0x00, byte(len(mpReachValue))}
	attrs = append(attrs, mpReachValue...)

	// Build UPDATE: no withdrawn, attrs with MP_REACH, no legacy NLRI
	payload := make([]byte, 2+0+2+len(attrs)+0)
	binary.BigEndian.PutUint16(payload[0:2], 0)                  // withdrawn len
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[4:], attrs)

	wu := NewWireUpdate(payload, 1) // ctxID=1

	mpr := wu.MPReach()
	if mpr == nil {
		t.Fatal("MPReach() returned nil")
	}

	if mpr.AFI() != 2 {
		t.Errorf("MPReach().AFI() = %d, want 2", mpr.AFI())
	}
	if mpr.SAFI() != 1 {
		t.Errorf("MPReach().SAFI() = %d, want 1", mpr.SAFI())
	}
}

// TestWireUpdate_MPUnreach verifies MP_UNREACH_NLRI extraction.
//
// VALIDATES: MPUnreach() extracts attribute code 15 from path attributes
// PREVENTS: Wrong attribute extraction for MP-BGP withdrawals.
func TestWireUpdate_MPUnreach(t *testing.T) {
	// Build MP_UNREACH_NLRI attribute (code 15)
	// Value: AFI(2) + SAFI(1) + Withdrawn
	mpUnreachValue := []byte{
		0x00, 0x02, // AFI = 2 (IPv6)
		0x01,                                                 // SAFI = 1 (unicast)
		0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02, 0x00, 0x00, // /64 prefix 2001:db8:2::
	}

	// Attribute header: Optional+non-transitive (0x80), code 15
	attrs := []byte{0x80, 0x0f, byte(len(mpUnreachValue))}
	attrs = append(attrs, mpUnreachValue...)

	// Build UPDATE: no withdrawn, attrs with MP_UNREACH, no legacy NLRI
	payload := make([]byte, 2+0+2+len(attrs)+0)
	binary.BigEndian.PutUint16(payload[0:2], 0)
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[4:], attrs)

	wu := NewWireUpdate(payload, 1)

	mpu := wu.MPUnreach()
	if mpu == nil {
		t.Fatal("MPUnreach() returned nil")
	}

	if mpu.AFI() != 2 {
		t.Errorf("MPUnreach().AFI() = %d, want 2", mpu.AFI())
	}
	if mpu.SAFI() != 1 {
		t.Errorf("MPUnreach().SAFI() = %d, want 1", mpu.SAFI())
	}
}

// TestWireUpdate_SourceCtxID verifies context ID is preserved.
//
// VALIDATES: SourceCtxID() returns the context ID passed to constructor
// PREVENTS: Lost context for zero-copy decisions.
func TestWireUpdate_SourceCtxID(t *testing.T) {
	payload := []byte{0x00, 0x00, 0x00, 0x00}
	ctxID := bgpctx.ContextID(42)

	wu := NewWireUpdate(payload, ctxID)

	if wu.SourceCtxID() != ctxID {
		t.Errorf("SourceCtxID() = %d, want %d", wu.SourceCtxID(), ctxID)
	}
}

// TestWireUpdate_Payload verifies raw payload access.
//
// VALIDATES: Payload() returns original buffer
// PREVENTS: Buffer copying when passthrough needed.
func TestWireUpdate_Payload(t *testing.T) {
	payload := []byte{0x00, 0x00, 0x00, 0x00}

	wu := NewWireUpdate(payload, 0)

	got := wu.Payload()
	if &got[0] != &payload[0] {
		t.Error("Payload() returned copy, want same underlying array")
	}
}

// TestWireUpdate_AttrsCached verifies AttributesWire is cached.
//
// VALIDATES: Multiple Attrs() calls return same instance.
// PREVENTS: Duplicate attribute parsing overhead.
func TestWireUpdate_AttrsCached(t *testing.T) {
	// Build UPDATE with attributes
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	payload := make([]byte, 2+0+2+len(attrs))
	binary.BigEndian.PutUint16(payload[0:2], 0)                  // withdrawn len
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[4:], attrs)

	wu := NewWireUpdate(payload, 1)

	// First call
	attrs1 := wu.Attrs()
	if attrs1 == nil {
		t.Fatal("Attrs() returned nil")
	}

	// Second call should return same instance
	attrs2 := wu.Attrs()
	if attrs1 != attrs2 {
		t.Error("Attrs() returned different instance, want same (cached)")
	}

	// MPReach uses cached Attrs internally
	// (will return nil since no MP_REACH, but shouldn't create new AttributesWire)
	_ = wu.MPReach()
	attrs3 := wu.Attrs()
	if attrs1 != attrs3 {
		t.Error("Attrs() after MPReach() returned different instance")
	}
}
