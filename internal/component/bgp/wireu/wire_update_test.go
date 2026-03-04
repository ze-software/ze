package wireu

import (
	"encoding/binary"
	"errors"
	"testing"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
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
	gotWithdrawn, err := wu.Withdrawn()
	if err != nil {
		t.Fatalf("Withdrawn() error = %v", err)
	}
	if len(gotWithdrawn) != len(withdrawn) {
		t.Errorf("Withdrawn() len = %d, want %d", len(gotWithdrawn), len(withdrawn))
	}
	for i := range withdrawn {
		if gotWithdrawn[i] != withdrawn[i] {
			t.Errorf("Withdrawn()[%d] = %02x, want %02x", i, gotWithdrawn[i], withdrawn[i])
		}
	}

	// Test NLRI()
	gotNLRI, err := wu.NLRI()
	if err != nil {
		t.Fatalf("NLRI() error = %v", err)
	}
	if len(gotNLRI) != len(nlri) {
		t.Errorf("NLRI() len = %d, want %d", len(gotNLRI), len(nlri))
	}
	for i := range nlri {
		if gotNLRI[i] != nlri[i] {
			t.Errorf("NLRI()[%d] = %02x, want %02x", i, gotNLRI[i], nlri[i])
		}
	}

	// Test Attrs() returns non-nil AttributesWire
	gotAttrs, err := wu.Attrs()
	if err != nil {
		t.Fatalf("Attrs() error = %v", err)
	}
	if gotAttrs == nil {
		t.Fatal("Attrs() returned nil, want *AttributesWire")
	}
	packed := gotAttrs.Packed()
	if len(packed) != len(attrs) {
		t.Errorf("Attrs().Packed() len = %d, want %d", len(packed), len(attrs))
	}
}

// TestWireUpdate_Empty verifies empty sections return nil,nil.
//
// VALIDATES: Empty withdrawn/attrs/NLRI return nil,nil (valid empty)
// PREVENTS: False errors on valid empty UPDATE.
func TestWireUpdate_Empty(t *testing.T) {
	// Empty UPDATE: WithdrawnLen=0, AttrLen=0, no NLRI
	payload := []byte{0x00, 0x00, 0x00, 0x00}

	wu := NewWireUpdate(payload, 0)

	wd, err := wu.Withdrawn()
	if err != nil {
		t.Errorf("Withdrawn() error = %v, want nil", err)
	}
	if wd != nil {
		t.Errorf("Withdrawn() = %v, want nil", wd)
	}

	attrs, err := wu.Attrs()
	if err != nil {
		t.Errorf("Attrs() error = %v, want nil", err)
	}
	if attrs != nil {
		t.Errorf("Attrs() = %v, want nil", attrs)
	}

	nlri, err := wu.NLRI()
	if err != nil {
		t.Errorf("NLRI() error = %v, want nil", err)
	}
	if nlri != nil {
		t.Errorf("NLRI() = %v, want nil", nlri)
	}
}

// TestWireUpdate_Malformed verifies truncated data returns error.
//
// VALIDATES: Short/truncated payloads return error
// PREVENTS: Silent corruption from malformed UPDATE.
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

			_, err := wu.Withdrawn()
			if err == nil {
				t.Error("Withdrawn() should return error for malformed payload")
			}

			_, err = wu.Attrs()
			if err == nil {
				t.Error("Attrs() should return error for malformed payload")
			}

			_, err = wu.NLRI()
			if err == nil {
				t.Error("NLRI() should return error for malformed payload")
			}
		})
	}
}

// TestWireUpdate_ErrorContext verifies error messages contain field context.
//
// VALIDATES: Errors include field name (withdrawn:, attrs:, nlri:, etc.)
// PREVENTS: Ambiguous error messages that don't identify the problem location.
func TestWireUpdate_ErrorContext(t *testing.T) {
	tests := []struct {
		name        string
		payload     []byte
		method      string
		wantContext string
	}{
		{
			name:        "withdrawn_truncated",
			payload:     []byte{0x00, 0x05, 0x01}, // claims 5 bytes, has 1
			method:      "Withdrawn",
			wantContext: "withdrawn:",
		},
		{
			name:        "attrs_truncated",
			payload:     []byte{0x00, 0x00, 0x00, 0x10, 0x40}, // claims 16 bytes attrs, has 1
			method:      "Attrs",
			wantContext: "attrs:",
		},
		{
			name:        "nlri_truncated",
			payload:     []byte{0x00}, // too short for withdrawn len
			method:      "NLRI",
			wantContext: "nlri:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wu := NewWireUpdate(tt.payload, 0)

			var err error
			switch tt.method {
			case "Withdrawn":
				_, err = wu.Withdrawn()
			case "Attrs":
				_, err = wu.Attrs()
			case "NLRI":
				_, err = wu.NLRI()
			}

			if err == nil {
				t.Fatalf("%s() should return error", tt.method)
			}
			if !errors.Is(err, ErrUpdateTruncated) {
				t.Errorf("%s() error = %v, want ErrUpdateTruncated", tt.method, err)
			}
			errStr := err.Error()
			if !contains(errStr, tt.wantContext) {
				t.Errorf("%s() error = %q, want context %q", tt.method, errStr, tt.wantContext)
			}
		})
	}
}

