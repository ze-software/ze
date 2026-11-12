# Spec: dropped-error-0-umbrella

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P1 |
| Release reason | Shipped code catches 325 errors whose only record is a debug or info line that the default WARN level hides, so a failed operation reads as done and leaves no trace (the MOBIKE route-source EPERM stopped UPDATE_SA_ADDRESSES on every nightly with an empty log); this umbrella owns the tree-wide zero |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

b2448d44e7 added a fifth refuse-by-default shape, `error-dropped-below-warn`
(`internal/le/arch/refusebydefault/dropped.go`, `droppedError`): a branch on
`err != nil` (or the else of `err == nil`) whose only record is a `Debug` or
`Info` log call, and which then falls through, continues, breaks or returns no
error. `./le arch refuse-by-default check` refuses it on changed lines.
`./le arch refuse-by-default report` lists **328 existing instances** at HEAD
on 2026-11-02 (`internal/component/ike/engine/mobike.go:576` and `:593`, the
MOBIKE route-source and NAT-keepalive drops, among them).

The rule (`ai/rules/points/go-standards/directives/log-a-dropped-error-at-warn-or-above.md`):
"An error that is caught and not returned MUST be logged at WARN or above, and
MUST NOT be logged at debug or info." and "Dropping an error is correct only
where going on is correct; a WARN does not make a default correct."

**Goal:** `./le arch refuse-by-default report` lists zero `error-dropped-below-warn`
findings outside the annotated list, tree-wide.

