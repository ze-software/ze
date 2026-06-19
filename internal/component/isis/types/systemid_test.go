package types

import (
	"bytes"
	"testing"
)

// VALIDATES: ParseSystemID round-trips the canonical dotted-hex form (AC-1).
// PREVENTS: format/parse drift for the 6-byte router identifier.
func TestSystemIDParseFormatRoundTrip(t *testing.T) {
	cases := []string{
		"0001.0002.0003",
		"0000.0000.0000",
		"ffff.ffff.ffff",
		"1921.6800.0001",
	}
	for _, in := range cases {
		id, err := ParseSystemID(in)
		if err != nil {
			t.Fatalf("ParseSystemID(%q) error: %v", in, err)
		}
		if got := id.String(); got != in {
			t.Errorf("round-trip: ParseSystemID(%q).String() = %q", in, got)
		}
	}
}

// VALIDATES: SystemIDFromBytes accepts exactly 6 bytes; WriteTo reproduces them (AC-2, AC-11).
// PREVENTS: wrong-length acceptance and serialize/parse asymmetry.
func TestSystemIDBytesRoundTrip(t *testing.T) {
	src := []byte{0x00, 0x01, 0x00, 0x02, 0x00, 0x03}
	id, err := SystemIDFromBytes(src)
	if err != nil {
		t.Fatalf("SystemIDFromBytes error: %v", err)
	}
	var buf [16]byte
	n := id.WriteTo(buf[:], 0)
	if n != SystemIDLen {
		t.Fatalf("WriteTo returned %d, want %d", n, SystemIDLen)
	}
	if !bytes.Equal(buf[:n], src) {
		t.Errorf("WriteTo produced %x, want %x", buf[:n], src)
	}
}

// VALIDATES: SystemIDFromBytes rejects lengths != 6 (boundary: 5 and 7).
// PREVENTS: out-of-range index / partial value leak from attacker wire input.
func TestSystemIDBytesLength(t *testing.T) {
	for _, n := range []int{0, 5, 7, 8} {
		if _, err := SystemIDFromBytes(make([]byte, n)); err == nil {
			t.Errorf("SystemIDFromBytes(len=%d) should error", n)
		}
	}
	if _, err := SystemIDFromBytes(make([]byte, 6)); err != nil {
		t.Errorf("SystemIDFromBytes(len=6) should succeed, got %v", err)
	}
}

// VALIDATES: ParseSystemID rejects wrong group count and malformed input.
// PREVENTS: silent corruption from malformed operator/config strings (R-4).
func TestSystemIDParseRejects(t *testing.T) {
	for _, in := range []string{
		"",
		"0001.0002",           // too short
		"0001.0002.0003.0004", // too long
		"0001.0002.000",       // odd nibble in last group
		"00g1.0002.0003",      // bad digit
		"000100020003",        // no separators -> still 6 bytes? must reject as not 3 groups? actually valid hex 6 bytes
	} {
		if _, err := ParseSystemID(in); err == nil {
			t.Errorf("ParseSystemID(%q) should error", in)
		}
	}
}

// VALIDATES: SystemID is usable as a comparable map key (AC-12).
// PREVENTS: pointer-identity surprises when keying adjacency / LSDB tables.
func TestSystemIDMapKey(t *testing.T) {
	m := map[SystemID]int{}
	a := SystemID{0, 1, 0, 2, 0, 3}
	b := SystemID{0, 1, 0, 2, 0, 3}
	m[a] = 7
	if m[b] != 7 {
		t.Error("equal SystemIDs must hash to the same map entry")
	}
	if a != b {
		t.Error("equal SystemIDs must compare equal with ==")
	}
}