// contains checks if s contains substr (simple helper to avoid import).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestWireUpdate_BoundaryConditions verifies exact boundary behavior.
//
// VALIDATES: Edge cases at length boundaries handled correctly.
// PREVENTS: Off-by-one errors in length validation.
//
// Note: With shared parsing, the entire UPDATE structure is validated upfront.
// Truncated payloads (< 4 bytes minimum) fail all accessors consistently.
// This is stricter than per-accessor validation but more predictable.
func TestWireUpdate_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name      string
		payload   []byte
		wantWdErr bool
		wantAtErr bool
		wantNlErr bool
	}{
		{
			name:      "exactly_1_byte",
			payload:   []byte{0x00},
			wantWdErr: true, // Truncated: need 4 bytes minimum (wdLen + attrLen)
			wantAtErr: true,
			wantNlErr: true,
		},
		{
			name:      "exactly_2_bytes_wd0",
			payload:   []byte{0x00, 0x00},
			wantWdErr: true, // Truncated: need 4 bytes minimum
			wantAtErr: true,
			wantNlErr: true,
		},
		{
			name:      "exactly_4_bytes_valid_empty",
			payload:   []byte{0x00, 0x00, 0x00, 0x00},
			wantWdErr: false, // withdrawn len = 0, valid
			wantAtErr: false, // attr len = 0, valid
			wantNlErr: false, // no NLRI, valid
		},
		{
			name:      "wd_len_points_to_exact_end",
			payload:   []byte{0x00, 0x02, 0xAA, 0xBB}, // wd=2, but no attr len field
			wantWdErr: true,                           // Truncated: no room for attrLen
			wantAtErr: true,
			wantNlErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wu := NewWireUpdate(tt.payload, 0)

			_, err := wu.Withdrawn()
			if (err != nil) != tt.wantWdErr {
				t.Errorf("Withdrawn() error = %v, wantErr = %v", err, tt.wantWdErr)
			}

			_, err = wu.Attrs()
			if (err != nil) != tt.wantAtErr {
				t.Errorf("Attrs() error = %v, wantErr = %v", err, tt.wantAtErr)
			}

			_, err = wu.NLRI()
			if (err != nil) != tt.wantNlErr {
				t.Errorf("NLRI() error = %v, wantErr = %v", err, tt.wantNlErr)
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
	attrs := make([]byte, 0, 4+len(mpReachValue))
	attrs = append(attrs, 0x90, 0x0e, 0x00, byte(len(mpReachValue)))
	attrs = append(attrs, mpReachValue...)

	// Build UPDATE: no withdrawn, attrs with MP_REACH, no legacy NLRI
	payload := make([]byte, 2+0+2+len(attrs)+0)
	binary.BigEndian.PutUint16(payload[0:2], 0)                  // withdrawn len
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[4:], attrs)

	wu := NewWireUpdate(payload, 1) // ctxID=1

	mpr, err := wu.MPReach()
	if err != nil {
		t.Fatalf("MPReach() error = %v", err)
	}
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
	attrs := make([]byte, 0, 3+len(mpUnreachValue))
	attrs = append(attrs, 0x80, 0x0f, byte(len(mpUnreachValue)))
	attrs = append(attrs, mpUnreachValue...)

	// Build UPDATE: no withdrawn, attrs with MP_UNREACH, no legacy NLRI
	payload := make([]byte, 2+0+2+len(attrs)+0)
	binary.BigEndian.PutUint16(payload[0:2], 0)
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[4:], attrs)

	wu := NewWireUpdate(payload, 1)

	mpu, err := wu.MPUnreach()
	if err != nil {
		t.Fatalf("MPUnreach() error = %v", err)
	}
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

// TestWireUpdate_SourceID verifies source ID get/set.
//
// VALIDATES: SourceID() returns value set by SetSourceID()
// PREVENTS: Lost source identity for message tracking.
func TestWireUpdate_SourceID(t *testing.T) {
	payload := []byte{0x00, 0x00, 0x00, 0x00}
	wu := NewWireUpdate(payload, 0)

	// Initially zero
	if wu.SourceID() != 0 {
		t.Errorf("SourceID() = %d, want 0 initially", wu.SourceID())
	}

	// Set and verify
	wu.SetSourceID(42)
	if wu.SourceID() != 42 {
		t.Errorf("SourceID() = %d, want 42", wu.SourceID())
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

// TestWireUpdate_AttrsConsistent verifies multiple Attrs() calls return same data.
//
// VALIDATES: Multiple Attrs() calls return consistent results.
// PREVENTS: Data corruption on repeated access.
func TestWireUpdate_AttrsConsistent(t *testing.T) {
	// Build UPDATE with attributes
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	payload := make([]byte, 2+0+2+len(attrs))
	binary.BigEndian.PutUint16(payload[0:2], 0)                  // withdrawn len
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[4:], attrs)

	wu := NewWireUpdate(payload, 1)

	// First call
	attrs1, err := wu.Attrs()
	if err != nil {
		t.Fatalf("Attrs() error: %v", err)
	}
	if attrs1 == nil {
		t.Fatal("Attrs() returned nil")
	}
	packed1 := attrs1.Packed()

	// Second call should return same data (new instance OK, data must match)
	attrs2, err := wu.Attrs()
	if err != nil {
		t.Fatalf("Attrs() second call error: %v", err)
	}
	packed2 := attrs2.Packed()

	// Verify data consistency
	if len(packed1) != len(packed2) {
		t.Errorf("Attrs() returned different lengths: %d vs %d", len(packed1), len(packed2))
	}
	for i := range packed1 {
		if packed1[i] != packed2[i] {
			t.Errorf("Attrs() data differs at byte %d", i)
			break
		}
	}
}

// TestWireUpdate_Withdrawn_Error verifies truncated withdrawn returns error.
//
// VALIDATES: Truncated payload returns error, not nil
// PREVENTS: Silent corruption from malformed UPDATE.
func TestWireUpdate_Withdrawn_Error(t *testing.T) {
	// Claims 5 bytes withdrawn, only has 1
	payload := []byte{0x00, 0x05, 0x01}

	wu := NewWireUpdate(payload, 0)

	_, err := wu.Withdrawn()
	if err == nil {
		t.Fatal("Withdrawn() should return error for truncated payload")
	}
	if !errors.Is(err, ErrUpdateTruncated) {
		t.Errorf("Withdrawn() error = %v, want ErrUpdateTruncated", err)
	}
}

// TestWireUpdate_Withdrawn_Empty verifies empty withdrawn returns nil,nil.
//
// VALIDATES: wdLen=0 returns nil,nil (valid empty)
// PREVENTS: False error on valid empty UPDATE.
func TestWireUpdate_Withdrawn_Empty(t *testing.T) {
	// Empty withdrawn, empty attrs, no NLRI
	payload := []byte{0x00, 0x00, 0x00, 0x00}

	wu := NewWireUpdate(payload, 0)

	data, err := wu.Withdrawn()
	if err != nil {
		t.Errorf("Withdrawn() error = %v, want nil", err)
	}
	if data != nil {
		t.Errorf("Withdrawn() = %v, want nil", data)
	}
}

// TestWireUpdate_Attrs_Error verifies truncated attrs returns error.
//
// VALIDATES: Truncated attrs section returns error
// PREVENTS: Silent corruption from malformed UPDATE.
func TestWireUpdate_Attrs_Error(t *testing.T) {
	// Claims 10 bytes attrs, only has 2
	payload := []byte{0x00, 0x00, 0x00, 0x0a, 0x40, 0x01}

	wu := NewWireUpdate(payload, 0)

	_, err := wu.Attrs()
	if err == nil {
		t.Fatal("Attrs() should return error for truncated payload")
	}
	if !errors.Is(err, ErrUpdateTruncated) {
		t.Errorf("Attrs() error = %v, want ErrUpdateTruncated", err)
	}
}

// TestWireUpdate_Attrs_Empty verifies empty attrs returns nil,nil.
//
// VALIDATES: attrLen=0 returns nil,nil (valid empty)
// PREVENTS: False error on withdraw-only UPDATE.
func TestWireUpdate_Attrs_Empty(t *testing.T) {
	// Empty withdrawn, empty attrs, no NLRI
	payload := []byte{0x00, 0x00, 0x00, 0x00}

	wu := NewWireUpdate(payload, 0)

	data, err := wu.Attrs()
	if err != nil {
		t.Errorf("Attrs() error = %v, want nil", err)
	}
	if data != nil {
		t.Errorf("Attrs() = %v, want nil", data)
	}
}

// TestWireUpdate_NLRI_Error verifies truncated NLRI returns error.
//
// VALIDATES: Truncated payload before NLRI section returns error
// PREVENTS: Silent corruption from malformed UPDATE.
func TestWireUpdate_NLRI_Error(t *testing.T) {
	// Missing attr length bytes entirely
	payload := []byte{0x00, 0x00}

	wu := NewWireUpdate(payload, 0)

	_, err := wu.NLRI()
	if err == nil {
		t.Fatal("NLRI() should return error for truncated payload")
	}
	if !errors.Is(err, ErrUpdateTruncated) {
		t.Errorf("NLRI() error = %v, want ErrUpdateTruncated", err)
	}
}

// TestWireUpdate_NLRI_Empty verifies no trailing NLRI returns nil,nil.
//
// VALIDATES: No trailing bytes after attrs returns nil,nil (valid)
// PREVENTS: False error on MP-BGP only UPDATE.
func TestWireUpdate_NLRI_Empty(t *testing.T) {
	// Empty withdrawn, empty attrs, no NLRI
	payload := []byte{0x00, 0x00, 0x00, 0x00}

	wu := NewWireUpdate(payload, 0)

	data, err := wu.NLRI()
	if err != nil {
		t.Errorf("NLRI() error = %v, want nil", err)
	}
	if data != nil {
		t.Errorf("NLRI() = %v, want nil", data)
	}
}

// TestWireUpdate_MPReach_NotPresent verifies missing attr returns nil,nil.
//
// VALIDATES: Missing MP_REACH_NLRI attribute returns nil,nil
// PREVENTS: False error on IPv4-only UPDATE.
func TestWireUpdate_MPReach_NotPresent(t *testing.T) {
	// UPDATE with ORIGIN only, no MP_REACH
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	payload := make([]byte, 2+0+2+len(attrs))
	binary.BigEndian.PutUint16(payload[0:2], 0)
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[4:], attrs)

	wu := NewWireUpdate(payload, 0)

	data, err := wu.MPReach()
	if err != nil {
		t.Errorf("MPReach() error = %v, want nil", err)
	}
	if data != nil {
		t.Errorf("MPReach() = %v, want nil", data)
	}
}

// TestWireUpdate_MPReach_Malformed verifies short MP_REACH returns error.
//
// VALIDATES: MP_REACH_NLRI shorter than minimum (5 bytes) returns error
// PREVENTS: Panic on malformed MP_REACH_NLRI.
func TestWireUpdate_MPReach_Malformed(t *testing.T) {
	// MP_REACH with only 3 bytes (need at least 5: AFI(2)+SAFI(1)+NHLen(1)+Reserved(1))
	mpReachValue := []byte{0x00, 0x01, 0x01} // AFI + SAFI only
	attrs := make([]byte, 0, 3+len(mpReachValue))
	attrs = append(attrs, 0x80, 0x0e, byte(len(mpReachValue)))
	attrs = append(attrs, mpReachValue...)

	payload := make([]byte, 2+0+2+len(attrs))
	binary.BigEndian.PutUint16(payload[0:2], 0)
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[4:], attrs)

	wu := NewWireUpdate(payload, 0)

	_, err := wu.MPReach()
	if err == nil {
		t.Fatal("MPReach() should return error for malformed attribute")
	}
	if !errors.Is(err, ErrUpdateMalformed) {
		t.Errorf("MPReach() error = %v, want ErrUpdateMalformed", err)
	}
}

