// Design: docs/architecture/ospf/ospf-1-types.md -- LSAge DoNotAge, MaxAge, and saturating arithmetic

package types

import "testing"

// VALIDATES: AC-7 - LS age masks DoNotAge, exposes MaxAge, and rejects invalid low-bit ages.
// PREVENTS: treating a DoNotAge LSA as a 32768-second purge candidate.
func TestLSAgeBitsAndMaxAge(t *testing.T) {
	zero := LSAge(0)
	if zero.Age() != 0 || zero.DoNotAge() || zero.IsMaxAge() {
		t.Fatalf("zero age flags wrong: age=%d do-not-age=%v max=%v", zero.Age(), zero.DoNotAge(), zero.IsMaxAge())
	}
	maxAge := LSAge(MaxAge)
	if !maxAge.IsMaxAge() {
		t.Fatalf("MaxAge did not report IsMaxAge")
	}
	frozen, err := lSAgeFromRaw(DoNotAgeBit)
	if err != nil {
		t.Fatalf("LSAgeFromRaw(DoNotAgeBit) returned error: %v", err)
	}
	if !frozen.DoNotAge() || frozen.Age() != 0 || frozen.IsMaxAge() {
		t.Fatalf("DoNotAge flags wrong: age=%d do-not-age=%v max=%v", frozen.Age(), frozen.DoNotAge(), frozen.IsMaxAge())
	}
	frozenMax, err := lSAgeFromRaw(DoNotAgeBit | MaxAge)
	if err != nil {
		t.Fatalf("LSAgeFromRaw(DoNotAgeBit|MaxAge) returned error: %v", err)
	}
	if !frozenMax.DoNotAge() || frozenMax.Age() != MaxAge || !frozenMax.IsMaxAge() {
		t.Fatalf("DoNotAge MaxAge flags wrong: age=%d do-not-age=%v max=%v", frozenMax.Age(), frozenMax.DoNotAge(), frozenMax.IsMaxAge())
	}
	if _, err := lSAgeFromRaw(MaxAge + 1); err == nil {
		t.Fatalf("LSAgeFromRaw(MaxAge+1) succeeded, want error")
	}
}

// VALIDATES: AC-7 - aging arithmetic saturates at MaxAge and preserves DoNotAge.
// PREVENTS: LS age arithmetic overflowing or aging frozen LSAs.
// RFC requirement: RFC2328-14-1 positive -- LS age is never incremented past MaxAge: adding beyond the remaining headroom saturates at MaxAge exactly (LSAge.Add, lsage.go:54-63).
func TestLSAgeAddSaturates(t *testing.T) {
	age := LSAge(MaxAge - 10)
	if got := age.Add(20); got.Age() != MaxAge || !got.IsMaxAge() {
		t.Fatalf("age.Add saturated to age=%d max=%v, want MaxAge", got.Age(), got.IsMaxAge())
	}
	frozen, err := lSAgeFromRaw(DoNotAgeBit | 10)
	if err != nil {
		t.Fatalf("LSAgeFromRaw returned error: %v", err)
	}
	if got := frozen.Add(20); got != frozen {
		t.Fatalf("DoNotAge Add changed age: got %#x want %#x", uint16(got), uint16(frozen))
	}
}

// VALIDATES: AC-7 - LSAge.String renders the masked age in seconds, ignoring the DoNotAge bit.
// PREVENTS: a frozen (DoNotAge) LSA rendering as 32768+ seconds instead of its real age.
func TestLSAgeString(t *testing.T) {
	if got := LSAge(0).String(); got != "0" {
		t.Errorf("LSAge(0).String() = %q, want 0", got)
	}
	if got := LSAge(MaxAge).String(); got != "3600" {
		t.Errorf("LSAge(MaxAge).String() = %q, want 3600", got)
	}
	frozen, err := lSAgeFromRaw(DoNotAgeBit | 42)
	if err != nil {
		t.Fatalf("LSAgeFromRaw returned error: %v", err)
	}
	if got := frozen.String(); got != "42" {
		t.Errorf("DoNotAge LSAge.String() = %q, want 42 (masked age)", got)
	}
}

// VALIDATES: AC-12 - LSAge parses and serializes exactly two big-endian bytes.
// PREVENTS: LSA header wire drift for the mutable LS Age field.
func TestLSAgeBytesRoundTrip(t *testing.T) {
	age, err := LSAgeFromBytes([]byte{0x0e, 0x10})
	if err != nil {
		t.Fatalf("LSAgeFromBytes returned error: %v", err)
	}
	if age.Age() != MaxAge {
		t.Fatalf("LSAgeFromBytes age=%d, want MaxAge", age.Age())
	}
	var buf [2]byte
	if n := age.WriteTo(buf[:], 0); n != LSAgeLen || buf != [2]byte{0x0e, 0x10} {
		t.Fatalf("LSAge.WriteTo n=%d bytes=%v", n, buf)
	}
	if _, err := LSAgeFromBytes([]byte{0x00}); err == nil {
		t.Fatalf("short age parse succeeded")
	}
}
