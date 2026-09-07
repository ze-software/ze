# Spec: verify-scope-5-suite-coverage-map

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | spec-verify-scope-2-change-set-selector (closed 2026-09-05; the selector is `internal/le/changed/selector.go` and `docs/architecture/testing/verify-freshness-scope.md`) |
| Phase | 3/5 |
| Handoff | - |
| Updated | 2026-09-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Derive which functional suites exercise which Go packages, by RECORDING it at
run time, so the functional stage can run only the suites a change can reach.

`./le functional` is 1472s of the 4418s full run and is now the largest
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
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` test file format: embedded files, options, expectations and commands
- [ ] `ai/rules/testing.md` - the four carriers, and how a `.ci` earns `functional/verify`
  → Constraint: non-unit evidence is monotonic per requirement and per tier. `checkEvidenceRatchet` (`internal/le/rfc/check_ratchets.go`) fires when a `.ci` loses its tier, and no annotation satisfies it
- [ ] `docs/functional-tests.md` - the suites, the runner, and the isolated binary set
  → Constraint: the default mode builds the binaries OUTSIDE the runner
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - what a scoped run already judges

**Key insights:**
- The suites cannot be run concurrently: only the `bgp` and `vpp` paths call `runner.ReservePorts` (`internal/test/runner/ports.go`), and suites registered through `registerCIRoot` take a deterministic port. `plan/journal/parallel-copies-collide-on-a-deterministic-port.md` records the collisions. This spec SELECTS suites; it does not parallelize them.
- `functionalSuitesFromGo` (`internal/le/rfc/check_baseline.go`) fails CLOSED by design: it parses `Gating` out of `internal/le/functional/suites.go` and raises on a source it cannot read, rather than assuming everything runs. That shapes the tier decision below. It was `functional_suites` in the Makefile era, and the ported name is what a reader has to grep for now.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/runner.go` - `(*Runner).Build` compiles `./cmd/ze` with `go build -tags TestBuildTags() -ldflags ...`, and returns `r.verifyPrebuilt()` early when `ze.test.no.build` is enabled
- [ ] `internal/test/runner/runner_exec_util.go` - `childEnv` returns `os.Environ()` plus `GOTRACEBACK=all` and `CGO_ENABLED=0`, so an exported variable reaches every spawned process with no per-test plumbing
- [ ] `internal/le/functional/suites.go` - `Gating`, the ordered gating suite list, and `GatingSuites`, which resolves it against the catalog
- [ ] `internal/le/functional/binaries.go` - `Prepare` builds the isolated set; `ZE_SUFFIX` names its directory, `ZE_TEST_CANONICAL` runs the session's own binaries in place, `ZE_COVER` turns on `-cover` and `coverRoot`
- [ ] `internal/le/functional/run.go` - `runGating` is the stage: it selects, announces and executes each suite, and `reduceCoverage` reduces the suite's `GOCOVERDIR` afterwards
- [ ] `internal/le/changed/selector.go` - `runSelector`, whose package answer this spec consumes
- [ ] `internal/le/rfc/check_baseline.go`, `carriers.go`, `check_ratchets.go` - `functionalSuitesFromGo`, `suiteCarriers`, `checkEvidenceRatchet`

**→ Constraint: the instrumentation goes where the binary is BUILT, and in the
default mode that is not the runner.** `Prepare` (`internal/le/functional/binaries.go`)
compiles the isolated set and the suite runs with `ze.test.no.build` enabled, so
`(*Runner).Build` takes its `verifyPrebuilt` branch and never compiles. Adding
`-cover` to `(*Runner).Build` alone instruments only `ZE_TEST_CANONICAL=1` runs,
which is not how the gate runs. Both producers need it, or the spec must name
which mode produces the map.

**Behavior to preserve:**
- `Gating` (`internal/le/functional/suites.go`) stays the single source of truth for which suites are gating.
- Every `.ci` that runs today still runs when its subject changes.
- The per-suite wall-clock budgets, including `ZE_SUITE_TIMEOUT_PLUGIN`.
- `ZE_SKIP_SUITES` keeps its meaning as an operator override.
- `functionalSuitesFromGo` keeps failing closed.

**Behavior to change:**
- The functional stage runs a subset of suites, chosen by the recorded map and the selector's package answer.

## Data Flow (MANDATORY)

### Entry Point
- A full functional run: every suite executes an instrumented `ze` and records what it reached.
- A scoped verify run: the functional stage reads the map and the selector's package answer.

