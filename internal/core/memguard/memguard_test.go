// Design: docs/architecture/memory/lifetime-contracts.md
//
// One test file, no build tag: it adapts to the active build via the Enabled
// constant so both the debug (real poison) and release (no-op) implementations
// are exercised by their respective `go test` / `go test -tags debug` runs.

package memguard

import "testing"

// TestPoisonRoundTrip proves the debug primitive: live bytes do not read as
// poisoned, and Poison makes them read as poisoned. In release builds Poison
// is a no-op and IsPoisoned is always false, so the same test asserts the
// no-op contract.
//
// VALIDATES: Poison overwrites bytes with a detectable pattern in debug and is
// a true no-op in release (AC-5 release parity, AC-1..AC-3 poison detection).
//
// PREVENTS: A silent lifetime violation reading recycled bytes as valid data.
func TestPoisonRoundTrip(t *testing.T) {
	b := []byte{1, 2, 3, 4, 5, 6, 7}

	if IsPoisonedForTest(b) {
		t.Fatalf("live data must not read as poisoned: %v", b)
	}

	Poison(b)

	if Enabled {
		if !IsPoisonedForTest(b) {
			t.Fatalf("debug: Poison then IsPoisoned must be true, got %v", b)
		}
		if b[0] == 1 {
			t.Fatalf("debug: Poison must overwrite the bytes, got %v", b)
		}
	} else {
		if b[0] != 1 {
			t.Fatalf("release: Poison must be a no-op, got %v", b)
		}
		if IsPoisonedForTest(b) {
			t.Fatal("release: IsPoisoned must always be false")
		}
	}
}

// TestPoisonEmptyAndNil verifies the helpers tolerate empty and nil slices
// (an interned empty attribute produces a zero-length buffer region).
func TestPoisonEmptyAndNil(t *testing.T) {
	Poison(nil)
	Poison([]byte{})
	if IsPoisonedForTest(nil) {
		t.Fatal("nil is never poisoned")
	}
	if IsPoisonedForTest([]byte{}) {
		t.Fatal("empty is never poisoned")
	}
}

// TestIsPoisonedRejectsPartialMatch proves IsPoisoned checks the full pattern:
// a slice that only partly matches must not read as poisoned. Debug-only, since
// release IsPoisoned is unconditionally false.
func TestIsPoisonedRejectsPartialMatch(t *testing.T) {
	if !Enabled {
		t.Skip("release build: IsPoisoned is unconditionally false")
	}
	b := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	Poison(b)
	b[len(b)-1] = 0x00 // corrupt one byte
	if IsPoisonedForTest(b) {
		t.Fatalf("a partially-overwritten poison region must not read as poisoned: %v", b)
	}
}
