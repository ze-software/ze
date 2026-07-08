# Spec: improve-4 -- File-Driven Protocol Conformance Fixtures

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-improve-3-event-replay |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-improve-0-umbrella.md` -- set context
4. `plan/spec-improve-3-event-replay.md` -- capture format this consumes
5. `docs/functional-tests.md` -- existing .ci harness

## Task

Ze has many test layers (.ci functional tests, interop scenarios against FRR/BIRD/
GoBGP, exabgp-compat, stress, QEMU), but no shared file-driven fixture format where one
directory tells a protocol scenario end to end: input config, input event stream,
expected operational state, expected outbound wire events, expected diagnostics. With
such a format, a maintainer can read a scenario without reading harness code, the
fixture doubles as a protocol narrative, and a captured production bug can be dropped
in as a new regression test.

Define `test/protocol/<name>/<scenario>/` fixtures whose event-stream input is the
spec-improve-3 capture format (JSONL), and a runner that loads config, replays the
stream, and diffs actual vs expected state/output/diagnostics. Build exactly ONE BGP
fixture first to prove the format; broadening to more scenarios and protocols is
explicitly follow-up work, not this spec.

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - .ci harness capabilities and conventions
  → Decision: (fill during design -- extend .ci runner vs dedicated fixture runner)
- [ ] `plan/spec-improve-3-event-replay.md` - capture/replay machinery this reuses
  → Constraint: fixture event streams use the versioned capture schema, no second format
- [ ] `ai/rules/functional-test-gate.md` - where fixture tests sit relative to .ci gate
  → Constraint: (fill during design)
- [ ] `plan/deterministic-simulation-analysis.md` - determinism requirements for stable expected-output diffs
  → Constraint: (fill during design)

### RFC Summaries (MUST for protocol work)
- First fixture exercises existing RFC 4271 behavior; no new protocol claims. Cite
  `rfc/short/rfc4271.md` sections in the fixture's README during design.

**Key insights:**
- The fixture format's value is that production captures (spec-improve-3) become
  regression tests with an expected-output block added.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/session.go` - session loop a fixture run drives via replay (:706)
- [ ] `internal/component/plugin/server/event_ring.go` - diagnostics trail a fixture can assert on (:47)
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - state dump commands usable as expected-state probes (:250-274)
- [ ] `test/` directory layout - existing suites (functional .ci, interop, exabgp-compat, stress) to confirm no equivalent fixture format exists (survey during design)

**Behavior to preserve:** (unless user explicitly said to change)
- Existing .ci, interop, exabgp-compat, and stress suites unchanged; fixtures are a new
  suite, not a migration.
- `make ze-verify` stage list changes only by adding the fixture stage (design decision
  whether it joins ze-functional-test or gets its own target).

**Behavior to change:** (only if user explicitly requested)
- None; additive test infrastructure.

## Data Flow (MANDATORY)

### Entry Point
- `make ze-conformance-test` (name per design) discovers `test/protocol/*/*/` fixture
  directories and runs each scenario.

### Transformation Path
1. Runner loads the scenario's input config and starts the component under test (BGP session harness from spec-improve-3).
2. Runner replays the scenario's JSONL event stream through the real processing path.
3. Runner collects actual operational state (via existing dump/show commands), outbound wire events, and diagnostics.
4. Runner diffs actual vs the scenario's expected files; mismatch fails with a readable diff naming the file and field.
5. A new scenario is authored by capturing (spec-improve-3) or hand-writing a stream, then recording expected outputs.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Fixture files ↔ runner | directory convention + versioned schemas | [ ] |
| Runner ↔ session | spec-improve-3 replay harness (same path as production) | [ ] |
| Runner ↔ state probes | existing dump/show command outputs | [ ] |

### Integration Points
- spec-improve-3 replay harness - drives the input stream.
- Existing state dump commands (adj-rib-in dump and peers/summary equivalents) - expected-state probes.
- `mk/` test targets + `scripts/status/verify_run.go` stage list - runner invocation.

