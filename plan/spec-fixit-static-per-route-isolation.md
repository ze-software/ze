# Spec: fixit-static-per-route-isolation

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/650-static-routes.md` - the "Blast radius" section that spawned this spec
4. `internal/plugins/static/inject.go`, `internal/plugins/static/register.go`, `internal/plugins/static/diff.go`

## Task

**[MEDIUM]** Today one unresolvable static route fails the WHOLE `static { }` section,
not just its own route. `routeManager.applyRoutes` joins every per-route error with
`errors.Join` (`internal/plugins/static/inject.go:92`) and returns the combined error;
`OnConfigure` (`internal/plugins/static/register.go:134,146`) propagates it, which aborts
daemon startup, and `OnConfigApply` (`register.go:156,170,180`) propagates it on a live
config change. So a single bad next-hop (e.g. an interface next-hop whose backend is
absent, or a device that does not exist yet) takes down every other static route with it.

This whole-section-fail behavior was a **deliberate, documented** choice in
`spec-fixit-static-interface-nexthops` (AC-3 / D-3 = "keep-and-document"), recorded in
`plan/learned/650-static-routes.md` ("Blast radius"). This spec is the deferred follow-up
to evaluate and (if approved) implement **per-route isolation**: log-and-skip a bad route,
keep the rest of the section programmed.

The change is NOT free: it alters the failure contract for ALL static routes and interacts
with `OnConfigApply`'s rollback path (`register.go:180` re-applies `oldRoutes` on failure).
It must be designed against those, not bolted on.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
- [ ] `plan/learned/650-static-routes.md` - the Blast-radius decision + the ECMP/emit/diff invariants this must not break
  → Constraint: any rework of `applyRoutes` MUST preserve the `routesEqual` diff short-circuit (`inject.go:84`, `diff.go:10`), because `WantsConfig` includes `interface` (`register.go:238`) and the diff is what stops an unrelated interface edit from reprogramming every static route.
  → Constraint: forward->forward route replacement must NOT emit a redistribute Remove (flap), forward->non-forward must (emit semantics live in `applyRouteLocked`, `inject.go:95`).
- [ ] `ai/rules/fail-closed-guards.md` - a skipped route must be observable (log + a surfaced state), never a silent no-op.

**Key insights:**
- `applyRoutes` already accumulates per-route errors in a slice (`errs`) and only joins at
  the end (`inject.go:71-92`) — the isolation seam is largely present; the open question is
  the **contract** (what does OnConfigure/OnConfigApply return when SOME routes failed?) and
  **observability** (how does the operator see which routes were skipped?).

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/plugins/static/inject.go` - `applyRoutes` (`:62`) accumulates per-route errors and returns `errors.Join(errs...)` (`:92`); `applyRouteLocked` (`:95`) programs one route and owns the redistribute emit.
  → Constraint: the per-route loop already isolates the FIB write; only the final join + the callers' propagation make it whole-section-fatal.
- [ ] `internal/plugins/static/register.go` - `OnConfigure` (`:134`) calls `applyRoutes` (`:146`) and a non-nil return aborts startup; `OnConfigApply` (`:156`) calls it for the new set (`:170`) and, on failure, re-applies the old set (`:180`, rollback).
- [ ] `internal/plugins/static/diff.go` - `routesEqual` (`:10`) is the unchanged-route short-circuit.

**Behavior to preserve:**
- The `routesEqual` diff short-circuit and the ECMP / redistribute-emit semantics (650).
- `OnConfigApply` rollback: a live edit that fails must not leave a half-applied section.
- Config-time validation still rejects genuinely invalid config (this spec is about
  runtime-unresolvable routes, not syntactic errors).

**Behavior to change:**
- A single unresolvable/failing route should not tear down the rest of the section
  (subject to the contract decided in Key Design Decisions). Exact scope TBD in DESIGN.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Config `static { route ... }` at daemon start (`OnConfigure`) or on a live edit
  (`OnConfigApply`), `internal/plugins/static/register.go`.

