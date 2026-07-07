// Design: docs/architecture/memory/lifetime-contracts.md — shared "held past boundary" vocabulary
//
// Package memguard is the one build-tagged poison primitive shared by every
// buffer-lifetime contract in ze. A lifetime contract lends bytes across a
// call/dispatch boundary and recycles them afterwards; a consumer that reads
// the bytes after the boundary reads recycled, reused, or freed data. In
// release builds that read is silently wrong; in debug builds the contract
// calls Poison at the recycle/release point so the stale read surfaces as a
// recognizable pattern (checked with IsPoisonedForTest in tests) instead of
// plausible-but-wrong route data.
//
// Vocabulary (see the design doc): a Borrow is a zero-copy slice valid only
// until a named Boundary; to use it past the Boundary a consumer must Retain
// it (refcount) or Own a copy before the Boundary. Poison is applied AT the
// Boundary so a Borrow that outlives it is caught.
//
// Cost model: Poison/IsPoisoned are real work in debug builds only. In release
// builds Enabled is a compile-time false constant and the bodies are no-ops,
// so a caller guarded by `if memguard.Enabled { ... }` is dead-code-eliminated
// — zero instructions, zero branches on the release hot path. Callers that
// build a slice expression as the argument MUST use the `if memguard.Enabled`
// guard so the slice header and its bounds check are elided in release too.
package memguard
