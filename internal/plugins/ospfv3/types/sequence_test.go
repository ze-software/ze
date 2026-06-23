// VALIDATES: spec-ospfv3-1-types AC-10 -- LSSequenceNumber represents the initial / max /
// reserved values and compares as a signed 32-bit version for later freshness checks.
// PREVENTS: a freshness comparison that mishandles the signed wrap or accepts the reserved
// 0x80000000.
package types

import "testing"

func TestOSPFv3SequenceBoundaries(t *testing.T) {
	if InitialSequenceNumber != LSSequenceNumber(int32(-0x7fffffff)) {
		t.Errorf("InitialSequenceNumber = %d", int32(InitialSequenceNumber))
	}
	if MaxSequenceNumber != LSSequenceNumber(0x7fffffff) {
		t.Errorf("MaxSequenceNumber = %#08x", uint32(MaxSequenceNumber))
	}
	if !MaxSequenceNumber.IsMax() {
		t.Error("Max predicate")
	}
	if !LSSequenceNumber(int32(-0x80000000)).IsReserved() {
		t.Error("0x80000000 should be reserved")
	}
	if InitialSequenceNumber.IsReserved() {
		t.Error("initial must not be reserved")
	}

	// Signed comparison: Max is newer than Initial; Initial is newer than nothing.
	if !MaxSequenceNumber.Newer(InitialSequenceNumber) {
		t.Error("Max should be Newer than Initial")
	}
	if InitialSequenceNumber.Newer(MaxSequenceNumber) {
		t.Error("Initial should not be Newer than Max")
	}

	// Next increments toward Max.
	if InitialSequenceNumber.Next() != InitialSequenceNumber+1 {
		t.Error("Next should increment")
	}
}
