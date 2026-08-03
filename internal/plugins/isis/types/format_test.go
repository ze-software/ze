package types

import "testing"

// VALIDATES: AppendTo for each fixed-width IS-IS identifier is zero-allocation
// (the buffer-first hot-path primitive) and String() costs at most the single
// unavoidable result copy, never the 2+ allocations fmt.Sprintf would incur.
// PREVENTS: per-call heap churn on CLI list / log hot paths (spec risk R-3).
//
// String() returns an owned string, so the result copy is one allocation by
// definition (ai/rules/performance.md "Tier 1: one allocation"). Hot paths
// use AppendTo into a caller-owned buffer, which is asserted zero-alloc here.
func TestStringNoAlloc(t *testing.T) {
	sysID := SystemID{0x00, 0x01, 0x00, 0x02, 0x00, 0x03}
	srcID := SourceID{0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00}
	lspID := LSPID{0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x01}

	cases := []struct {
		name     string
		appendTo func([]byte) []byte
		str      func() string
	}{
		{"SystemID", sysID.AppendTo, sysID.String},
		{"SourceID", srcID.AppendTo, srcID.String},
		{"LSPID", lspID.AppendTo, lspID.String},
	}
	for _, c := range cases {
		t.Run(c.name+"/AppendTo", func(t *testing.T) {
			var scratch [32]byte
			allocs := testing.AllocsPerRun(100, func() {
				_ = c.appendTo(scratch[:0])
			})
			if allocs != 0 {
				t.Errorf("%s.AppendTo allocated %v times per run, want 0", c.name, allocs)
			}
		})
		t.Run(c.name+"/String", func(t *testing.T) {
			var sink string
			allocs := testing.AllocsPerRun(100, func() {
				sink = c.str()
			})
			_ = sink
			if allocs > 1 {
				t.Errorf("%s.String() allocated %v times per run, want at most 1", c.name, allocs)
			}
		})
	}
}

// VALIDATES: appendDottedHex groups octets in pairs with '.' separators.
// PREVENTS: malformed canonical formatting (wrong grouping, missing dots).
//
// TestAppendDottedHex covers the shared formatting helper across even and odd
// byte counts.
func TestAppendDottedHex(t *testing.T) {
	cases := []struct {
		in   []byte
		want string
	}{
		{[]byte{0x00, 0x01, 0x00, 0x02, 0x00, 0x03}, "0001.0002.0003"},
		{[]byte{0x49, 0x00, 0x01}, "4900.01"},
		{[]byte{0x49}, "49"},
		{nil, ""},
	}
	for _, c := range cases {
		got := string(appendDottedHex(nil, c.in))
		if got != c.want {
			t.Errorf("appendDottedHex(%x) = %q, want %q", c.in, got, c.want)
		}
	}
}

// VALIDATES: parseDottedHex rejects odd nibbles, bad digits, and length mismatch.
// PREVENTS: silent corruption from malformed dotted-hex input (spec risk R-4).
//
// TestParseDottedHexErrors covers the rejection paths of the shared parse
// helper (R-4: malformed dotted-hex must error, not silently corrupt).
func TestParseDottedHexErrors(t *testing.T) {
	t.Run("odd nibble", func(t *testing.T) {
		var dst [3]byte
		if err := parseDottedHex(dst[:], "490.0001"); err == nil {
			t.Fatal("expected error for odd-nibble group")
		}
	})
	t.Run("bad digit", func(t *testing.T) {
		var dst [2]byte
		if err := parseDottedHex(dst[:], "00gg"); err == nil {
			t.Fatal("expected error for non-hex digit")
		}
	})
	t.Run("too short", func(t *testing.T) {
		var dst [6]byte
		if err := parseDottedHex(dst[:], "0001.0002"); err == nil {
			t.Fatal("expected ErrWrongLength for short input")
		}
	})
	t.Run("too long", func(t *testing.T) {
		var dst [2]byte
		if err := parseDottedHex(dst[:], "0001.0002.0003"); err == nil {
			t.Fatal("expected ErrWrongLength for long input")
		}
	})
	t.Run("exact", func(t *testing.T) {
		var dst [6]byte
		if err := parseDottedHex(dst[:], "0001.0002.0003"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := [6]byte{0x00, 0x01, 0x00, 0x02, 0x00, 0x03}
		if dst != want {
			t.Errorf("got %x, want %x", dst, want)
		}
	})
}