### Architectural Verification
- [ ] No bypassed layers (fixtures exercise the real read/process path via replay)
- [ ] No unintended coupling (runner depends on capture format package + CLI probes only)
- [ ] No duplicated functionality (does not reimplement .ci or interop suites)
- [ ] Registration over hardcoding -- runner discovers fixtures from the directory tree; per-protocol probes register, no scenario switch in the runner (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | State/output dumps are deterministic enough to diff byte-stably (ordering, timestamps) | JSON format rules exist (`ai/rules/json-format.md`) | Expected files flake; need canonicalization pass | Run the first fixture 100x during implementation | unvalidated |
| A-2 | The spec-improve-3 replay harness can expose outbound wire events for assertion | replay harness design (spec-improve-3) | Fixtures assert state only in v1 | Coordinate with spec-improve-3 design | unvalidated |
| A-3 | One directory format fits future protocols (OSPF/IS-IS) without redesign | the review reports another daemon running 10+ protocols on one such format (unverified) | Format revision needed when a second protocol lands | Sketch an OSPF scenario on paper during design (no implementation) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Expected-output files rot as features evolve | frequent fixture churn in PRs | keep expected files small and semantic (state slices, not full dumps); regeneration command with review diff |
| R-2 | Fixture suite duplicates what a .ci already covers, doubling maintenance | design-phase overlap review | first fixture targets a scenario .ci cannot express (byte-level captured stream in, state out) |
| R-3 | Runner grows protocol-specific logic | code review | probes register per protocol; runner stays generic |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| make ze-conformance-test | → | fixture discovery + runner | TestFixtureRunnerDiscovers |
| first BGP scenario directory | → | replay -> state diff -> pass/fail | test/protocol/bgp/basic-session scenario (runner-executed) |
| deliberately broken expected file | → | readable diff failure | TestFixtureRunnerFailsWithDiff |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-conformance-test` | Discovers and runs all `test/protocol/*/*/` scenarios |
| AC-2 | First BGP scenario (session establish + UPDATEs) | Passes: actual state/output/diagnostics match expected files |
| AC-3 | Mutated expected file | Fails with a diff naming file and mismatching field |
| AC-4 | Scenario missing a required file | Clear error naming the missing file, not a panic |
| AC-5 | Same scenario run repeatedly | Identical result (determinism, A-1) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Developer converts a spec-improve-3 production capture into a regression test | capture file + config + recorded expected state -> new scenario dir | test/protocol/bgp/basic-session scenario |
| 2 | Maintainer reads a scenario to understand protocol behavior | fixture directory is self-describing | scenario README convention (design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestFixtureRunnerDiscovers | runner package test | directory discovery, schema validation | |
| TestFixtureRunnerFailsWithDiff | runner package test | mismatch reporting quality | |
| TestFixtureCanonicalization | runner package test | deterministic diffable output (A-1) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (none numeric; N/A) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| basic-session | `test/protocol/bgp/basic-session/` (runner suite; harness-level equivalent of a .ci) | replayed session produces expected RIB state + outbound events | |
| runner-gate | `test/protocol/runner.ci` (if .ci integration chosen at design) | conformance stage wired into functional gate | |

### Interop Tests (MANDATORY for protocol features)
- N/A: no new wire behavior; interop suites remain the cross-daemon check.

## Files to Modify
- `mk/` test makefiles - `ze-conformance-test` target
- `scripts/status/verify_run.go` - add stage (design decision: default vs opt-in) (:112-146 stage lists)
- `docs/functional-tests.md` - document the fixture format

## Files to Create
- fixture runner (location per module tiers during design)
- `test/protocol/bgp/basic-session/` - config, events.jsonl, expected-state, expected-output, expected-diagnostics, README
- `test/protocol/README.md` - format specification

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - runner skeleton + make target + failing discovery test
2. **Phase: format + first scenario** - schema, canonicalization, basic-session fixture
3. **Phase: diff quality + failure modes** (AC-3, AC-4)
4. **Phase: verify integration** - stage wiring, determinism soak (AC-5)
5. `make ze-verify`, learned summary, two-commit closure

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-5 with file:line |
| Correctness | replay path is the production path; expected files semantic, not dump-everything |
| Registration over hardcoding | probes register per protocol; runner generic (`ai/rules/plugin-self-containment.md`) |
| Rule: no-layering | no parallel mini-.ci dialect; one fixture format |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | runner parses fixture files defensively (they may come from captures of hostile peers) |
| Resource exhaustion | scenario timeouts; bounded replay |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- (fill during design)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One BGP fixture first | fixture sets for every protocol | prove the format before spending on breadth (the review's own advice) |
| Reuse spec-improve-3 capture schema for event input | separate fixture event dialect | production captures become fixtures with zero translation |

## Known Limitations
- v1 covers single-session BGP scenarios; topology/convergence fixtures are follow-up.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