// TestWireUpdate_MPReach_AttrsError verifies MPReach propagates Attrs() error.
//
// VALIDATES: When Attrs() fails, MPReach wraps and returns that error
// PREVENTS: Silent failure when underlying parse fails.
func TestWireUpdate_MPReach_AttrsError(t *testing.T) {
	// Truncated payload - Attrs() will fail
	payload := []byte{0x00, 0x00, 0x00, 0x0a, 0x40, 0x01} // Claims 10 bytes attrs, only 2

	wu := NewWireUpdate(payload, 0)

	_, err := wu.MPReach()
	if err == nil {
		t.Fatal("MPReach() should return error when Attrs() fails")
	}
	if !errors.Is(err, ErrUpdateTruncated) {
		t.Errorf("MPReach() error = %v, want ErrUpdateTruncated", err)
	}
}

// TestWireUpdate_MPUnreach_AttrsError verifies MPUnreach propagates Attrs() error.
//
// VALIDATES: When Attrs() fails, MPUnreach wraps and returns that error
// PREVENTS: Silent failure when underlying parse fails.
func TestWireUpdate_MPUnreach_AttrsError(t *testing.T) {
	// Truncated payload - Attrs() will fail
	payload := []byte{0x00, 0x00, 0x00, 0x0a, 0x40, 0x01} // Claims 10 bytes attrs, only 2

	wu := NewWireUpdate(payload, 0)

	_, err := wu.MPUnreach()
	if err == nil {
		t.Fatal("MPUnreach() should return error when Attrs() fails")
	}
	if !errors.Is(err, ErrUpdateTruncated) {
		t.Errorf("MPUnreach() error = %v, want ErrUpdateTruncated", err)
	}
}

