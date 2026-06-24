// Design: plan/learned/955-ospf-1-types.md -- LinkStateID parse, format, and wire tests

package types

import (
	"bytes"
	"testing"
)

// VALIDATES: AC-12 - LinkStateID parse, format, bytes, and WriteTo round-trip.
// PREVENTS: LSA header parsing from treating the type-specific ID as a string or slice.
func TestLinkStateIDRoundTrip(t *testing.T) {
	id, err := ParseLinkStateID("192.0.2.7")
	if err != nil {
		t.Fatalf("ParseLinkStateID returned error: %v", err)
	}
	if got := id.String(); got != "192.0.2.7" {
		t.Fatalf("LinkStateID.String() = %q, want 192.0.2.7", got)
	}
	fromBytes, err := LinkStateIDFromBytes([]byte{192, 0, 2, 7})
	if err != nil {
		t.Fatalf("LinkStateIDFromBytes returned error: %v", err)
	}
	if id != fromBytes {
		t.Fatalf("LinkStateID round-trip mismatch: %v != %v", id, fromBytes)
	}
	var buf [8]byte
	if n := id.WriteTo(buf[:], 1); n != LinkStateIDLen {
		t.Fatalf("LinkStateID.WriteTo wrote %d bytes, want %d", n, LinkStateIDLen)
	}
	if got := buf[1:5]; !bytes.Equal(got, []byte{192, 0, 2, 7}) {
		t.Fatalf("LinkStateID.WriteTo bytes = %v, want [192 0 2 7]", got)
	}
}

// VALIDATES: AC-2 - LinkStateIDFromBytes rejects lengths other than 4.
// PREVENTS: malformed LSA headers leaking partial Link State IDs.
func TestLinkStateIDFromBytesRejectsWrongLength(t *testing.T) {
	for _, input := range [][]byte{{1, 2, 3}, {1, 2, 3, 4, 5}} {
		if _, err := LinkStateIDFromBytes(input); err == nil {
			t.Fatalf("LinkStateIDFromBytes(%v) succeeded, want error", input)
		}
	}
}
