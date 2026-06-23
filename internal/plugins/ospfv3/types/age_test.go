// VALIDATES: spec-ospfv3-1-types -- LSAge range (0..MaxAge) and the MaxAge predicate.
// PREVENTS: an age comparison that misses the MaxAge flush boundary.
package types

import "testing"

func TestOSPFv3AgeBoundaries(t *testing.T) {
	if MaxAge != LSAge(3600) {
		t.Errorf("MaxAge = %d, want 3600", MaxAge)
	}
	if !MaxAge.IsMaxAge() {
		t.Error("MaxAge.IsMaxAge() false")
	}
	if LSAge(3599).IsMaxAge() {
		t.Error("3599 reported MaxAge")
	}
	if LSAge(0) > MaxAge || LSAge(3600) > MaxAge {
		t.Error("0 and 3600 should be in range")
	}
	if LSAge(3601) <= MaxAge {
		t.Error("3601 should be out of range")
	}
	buf := make([]byte, 2)
	if n := LSAge(0x0102).WriteTo(buf, 0); n != 2 || buf[0] != 0x01 || buf[1] != 0x02 {
		t.Errorf("WriteTo n=%d buf=%v", n, buf)
	}
}
