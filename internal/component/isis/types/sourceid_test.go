package types

import (
	"bytes"
	"testing"
)

// VALIDATES: SourceID composes SystemID + pseudonode and round-trips its string.
// PREVENTS: pseudonode byte being lost or swapped in format/parse.
func TestSourceIDParseFormatRoundTrip(t *testing.T) {
	cases := []struct {
		str  string
		sys  SystemID
		pnid uint8
	}{
		{"0001.0002.0003.00", SystemID{0, 1, 0, 2, 0, 3}, 0},
		{"0001.0002.0003.07", SystemID{0, 1, 0, 2, 0, 3}, 7},
		{"ffff.ffff.ffff.ff", SystemID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 0xff},
	}
	for _, c := range cases {
		id, err := ParseSourceID(c.str)
		if err != nil {
			t.Fatalf("ParseSourceID(%q) error: %v", c.str, err)
		}
		if id.SystemID() != c.sys {
			t.Errorf("%q SystemID = %v, want %v", c.str, id.SystemID(), c.sys)
		}
		if id.PseudonodeID() != c.pnid {
			t.Errorf("%q PseudonodeID = %d, want %d", c.str, id.PseudonodeID(), c.pnid)
		}
		if got := id.String(); got != c.str {
			t.Errorf("round-trip %q -> %q", c.str, got)
		}
	}
}

// VALIDATES: pseudonode 0 marks a router, non-zero marks a LAN pseudonode.
// PREVENTS: misclassifying pseudonode LSPs (isis-8 relies on this).
func TestSourceIDPseudonode(t *testing.T) {
	router := NewSourceID(SystemID{0, 1, 0, 2, 0, 3}, 0)
	if router.IsPseudonode() {
		t.Error("pseudonode ID 0 must be a router, not a pseudonode")
	}
	lan := NewSourceID(SystemID{0, 1, 0, 2, 0, 3}, 1)
	if !lan.IsPseudonode() {
		t.Error("pseudonode ID != 0 must be a pseudonode")
	}
}

// VALIDATES: SourceIDFromBytes accepts exactly 7 bytes; WriteTo reproduces them.
// PREVENTS: wrong-length acceptance (boundary 6 and 8) and serialize asymmetry.
func TestSourceIDBytesRoundTrip(t *testing.T) {
	src := []byte{0, 1, 0, 2, 0, 3, 0x05}
	id, err := SourceIDFromBytes(src)
	if err != nil {
		t.Fatalf("SourceIDFromBytes error: %v", err)
	}
	var buf [16]byte
	n := id.WriteTo(buf[:], 0)
	if n != SourceIDLen || !bytes.Equal(buf[:n], src) {
		t.Fatalf("WriteTo = %x (n=%d), want %x", buf[:n], n, src)
	}
	for _, l := range []int{6, 8} {
		if _, err := SourceIDFromBytes(make([]byte, l)); err == nil {
			t.Errorf("SourceIDFromBytes(len=%d) should error", l)
		}
	}
}
