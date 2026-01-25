package nlri

import (
	"bytes"
	"testing"
)

// TestNewWireNLRI verifies constructor creates WireNLRI.
//
// VALIDATES: Constructor stores family, data, hasAddPath.
// PREVENTS: Missing initialization.
func TestNewWireNLRI(t *testing.T) {
	data := []byte{24, 10, 0, 0} // 10.0.0.0/24
	w, err := NewWireNLRI(IPv4Unicast, data, false)
	if err != nil {
		t.Fatalf("NewWireNLRI: %v", err)
	}
	if w == nil {
		t.Fatal("NewWireNLRI returned nil")
	}
	if w.Family() != IPv4Unicast {
		t.Errorf("Family: want %s, got %s", IPv4Unicast, w.Family())
	}
}

// TestWireNLRI_Family verifies Family() returns correct value.
//
// VALIDATES: Family getter works.
// PREVENTS: Wrong family returned.
func TestWireNLRI_Family(t *testing.T) {
	tests := []struct {
		family Family
	}{
		{IPv4Unicast},
		{IPv6Unicast},
		{IPv4VPN},
	}
	for _, tt := range tests {
		w, _ := NewWireNLRI(tt.family, []byte{24, 10, 0, 0}, false)
		if w.Family() != tt.family {
			t.Errorf("Family: want %s, got %s", tt.family, w.Family())
		}
	}
}

// TestWireNLRI_Bytes_NoAddPath verifies Bytes() returns full data.
//
// VALIDATES: Bytes returns data without path-id prefix.
// PREVENTS: Truncation or modification.
func TestWireNLRI_Bytes_NoAddPath(t *testing.T) {
	data := []byte{24, 10, 0, 0} // 10.0.0.0/24
	w, _ := NewWireNLRI(IPv4Unicast, data, false)
	if !bytes.Equal(w.Bytes(), data) {
		t.Errorf("Bytes: want %x, got %x", data, w.Bytes())
	}
}

// TestWireNLRI_Bytes_WithAddPath verifies Bytes() returns full data including path-id.
//
// VALIDATES: Bytes returns full data with path-id prefix.
// PREVENTS: Path-id being stripped when not requested.
func TestWireNLRI_Bytes_WithAddPath(t *testing.T) {
	// path-id=1 + 10.0.0.0/24
	data := []byte{0, 0, 0, 1, 24, 10, 0, 0}
	w, _ := NewWireNLRI(IPv4Unicast, data, true)
	if !bytes.Equal(w.Bytes(), data) {
		t.Errorf("Bytes: want %x, got %x", data, w.Bytes())
	}
}

// TestWireNLRI_Len verifies Len() returns payload length (without path-id).
//
// VALIDATES: Len returns payload byte count per NLRI interface contract.
// PREVENTS: Wrong length calculation.
func TestWireNLRI_Len(t *testing.T) {
	tests := []struct {
		data   []byte
		addp   bool
		expect int
	}{
		{[]byte{24, 10, 0, 0}, false, 4},            // No path-id: payload = 4
		{[]byte{0, 0, 0, 1, 24, 10, 0, 0}, true, 4}, // With path-id: payload = 8 - 4 = 4
	}
	for _, tt := range tests {
		w, _ := NewWireNLRI(IPv4Unicast, tt.data, tt.addp)
		if w.Len() != tt.expect {
			t.Errorf("Len: want %d, got %d", tt.expect, w.Len())
		}
	}
}

// TestWireNLRI_PathID_NoAddPath verifies PathID() returns 0 when !hasAddPath.
//
// VALIDATES: PathID returns 0 for non-ADD-PATH NLRI.
// PREVENTS: Incorrect path-id extraction.
func TestWireNLRI_PathID_NoAddPath(t *testing.T) {
	data := []byte{24, 10, 0, 0}
	w, _ := NewWireNLRI(IPv4Unicast, data, false)
	if w.PathID() != 0 {
		t.Errorf("PathID: want 0, got %d", w.PathID())
	}
}

// TestWireNLRI_PathID_WithAddPath verifies PathID() extracts from data when hasAddPath.
//
// VALIDATES: PathID extracts 4-byte path-id from data.
// PREVENTS: Wrong path-id value.
func TestWireNLRI_PathID_WithAddPath(t *testing.T) {
	// path-id=258 (0x00000102) + 10.0.0.0/24
	data := []byte{0, 0, 1, 2, 24, 10, 0, 0}
	w, _ := NewWireNLRI(IPv4Unicast, data, true)
	if w.PathID() != 258 {
		t.Errorf("PathID: want 258, got %d", w.PathID())
	}
}

