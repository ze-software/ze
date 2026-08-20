# Spec: verify-scope-5-suite-coverage-map

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `plan/spec-verify-scope-2-change-set-selector.md` |
| Phase | 1/5 |
| Deferral shard | `plan/deferrals/verify-scope.md` |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Derive which functional suites exercise which Go packages, by RECORDING it at
run time, so the functional stage can run only the suites a change can reach.

`ze-functional-test` is 1472s of the 4418s full run and is now the largest
remaining cost: sub-spec 3 cut the staticcheck matrix from 38 rows to 3 for a
feature-local change (12.1x, measured), and sub-spec 2 scoped the lint and unit
stages. The functional suite is the one stage still judging everything.

**Every static route to a package-to-suite map was measured, and every one
failed.** This spec exists because of that measurement rather than instead of it:

| Candidate | Coverage | Why it fails |
|-----------|----------|--------------|
| `exec=go test` naming a package in the `.ci` text | 69 of 1685 `.ci` (4.1%), only `ospf` and `ospfv3` | an OSPF-team idiom, not a convention. Reaches none of the four expensive suites |
| `.ci` filename prefix | 59% of `test/plugin` name-match a tag or a directory | all 665 sit in ONE suite, so the answer is only run-`plugin`-or-not, and the 41% unmapped force fail-open |
| Suite name equals package name | 9 of 24 | `plugin` to `internal/component/plugin` is FALSE: that suite spans bgp, cli, api, iface and web |
| The import graph | 87% of the module | `go list -deps ./cmd/ze` links 562 of 646 packages, so every suite "exercises" almost everything |
| A declared annotation in the `.ci` grammar | zero | every `option=` value is an execution knob; nothing names a subject |

**What is left is to observe it.** A suite runs the daemon, and an instrumented
daemon reports the packages it executed. That answer is derived, needs no
hand-written rows, and refreshes itself every time the suite runs.

**Its honest weakness, stated once and carried into the design.** Coverage
records what a suite REACHED, not what it is about. A change adding a code path
no suite reaches today is a false negative. The risk is narrower than it sounds
at package granularity: a suite joins a package's set if it executes any
statement in it, so the residual case is a suite that newly begins executing a
package at all.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/testing.md` - the four carriers, and how a `.ci` earns `functional/verify`
  → Constraint: non-unit evidence is monotonic per requirement and per tier. `check_evidence_ratchet` fires when a `.ci` loses its tier, and no annotation satisfies it
- [ ] `docs/functional-tests.md` - the suites, the runner, and the isolated binary set
  → Constraint: the default mode builds the binaries OUTSIDE the runner
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - what a scoped run already judges

**Key insights:**
- The suites cannot be run concurrently: only the `bgp` and `vpp` paths call `runner.ReservePorts` (`internal/test/runner/ports.go`), and suites registered through `registerCIRoot` take a deterministic port. `plan/journal/parallel-copies-collide-on-a-deterministic-port.md` records the collisions. This spec SELECTS suites; it does not parallelize them.
- `functional_suites` (`scripts/dev/rfc_requirements.py`) fails CLOSED by design: an unreadable or ambiguous recipe raises rather than assuming everything runs, and its own comment calls the opposite "the exact zero-that-looks-like-an-answer this module refuses elsewhere". That shapes the tier decision below.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/runner.go` - `(*Runner).Build` compiles `./cmd/ze` with `go build -tags TestBuildTags() -ldflags ...`, and returns `r.verifyPrebuilt()` early when `ze.test.no.build` is enabled
- [ ] `internal/test/runner/runner_exec_util.go` - `childEnv` returns `os.Environ()` plus `GOTRACEBACK=all` and `CGO_ENABLED=0`, so an exported variable reaches every spawned process with no per-test plumbing
- [ ] `mk/test-functional.mk` - `ZE_ALT_BUILD` compiles ze, ze-test and ze-stripped into `tmp/testbin-*`; `ZE_TEST_RUN` sets `ZE_TEST_NO_BUILD=1`; `run_suite` executes the 24 `all_suites` entries in order
- [ ] `scripts/checks/verify_scope_selector.go` - `runSelector`, whose package answer this spec consumes
- [ ] `scripts/dev/rfc_requirements.py` - `functional_suites`, `_suite_carriers`, `check_evidence_ratchet`

**→ Constraint: the instrumentation goes where the binary is BUILT, and in the
default mode that is not the runner.** `ZE_ALT_BUILD` (`mk/test-functional.mk`)
compiles the isolated set and `ZE_TEST_RUN` sets `ZE_TEST_NO_BUILD=1`, so
`(*Runner).Build` takes its `verifyPrebuilt` branch and never compiles. Adding
`-cover` to `(*Runner).Build` alone instruments only `ZE_TEST_CANONICAL=1` runs,
which is not how the gate runs. Both producers need it, or the spec must name
which mode produces the map.

