# Spec: rib-arch-7 -- Multi-Peer LLGR Readvertisement Functional Fixture

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-rib-arch-0-umbrella.md` - set context
4. Existing single-peer fixtures: `test/plugin/llgr-readvertise.ci`, `llgr-rib-stale.ci`, `llgr-transition.ci`
5. `internal/component/bgp/plugins/gr/gr_egress.go` - `LLGREgressFilter`
6. `rfc/short/rfc9494.md`

## Task

LLGR (Long-Lived Graceful Restart, RFC 9494) has **single-peer** functional coverage:
`test/plugin/llgr-readvertise.ci`, `test/plugin/llgr-rib-stale.ci`,
`test/plugin/llgr-transition.ci`, `test/plugin/plugin-gr-llgr-capa.ci`.

GAP: no **multi-peer partial-deployment** fixture. In a real deployment some peers are
LLGR-capable and some are not; the LLGR egress filter (`LLGREgressFilter`,
`internal/component/bgp/plugins/gr/gr_egress.go:57`) tags stale routes with the LLGR_STALE
community for LLGR-capable peers and converts them to withdrawals for non-LLGR EBGP peers
(`filterapi.ModAccumulator.SetWithdraw`, RFC 9494). Add a `.ci` fixture that exercises this
readvertisement split across a mixed set of peers: LLGR-capable peers keep the stale route
(marked LLGR_STALE, depreferenced), non-LLGR peers receive a withdrawal.

Primarily a test-coverage gap. Feature code changes only if the fixture reveals a
correctness bug in the multi-peer path.

## Required Reading

### Architecture Docs
- [ ] `internal/component/bgp/plugins/gr/gr_egress.go` - `LLGREgressFilter` (:57): per-destination stale-route handling
  → Constraint: the fixture must assert BOTH branches (LLGR-capable keep+mark vs non-LLGR withdraw) in one topology.
- [ ] `test/plugin/llgr-readvertise.ci` - the existing single-peer fixture
  → Constraint: extend the pattern; do not duplicate single-peer assertions.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9494.md` - Long-Lived Graceful Restart
  → Constraint: LLGR_STALE community, depreference, and readvertisement-to-non-LLGR-peers rules.

**Key insights:**
- The per-peer branch already exists and is single-peer-tested; the missing coverage is the two branches firing together across a mixed peer set.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; anchors verified 2026-07-08)
- [ ] `internal/component/bgp/plugins/gr/gr_egress.go` - `LLGREgressFilter` (:57): egress filter that, for stale routes, marks LLGR_STALE for LLGR-capable peers and calls `mods.SetWithdraw()` for non-LLGR EBGP peers
- [ ] `internal/component/bgp/filterapi/filterapi.go` - `ModAccumulator.SetWithdraw` (:151): "Used by LLGR egress filter (RFC 9494) to withdraw stale routes from EBGP non-LLGR peers"
- [ ] `test/plugin/llgr-readvertise.ci`, `llgr-rib-stale.ci`, `llgr-transition.ci` - single-peer LLGR fixtures

**Behavior to preserve:**
- All existing LLGR behaviour and single-peer fixtures; this only adds a multi-peer fixture.

**Behavior to change:**
- None expected. Feature code changes only if the fixture uncovers a multi-peer bug.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A peer's graceful-restart window expires; its routes become LLGR-stale, triggering readvertisement to all other peers

### Transformation Path
1. Routes from the restarting peer are marked stale (LLGR)
2. For each destination peer, `LLGREgressFilter` runs (`gr_egress.go:57`)
3. LLGR-capable peer: route kept, tagged LLGR_STALE, depreferenced
4. Non-LLGR EBGP peer: `mods.SetWithdraw()` converts the announce to a withdrawal
5. The `.ci` fixture asserts both outcomes in one mixed-peer topology

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| stale route → egress filter | `LLGREgressFilter` per destination peer | [ ] |
| filter → wire | LLGR_STALE-tagged announce vs withdrawal, per peer capability | [ ] |

### Integration Points
- `LLGREgressFilter` (`gr_egress.go:57`) - the per-peer decision under test
- `ModAccumulator.SetWithdraw` (`filterapi.go:151`) - the withdraw branch
- `test/plugin/*.ci` harness - the multi-peer topology

### Architectural Verification
- [ ] No bypassed layers (fixture drives real peers through the real egress filter)
- [ ] No unintended coupling (fixture-only; no product code coupling introduced)
- [ ] No duplicated functionality (extends single-peer fixtures, not a re-test)
- [ ] Registration over hardcoding - no product registration change; test-only (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `.ci` harness can express a mixed LLGR-capable / non-LLGR multi-peer topology | existing multi-peer plugin fixtures | Need harness support; larger scope | check `test/plugin` harness capabilities at design | unvalidated |
| A-2 | The multi-peer path has no latent bug | single-peer branches pass today | Fixture goes red; fix feature code | run the new fixture at implement | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Fixture reveals a real multi-peer bug (per `feedback_sleep_hides_races` this is the point) | new `.ci` fails | diagnose and fix `gr_egress.go` (or its callers), add a regression assertion |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| GR window expiry with mixed LLGR-capable + non-LLGR peers | → | `LLGREgressFilter` per-peer split (keep+mark vs withdraw) | `test/plugin/llgr-multipeer-readvertise.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Stale routes readvertised to an LLGR-capable peer | route kept, tagged LLGR_STALE, depreferenced |
| AC-2 | Same stale routes readvertised to a non-LLGR EBGP peer | route converted to a withdrawal |
| AC-3 | Both peers present in one topology | both outcomes occur from the same readvertisement event |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (existing unit coverage of `LLGREgressFilter`) | `internal/component/bgp/plugins/gr/gr_egress_test.go` | per-peer branch logic (already covered; extend only if a gap is found) | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `llgr-multipeer-readvertise` (new) | `test/plugin/llgr-multipeer-readvertise.ci` | mixed LLGR-capable + non-LLGR peers: stale routes kept-and-marked vs withdrawn, from one readvertisement | |

### Interop Tests (MANDATORY for protocol features)
- N/A: exercises Ze's own egress split across peers; existing LLGR interop coverage guards wire behaviour. Add interop only if the fixture reveals a wire-format gap.

## Files to Modify

- `internal/component/bgp/plugins/gr/gr_egress.go` - `LLGREgressFilter` (verify only; change only if the fixture reveals a multi-peer bug)

## Implementation Steps

1. **Phase: design** - confirm the harness supports a mixed multi-peer topology (A-1); define the fixture.
2. **Phase: fixture** - write `test/plugin/llgr-multipeer-readvertise.ci` asserting AC-1..AC-3.
3. **Phase: fix (only if red)** - if the fixture uncovers a bug, diagnose and fix `gr_egress.go` (`ai/rules/diagnosis-before-fix.md`).
4. **Full verification** - `make ze-verify`.
5. **Complete spec** - audit, learned summary, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Multi-peer LLGR fixture asserts both branches from one event
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## RFC Documentation

N/A: this is a functional test fixture, not enforcing code. RFC 9494 LLGR behaviour is
already documented at its enforcing site (`internal/component/bgp/plugins/gr/gr_egress.go`,
`filterapi.go:149`). Add `// RFC 9494 Section X.Y` comments only if step 3 changes feature code.

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `spec-rib-arch-0-umbrella.md`.
