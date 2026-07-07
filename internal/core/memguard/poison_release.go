//go:build !debug

// Design: docs/architecture/memory/lifetime-contracts.md

package memguard

// Enabled reports whether poisoning is active. False in release builds, where
// it is a compile-time constant so `if memguard.Enabled { ... }` blocks are
// dead-code-eliminated: zero instructions on the release hot path.
const Enabled = false

// Poison is a no-op in release builds. The lifetime contracts keep their exact
// zero-copy borrow semantics; only debug builds pay the poison cost.
func Poison(_ []byte) {}

// IsPoisonedForTest is unconditionally false in release builds: nothing is ever
// poisoned, so no read can be flagged. Test-only predicate (see the debug build).
func IsPoisonedForTest(_ []byte) bool { return false }