// TestWireUpdate_MPUnreach_NotPresent verifies missing attr returns nil,nil.
//
// VALIDATES: Missing MP_UNREACH_NLRI attribute returns nil,nil
// PREVENTS: False error on IPv4-only UPDATE.
func TestWireUpdate_MPUnreach_NotPresent(t *testing.T) {
	// UPDATE with ORIGIN only, no MP_UNREACH
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	payload := make([]byte, 2+0+2+len(attrs))
	binary.BigEndian.PutUint16(payload[0:2], 0)
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[4:], attrs)

	wu := NewWireUpdate(payload, 0)

	data, err := wu.MPUnreach()
	if err != nil {
		t.Errorf("MPUnreach() error = %v, want nil", err)
	}
	if data != nil {
		t.Errorf("MPUnreach() = %v, want nil", data)
	}
}

// TestWireUpdate_MPUnreach_Malformed verifies short MP_UNREACH returns error.
//
// VALIDATES: MP_UNREACH_NLRI shorter than minimum (3 bytes) returns error
// PREVENTS: Panic on malformed MP_UNREACH_NLRI.
func TestWireUpdate_MPUnreach_Malformed(t *testing.T) {
	// MP_UNREACH with only 2 bytes (need at least 3: AFI(2)+SAFI(1))
	mpUnreachValue := []byte{0x00, 0x01} // AFI only
	attrs := make([]byte, 0, 3+len(mpUnreachValue))
	attrs = append(attrs, 0x80, 0x0f, byte(len(mpUnreachValue)))
	attrs = append(attrs, mpUnreachValue...)

	payload := make([]byte, 2+0+2+len(attrs))
	binary.BigEndian.PutUint16(payload[0:2], 0)
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[4:], attrs)

	wu := NewWireUpdate(payload, 0)

	_, err := wu.MPUnreach()
	if err == nil {
		t.Fatal("MPUnreach() should return error for malformed attribute")
	}
	if !errors.Is(err, ErrUpdateMalformed) {
		t.Errorf("MPUnreach() error = %v, want ErrUpdateMalformed", err)
	}
}