**Behavior to preserve:**
- `all_suites` stays the single source of truth for which suites are gating.
- Every `.ci` that runs today still runs when its subject changes.
- The per-suite wall-clock budgets, including `ZE_SUITE_TIMEOUT_PLUGIN`.
- `ZE_SKIP_SUITES` keeps its meaning as an operator override.
- `functional_suites` keeps failing closed.

**Behavior to change:**
- The functional stage runs a subset of suites, chosen by the recorded map and the selector's package answer.

## Data Flow (MANDATORY)

### Entry Point
- A full functional run: every suite executes an instrumented `ze` and records what it reached.
- A scoped verify run: the functional stage reads the map and the selector's package answer.

### Transformation Path
1. The binary producer compiles `./cmd/ze` with `-cover`.
2. `run_suite` exports a per-suite `GOCOVERDIR`; `childEnv` carries it to every spawned `ze`.
3. After each suite, `go tool covdata` reduces that directory to the set of packages the suite executed.
4. The per-suite sets are written to one derived artifact, with the HEAD it was produced at.
5. A later scoped run intersects the selector's package answer with that artifact and skips the suites no changed package reaches.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| make ↔ instrumented binary | `-cover` at the build site, `GOCOVERDIR` per suite | No |
| Suite run ↔ map | `go tool covdata` over the suite's directory | No |
| Map ↔ functional stage | the derived artifact, read to compute `ZE_SKIP_SUITES` | No |

### Integration Points
- `ZE_ALT_BUILD` and `(*Runner).Build` - the two binary producers.
- `run_suite` (`mk/test-functional.mk`) - the per-suite environment and the post-suite reduction.
- `runSelector` - the package answer this consumes.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An instrumented `ze` is behaviourally identical to the shipped one for every `.ci` | `-cover` adds counters, not logic | Suites fail only under instrumentation, and the map cannot be produced | Run the full functional suite instrumented and compare the pass set against the uninstrumented run | **broken** (2026-08-19) |
| A-2 | `GOCOVERDIR` reaches every process a `.ci` starts | `childEnv` returns `os.Environ()` plus extras | Some suites record nothing and read as covering no package | Assert a non-empty profile for every suite in `all_suites` | **broken** (2026-08-19) |
| A-3 | Per-suite package sets are small enough to be worth selecting on | untested. The daemon LINKS 562 of 646 packages, and this spec bets it EXECUTES far fewer per suite | The map selects nearly every suite and buys nothing | **Measure first, in phase 1, before building anything else** | **broken** (2026-08-19) |

### Phase 1 measurement (2026-08-19)

Two full functional runs on the same tree, back to back, on a shared and
contended box: instrumented (`ZE_COVER=1`) then uninstrumented.

**A-3 is broken, and it is the one that stops the spec.** Every suite that
records anything records 423 to 513 of the module's 646 packages (65.5% to
79.4%). The intersection over all 20 recording suites is 423 packages, so a
change to any of those selects every suite; the union is 534, so the map can
never exclude more than 112 packages from every suite. The producing mechanism
is Ze's own registration pattern: `init()` in each `register.go` executes on
every process start, so a package counts as executed whatever the command does.
One `ze show version` already records 425 packages, and 242 of them are covered
only inside `register.go`. Three unrelated commands recorded 426, 426 and 424,
with a union of 428.

**A-2 is broken.** `editor` (166 tests, all passing) records nothing, because
the `.et` editor runs inside `ze-test`, which is the harness rather than the
subject. `web` records a meta file and zero counters over 97 tests. `runner`
tests the harness. `policy` skips all 6 tests unprivileged. `childEnv`
(`internal/test/runner/runner_exec_util.go`) does carry the variable, but a
RELATIVE `GOCOVERDIR` is resolved against the per-test directory
`(*Runner).runOrchestrated` sets (`internal/test/runner/runner_exec.go`), and
the Go runtime then prints `coverage meta-data emit failed` on the child's
stderr. That part was an instrumentation defect and is fixed with `$(abspath)`.

