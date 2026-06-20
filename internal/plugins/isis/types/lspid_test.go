package types

import (
	"bytes"
	"testing"
)

// VALIDATES: ParseLSPID decodes "0001.0002.0003.00-01" into SystemID/pseudonode/
// LSP number and round-trips the string (AC-3).
// PREVENTS: the '-' separator or LSP-number byte being mis-parsed.
func TestLSPIDParseFormatRoundTrip(t *testing.T) {
	cases := []struct {
		str    string
		sys    SystemID
		pnid   uint8
		lspNum uint8
	}{
		{"0001.0002.0003.00-01", SystemID{0, 1, 0, 2, 0, 3}, 0, 1},
		{"0001.0002.0003.00-00", SystemID{0, 1, 0, 2, 0, 3}, 0, 0},
		{"0001.0002.0003.02-ff", SystemID{0, 1, 0, 2, 0, 3}, 2, 0xff},
		{"ffff.ffff.ffff.ff-ff", SystemID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 0xff, 0xff},
	}
	for _, c := range cases {
		id, err := ParseLSPID(c.str)
		if err != nil {
			t.Fatalf("ParseLSPID(%q) error: %v", c.str, err)
		}
		if id.SystemID() != c.sys {
			t.Errorf("%q SystemID = %v want %v", c.str, id.SystemID(), c.sys)
		}
		if id.PseudonodeID() != c.pnid {
			t.Errorf("%q pnid = %d want %d", c.str, id.PseudonodeID(), c.pnid)
		}
		if id.LSPNumber() != c.lspNum {
			t.Errorf("%q lspNum = %d want %d", c.str, id.LSPNumber(), c.lspNum)
		}
		if got := id.String(); got != c.str {
			t.Errorf("round-trip %q -> %q", c.str, got)
		}
	}
}

// VALIDATES: LSPIDFromBytes accepts exactly 8 bytes; WriteTo reproduces them (AC-11).
// PREVENTS: wrong-length acceptance (boundary 7 and 9) and serialize asymmetry.
func TestLSPIDBytesRoundTrip(t *testing.T) {
	src := []byte{0, 1, 0, 2, 0, 3, 0x02, 0x01}
	id, err := LSPIDFromBytes(src)
	if err != nil {
		t.Fatalf("LSPIDFromBytes error: %v", err)
	}
	var buf [16]byte
	n := id.WriteTo(buf[:], 0)
	if n != LSPIDLen || !bytes.Equal(buf[:n], src) {
		t.Fatalf("WriteTo = %x (n=%d) want %x", buf[:n], n, src)
	}
	for _, l := range []int{7, 9} {
		if _, err := LSPIDFromBytes(make([]byte, l)); err == nil {
			t.Errorf("LSPIDFromBytes(len=%d) should error", l)
		}
	}
}

// VALIDATES: LSPID total order matches big-endian byte order and is
// equality-consistent; usable to bound a CSNP start/end LSPID range (AC-10).
// PREVENTS: CSNP range bounding errors (spec risk R-1) from wrong ordering.
func TestLSPIDOrder(t *testing.T) {
	mk := func(sys [6]byte, pnid, num uint8) LSPID {
		return NewLSPID(NewSourceID(SystemID(sys), pnid), num)
	}
	a := mk([6]byte{0, 0, 0, 0, 0, 1}, 0, 0)
	b := mk([6]byte{0, 0, 0, 0, 0, 1}, 0, 1) // higher LSP number
	c := mk([6]byte{0, 0, 0, 0, 0, 1}, 1, 0) // higher pseudonode
	d := mk([6]byte{0, 0, 0, 0, 0, 2}, 0, 0) // higher system id

	aCopy := a
	if a.Compare(aCopy) != 0 {
		t.Error("Compare(equal) must be 0")
	}
	if a.Compare(b) >= 0 {
		t.Error("lower LSP number must sort before higher")
	}
	if b.Compare(c) >= 0 {
		t.Error("lower pseudonode must sort before higher (more significant)")
	}
	if c.Compare(d) >= 0 {
		t.Error("lower system id must sort before higher (most significant)")
	}
	// Range bounding: every id between start and end inclusive must compare in
	// [start, end].
	start, end := a, d
	for _, id := range []LSPID{a, b, c, d} {
		if id.Compare(start) < 0 || id.Compare(end) > 0 {
			t.Errorf("%s should be within CSNP range [%s, %s]", id, start, end)
		}
	}
	if a.Less(b) != true || b.Less(a) != false {
		t.Error("Less must agree with Compare")
	}
}

// VALIDATES: LSPID parse rejects a missing or malformed LSP-number separator.
// PREVENTS: accepting "0001.0002.0003.00" (a SourceID) as an LSPID.
func TestLSPIDParseRejects(t *testing.T) {
	for _, in := range []string{
		"",
		"0001.0002.0003.00",    // no LSP number
		"0001.0002.0003.00-",   // empty LSP number
		"0001.0002.0003.00-1",  // odd nibble LSP number
		"0001.0002.0003-00-01", // separator in wrong place
		"0001.0002.0003.00-zz", // bad LSP number digit
	} {
		if _, err := ParseLSPID(in); err == nil {
			t.Errorf("ParseLSPID(%q) should error", in)
		}
	}
}

// VALIDATES: LSPID is usable as a comparable map key (AC-12, isis-6 LSDB index).
// PREVENTS: pointer-identity surprises when keying the LSDB.
func TestLSPIDMapKey(t *testing.T) {
	m := map[LSPID]int{}
	a := NewLSPID(NewSourceID(SystemID{0, 1, 0, 2, 0, 3}, 0), 1)
	b := NewLSPID(NewSourceID(SystemID{0, 1, 0, 2, 0, 3}, 0), 1)
	m[a] = 9
	if m[b] != 9 || a != b {
		t.Error("equal LSPIDs must be == and key the same map entry")
	}
}