### Transformation Path
1. The binary producer compiles `./cmd/ze` with `-cover`.
2. `runGating` exports a per-suite `GOCOVERDIR`; `childEnv` carries it to every spawned `ze`.
3. After each suite, `go tool covdata textfmt` reduces that directory to the set of packages the suite REACHED: a package with one covered block that is neither in `register.go` nor inside a `func init(` body. Counting a package as executed instead is what phase 1 measured and what broke A-3.
4. The per-suite sets are written to one derived artifact, with the HEAD it was produced at.
5. A later scoped run intersects the selector's package answer with that artifact and skips the suites no changed package reaches.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| make ↔ instrumented binary | `-cover` at the build site, `GOCOVERDIR` per suite | No |
| Suite run ↔ map | `go tool covdata` over the suite's directory | No |
| Map ↔ functional stage | the derived artifact, read to compute `ZE_SKIP_SUITES` | No |

### Integration Points
- `Prepare` (`internal/le/functional/binaries.go`) and `(*Runner).Build` - the two binary producers.
- `runGating` and `reduceCoverage` (`internal/le/functional/run.go`) - the per-suite environment and the post-suite reduction.
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
| A-1 | An instrumented `ze` is behaviourally identical to the shipped one for every `.ci` | `-cover` adds counters, not logic | Suites fail only under instrumentation, and the map cannot be produced | Run the full functional suite instrumented and compare the pass set against the uninstrumented run | **broken for `ui` only** (2026-09-07). `encode` is identical over 9 runs and `parse` differs only on a test flaky in both arms. `ui` loses 7 per-test TIMEs over 3 instrumented runs against 0 over 3 uninstrumented ones, on 7 different tests, none of them reproducing across every instrumented run. The per-suite budget is refuted as the mechanism |
| A-2 | `GOCOVERDIR` reaches every process a `.ci` starts | `childEnv` returns `os.Environ()` plus extras | Some suites record nothing and read as covering no package | Assert a non-empty profile for every suite in `Gating` | **broken** (2026-08-19) |
| A-3 | Per-suite package sets are small enough to be worth selecting on | untested. The daemon LINKS 562 of 646 packages, and this spec bets it EXECUTES far fewer per suite | The map selects nearly every suite and buys nothing | **Measure first, in phase 1, before building anything else** | **broken** as EXECUTED (2026-08-19); **holds** as REACHED (2026-09-07, phase 1b) |

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

### Phase 1b measurement (2026-09-07)

Three suites, instrumented one at a time on the same tree: `encode` (BGP wire
encoding, 59 tests), `parse` (config parsing, 331 tests, the largest suite that
is not `plugin`) and `ui` (CLI, completion and the native le fixtures, 258
tests). They are the three largest suites phase 1 recorded as reaching different
areas, and `plugin` was left out because its 1500s budget buys nothing this
measurement needs. The whole measurement took 14 minutes of wall clock, of which
`ui` was 615s.

**A-3 holds under a refined definition, and the phase 1 definition is what was
broken.** A package counts as REACHED by a suite only when the suite covered one
block that is neither in `register.go` nor inside a `func init(` body. Every
other block is what a process runs on any start, which is what phase 1 counted.

| Set | Executed (phase 1) | Reached (refined) |
|-----|-------------------|-------------------|
| `ze show version` | 435 | 115 |
| `encode` | 475 | 189 |
| `parse` | 460 | 167 |
| `ui` | 495 | 250 |
| intersection of the three suites | 443 | 126 |
| union of the three suites | 505 | 273 |

