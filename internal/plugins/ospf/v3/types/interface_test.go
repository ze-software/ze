// VALIDATES: spec-ospfv3-1-types AC-3 -- InterfaceID serializes as a 32-bit value;
// the zero-policy predicate distinguishes an active interface (non-zero) from the
// placeholder zero.
// PREVENTS: an Interface ID truncated below 32 bits or a zero accepted as an active link.
package types

import "testing"

func TestOSPFv3InterfaceIDBoundaries(t *testing.T) {
	from, err := InterfaceIDFromBytes([]byte{0x00, 0x00, 0x01, 0x00})
	if err != nil || from != InterfaceID(256) {
		t.Fatalf("InterfaceIDFromBytes = %d, %v", from, err)
	}
	if _, err := InterfaceIDFromBytes([]byte{1, 2, 3}); err == nil {
		t.Error("InterfaceIDFromBytes accepted 3 bytes")
	}

	buf := make([]byte, 4)
	if n := InterfaceID(256).WriteTo(buf, 0); n != 4 {
		t.Fatalf("WriteTo = %d, want 4", n)
	}
	if buf[2] != 0x01 || buf[3] != 0x00 {
		t.Errorf("WriteTo bytes = %v", buf)
	}

	if InterfaceID(0).IsActive() {
		t.Error("zero Interface ID reported active")
	}
	if !InterfaceID(1).IsActive() {
		t.Error("non-zero Interface ID reported inactive")
	}
}