// TestWireUpdate_TruncatedAfterWithdrawn verifies truncated payload detection.
//
// VALIDATES: Payload missing attrLen field is detected as truncated.
// PREVENTS: Processing malformed UPDATE with incomplete structure.
//
// Note: With shared parsing, the entire structure is validated upfront.
// All accessors fail consistently for a truncated payload.
func TestWireUpdate_TruncatedAfterWithdrawn(t *testing.T) {
	// wdLen=2, payload has exactly 4 bytes (2 for len + 2 for withdrawn data)
	// No bytes remaining for attrLen field - truncated
	payload := []byte{0x00, 0x02, 0xAA, 0xBB}

	wu := NewWireUpdate(payload, 0)

	// All accessors should fail - payload is truncated (no attrLen field)
	_, err := wu.Withdrawn()
	if err == nil {
		t.Error("Withdrawn() should fail on truncated payload")
	}
	if !errors.Is(err, ErrUpdateTruncated) {
		t.Errorf("Withdrawn() error = %v, want ErrUpdateTruncated", err)
	}

	_, err = wu.Attrs()
	if err == nil {
		t.Error("Attrs() should fail on truncated payload")
	}
	if !errors.Is(err, ErrUpdateTruncated) {
		t.Errorf("Attrs() error = %v, want ErrUpdateTruncated", err)
	}

	_, err = wu.NLRI()
	if err == nil {
		t.Error("NLRI() should fail on truncated payload")
	}
	if !errors.Is(err, ErrUpdateTruncated) {
		t.Errorf("NLRI() error = %v, want ErrUpdateTruncated", err)
	}
}

// TestWireUpdate_Attrs_TruncatedByOne verifies off-by-one truncation detected.
//
// VALIDATES: attrLen=4 with only 3 bytes present returns error
// PREVENTS: Reading beyond buffer on off-by-one truncation.
func TestWireUpdate_Attrs_TruncatedByOne(t *testing.T) {
	// wdLen=0, attrLen=4, but only 3 bytes of attrs present
	payload := []byte{0x00, 0x00, 0x00, 0x04, 0x40, 0x01, 0x01}

	wu := NewWireUpdate(payload, 0)

	_, err := wu.Attrs()
	if err == nil {
		t.Fatal("Attrs() should fail when attrs truncated by one byte")
	}
	if !errors.Is(err, ErrUpdateTruncated) {
		t.Errorf("Attrs() error = %v, want ErrUpdateTruncated", err)
	}
}

// TestWireUpdate_NLRI_WithEmptyAttrs verifies NLRI extraction when attrLen=0.
//
// VALIDATES: When attrLen=0, trailing bytes are returned as NLRI
// PREVENTS: Missing NLRI when attrs section is empty.
func TestWireUpdate_NLRI_WithEmptyAttrs(t *testing.T) {
	// wdLen=0, attrLen=0, then NLRI bytes
	nlriBytes := []byte{0x18, 0x0A, 0x00, 0x00} // /24 prefix 10.0.0.x
	payload := make([]byte, 0, 4+len(nlriBytes))
	payload = append(payload, 0x00, 0x00, 0x00, 0x00)
	payload = append(payload, nlriBytes...)

	wu := NewWireUpdate(payload, 0)

	// Attrs should return nil (empty)
	attrs, err := wu.Attrs()
	if err != nil {
		t.Errorf("Attrs() error = %v, want nil", err)
	}
	if attrs != nil {
		t.Errorf("Attrs() = %v, want nil", attrs)
	}

	// NLRI should return the trailing bytes
	nlri, err := wu.NLRI()
	if err != nil {
		t.Errorf("NLRI() error = %v, want nil", err)
	}
	if len(nlri) != len(nlriBytes) {
		t.Errorf("NLRI() len = %d, want %d", len(nlri), len(nlriBytes))
	}
	for i, b := range nlriBytes {
		if nlri[i] != b {
			t.Errorf("NLRI()[%d] = %02x, want %02x", i, nlri[i], b)
		}
	}
}

// TestWireUpdate_NLRI_SingleByte verifies minimal NLRI (1 byte) is returned.
//
// VALIDATES: Single byte NLRI is correctly extracted
// PREVENTS: Off-by-one in NLRI boundary detection.
func TestWireUpdate_NLRI_SingleByte(t *testing.T) {
	// wdLen=0, attrLen=0, single byte NLRI (e.g., /8 prefix length only)
	payload := []byte{0x00, 0x00, 0x00, 0x00, 0x08}

	wu := NewWireUpdate(payload, 0)

	nlri, err := wu.NLRI()
	if err != nil {
		t.Errorf("NLRI() error = %v, want nil", err)
	}
	if len(nlri) != 1 {
		t.Errorf("NLRI() len = %d, want 1", len(nlri))
	}
	if nlri[0] != 0x08 {
		t.Errorf("NLRI()[0] = %02x, want 0x08", nlri[0])
	}
}

// TestWireUpdate_NLRI_AttrLenTruncated verifies NLRI fails when attrLen claims too much.
//
// VALIDATES: When attrLen exceeds remaining payload, NLRI returns error
// PREVENTS: Returning garbage NLRI from invalid offset.
func TestWireUpdate_NLRI_AttrLenTruncated(t *testing.T) {
	// wdLen=0, attrLen=10, but only 2 bytes of attrs present
	// nlriStart would be 4 + 10 = 14, but payload is only 6 bytes
	payload := []byte{0x00, 0x00, 0x00, 0x0a, 0x40, 0x01}

	wu := NewWireUpdate(payload, 0)

	_, err := wu.NLRI()
	if err == nil {
		t.Fatal("NLRI() should fail when attrLen exceeds payload")
	}
	if !errors.Is(err, ErrUpdateTruncated) {
		t.Errorf("NLRI() error = %v, want ErrUpdateTruncated", err)
	}
}

