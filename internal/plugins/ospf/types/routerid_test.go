// Design: docs/architecture/ospf/ospf-1-types.md -- RouterID parse, format, and wire tests

package types

import (
	"bytes"
	"testing"
)

// VALIDATES: AC-1 - ParseRouterID returns a 4-byte value whose String and bytes round-trip.
// PREVENTS: storing Router IDs as slice-backed net.IP values with pointer identity surprises.
func TestRouterIDParseFormatRoundTrip(t *testing.T) {
	id, err := ParseRouterID("10.0.0.1")
	if err != nil {
		t.Fatalf("ParseRouterID returned error: %v", err)
	}
	if got := id.String(); got != "10.0.0.1" {
		t.Fatalf("RouterID.String() = %q, want 10.0.0.1", got)
	}
	fromBytes, err := RouterIDFromBytes([]byte{10, 0, 0, 1})
	if err != nil {
		t.Fatalf("RouterIDFromBytes returned error: %v", err)
	}
	if id != fromBytes {
		t.Fatalf("RouterID round-trip mismatch: %v != %v", id, fromBytes)
	}
	var buf [8]byte
	if n := id.WriteTo(buf[:], 2); n != RouterIDLen {
		t.Fatalf("RouterID.WriteTo wrote %d bytes, want %d", n, RouterIDLen)
	}
	if got := buf[2:6]; !bytes.Equal(got, []byte{10, 0, 0, 1}) {
		t.Fatalf("RouterID.WriteTo bytes = %v, want [10 0 0 1]", got)
	}
}

// VALIDATES: AC-2 - RouterIDFromBytes rejects lengths other than 4.
// PREVENTS: attacker-controlled wire lengths causing partial values or slice panics.
func TestRouterIDFromBytesRejectsWrongLength(t *testing.T) {
	for _, input := range [][]byte{{1, 2, 3}, {1, 2, 3, 4, 5}} {
		if _, err := RouterIDFromBytes(input); err == nil {
			t.Fatalf("RouterIDFromBytes(%v) succeeded, want error", input)
		}
	}
}

// VALIDATES: AC-1 - RouterID.Equal reports value identity for the comparable 4-byte array.
// PREVENTS: a byte-level or pointer comparison that treats two equal Router IDs as distinct.
func TestRouterIDEqual(t *testing.T) {
	a := RouterID{10, 0, 0, 1}
	same := RouterID{10, 0, 0, 1}
	diff := RouterID{10, 0, 0, 2}
	if !a.Equal(same) {
		t.Errorf("RouterID.Equal(identical) = false, want true")
	}
	if a.Equal(diff) {
		t.Errorf("RouterID.Equal(%v,%v) = true, want false", a, diff)
	}
	// Every octet position must matter.
	for i := range a {
		other := a
		other[i] ^= 0xff
		if a.Equal(other) {
			t.Errorf("RouterID.Equal ignored octet %d: %v vs %v", i, a, other)
		}
	}
}