### Transformation Path
1. Config sections → parsed `[]staticRoute`.
2. `applyRoutes` (`inject.go:62`) diffs against `rm.routes`, removes stale, applies changed
   via `applyRouteLocked` (`:95`), accumulates errors, `errors.Join` (`:92`).
3. Caller (`OnConfigure`/`OnConfigApply`) propagates the joined error → startup abort or
   rollback.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ static plugin | `OnConfigure`/`OnConfigApply` sdk callbacks | [ ] |
| static ↔ FIB backend | `applyRouteLocked` → netlink / vpp backend | [ ] |
| static ↔ redistribute bus | `emitRouteChange` in `applyRouteLocked` | [ ] |

### Integration Points
- `routeManager.applyRoutes` / `applyRouteLocked` (`inject.go`), `OnConfigure`/`OnConfigApply`
  (`register.go`), the redistribute emit, and any new "skipped routes" observability surface
  (doctor check and/or `static show` column).

### Architectural Verification
- [ ] No bypassed layers (isolation happens in `applyRoutes`, callers unchanged in shape)
- [ ] No unintended coupling (rollback contract preserved)
- [ ] No duplicated functionality (reuse the existing per-route error accumulation)
- [ ] Registration over hardcoding — N/A (no new command/view/family)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The per-route error accumulation in `applyRoutes` (`inject.go:71-92`) is the only place that makes a single failure section-fatal; callers just propagate | read `inject.go` + `register.go` | isolation needs deeper surgery than changing the return contract | re-read both files + trace `OnConfigApply` rollback | unvalidated |
| A-2 | Operators want log-and-skip for RUNTIME-unresolvable routes (absent device/backend), not for syntactically invalid config | 650 Blast-radius note; needs Thomas's product call | wrong default; could hide real config errors | user decision in DESIGN | unvalidated |
| A-3 | Skipping a route on `OnConfigApply` does not corrupt the diff baseline (`rm.routes`) for the next apply | `inject.go` diff logic | subsequent applies mis-diff, leaving stale routes | targeted test over apply→skip→re-apply | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Silent skip masks a genuine misconfiguration | operators surprised by a missing route | REQUIRE observability (doctor check + `static show` skipped state); never a bare log |
| R-2 | Interaction with `OnConfigApply` rollback (`register.go:180`): partial-apply + skip + rollback leaves an inconsistent FIB | rollback test flaps | design the contract so a skip is a deterministic terminal state, and rollback re-derives from `rm.routes` |
| R-3 | Breaking the `routesEqual` short-circuit so an interface edit reprograms all static routes | 650 R-10 regression | keep the diff; add tests asserting an unrelated interface edit reprograms nothing |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `static { route <good> ; route <unresolvable> }` at startup | → | `applyRoutes` skips the bad route, programs the good one, daemon still starts | `TestApplyRoutesSkipsUnresolvableKeepsRest` |

## Acceptance Criteria

