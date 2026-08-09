// Design: docs/architecture/ospf/ospf-1-types.md -- LSSequenceNumber freshness and wrap boundaries

package types

import "testing"

// VALIDATES: AC-5 - RFC 2328 signed freshness orders MaxSequenceNumber after InitialSequenceNumber.
// PREVENTS: unsigned or wraparound comparisons that lose newer LSAs.
func TestLSSequenceFreshness(t *testing.T) {
	if !MaxSequenceNumber.NewerThan(InitialSequenceNumber) {
		t.Fatalf("MaxSequenceNumber was not newer than InitialSequenceNumber")
	}
	if InitialSequenceNumber.NewerThan(MaxSequenceNumber) {
		t.Fatalf("InitialSequenceNumber was newer than MaxSequenceNumber")
	}
	if MaxSequenceNumber.NewerThan(MaxSequenceNumber) {
		t.Fatalf("equal sequence reported newer")
	}
	if !ReservedSequenceNumber.IsReserved() {
		t.Fatalf("reserved sequence 0x80000000 was not reserved")
	}
}

// VALIDATES: AC-6 - max sequence and increment helpers expose wrap without returning reserved 0x80000000.
// PREVENTS: re-origination code from producing the reserved sequence number on the wire.
func TestLSSequenceWraparound(t *testing.T) {
	if !MaxSequenceNumber.IsMax() {
		t.Fatalf("MaxSequenceNumber did not report IsMax")
	}
	next := InitialSequenceNumber.Next()
	if next != InitialSequenceNumber+1 {
		t.Fatalf("InitialSequenceNumber.Next() = %#x, want %#x", uint32(next), uint32(InitialSequenceNumber+1))
	}
	wrapped, didWrap := MaxSequenceNumber.NextChecked()
	if !didWrap {
		t.Fatalf("MaxSequenceNumber.NextChecked did not report wrap")
	}
	if wrapped != InitialSequenceNumber {
		t.Fatalf("MaxSequenceNumber.NextChecked = %#x, want InitialSequenceNumber", uint32(wrapped))
	}
	if MaxSequenceNumber.Next() == ReservedSequenceNumber {
		t.Fatalf("MaxSequenceNumber.Next produced reserved sequence")
	}
}

// VALIDATES: AC-5 - LSSequenceNumber.String renders the RFC 2328 SIGNED value, so
// InitialSequenceNumber (0x80000001) shows as a negative number below MaxSequenceNumber.
// PREVENTS: an unsigned rendering that would make the initial sequence look larger than the max.
func TestLSSequenceString(t *testing.T) {
	cases := []struct {
		s    LSSequenceNumber
		want string
	}{
		{LSSequenceNumber(0), "0"},
		{LSSequenceNumber(1), "1"},
		{MaxSequenceNumber, "2147483647"},
		{InitialSequenceNumber, "-2147483647"},
		{ReservedSequenceNumber, "-2147483648"},
	}
	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("LSSequenceNumber(%#x).String() = %q, want %q", uint32(tc.s), got, tc.want)
		}
	}
}

// VALIDATES: AC-12 - LSSequenceNumber parses and serializes exactly four big-endian bytes.
// PREVENTS: wire codec endian drift for the LSA version field.
func TestLSSequenceBytesRoundTrip(t *testing.T) {
	seq, err := LSSequenceNumberFromBytes([]byte{0x80, 0x00, 0x00, 0x01})
	if err != nil {
		t.Fatalf("LSSequenceNumberFromBytes returned error: %v", err)
	}
	if seq != InitialSequenceNumber {
		t.Fatalf("sequence = %#x, want InitialSequenceNumber", uint32(seq))
	}
	var buf [4]byte
	if n := seq.WriteTo(buf[:], 0); n != LSSequenceNumberLen {
		t.Fatalf("LSSequenceNumber.WriteTo wrote %d, want %d", n, LSSequenceNumberLen)
	}
	if got := buf; got != [4]byte{0x80, 0x00, 0x00, 0x01} {
		t.Fatalf("sequence bytes = %v, want [128 0 0 1]", got)
	}
	if _, err := LSSequenceNumberFromBytes([]byte{0x80, 0x00, 0x00}); err == nil {
		t.Fatalf("short sequence parse succeeded")
	}
}
