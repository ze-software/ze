---
kind: directive
level: MUST
stage:
---
**A `ze-verify` run MUST execute these stages, in order:**

1. **Lint** (full or changed-only depending on target)
2. **Cached full pass** (`go test` without `-race`): Go caches results by source hash.
   The pass uses `ze_core` plus the default-on feature tags from `feature-gates.txt`,
   matching the shipped `make ze` feature set. It also runs the bare `ze_core`
   hub compile-out checks so absent-feature tests still execute.
   When nothing changed, this completes in under 1 second. Catches logic regressions
   across the entire codebase.
3. **Race pass on changed groups only** (`go test -race` on component groups containing
   modified `.go` files): catches data races in what you touched, without recompiling
   everything. Group detection uses `scripts/dev/changed-groups.sh`.
4. **Functional tests** (13 suites via `ze-test`)
5. **ExaBGP compatibility**