**A-1 is broken.** On one tree, `plugin` and `ui` pass uninstrumented and fail
instrumented: `plugin` 628/628 against 626/628 (`bfd-auth-sha1` times out,
`redistribute-as112-community` gains a `nopeer` community), `ui` 184/184
against 181/184 then 177/184, every failure a `daemon did not become ready`.
The same `ui` test passes serially under both binaries, so the difference is
load: the instrumented full run costs 2106s of suite time against 1452s (+45%),
and 2236s of wall clock against 1472s (+52%).

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The map skips a suite that would have caught the change | CI red on a locally green change | The map may only NARROW from a package it records; anything it does not know widens to every suite |
| R-2 | A stale map omits a suite that now covers the package | a suite newly reaches a package and nobody notices | A package is answerable only when the map records it AND no commit since the map's recorded HEAD touched it |
| R-3 | Instrumentation costs more than selection saves | the instrumented full run exceeds the uninstrumented one by more than selection saves | Phase 1 measures both before anything depends on it |
| R-4 | Per-change suite skipping lowers a tagged RFC requirement's derived tier | `make ze-rfc-check` reports a tier change | See the tier decision below: the derivation stays on `all_suites`, and the ledger is diffed before and after |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A functional suite that would have failed is skipped, and a defect lands. No runtime behavior changes: every file is build and test tooling |
| How is it reverted? | Single commit revert. Absent the artifact the stage runs every suite, which is today's behavior |
| Who else touches this path? | Every session runs the functional suite; the RFC ledger reads the suite list |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-functional-test` | → | the per-suite `GOCOVERDIR` export in `run_suite` | `TestEverySuiteRecordsACoverageProfile` |
| a recorded map plus a package answer | → | the computed `ZE_SKIP_SUITES` | `TestSuiteSelectionSkipsOnlyUnreachedSuites` |
| an absent or stale map | → | the fail-open branch | `TestAbsentMapRunsEverySuite` |
| `make ze-rfc-check` | → | `functional_suites` reading `all_suites` | `test_functional_tier_is_unchanged_by_selection` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A full instrumented functional run completes | Every suite in `all_suites` has a non-empty recorded package set, and the pass set equals the uninstrumented run's |
| AC-2 | The instrumented full run is measured against the uninstrumented one | The added cost is stated as a number, and it is smaller than selection saves on a feature-local change |
| AC-3 | The change set is one `internal/component/ssh` file and the map is current | The stage runs the suites whose recorded set contains that package, and no others |
| AC-4 | The map does not record a changed package | Every suite runs, and the stage names the package it could not answer for |
| AC-5 | The map exists, but a commit since its recorded HEAD touched the changed package | That package is treated as unknown, so every suite runs |
| AC-6 | The map is absent entirely | Every suite runs, exactly as today |
| AC-7 | `make ze-rfc-check` runs before and after | No requirement loses its `functional/verify` tier, and `check_evidence_ratchet` stays green |
| AC-8 | An operator sets `ZE_SKIP_SUITES` | Those suites are still skipped, and the map cannot re-add them |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEverySuiteRecordsACoverageProfile` | `scripts/dev/functional_suite_test.py` | AC-1: the export reaches every suite | |
| `TestSuiteSelectionSkipsOnlyUnreachedSuites` | `scripts/checks/verify_scope_suites_test.go` | AC-3 | |
| `TestAbsentMapRunsEverySuite` | `scripts/checks/verify_scope_suites_test.go` | AC-4, AC-6: the fail-open branches | |
| `TestStaleMapTreatsTouchedPackagesAsUnknown` | `scripts/checks/verify_scope_suites_test.go` | AC-5 | |
| `TestEmptyRecordedSetIsARefusal` | `scripts/checks/verify_scope_suites_test.go` | a suite recording nothing must fail, never read as covering nothing | |
| `TestOperatorSkipStillWins` | `scripts/dev/functional_suite_test.py` | AC-8 | |
| `test_functional_tier_is_unchanged_by_selection` | `scripts/dev/rfc_requirements_test.py` | AC-7 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| suites selected | 0-24 | 24 | N/A | N/A |
| packages in a suite's recorded set | 1-646 | 646 | 0 | N/A |

<!-- Zero suites is valid: a docs-only change reaches none. Zero packages in a
     suite's recorded set is NOT: it means the recording broke, and reading it
     as "this suite covers nothing" would skip that suite for ever. -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `verify-scope-suite-map` | `test/runner/verify-scope-suite-map.ci` | A developer edits one SSH file and the functional stage runs only the suites observed to reach it | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Scope is tooling. No wire-visible behavior changes | |

## Files to Modify
- `mk/test-functional.mk` - the `-cover` build, the per-suite `GOCOVERDIR`, the post-suite reduction, the computed `ZE_SKIP_SUITES`
- `internal/test/runner/runner.go` - `(*Runner).Build` for the canonical-mode producer
- `docs/functional-tests.md`, `docs/architecture/testing/verify-freshness-scope.md`
- `ai/rules/testing.md` - what the `functional/verify` tier now MEANS