The work needs ten doors, so it is split by door (`ai/META-DECISIONS.md`: "a
spec whose edits need many doors is split into narrower child specs"). Each
child owns the instances under its door's base paths and the zero for them:

| Child | Door | Instances | Bucket |
|-------|------|-----------|--------|
| `plan/release/fixes/spec-dropped-error-1-bgp-engine.md` | `bgp-engine.md` | 57 | release fixes |
| `plan/release/fixes/spec-dropped-error-2-bgp-plugins.md` | `bgp-plugins.md` | 63 | release fixes |
| `plan/release/fixes/spec-dropped-error-3-access-security.md` | `access-security.md` | 60 | release fixes |
| `plan/release/fixes/spec-dropped-error-4-system-plugins.md` | `system-plugins.md` | 45 | release fixes |
| `plan/release/fixes/spec-dropped-error-5-forwarding.md` | `forwarding.md` | 35 | release fixes |
| `plan/release/fixes/spec-dropped-error-6-routing-protocols.md` | `routing-protocols.md` | 28 | release fixes |
| `plan/release/fixes/spec-dropped-error-7-plugin-infra.md` | `plugin-infra.md` | 20 | release fixes |
| `plan/release/fixes/spec-dropped-error-8-core-hub-appliance.md` | `core.md`, `hub-engine.md`, `appliance.md` | 9 | release fixes |
| `plan/release/fixes/spec-dropped-error-9-api-cli-config.md` | `api-surfaces.md`, `commands-cli.md`, `config-yang.md` | 8 | release fixes |
| `plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md` | `system.md` (`internal/chaos`), `le-tooling.md` | 3 | post-release engineering: no shipped binary links them |

A child takes the door whose `door-base` is the longest prefix of each
instance's path (`ai/doors/README.md`), so `internal/component/bgp/plugins/`
is child 2 and `internal/component/config/system/` is child 8.

The umbrella itself changes no code. It holds the resolution policy every child
applies, owns the tree-wide acceptance, and closes after the last child.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/ze-go-style.md` - "A zero value is never an answer", the "Error dropped below WARN" row
  → Constraint: the three resolutions are return the error, count it, or log it at WARN; the annotation is the one escape and names a guard
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: a WARN line an operator reads says what went wrong in their terms, why it matters, and the exact next step; a Go error chain may follow the explanation, never replace it
- [ ] `ai/rules/points/go-standards/directives/log-a-dropped-error-at-warn-or-above.md`
  → Constraint: a WARN does not make a default correct; an instance where going on is wrong is a behavior fix, not a log-level change
- [ ] `ai/doors/le-tooling.md` - the door for the check
  → Constraint: the check is `internal/le`; product code never imports it

**Key insights:**
- The report is the population. Line numbers drift; the report is re-run, never a stored list.
- A branch that records the error (an assignment that mentions it, an increment) or hands it on is not this shape (`recordsMiss`, `mentions` in `refusebydefault.go`).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - `droppedError`, `caughtError`, `belowWarn` (`Debug`, `Info`, their `Context` forms, `Log` at `LevelDebug` or `LevelInfo`), `drops`
- [ ] `internal/le/arch/refusebydefault/annotation.go` - `// refuse-by-default: guards <Symbol> -- <why>`, on the shape's line or the line above, `<Symbol>` visible in the file
- [ ] `internal/le/arch/refusebydefault/refusebydefault.go` - `recordsMiss`, `mentions`, the report's `annotated` and `findings` lists

**Behavior to preserve:**
- The check and its rule text. This work changes the population, not the detector.

**Behavior to change:**
- Every listed instance is resolved by one of the three answers in Key Design Decisions.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError`; `./le arch refuse-by-default report` | Lists 328 instances; `check` refuses a new one on changed lines | Applies unchanged: it is the acceptance check of every child |
| The guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts `refuse-by-default: guards <Symbol> -- <why>` for a deliberate answer | Extends its use: the third resolution, only where going on is correct and nobody can act on the failure |
| Operator warnings bus | `internal/core/report` (`show warnings`, `docs/architecture/core-design.md` section 15) | Holds warnings an operator lists | Applies where a failure is a lasting condition rather than an event; process-local, so a forked plugin uses its log |
| Prometheus metrics registry | `docs/plugin-development/metrics.md` | Per-subsystem counters | Applies for "count it" on a per-packet or per-route path, where a WARN per event would flood |
| User-facing error rule | `ai/rules/user-facing-errors.md` | What, why, fix in every message | Applies to every WARN a child writes |
| Earlier refuse-by-default backlog | `spec-refuse-instead-of-defaulting` (verification) | The original four shapes and the fifth dropped-error check | Bounded overlap: `Reactor.notifyOneMessage` records parse failures, and `buildWithdrawalPayload` / `fwdWithdrawal.of` propagate conversion errors to both forwarding WARN callers. This umbrella's children retain the remaining dropped-error backlog. |
| Discarded-error lint ratchet | `plan/post-release/engineering/spec-lint-ratchet-clock-and-discarded-errors.md` (ready) | `_ = f()` discards with no log at all | Does not overlap: a blank discard has no log line, which this shape does not read |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failure inside a shipped function: an error value returned by a call.

### Transformation Path
1. The branch catches the error.
2. Today: a `Debug` or `Info` line, then the function goes on. With the default WARN level the line is dropped.
3. After: the error is returned to a caller that acts, or counted, or written at WARN with what, why and fix; or the branch carries the annotation naming the guard that makes going on correct.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Function ↔ caller | Returned error | Per child |
| Daemon ↔ operator | Log at WARN, `show warnings`, a metric | Per child |

### Integration Points
- `slogutil.Logger` per subsystem; the level an operator sees by default is WARN.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Errors go to the caller, the log or the registry each subsystem already uses |
| No unintended coupling (components stay isolated) | Yes | No new import; a forked plugin cannot reach `internal/core/report`, so it logs |
| No duplicated functionality (extends existing, does not recreate) | Yes | No new logger, counter family or warning store |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No hot-path buffer is touched; per-packet paths count rather than format |
| Registration over hardcoding, outbound | N-A | Nothing registers |
| Registration over hardcoding, inbound | N-A | No list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The report finds the same population the children list (328 at HEAD 2026-11-02) | `./le arch refuse-by-default report` run on 2026-11-02 | A child's table is stale; it re-runs the report and works from that | Each child runs the report filtered to its paths before its first edit | unvalidated |
| A-2 | Every child's paths partition the population with no overlap | Longest door-base prefix per path, computed for this split | An instance owned twice or never | Sum of child counts equals the report total | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A WARN on a per-packet or per-route path floods the log | One test run writes the line thousands of times | Count it in an existing metric family and WARN on the first occurrence of a condition, never per event |
| R-2 | An annotation used to make the number zero where going on is wrong | Annotation reason restates the drop instead of naming the guard | Review reads each annotation's guard; the rule says a WARN or annotation never makes a default correct |
| R-3 | New instances land while the children work | Report total rises | `check` refuses a new one on changed lines; the umbrella's AC reads the tree at closure |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Logs gain lines, or a caller now sees an error it did not before and changes behavior |
| How is it reverted? | Per child commit |
| Who else touches this path? | Every session touching a listed line, since `check` now refuses the shape there |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over the whole tree | `TestRefuseByDefaultRefusesEachRuleFixture` (the `dropped.go` case) and `TestRefuseByDefaultReportListsTree` in `internal/le/arch/refusebydefault/refusebydefault_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le arch refuse-by-default report '|' json` after the last child closes | No entry in `findings` carries rule `error-dropped-below-warn` |
| AC-2 | Every child spec | Closed, each with its own AC-1 green over its paths |
| AC-3 | Every annotation the children added | Names a guard symbol the file can see and a reason that says why going on is correct (`annotated` list, `guard` field set) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Existing shape tests | `internal/le/arch/refusebydefault/refusebydefault_test.go` | The detector still refuses `testdata/dropped.go` and accepts `testdata/annotated.go` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated findings of the shape | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Per child | Each child names its suites | An operator sees the failure at the default level | |

## Files to Modify
- None in the umbrella. Each child names its files.

## Files to Create
- The ten child specs in the Task table (written with this umbrella).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Per child; no child adds config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new dependency |
| Prometheus counters/metrics | N-A | Per child, where one counts |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | Per child |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | No | The umbrella changes no source |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- run the report, confirm A-1 and A-2.
2. **Phase: Children** -- each child runs as its own spec, in any order; `spec-dropped-error-3-access-security.md` first, since it holds the MOBIKE drops.
3. **Phase: Closure** -- AC-1 over the tree, then close.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Report clean of the shape |
| Rule: refuse-by-default | No annotation hides a drop where going on is wrong |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero unannotated findings of the shape | `./le arch refuse-by-default report '|' json`, filtered on `rule` |
| Detector unchanged and green | `./le job run label unit-pkg command go test ./internal/le/arch/refusebydefault/` |
| Change set judged | `./le verify changed` after each child's closing commit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | A WARN or a returned error that carries a secret (a key, a password, a RADIUS shared secret) in its text |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `dropped-error-logged-below-warn.md` | 2026-11-02 | 328 existing instances the refuse-by-default report lists |
| `dropped-error-logged-below-warn.md` | 2026-11-01 | MOBIKE source lookup `routeSourceTo` (fixed by d6ef7093a8; listed as the class's origin) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Each instance is judged first on whether going on is correct. Where it is not, the error is returned or the behavior fixed | Raise every line to WARN | The rule: a WARN does not make a default correct |
| Where going on is correct and an operator can act: WARN with what, why and fix (`ai/rules/user-facing-errors.md`); on a per-event hot path, count it in the subsystem's existing metrics and WARN once per condition | WARN per event | A per-packet WARN floods the log and hides the next failure |
| Where going on is correct and nobody can act (teardown close, a reply to a client that left): `// refuse-by-default: guards <Symbol> -- <why>` | Debug line kept | The annotation is the check's one accepted escape and names why the drop is the answer |
| Split by door | One spec | Ten doors; META: a spec whose edits need many doors is split |
| Dev tooling instances in post-release engineering | Same bucket | No shipped binary links `internal/chaos` or `internal/test/runner` |

## Known Limitations
- None: every instance is owned by a child.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, per child, and `./le verify changed` over each child's closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-0-umbrella.md` only, in the same `./le commit create` script
