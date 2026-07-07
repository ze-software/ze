//go:build debug

// Design: docs/architecture/memory/lifetime-contracts.md

package memguard

// Enabled reports whether poisoning is active. True only in debug builds.
// Callers gate slice-argument construction with `if memguard.Enabled` so the
// argument is elided from release builds; see the package doc.
const Enabled = true

// poisonPattern is the repeating byte sequence written by Poison and matched by
// IsPoisoned. Four distinct bytes (a "dead beef" nod) make a coincidental match
// against real route data astronomically unlikely: an n-byte region matches
// only if every byte equals poisonPattern[i&3]. Debug-only — release Poison is
// a no-op and never reads it.
var poisonPattern = [4]byte{0xDE, 0xAD, 0xBE, 0xEF}

// Poison overwrites b with the repeating poison pattern so any later read of a
// borrowed slice into this memory reads a recognizable, obviously-invalid
// value instead of recycled route data. Called at a lifetime Boundary (slot
// release, buffer recycle, handle Release). No-op on an empty slice.
func Poison(b []byte) {
	for i := range b {
		b[i] = poisonPattern[i&3]
	}
}

// IsPoisonedForTest reports whether b holds the full poison pattern. A non-empty
// region reads as poisoned only if every byte equals poisonPattern[i&3]; a
// single differing byte fails the check, so live data and partially-overwritten
// regions are not mistaken for poison. Empty and nil are never poisoned.
//
// The ForTest suffix marks this as a test-only predicate: production code calls
// Poison at boundaries, tests call IsPoisonedForTest to assert a violation was
// caught. It has no production caller by design.
func IsPoisonedForTest(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for i := range b {
		if b[i] != poisonPattern[i&3] {
			return false
		}
	}
	return true
}