// TestWireNLRI_WriteNLRI_NoMismatch verifies WriteNLRI returns data when no mismatch.
//
// VALIDATES: WriteNLRI passes through when source and target match.
// PREVENTS: Unnecessary modification.
func TestWireNLRI_WriteNLRI_NoMismatch(t *testing.T) {
	data := []byte{24, 10, 0, 0}
	w, _ := NewWireNLRI(IPv4Unicast, data, false)

	// no ADD-PATH, matches source
	buf := make([]byte, 100)
	n := WriteNLRI(w, buf, 0, false)
	if !bytes.Equal(buf[:n], data) {
		t.Errorf("WriteNLRI(false): want %x, got %x", data, buf[:n])
	}
}

// TestWireNLRI_WriteNLRI_StripPathID verifies WriteNLRI strips path-id when src has, target doesn't.
//
// VALIDATES: RFC 7911 path-id stripping works.
// PREVENTS: Path-id leaked to non-ADD-PATH peer.
func TestWireNLRI_WriteNLRI_StripPathID(t *testing.T) {
	// path-id=1 + 10.0.0.0/24
	data := []byte{0, 0, 0, 1, 24, 10, 0, 0}
	w, _ := NewWireNLRI(IPv4Unicast, data, true)

	// Target has no ADD-PATH -> strip path-id
	buf := make([]byte, 100)
	n := WriteNLRI(w, buf, 0, false)
	expect := []byte{24, 10, 0, 0}
	if !bytes.Equal(buf[:n], expect) {
		t.Errorf("WriteNLRI(false): want %x, got %x", expect, buf[:n])
	}
}

// TestWireNLRI_WriteNLRI_PrependNOPATH verifies WriteNLRI prepends NOPATH when src lacks, target expects.
//
// VALIDATES: RFC 7911 NOPATH prepending works.
// PREVENTS: Missing path-id for ADD-PATH peer.
func TestWireNLRI_WriteNLRI_PrependNOPATH(t *testing.T) {
	data := []byte{24, 10, 0, 0}
	w, _ := NewWireNLRI(IPv4Unicast, data, false)

	// Target expects ADD-PATH -> prepend NOPATH (0x00000000)
	buf := make([]byte, 100)
	n := WriteNLRI(w, buf, 0, true)
	expect := []byte{0, 0, 0, 0, 24, 10, 0, 0}
	if !bytes.Equal(buf[:n], expect) {
		t.Errorf("WriteNLRI(true): want %x, got %x", expect, buf[:n])
	}
}

// TestNewWireNLRI_Malformed verifies constructor returns error when hasAddPath but len < 4.
//
// VALIDATES: Constructor validates data length for ADD-PATH.
// PREVENTS: Panic on short data.
func TestNewWireNLRI_Malformed(t *testing.T) {
	// Too short for ADD-PATH (need at least 4 bytes for path-id)
	data := []byte{24, 10}
	_, err := NewWireNLRI(IPv4Unicast, data, true)
	if err == nil {
		t.Error("NewWireNLRI: expected error for short data with addpath")
	}
}

// TestWireNLRI_WriteTo verifies WriteTo copies packed data to buffer.
//
// VALIDATES: WriteTo writes correct bytes at offset.
// PREVENTS: Wrong data or offset handling.
func TestWireNLRI_WriteTo(t *testing.T) {
	data := []byte{24, 10, 0, 0}
	w, _ := NewWireNLRI(IPv4Unicast, data, false)

	buf := make([]byte, 10)
	n := w.WriteTo(buf, 2)
	if n != 4 {
		t.Errorf("WriteTo: want 4 bytes, wrote %d", n)
	}
	if !bytes.Equal(buf[2:6], data) {
		t.Errorf("WriteTo: want %x at offset 2, got %x", data, buf[2:6])
	}
}

// TestWireNLRI_String verifies String() returns readable format.
//
// VALIDATES: String returns meaningful representation.
// PREVENTS: Panic or empty string.
func TestWireNLRI_String(t *testing.T) {
	data := []byte{24, 10, 0, 0}
	w, _ := NewWireNLRI(IPv4Unicast, data, false)
	s := w.String()
	if s == "" {
		t.Error("String: returned empty")
	}
	// Should contain family and length
	if len(s) < 10 {
		t.Errorf("String: too short: %q", s)
	}
}
