package types

import (
	"bytes"
	"testing"
)

// VALIDATES: SequenceNumber 0 is the reserved value (never a valid version) and
// the increment helper never produces 0 (AC-8). Purge is signaled by
// RemainingLifetime 0, NOT by sequence 0.
// PREVENTS: masking the reserved-zero rule (spec risk R-2, origination bugs).
func TestSequenceNumberReserved(t *testing.T) {
	if !SequenceNumber(0).IsReserved() {
		t.Error("SequenceNumber(0) must report IsReserved()")
	}
	if SequenceNumber(1).IsReserved() {
		t.Error("SequenceNumber(1) must not be reserved")
	}
	if SequenceNumber(MaxSequenceNumber).IsReserved() {
		t.Error("the maximum sequence must not be reserved")
	}

	// FirstSequenceNumber is the first valid originated version (1).
	if FirstSequenceNumber != 1 {
		t.Errorf("FirstSequenceNumber = %d, want 1", FirstSequenceNumber)
	}

	// Next from the reserved 0 yields the first valid version (1), never 0.
	if got := SequenceNumber(0).Next(); got != FirstSequenceNumber {
		t.Errorf("SequenceNumber(0).Next() = %d, want %d", got, FirstSequenceNumber)
	}
	if got := SequenceNumber(1).Next(); got != 2 {
		t.Errorf("SequenceNumber(1).Next() = %d, want 2", got)
	}
}

// VALIDATES: Next at the 32-bit maximum reports wraparound rather than silently
// wrapping to 0 (which would be the reserved value).
// PREVENTS: a silent wrap to the reserved 0 (origination loop/flap, R-2).
func TestSequenceNumberWrap(t *testing.T) {
	maxSeq := SequenceNumber(MaxSequenceNumber)
	next, wrapped := maxSeq.NextChecked()
	if !wrapped {
		t.Error("Next at max must report wraparound")
	}
	// On wrap the value is not auto-advanced inside the type; runtime (isis-6)
	// purges then re-originates from FirstSequenceNumber. The helper must not
	// silently return the reserved 0.
	if next == 0 {
		t.Error("NextChecked must not silently produce the reserved 0 on wrap")
	}

	mid := SequenceNumber(5)
	if _, wrapped := mid.NextChecked(); wrapped {
		t.Error("Next below max must not report wraparound")
	}
}

// VALIDATES: SequenceNumber serializes to exactly 4 big-endian octets and
// round-trips through FromBytes (AC-11).
// PREVENTS: serialize/parse asymmetry for the LSP version field.
func TestSequenceNumberBytes(t *testing.T) {
	s := SequenceNumber(0x01020304)
	var buf [8]byte
	n := s.WriteTo(buf[:], 0)
	if n != SequenceNumberLen {
		t.Fatalf("WriteTo returned %d, want %d", n, SequenceNumberLen)
	}
	want := []byte{0x01, 0x02, 0x03, 0x04}
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("serialize = %x, want %x", buf[:n], want)
	}
	back, err := SequenceNumberFromBytes(buf[:n])
	if err != nil {
		t.Fatalf("SequenceNumberFromBytes error: %v", err)
	}
	if back != s {
		t.Errorf("round-trip = %#x, want %#x", uint32(back), uint32(s))
	}
	for _, l := range []int{0, 3, 5} {
		if _, err := SequenceNumberFromBytes(make([]byte, l)); err == nil {
			t.Errorf("SequenceNumberFromBytes(len=%d) should error", l)
		}
	}
}