The intersection is the number that decides the spec, and it falls from 443
packages (68.6% of the module's 646) to 126 (19.5%). The union falls from 505 to
273, so 303 of the 576 packages the instrumented binaries carry are reached by
none of the three suites, and the other 70 of the module's 646 are not linked
into `ze` at all. The floor a trivial command sets falls the same way, from 435
to 115, and 112 of those 115 sit inside the three-suite intersection: they are
the daemon start path (57 packages under `internal/component`, 35 under
`internal/core`, 24 under `internal/plugins`), which selects every suite whatever
the definition says.

Two packages show the mechanism. `internal/component/bgp/plugins/nlri/mvpn` is
EXECUTED by all three suites and REACHED by `encode` alone.
`internal/component/ssh` is executed by `ui` alone, and it is the change set AC-3
names.

**The refined definition clears the stop rule in Implementation Step 1.** The
largest of the three sets is 250 of the module's 646 packages (38.7%), which is
not most of the module, and no set exceeds 43.4% of the 576 packages the
instrumented binaries carry.

Method: `go tool covdata textfmt` over each suite's `GOCOVERDIR`, then `go/ast`
over each covered file for the line range of every `func init(` it declares. A
file that declares several init functions yields several ranges, and a block
inside any of them is discarded. No file failed to parse. Each suite ran through
`./le functional <suite>` with `ZE_COVER=1` and an absolute `GOCOVERDIR`, so the
raw directories survived for a textfmt reduction, which is not what
`reduceCoverage` (`internal/le/functional/run.go`) performs.

Every set here is a LOWER bound. Under `-cover` on a loaded box `encode` passed
54/59, `parse` 328/331 and `ui` 238/258 with one timeout. No uninstrumented run
was made beside them, so whether those failures are the A-1 cost or predate it is
not measured here. A failed test still records the packages it reached before it
failed.

### A-1 attribution (2026-09-07)

Nine `encode` runs, nine `parse` runs and six `ui` runs on one tree
(`f8c8b29eb3`), uninstrumented and instrumented in adjacent pairs, compared test
by test rather than by count. The hypothesis under test was that the losses are
the per-suite wall-clock budget being exceeded under instrumentation and load.
**It is refuted.**

| Suite | Uninstrumented | Instrumented | Verdict |
|-------|----------------|--------------|---------|
| `encode` | 54/59, failed [10, 13, 14, 31, 53], 4 runs | 54/59, the same five ids, 5 runs | identical. Phase 1b's 54/59 is pre-existing |
| `parse` | 329 or 328/331, always [253, 254], 4 runs | 328 or 329/331, 5 runs | [253, 254] pre-existing. `tacacs-key-required` (302) fails in 4 of 5 instrumented and 2 of 4 uninstrumented runs: flaky in BOTH arms |
| `ui` | 235, 243, 243 of 260. 0 TIME in 3 runs | 243, 240, 236 of 260. 7 TIME in 3 runs | 16 tests fail in all six runs. The TIME verdicts are the one asymmetry |

**The budget is not the mechanism, and three measurements say so.** `encode`
used at most 51.6s and `parse` at most 80.2s of a 600s cap while failing, so no
cap was near. Raising both to 1800s changed no verdict. Both members of two
`ui` pairs ran at 1800s and used 502s to 558s, and the instrumented losses
persisted. The gates that DO decide these verdicts are per-TEST and no
`ZE_SUITE_TIMEOUT_<SUITE>` reaches either: `Timings.SuggestedTimeout`
(`internal/test/runner/timing.go`) gives `min(30s, max(5s, 5x baseline))`,
`ParallelTimeoutHeadroom` (`internal/test/runner/parallel.go`) multiplies it by
3, and a fixture's own readiness wait is a fixed 20s that nothing scales
(`Poll(ctx, 200, 100*time.Millisecond, ...)`, `internal/test/fixture/fixture.go`).
No per-suite budget was changed.

**The 7 instrumented TIME verdicts fell on 7 different tests, and 6 of them
normally run under 1s.** `cli-completion-addpath-fields` 550ms, `-json-output`
534ms, `-peer-set-all` 513ms, `-plugin-external` 531ms, `-process-content`
549ms, `debug-enable-show` 258ms, each expiring at 16.0s to 19.4s. A test whose
baseline is under 1s takes the 5s FLOOR, which the headroom widens to 15s, so it
holds the smallest deadline in the suite and a suite-wide slowdown of any size
reaches it first. `le-ste-answers` is the seventh and it is the opposite case:
445.0s and 454.3s of a 600s suite in two runs that passed.

**No single test fails under instrumentation and passes without it.**
`show-column-order-absent-unchanged` is the closest at 2 of 3 instrumented
against 0 of 3 uninstrumented. In the pair taken on a loaded box the
INSTRUMENTED run passed more (243 against 235), and its nine extra passes
include the `daemon did not become ready` tests phase 1 attributed to
instrumentation. That direction reverses with load, so phase 1's `ui` finding is
load and not `-cover`.

**Instrumentation costs a suite between 0.96x and 2.45x, and costs a process
nothing measurable.** Suite seconds over the comparable-load rounds: `encode`
24.0 instrumented against 20.6 (1.16x), `parse` 63.2 against 25.8 (2.45x), `ui`
502s to 558s against 524s to 531s. Outside the suite, on the same two binaries,
`ze show version` took 0.21s instrumented against 0.22s, `ze config completion`
0.52s against 0.53s, `ze config import` into a fresh zefs store 0.39s against
0.42s, and 80 processes at 8-way concurrency 5.76s against 5.63s. `parse` is
therefore 2.45x slower for a reason that is inside the suite and that this
measurement did not locate.

**`ui` is at 87% to 100% of its 600s budget in BOTH arms** (588.7s, 524s and
531s uninstrumented; 600.7s, 502s and 558s instrumented). The one budget kill
observed, exit 124 on an instrumented run, fell on the arm holding the busier
window, and one test is 445s of it. That is budget creep in `ui`, independent of
this spec.

Two comparisons are unsound and are not read as evidence. The first `ui` pair
ran at load 62 against load 11, which is the pair whose direction reverses. A
commit landed during the first `ui` pair (`f8c8b29eb3`, `internal/le/doc/check`)
and another during the last `ui` run (`61a2b21e52`, docs and website only). The
box carried a load average of 20 to 60 across 32 cores for every `encode` and
`parse` round, and 2 to 31 for the `ui` pairs of runs 2 and 3.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The map skips a suite that would have caught the change | CI red on a locally green change | The map may only NARROW from a package it records; anything it does not know widens to every suite |
| R-2 | A stale map omits a suite that now covers the package | a suite newly reaches a package and nobody notices | A package is answerable only when the map records it AND no commit since the map's recorded HEAD touched it |
| R-3 | Instrumentation costs more than selection saves | the instrumented full run exceeds the uninstrumented one by more than selection saves | Phase 1 measures both before anything depends on it |
| R-4 | Per-change suite skipping lowers a tagged RFC requirement's derived tier | `./le rfc check` reports a tier change | See the tier decision below: the derivation stays on `Gating`, and the ledger is diffed before and after |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A functional suite that would have failed is skipped, and a defect lands. No runtime behavior changes: every file is build and test tooling |
| How is it reverted? | Single commit revert. Absent the artifact the stage runs every suite, which is today's behavior |
| Who else touches this path? | Every session runs the functional suite; the RFC ledger reads the suite list |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le functional` | → | the per-suite `GOCOVERDIR` export in `runGating` | `TestEverySuiteRecordsACoverageProfile` |
| a recorded map plus a package answer | → | the computed `ZE_SKIP_SUITES` | `TestSuiteSelectionSkipsOnlyUnreachedSuites` |
| an absent or stale map | → | the fail-open branch | `TestAbsentMapRunsEverySuite` |
| `./le rfc check` | → | `functionalSuitesFromGo` reading `Gating` | `test_functional_tier_is_unchanged_by_selection` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A full instrumented functional run completes | Every suite in `Gating` that CAN record has a non-empty recorded package set, a suite that records nothing is omitted from the map rather than written empty, and no test passes uninstrumented and fails instrumented across paired runs |
| AC-2 | The instrumented full run is measured against the uninstrumented one | The added cost is stated as a number, and it is smaller than selection saves on a feature-local change |
| AC-3 | The change set is one `internal/component/ssh` file and the map is current | The stage runs the suites whose recorded set contains that package, and no others |
| AC-4 | The map does not record a changed package | Every suite runs, and the stage names the package it could not answer for |
| AC-5 | The map exists, but a commit since its recorded HEAD touched the changed package | That package is treated as unknown, so every suite runs |
| AC-6 | The map is absent entirely | Every suite runs, exactly as today |
| AC-7 | `./le rfc check` runs before and after | No requirement loses its `functional/verify` tier, and `checkEvidenceRatchet` stays green |
| AC-8 | An operator sets `ZE_SKIP_SUITES` | Those suites are still skipped, and the map cannot re-add them |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEverySuiteRecordsACoverageProfile` | `internal/le/` | AC-1: the export reaches every suite | |
| `TestSuiteSelectionSkipsOnlyUnreachedSuites` | `internal/le/functional/suitemap_test.go` | AC-3 | | <!-- doc-links: ignore (artifact a later phase of this spec will create) -->
| `TestAbsentMapRunsEverySuite` | `internal/le/functional/suitemap_test.go` | AC-4, AC-6: the fail-open branches | | <!-- doc-links: ignore (artifact a later phase of this spec will create) -->
| `TestStaleMapTreatsTouchedPackagesAsUnknown` | `internal/le/functional/suitemap_test.go` | AC-5 | | <!-- doc-links: ignore (artifact a later phase of this spec will create) -->
| `TestEmptyRecordedSetIsARefusal` | `internal/le/functional/suitemap_test.go` | a suite recording nothing must fail, never read as covering nothing | | <!-- doc-links: ignore (artifact a later phase of this spec will create) -->
| `TestOperatorSkipStillWins` | `internal/le/` | AC-8 | |
| `test_functional_tier_is_unchanged_by_selection` | `internal/le/` | AC-7 | |

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
| `verify-scope-suite-map` | `test/runner/verify-scope-suite-map.ci` | A developer edits one SSH file and the functional stage runs only the suites observed to reach it | | <!-- doc-links: ignore (artifact a later phase of this spec will create) -->

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Scope is tooling. No wire-visible behavior changes | |

## Files to Modify
- `internal/le/functional/binaries.go` - the `-cover` build and `coverRoot`
- `internal/le/functional/run.go` - the per-suite `GOCOVERDIR`, `reduceCoverage`, and the run list `runGating` computes
- `internal/test/runner/runner.go` - `(*Runner).Build` for the canonical-mode producer
- `docs/functional-tests.md`, `docs/architecture/testing/verify-freshness-scope.md`
- `ai/rules/testing.md` - what the `functional/verify` tier now MEANS

## Files to Create
- `internal/le/functional/suitemap.go` - the map reader and the suite selector (created 2026-09-07, phase 2). NOT `internal/le/changed/scope.go`, which exists and holds the `ZE_VERIFY_SCOPE_PACKAGES` change-scope answer: the map is keyed by suite names, which only `internal/le/functional` declares
- `internal/le/functional/suitemap_test.go` (created 2026-09-07, phase 2)
- `internal/le/functional/reach.go` - the per-suite reduction: `go tool covdata textfmt` plus the `go/ast` init-body ranges that separate a REACHED package from a linked one (created 2026-09-07, phase 3)
- `internal/le/functional/reach_test.go` (created 2026-09-07, phase 3)
- `test/runner/verify-scope-suite-map.ci` <!-- doc-links: ignore (artifact a later phase of this spec will create) -->

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
| The map exists and is derived | `./le functional` produces it; no hand-written rows |
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
| A package is REACHED by a suite only when the suite covers a block outside `register.go` and outside every `func init(` in the file | Count any covered block, as phase 1 did | Ze registers by running `init()` in each `register.go` on process start, so every package a binary links counts as executed on any command. Phase 1b measured both: the three-suite intersection is 443 packages counting executions and 126 counting reaches, and the floor a `ze show version` sets falls from 435 to 115 |
| Observe the mapping at run time | Derive it from `.ci` text, filenames, suite names, or the import graph | All four measured and rejected: 4.1% coverage, one-suite granularity, a false name match, and 87% of the module |
| The map is a DERIVED artifact under `tmp/`, not committed | Commit it and gate its freshness | A committed map needs a staleness gate, and staleness here is detectable only by re-running the suites. An absent map costs today's behavior, so absence is safe and needs no gate |
| A package is answerable only if the map records it AND no commit since the map's HEAD touched it | Trust the map until it is regenerated | The stale-map risk is a suite that newly reaches a package. Treating touched packages as unknown bounds it cheaply, with `git diff --name-only <map-sha> HEAD` |
| **The tier derivation stays on `Gating`; it does NOT read the map** | Point `functionalSuitesFromGo` at the recorded map | `functionalSuitesFromGo` fails CLOSED by design, and the map can legitimately be absent. Making a fail-closed derivation depend on an optional artifact inverts it. The tier's meaning becomes "this suite runs when its subject changes", which is the standard `./le changed scope` and `ze-unit-test-changed` already meet. `ai/rules/testing.md` must SAY that rather than leave the older reading standing |
| Select suites; do not parallelize them | Run the suites concurrently | Only `bgp` and `vpp` reserve ports; the rest take deterministic ones, and the collisions are already journalled |

## Known Limitations
- Coverage records what a suite REACHED. A change adding a code path no suite reaches today is a false negative, bounded by package granularity.
- CI starts from a fresh checkout with no map, so CI runs every suite. That is correct, and it is why the CI shards exist.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
