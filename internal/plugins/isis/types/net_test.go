package types

import (
	"bytes"
	"testing"
)

// VALIDATES: ParseNET decodes "49.0001.0000.0000.0001.00" into AreaID/SystemID/
// SEL and round-trips the canonical string (AC-4).
// PREVENTS: the area/system/SEL split (last 7 bytes = SystemID+SEL) being wrong.
func TestNETParseFormatRoundTrip(t *testing.T) {
	cases := []struct {
		str  string
		area []byte
		sys  SystemID
		sel  uint8
	}{
		{
			"49.0001.0000.0000.0001.00",
			[]byte{0x49, 0x00, 0x01},
			SystemID{0, 0, 0, 0, 0, 1},
			0x00,
		},
		{
			// Minimal: 1-byte area, total 8 bytes.
			"49.0000.0000.0001.00",
			[]byte{0x49},
			SystemID{0, 0, 0, 0, 0, 1},
			0x00,
		},
	}
	for _, c := range cases {
		n, err := ParseNET(c.str)
		if err != nil {
			t.Fatalf("ParseNET(%q) error: %v", c.str, err)
		}
		if !bytes.Equal(n.AreaID().Bytes(), c.area) {
			t.Errorf("%q AreaID = %x, want %x", c.str, n.AreaID().Bytes(), c.area)
		}
		if n.SystemID() != c.sys {
			t.Errorf("%q SystemID = %v, want %v", c.str, n.SystemID(), c.sys)
		}
		if n.SEL() != c.sel {
			t.Errorf("%q SEL = %#x, want %#x", c.str, n.SEL(), c.sel)
		}
		if got := n.String(); got != c.str {
			t.Errorf("round-trip %q -> %q", c.str, got)
		}
	}
}

// VALIDATES: NETFromBytes parses correctly over the full AreaID range 1..13
// bytes (total 8..20) and the accessors return the right slices (AC-5).
// PREVENTS: off-by-one in the AreaID / SystemID / SEL split at the boundaries.
func TestNETAccessors(t *testing.T) {
	for areaLen := 1; areaLen <= MaxAreaIDLen; areaLen++ {
		raw := make([]byte, 0, areaLen+SystemIDLen+1)
		for i := range areaLen {
			raw = append(raw, byte(0x40+i))
		}
		sys := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
		raw = append(raw, sys...)
		raw = append(raw, 0x00) // SEL
		n, err := nETFromBytes(raw)
		if err != nil {
			t.Fatalf("NETFromBytes(areaLen=%d) error: %v", areaLen, err)
		}
		if got := n.AreaID().Bytes(); !bytes.Equal(got, raw[:areaLen]) {
			t.Errorf("areaLen=%d AreaID = %x, want %x", areaLen, got, raw[:areaLen])
		}
		gotSys := n.SystemID()
		if !bytes.Equal(gotSys[:], sys) {
			t.Errorf("areaLen=%d SystemID = %x, want %x", areaLen, gotSys[:], sys)
		}
		if n.SEL() != 0 {
			t.Errorf("areaLen=%d SEL = %#x, want 0", areaLen, n.SEL())
		}
	}
}

// VALIDATES: NETFromBytes rejects total length 7 (below 8) and 21 (above 20) (AC-6).
// PREVENTS: out-of-bound area sizes; resource exhaustion from a bad length.
func TestNETBytesBounds(t *testing.T) {
	if _, err := nETFromBytes(make([]byte, MinNETLen-1)); err == nil {
		t.Errorf("NETFromBytes(len=%d) should error (below min)", MinNETLen-1)
	}
	if _, err := nETFromBytes(make([]byte, MaxNETLen+1)); err == nil {
		t.Errorf("NETFromBytes(len=%d) should error (above max)", MaxNETLen+1)
	}
	if _, err := nETFromBytes(make([]byte, MinNETLen)); err != nil {
		t.Errorf("NETFromBytes(len=%d) should succeed: %v", MinNETLen, err)
	}
	if _, err := nETFromBytes(make([]byte, MaxNETLen)); err != nil {
		t.Errorf("NETFromBytes(len=%d) should succeed: %v", MaxNETLen, err)
	}
}

// VALIDATES: NET WriteTo reproduces the exact input octets (AC-11).
// PREVENTS: serialize/parse asymmetry for the variable-length NET.
func TestNETWriteToRoundTrip(t *testing.T) {
	raw := []byte{0x49, 0x00, 0x01, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00}
	n, err := nETFromBytes(raw)
	if err != nil {
		t.Fatalf("NETFromBytes error: %v", err)
	}
	buf := make([]byte, 32)
	w := n.WriteTo(buf, 0)
	if w != len(raw) || !bytes.Equal(buf[:w], raw) {
		t.Fatalf("WriteTo = %x (n=%d), want %x", buf[:w], w, raw)
	}
}

// VALIDATES: ParseNET rejects malformed input (odd nibble, bad digit, too short,
// too long).
// PREVENTS: silent corruption from malformed NET config strings (R-4).
func TestNETParseRejects(t *testing.T) {
	for _, in := range []string{
		"",
		"49.0000.0000.000",     // odd nibble in a group
		"49.0000.0000.00zz.00", // bad digit
		"0000.0000.0001.00",    // 7 bytes total (below min)
		"49.00.00.00.00.00.00.00.00.00.00.00.00.00.0000.0000.0001.00", // too long
	} {
		if _, err := ParseNET(in); err == nil {
			t.Errorf("ParseNET(%q) should error", in)
		}
	}
}

// VALIDATES: AreaID equality and ordering for variable-length values, including
// the equal-prefix-different-length case (AC-10, gates L1 adjacency).
// PREVENTS: CSNP/area-match bugs from wrong length-first vs lexicographic order (R-1).
func TestAreaIDEqualAndOrder(t *testing.T) {
	a := areaIDFromBytesUnchecked([]byte{0x49, 0x00, 0x01})
	aCopy := areaIDFromBytesUnchecked([]byte{0x49, 0x00, 0x01})
	b := areaIDFromBytesUnchecked([]byte{0x49, 0x00, 0x02})
	shorter := areaIDFromBytesUnchecked([]byte{0x49, 0x00}) // prefix of a
	one := areaIDFromBytesUnchecked([]byte{0x49})           // 1-byte boundary
	thirteen := areaIDFromBytesUnchecked(bytes.Repeat([]byte{0x49}, MaxAreaIDLen))

	if !a.Equal(aCopy) {
		t.Error("identical areas must be Equal")
	}
	if a.Equal(b) {
		t.Error("different areas must not be Equal")
	}
	if a.Compare(aCopy) != 0 {
		t.Error("Compare of equal areas must be 0")
	}
	if a.Compare(b) >= 0 {
		t.Error("0x490001 must sort before 0x490002")
	}
	// Equal prefix, different length: the shorter (prefix) sorts first
	// (bytes.Compare semantics: a is a prefix of b means a < b).
	if shorter.Compare(a) >= 0 {
		t.Error("shorter equal-prefix area must sort before the longer one")
	}
	if one.Len() != 1 || thirteen.Len() != MaxAreaIDLen {
		t.Errorf("boundary lengths: got %d and %d", one.Len(), thirteen.Len())
	}
}