<!-- Provisional — finalized in DESIGN once the contract (A-2) is decided with Thomas. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A static section with one runtime-unresolvable route and N good routes, at startup | the N good routes are programmed; the daemon starts; the bad route is skipped (not fatal) |
| AC-2 | Same, on a live `OnConfigApply` edit | good routes applied, bad route skipped, rollback contract intact (no half-applied inconsistency) |
| AC-3 | A skipped route | is observable: a doctor check flags it and/or `static show` marks it, with the reason (missing device / missing `interface { backend }`) |
| AC-4 | A syntactically INVALID config (not runtime-unresolvable) | still rejected at validation time (isolation does not weaken config validation) |
| AC-5 | An unrelated `interface` edit while static routes exist | reprograms NO static route (the `routesEqual` short-circuit is preserved — 650 R-10) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures a static route to an interface whose backend is absent, alongside good routes | `OnConfigure` → `applyRoutes` → skip bad, program good → `static show` marks the skipped route | functional `.ci` (TBD) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyRoutesSkipsUnresolvableKeepsRest` | `internal/plugins/static/inject_test.go` | AC-1 | |
| `TestApplyRoutesSkipPreservesDiffBaseline` | `internal/plugins/static/inject_test.go` | AC-2 / A-3 | |
| `TestUnrelatedInterfaceEditReprogramsNothing` | `internal/plugins/static/inject_test.go` | AC-5 / R-3 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| skipped-route count | 0..N | N | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `static-per-route-isolation` | `test/static/*.ci` | one bad route does not drop the rest of the section | |

### Interop Tests (MANDATORY for protocol features)
- N/A: static-route programming is not a wire protocol.

## Files to Modify
- `internal/plugins/static/inject.go` - `applyRoutes` return contract (skip vs fail) + skipped-route bookkeeping
- `internal/plugins/static/register.go` - `OnConfigure`/`OnConfigApply` handling of a partial-success result
- possibly `internal/plugins/static/doctor.go` - a "skipped static route" doctor check (AC-3)
- possibly the `static show` renderer - a skipped-state column (AC-3)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No | no new config surface (behavior change only) |
| Doctor check for runtime dependencies | [ ] Likely | `internal/plugins/static/doctor.go`, `internal/core/diagnostic/codes.go` (AC-3) |
| Functional test for new behavior | [ ] Yes | `test/static/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | [ ] Verify | `plan/learned/650-static-routes.md` Blast-radius note must be updated if the whole-section-fail contract changes |

## Files to Create
- `internal/plugins/static/inject_test.go` additions (or a new `isolation_test.go`)
- `test/static/static-per-route-isolation.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify / Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 13. /ze-review gate | Review Gate section |
| 14. Close | two-commit closure |

### Implementation Phases

1. **Phase: DESIGN the contract (BLOCKING FIRST)** — decide with Thomas (A-2): does isolation
   apply to runtime-unresolvable routes only? What does `OnConfigure` return on partial success
   (nil + warnings, or a distinct non-fatal error)? How does it compose with `OnConfigApply`
   rollback (R-2)? No code until this is resolved.
2. **Phase: Wiring** — `applyRoutes` returns a partial-success result (applied + skipped);
   failing `TestApplyRoutesSkipsUnresolvableKeepsRest`.
3. **Phase: observability** — doctor check + `static show` skipped state (AC-3).
4. **Phase: rollback safety** — `OnConfigApply` skip + rollback tests (AC-2 / R-2).
5. **Full verification / review / close.**

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has code + test |
| Correctness | ECMP/emit/diff invariants from 650 preserved; rollback contract intact |
| Fail-closed | A skipped route is observable (doctor + show), never a silent drop (`ai/rules/fail-closed-guards.md`) |
| No-regression | `routesEqual` short-circuit preserved (AC-5 / 650 R-10) |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Failure visibility | a skipped route must not silently reduce reachability without an operator-visible signal |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Contract unclear | STOP — this is the A-2 product decision; ask Thomas |
| Rollback test flaps | Re-read `OnConfigApply` (`register.go:156-180`); the contract, not the test, is likely wrong |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| (OPEN) isolation scope: runtime-unresolvable only vs all failures | whole-section fail (status quo, kept in fixit-static-interface-nexthops) | A-2 — needs Thomas's product call before implementation |

## Known Limitations
- Skeleton only. The contract (A-2) and the observability surface (AC-3) are unresolved and
  must be decided in the DESIGN phase before any implementation.
- Scope is the static plugin's own config-apply path; it does not change sysrib/fib-kernel or
  BGP RIB behavior.

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | (not yet run — skeleton) | | | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)

## Notes
- Created 2026-07-19 as the destination for the per-route-isolation deferral recorded in
  `plan/deferrals.md` and `plan/learned/650-static-routes.md` when `fixit-static-interface-nexthops`
  chose (D-3) to keep-and-document the whole-section-fail behavior. Grounded in the current
  `internal/plugins/static/` code; NOT yet researched to `design` depth (status: skeleton).