// TestWireUpdate_AllSections verifies all three sections parse correctly together.
//
// VALIDATES: Withdrawn, Attrs, and NLRI all return correct data from same payload
// PREVENTS: Offset calculation errors affecting multiple sections.
func TestWireUpdate_AllSections(t *testing.T) {
	// Build payload with all sections populated
	withdrawn := []byte{0x10, 0x0A}         // /16 10.x.x.x
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	nlri := []byte{0x18, 0xC0, 0xA8, 0x01}  // /24 192.168.1.x

	payload := make([]byte, 2+len(withdrawn)+2+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(withdrawn))) //nolint:gosec // test data, small fixed slice
	copy(payload[2:], withdrawn)
	offset := 2 + len(withdrawn)
	binary.BigEndian.PutUint16(payload[offset:], uint16(len(attrs))) //nolint:gosec // test data, small fixed slice
	copy(payload[offset+2:], attrs)
	copy(payload[offset+2+len(attrs):], nlri)

	wu := NewWireUpdate(payload, 0)

	// Verify Withdrawn
	gotWd, err := wu.Withdrawn()
	if err != nil {
		t.Fatalf("Withdrawn() error = %v", err)
	}
	if len(gotWd) != len(withdrawn) {
		t.Errorf("Withdrawn() len = %d, want %d", len(gotWd), len(withdrawn))
	}

	// Verify Attrs
	gotAttrs, err := wu.Attrs()
	if err != nil {
		t.Fatalf("Attrs() error = %v", err)
	}
	if gotAttrs == nil {
		t.Fatal("Attrs() = nil, want non-nil")
	}
	if len(gotAttrs.Packed()) != len(attrs) {
		t.Errorf("Attrs().Packed() len = %d, want %d", len(gotAttrs.Packed()), len(attrs))
	}

	// Verify NLRI
	gotNlri, err := wu.NLRI()
	if err != nil {
		t.Fatalf("NLRI() error = %v", err)
	}
	if len(gotNlri) != len(nlri) {
		t.Errorf("NLRI() len = %d, want %d", len(gotNlri), len(nlri))
	}
}

// TestWireUpdate_NLRIIterator verifies NLRI iteration via iterator.
//
// VALIDATES: NLRIIterator returns iterator over NLRI section.
// PREVENTS: Incorrect NLRI traversal.
func TestWireUpdate_NLRIIterator(t *testing.T) {
	// Build UPDATE with multiple NLRIs: 10.0.0.0/8, 192.168.1.0/24
	nlriBytes := []byte{
		0x08, 0x0A, // 10.0.0.0/8
		0x18, 0xC0, 0xA8, 0x01, // 192.168.1.0/24
	}
	payload := make([]byte, 4+len(nlriBytes))
	binary.BigEndian.PutUint16(payload[0:2], 0) // withdrawn len
	binary.BigEndian.PutUint16(payload[2:4], 0) // attr len
	copy(payload[4:], nlriBytes)

	wu := NewWireUpdate(payload, 0)

	iter, err := wu.NLRIIterator(false)
	if err != nil {
		t.Fatalf("NLRIIterator() error = %v", err)
	}
	if iter == nil {
		t.Fatal("NLRIIterator() returned nil")
	}

	// First prefix: 10.0.0.0/8
	prefix, pathID, ok := iter.Next()
	if !ok {
		t.Fatal("expected first NLRI")
	}
	if len(prefix) != 2 || prefix[0] != 0x08 || prefix[1] != 0x0A {
		t.Errorf("first prefix = %v, want [0x08, 0x0A]", prefix)
	}
	if pathID != 0 {
		t.Errorf("first pathID = %d, want 0", pathID)
	}

	// Second prefix: 192.168.1.0/24
	prefix, _, ok = iter.Next()
	if !ok {
		t.Fatal("expected second NLRI")
	}
	if len(prefix) != 4 {
		t.Errorf("second prefix len = %d, want 4", len(prefix))
	}

	// No more
	_, _, ok = iter.Next()
	if ok {
		t.Error("expected no more NLRIs")
	}
}

// TestWireUpdate_NLRIIteratorEmpty verifies empty NLRI returns nil iterator.
//
// VALIDATES: Empty NLRI section returns nil iterator (not error).
// PREVENTS: False error on MP-BGP only UPDATE.
func TestWireUpdate_NLRIIteratorEmpty(t *testing.T) {
	payload := []byte{0x00, 0x00, 0x00, 0x00} // empty update

	wu := NewWireUpdate(payload, 0)

	iter, err := wu.NLRIIterator(false)
	if err != nil {
		t.Errorf("NLRIIterator() error = %v, want nil", err)
	}
	if iter != nil {
		t.Error("NLRIIterator() should return nil for empty NLRI")
	}
}

