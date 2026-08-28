---
kind: directive
level: MUST
stage:
---
**A `./le verify current mode full` run MUST execute these stages, in order:**

1. **Lint** (full or changed-only depending on target)
2. **Cached full pass** (`go test` without `-race`): Go caches a verdict against the
   files the TEST BINARY OPENED. That is narrower than a source hash, and the
   difference decides whether a mutation proof means anything: a producer the test
   reaches through `exec` is not one of those files, so editing it leaves the verdict
   cached and the tool answers `ok (cached)` for a run that never happened.
   The pass uses `ze_core` plus the default-on feature tags from `feature-gates.txt`,
   matching the shipped `ze_core` feature set. It also runs the bare `ze_core`
   hub compile-out checks so absent-feature tests still execute.
   When nothing changed, this completes in under 1 second. Catches logic regressions
   across the entire codebase.
3. **Race pass on changed groups only** (`go test -race` on component groups containing
   modified `.go` files): catches data races in what you touched, without recompiling
   everything. Group detection uses `internal/le/changed/changed.go`.
4. **Functional tests** (13 suites via `ze-test`)
5. **ExaBGP compatibility**
