// Related: jobkey.go -- the fingerprint these tests drive from its entry point
//
// VALIDATES: spec-le-is-a-ze-binary AC-11. The complete native command,
// arguments included, identifies the work.
// PREVENTS: two package-scoped jobs sharing one verdict.

package lejob

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestJobKeyIsTheCommand verifies the exact byte stream used for the key.
func TestJobKeyIsTheCommand(t *testing.T) {
	sum := sha256.Sum256([]byte("CMD=./le verify-deps unit-race-changed scope ./a\n"))
	want := hex.EncodeToString(sum[:])

	got := jobKey([]string{"./le", "verify-deps", "unit-race-changed", "scope", "./a"})
	if got != want {
		t.Errorf("JobKey = %s, want %s", got, want)
	}
}

// TestJobKeySeparatesDifferentWork verifies arguments are part of the key.
func TestJobKeySeparatesDifferentWork(t *testing.T) {
	onA := jobKey([]string{"./le", "verify-deps", "unit-race-changed", "scope", "./a"})
	onB := jobKey([]string{"./le", "verify-deps", "unit-race-changed", "scope", "./b"})
	if onA == onB {
		t.Error("two packages share one key")
	}
	if again := jobKey([]string{"./le", "verify-deps", "unit-race-changed", "scope", "./a"}); again != onA {
		t.Error("the same work keyed twice answered two keys")
	}
	other := jobKey([]string{"./le", "verify-lint", "run", "scope", "./a"})
	if other == onA {
		t.Error("two commands share one key")
	}
}