## Files to Create
- `scripts/checks/verify_scope_suites.go` - the map reader and the suite selector
- `scripts/checks/verify_scope_suites_test.go`
- `test/runner/verify-scope-suite-map.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Test tooling only |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | N-A | Make targets and env vars only |
| CLI grammar (keyword before value) | N-A | as above |
| Editor autocomplete | N-A | as above |
| Functional test for new RPC/API | N-A | No RPC or API added |
| Pipe completeness | N-A | No `ze` CLI output added |
| Env var registration | N-A | `GOCOVERDIR` is Go's own; no `ze.*` leaf added |
| Doctor check for runtime dependencies | N-A | Nothing new in the shipped daemon |
| Prometheus counters/metrics | N-A | No daemon-observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Developer tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `ai/rules/testing.md`: what the tier means under selection. Regenerate the ledger and diff it |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: instrumentation and suite selection |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md`: the map's contract and its fail-open rule |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep for anchors naming `test-functional.mk` and `runner.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/functional-tests.md` documents `ZE_SKIP_SUITES` and the isolated binary set |

## Implementation Steps

1. **Phase: Measure before building (MANDATORY FIRST)** -- A-3 is what the whole spec rests on and it is untested
   - Instrument the build, export a per-suite `GOCOVERDIR`, run the full functional suite once, and reduce each directory with `go tool covdata`
   - Report the package-set size per suite, the pass set against the uninstrumented run, and the added wall clock
   - **If a suite's set is most of the module, say so and STOP.** The map would then select nearly every suite and this spec is not worth building. A-1, A-2 and A-3 are confirmed or broken here
2. **Phase: Wiring** -- the artifact exists and the selector reads it, selecting nothing yet
   - Tests: `TestAbsentMapRunsEverySuite`
3. **Phase: The map** -- the per-suite reduction, written with its HEAD recorded
   - Tests: `TestEverySuiteRecordsACoverageProfile`, `TestEmptyRecordedSetIsARefusal`
4. **Phase: Selection** -- intersect with the package answer, compute `ZE_SKIP_SUITES`, fail open on anything unknown
   - Tests: `TestSuiteSelectionSkipsOnlyUnreachedSuites`, `TestStaleMapTreatsTouchedPackagesAsUnknown`, `TestOperatorSkipStillWins`, `verify-scope-suite-map.ci`
5. **Phase: The tier's meaning** -- state in `ai/rules/testing.md` what `functional/verify` means under selection, regenerate the ledger and diff it
   - Tests: `test_functional_tier_is_unchanged_by_selection`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Both binary producers are instrumented, or the spec names which mode produces the map |
| Correctness | A suite recording an EMPTY set fails loudly. Reading it as "covers nothing" would skip that suite for ever |
| Naming | One name for the map, used by the stage and by any later consumer |
| Data flow | The map only ever narrows from a package it records; every unknown widens |
| Rule: `ai/rules/rfc-compliance.md` | No requirement loses a tier. Diff the ledger, do not assume |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The map exists and is derived | `make ze-functional-test` produces it; no hand-written rows |
| Selection works | The stage prints which suites it skipped and why |
| The instrumented cost is known | Phase 1's measurement, recorded in this spec |
| The ledger is unchanged in tier | `git diff ai/RFC-REQUIREMENTS.md rfc/requirements/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The map is read from a file under `tmp/`, which several sessions share. A malformed or truncated map must widen, never narrow |
| Resource exhaustion | `GOCOVERDIR` accumulates one file per process exit, and a suite starts many daemons. State the bound and clean up per suite |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Every static signal was measured and every one failed, which is why an observed map is the answer rather than the first idea. The measurements sit in the Task table so a later reader does not re-propose a filename-prefix map.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Observe the mapping at run time | Derive it from `.ci` text, filenames, suite names, or the import graph | All four measured and rejected: 4.1% coverage, one-suite granularity, a false name match, and 87% of the module |
| The map is a DERIVED artifact under `tmp/`, not committed | Commit it and gate its freshness | A committed map needs a staleness gate, and staleness here is detectable only by re-running the suites. An absent map costs today's behavior, so absence is safe and needs no gate |
| A package is answerable only if the map records it AND no commit since the map's HEAD touched it | Trust the map until it is regenerated | The stale-map risk is a suite that newly reaches a package. Treating touched packages as unknown bounds it cheaply, with `git diff --name-only <map-sha> HEAD` |
| **The tier derivation stays on `all_suites`; it does NOT read the map** | Point `functional_suites` at the recorded map | `functional_suites` fails CLOSED by design, and the map can legitimately be absent. Making a fail-closed derivation depend on an optional artifact inverts it. The tier's meaning becomes "this suite runs when its subject changes", which is the standard `ze-lint-changed` and `ze-unit-test-changed` already meet. `ai/rules/testing.md` must SAY that rather than leave the older reading standing |
| Select suites; do not parallelize them | Run the suites concurrently | Only `bgp` and `vpp` reserve ports; the rest take deterministic ones, and the collisions are already journalled |

## Known Limitations
- Coverage records what a suite REACHED. A change adding a code path no suite reaches today is a false negative, bounded by package granularity.
- CI starts from a fresh checkout with no map, so CI runs every suite. That is correct, and it is why the CI shards exist.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