// TestWireUpdate_WithdrawnIterator verifies withdrawn routes iteration.
//
// VALIDATES: WithdrawnIterator returns iterator over withdrawn section.
// PREVENTS: Incorrect withdrawn route traversal.
func TestWireUpdate_WithdrawnIterator(t *testing.T) {
	// Build UPDATE with withdrawn: 10.0.0.0/8, 172.16.0.0/16
	wdBytes := []byte{
		0x08, 0x0A, // 10.0.0.0/8
		0x10, 0xAC, 0x10, // 172.16.0.0/16
	}
	payload := make([]byte, 2+len(wdBytes)+2)
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(wdBytes))) //nolint:gosec // test
	copy(payload[2:], wdBytes)
	binary.BigEndian.PutUint16(payload[2+len(wdBytes):], 0) // attr len

	wu := NewWireUpdate(payload, 0)

	iter, err := wu.WithdrawnIterator(false)
	if err != nil {
		t.Fatalf("WithdrawnIterator() error = %v", err)
	}
	if iter == nil {
		t.Fatal("WithdrawnIterator() returned nil")
	}

	// First: 10.0.0.0/8
	prefix, _, ok := iter.Next()
	if !ok {
		t.Fatal("expected first withdrawn")
	}
	if len(prefix) != 2 {
		t.Errorf("first prefix len = %d, want 2", len(prefix))
	}

	// Second: 172.16.0.0/16
	prefix, _, ok = iter.Next()
	if !ok {
		t.Fatal("expected second withdrawn")
	}
	if len(prefix) != 3 {
		t.Errorf("second prefix len = %d, want 3", len(prefix))
	}

	// No more
	_, _, ok = iter.Next()
	if ok {
		t.Error("expected no more withdrawn")
	}
}

// TestWireUpdate_WithdrawnIteratorEmpty verifies empty withdrawn returns nil.
//
// VALIDATES: Empty withdrawn section returns nil iterator.
// PREVENTS: False error on announce-only UPDATE.
func TestWireUpdate_WithdrawnIteratorEmpty(t *testing.T) {
	payload := []byte{0x00, 0x00, 0x00, 0x00}

	wu := NewWireUpdate(payload, 0)

	iter, err := wu.WithdrawnIterator(false)
	if err != nil {
		t.Errorf("WithdrawnIterator() error = %v, want nil", err)
	}
	if iter != nil {
		t.Error("WithdrawnIterator() should return nil for empty withdrawn")
	}
}

// TestWireUpdate_AttrIterator verifies attribute iteration.
//
// VALIDATES: AttrIterator returns iterator over path attributes.
// PREVENTS: Incorrect attribute traversal.
func TestWireUpdate_AttrIterator(t *testing.T) {
	// Build UPDATE with ORIGIN + MED
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64, // MED 100
	}
	payload := make([]byte, 4+len(attrs))
	binary.BigEndian.PutUint16(payload[0:2], 0)                  // withdrawn len
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attrs))) //nolint:gosec // test
	copy(payload[4:], attrs)

	wu := NewWireUpdate(payload, 0)

	iter, err := wu.AttrIterator()
	if err != nil {
		t.Fatalf("AttrIterator() error = %v", err)
	}
	if iter == nil {
		t.Fatal("AttrIterator() returned nil")
	}

	// First: ORIGIN
	typeCode, _, value, ok := iter.Next()
	if !ok {
		t.Fatal("expected first attribute")
	}
	if typeCode != 1 { // ORIGIN
		t.Errorf("first typeCode = %d, want 1", typeCode)
	}
	if len(value) != 1 {
		t.Errorf("first len(value) = %d, want 1", len(value))
	}

	// Second: MED
	typeCode, _, value, ok = iter.Next()
	if !ok {
		t.Fatal("expected second attribute")
	}
	if typeCode != 4 { // MED
		t.Errorf("second typeCode = %d, want 4", typeCode)
	}
	if len(value) != 4 {
		t.Errorf("second len(value) = %d, want 4", len(value))
	}

	// No more
	_, _, _, ok = iter.Next()
	if ok {
		t.Error("expected no more attributes")
	}
}

// TestWireUpdate_AttrIteratorEmpty verifies empty attrs returns nil.
//
// VALIDATES: Empty attributes section returns nil iterator.
// PREVENTS: False error on withdraw-only UPDATE.
func TestWireUpdate_AttrIteratorEmpty(t *testing.T) {
	payload := []byte{0x00, 0x00, 0x00, 0x00}

	wu := NewWireUpdate(payload, 0)

	iter, err := wu.AttrIterator()
	if err != nil {
		t.Errorf("AttrIterator() error = %v, want nil", err)
	}
	if iter != nil {
		t.Error("AttrIterator() should return nil for empty attrs")
	}
}

// TestWireUpdate_CachedParsing verifies offsets are parsed once and reused.
//
// VALIDATES: Multiple accessor calls don't re-parse the UPDATE structure.
// PREVENTS: Performance regression from repeated offset calculation.
func TestWireUpdate_CachedParsing(t *testing.T) {
	// Build UPDATE with all sections
	withdrawn := []byte{0x10, 0x0a}
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	nlri := []byte{0x18, 0xc0, 0xa8, 0x01}

	payload := make([]byte, 2+len(withdrawn)+2+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(withdrawn))) //nolint:gosec // G115: test data
	copy(payload[2:], withdrawn)
	offset := 2 + len(withdrawn)
	binary.BigEndian.PutUint16(payload[offset:], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[offset+2:], attrs)
	copy(payload[offset+2+len(attrs):], nlri)

	wu := NewWireUpdate(payload, 0)

	// Call all three accessors multiple times
	// They should all succeed and return consistent data
	for i := range 3 {
		wd, err := wu.Withdrawn()
		if err != nil {
			t.Fatalf("Withdrawn() call %d error: %v", i, err)
		}
		if len(wd) != len(withdrawn) {
			t.Errorf("Withdrawn() call %d len = %d, want %d", i, len(wd), len(withdrawn))
		}

		at, err := wu.Attrs()
		if err != nil {
			t.Fatalf("Attrs() call %d error: %v", i, err)
		}
		if at == nil {
			t.Errorf("Attrs() call %d returned nil", i)
		}

		nl, err := wu.NLRI()
		if err != nil {
			t.Fatalf("NLRI() call %d error: %v", i, err)
		}
		if len(nl) != len(nlri) {
			t.Errorf("NLRI() call %d len = %d, want %d", i, len(nl), len(nlri))
		}
	}
}

