# Spec: dropped-error-10-dev-tooling

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P3 |
| Release reason | Release-independent: 3 dropped errors in the chaos dashboard and the functional test runner, which no shipped binary links; they block only the tree-wide zero of `plan/release/fixes/spec-dropped-error-0-umbrella.md` |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Doors: `ai/doors/system.md`
(`internal/chaos`, which no subsystem door covers) and `ai/doors/le-tooling.md`
(`internal/test`).

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under `internal/chaos/` and
`internal/test/`, plus any under `internal/le/`. At HEAD on 2026-11-02:
**3 instances in 2 files**.

| Instance | Today | Likely answer |
|----------|-------|---------------|
| `internal/test/runner/peer_contract.go:393`, `EncodingTests.validateOnePeerBlock` | The runner's own parse of a `ze-peer` `expect=` line fails; its comment says that copy is reporting only, so the error stays a debug log | Answer 3 if the read confirms `ze-peer` parses and refuses the same line itself (the guard to name); otherwise answer 2, since a `.ci` author can fix the line |
| `internal/chaos/web/handlers.go:347`, `Dashboard.handlePeerPin` | A failed response or state step logged at Debug | Read, then answer 2 or 3 |
| `internal/chaos/web/handlers.go:385`, `Dashboard.handleSidebarStats` | Same | Same |

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions): 1, return it where going on is wrong; 2, WARN
with what, why and the fix where a developer can act; 3,
`// refuse-by-default: guards <Symbol> -- <why>` where nobody can act.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/le-tooling.md`
  → Constraint: the runner can make a test vacuous (`plan/journal/green-that-could-not-have-been-red.md`, 2026-09-06); an `expect=` line the runner cannot parse must not let a `.ci` pass on a line nobody checked
  → Constraint: never import `internal/le` from product code
- [ ] `ai/doors/system.md`
  → Constraint: `internal/chaos` is development tooling under `internal/`; no shipped flavor links it
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` format the runner reads
  → Constraint: `ze-peer` owns its peer block; the runner's copy is for progress output
- [ ] `docs/contributing/ze-go-style.md` - "Error dropped below WARN"
  → Constraint: return, count, or WARN; the annotation names a guard

**Key insights:**
- The runner instance is the one with a real risk: if `ze-peer` were not to refuse the same line, the debug log would hide a vacuous expectation.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - the shape
- [ ] `internal/test/runner/peer_contract.go` - `validateOnePeerBlock`: the runner's parse of a peer `expect=` or `action=` line, errors at Debug

**Behavior to preserve:**
- The `.ci` format and the chaos dashboard's pages.

**Behavior to change:**
- Each listed instance resolved by one answer.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists these 3 | Applies: the acceptance check |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts a named guard | Applies: answer 3 only |
| `ze-peer` expectation parser | `internal/test/peer` | Owns and refuses its own peer block lines | Applies: the guard the runner instance names, if it refuses the same line |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test <suite> <name>` reading a `.ci`; a browser request to the chaos dashboard.

### Transformation Path
1. The branch catches the error.
2. It WARNs or carries the annotation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Runner ↔ `ze-peer` | The peer block on the peer's stdin | No |

### Integration Points
- The runner's record logger and the dashboard's logger.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Existing loggers |
| No unintended coupling (components stay isolated) | Yes | No new import |
| No duplicated functionality (extends existing, does not recreate) | Yes | No second parser |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Tooling |
| Registration over hardcoding, outbound | N-A | Nothing registers |
| Registration over hardcoding, inbound | N-A | No list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ze-peer` refuses an `expect=` line the runner cannot parse | The comment at `peer_contract.go:390` | The runner's debug log hides a vacuous expectation: answer 2 or a refusal | A `.ci` with a malformed `expect=` line run through `./le test functional runner` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A WARN in the runner prints on every run of a `.ci` with a line the runner's copy does not understand but `ze-peer` does | Runner output gains a line per test | Only lines both refuse are reported |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Runner or dashboard output only |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Every session running functional tests |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over this child's paths | Report filtered to `internal/chaos/`, `internal/test/`, `internal/le/`: zero findings of the shape |
| A `.ci` peer block with a malformed `expect=` line | → | `EncodingTests.validateOnePeerBlock` | `TestPeerBlockMalformedExpectIsReportedOrRefused` (runner package) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to this child's paths | No `findings` entry with rule `error-dropped-below-warn` |
| AC-2 | A `.ci` with a malformed peer `expect=` line | The run fails or warns naming the line; it never passes silently |
| AC-3 | Each annotation | Names a guard the file can see |
| AC-4 | `./le test functional runner` and the runner package tests | Green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerBlockMalformedExpectIsReportedOrRefused` | `internal/test/runner/peer_contract_test.go` | AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated findings under this child's paths | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/runner/*.ci` | `test/runner/` | The runner primitives still pass | |

## Files to Modify
- `internal/chaos/web/handlers.go` - 2: `Dashboard.handlePeerPin` 347; `Dashboard.handleSidebarStats` 385
- `internal/test/runner/peer_contract.go` - 1: `EncodingTests.validateOnePeerBlock` 393

Lines are those of HEAD on 2026-11-02; the report is the current list.

## Files to Create
- Test additions in the runner package; no new source file.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Tooling |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output change |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No dependency |
| Prometheus counters/metrics | N-A | No counter |
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
| 10 | Test infrastructure changed? | If AC-2 makes the runner refuse | `docs/architecture/testing/ci-format.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `docs/architecture/chaos-web-dashboard.md`, `docs/architecture/testing/ci-format.md`; confirm with `./le spec citation anchors spec plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- run the report filtered to this child's paths; validate A-1 with a malformed `.ci` line.
2. **Phase: Resolve** -- the runner instance by A-1's answer, the two dashboard instances after reading them.
3. **Phase: Prove** -- AC-1 to AC-4.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 3 instances answered at file:line |
| Correctness | No `.ci` can pass on an expectation nobody parsed |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered by path |
| Runner | `./le test functional runner` and `./le job run label unit-pkg command go test ./internal/test/runner/` |
| Change set judged | `./le verify changed` after the closing commit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N-A: test inputs only |

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

The class row for the whole population is owned by the umbrella.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Post-release engineering bucket | Release fixes with the others | No shipped binary links these files |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, tooling

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md` only, in the same `./le commit create` script
