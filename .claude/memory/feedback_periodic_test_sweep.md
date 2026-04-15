---
name: Periodic test gap sweeps catch predictable patterns
description: Untested code falls into three predictable categories -- pure functions covered only by integration, platform code assumed to need root, and missing test infrastructure support
type: feedback
originSessionId: e7a7ff86-ec52-4b35-a8cf-057497053ae4
---
A periodic test gap sweep (inventory all _test.go files vs production .go files) catches gaps that accumulate silently. The gaps follow three predictable patterns:

1. **Pure functions covered only by integration tests** (ze-analyse, rs helpers, resolve handlers) -- nobody thought to unit-test them because .ci tests exercise them indirectly. But integration tests don't catch content-level bugs (count-only assertions).
2. **Platform-specific code assumed untestable** (ifacedhcp) -- the syscall-heavy paths genuinely need root, but validation, mapping, and constructor paths are pure functions that are trivially testable.
3. **Missing test infrastructure support** (decode runner only handled 3 of 5 BGP message types) -- the gap exists because nobody needed it yet, not because it's hard to add.

**How to apply:** When adding a new package or feature, check that the pure-function helpers have direct unit tests, not just indirect coverage through integration. When a test runner doesn't support a feature, add support rather than leaving the gap.