// TestWireUpdate_CachedError verifies malformed payload error is cached.
//
// VALIDATES: Once parsing fails, subsequent calls return cached error.
// PREVENTS: Repeated parsing of known-bad payloads.
func TestWireUpdate_CachedError(t *testing.T) {
	// Malformed: claims 5 bytes withdrawn, only has 1
	payload := []byte{0x00, 0x05, 0x01}

	wu := NewWireUpdate(payload, 0)

	// First call should fail
	_, err1 := wu.Withdrawn()
	if err1 == nil {
		t.Fatal("Withdrawn() should fail on malformed payload")
	}
	if !errors.Is(err1, ErrUpdateTruncated) {
		t.Errorf("Withdrawn() error = %v, want ErrUpdateTruncated", err1)
	}

	// Second call should return same error (cached)
	_, err2 := wu.Attrs()
	if err2 == nil {
		t.Fatal("Attrs() should fail on malformed payload")
	}
	if !errors.Is(err2, ErrUpdateTruncated) {
		t.Errorf("Attrs() error = %v, want ErrUpdateTruncated", err2)
	}

	// Third call to different accessor should also return cached error
	_, err3 := wu.NLRI()
	if err3 == nil {
		t.Fatal("NLRI() should fail on malformed payload")
	}
	if !errors.Is(err3, ErrUpdateTruncated) {
		t.Errorf("NLRI() error = %v, want ErrUpdateTruncated", err3)
	}
}

// TestWireUpdate_IsEOR verifies End-of-RIB marker detection.
//
// VALIDATES: RFC 4724 Section 2 EOR detection for IPv4 unicast and multiprotocol families.
// PREVENTS: Missing EOR detection that would break graceful restart stale route purge.
func TestWireUpdate_IsEOR(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		wantEOR  bool
		wantAFI  uint16
		wantSAFI uint8
	}{
		{
			name:     "IPv4 unicast EOR (empty UPDATE)",
			payload:  []byte{0x00, 0x00, 0x00, 0x00},
			wantEOR:  true,
			wantAFI:  1,
			wantSAFI: 1,
		},
		{
			name: "IPv6 unicast EOR (MP_UNREACH_NLRI with AFI/SAFI only)",
			// WithdrawnLen=0, AttrLen=7, MP_UNREACH attr: flags=0x90 (opt+ext-len), code=15, len=3, AFI=2, SAFI=1
			payload: []byte{
				0x00, 0x00, // Withdrawn routes length = 0
				0x00, 0x07, // Total path attribute length = 7
				0x90, 0x0f, 0x00, 0x03, // Attr: optional, extended-length, code 15, length 3
				0x00, 0x02, // AFI = 2 (IPv6)
				0x01, // SAFI = 1 (unicast)
			},
			wantEOR:  true,
			wantAFI:  2,
			wantSAFI: 1,
		},
		{
			name: "not EOR: MP_UNREACH with actual withdrawn routes",
			// MP_UNREACH with AFI=2, SAFI=1, plus a withdrawn /64 prefix.
			payload: func() []byte {
				mpValue := []byte{
					0x00, 0x02, // AFI = 2
					0x01,                                                 // SAFI = 1
					0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00, // /64 prefix
				}
				attrs := []byte{0x80, 0x0f, byte(len(mpValue))}
				attrs = append(attrs, mpValue...)
				p := make([]byte, 4+len(attrs))
				binary.BigEndian.PutUint16(p[2:4], uint16(len(attrs))) //nolint:gosec // test
				copy(p[4:], attrs)
				return p
			}(),
			wantEOR: false,
		},
		{
			name: "not EOR: UPDATE with NLRI",
			// WithdrawnLen=0, AttrLen=0, NLRI=10.0.0.0/24
			payload: []byte{0x00, 0x00, 0x00, 0x00, 0x18, 0x0a, 0x00, 0x00},
			wantEOR: false,
		},
		{
			name: "not EOR: UPDATE with withdrawn routes",
			// WithdrawnLen=4, Withdrawn=10.0.0.0/24, AttrLen=0
			payload: []byte{0x00, 0x04, 0x18, 0x0a, 0x00, 0x00, 0x00, 0x00},
			wantEOR: false,
		},
		{
			name:    "not EOR: malformed payload",
			payload: []byte{0x00},
			wantEOR: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wu := NewWireUpdate(tt.payload, 0)
			family, isEOR := wu.IsEOR()
			if isEOR != tt.wantEOR {
				t.Errorf("IsEOR() = %v, want %v", isEOR, tt.wantEOR)
			}
			if tt.wantEOR {
				if family.AFI != nlri.AFI(tt.wantAFI) {
					t.Errorf("IsEOR() AFI = %d, want %d", family.AFI, tt.wantAFI)
				}
				if family.SAFI != nlri.SAFI(tt.wantSAFI) {
					t.Errorf("IsEOR() SAFI = %d, want %d", family.SAFI, tt.wantSAFI)
				}
			}
		})
	}
}
