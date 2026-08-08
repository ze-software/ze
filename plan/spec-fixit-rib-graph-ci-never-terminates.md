# Spec: fixit-rib-graph-ci-never-terminates

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` |
| Updated | 2026-08-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`test/plugin/rib-graph.ci` times out at 20s when run ALONE on an unloaded
box, while every one of its own assertions reports OK and every expected
message is received.

A test that finishes its work and then never terminates is a defect in whatever
should signal completion. Three candidates, none yet confirmed against its
producing function: a barrier that never fires, an observer that never exits,
or a shutdown that never arrives.

**Raising the timeout is NOT the fix.** The assertions already pass, so a larger
budget would only make the hang slower to observe. The hang is the finding.

**Found** 2026-08-08 while re-running the plugin suite to verify unrelated
fixes during the repair of GitHub Actions run 31225029268. `rib-graph` was not
one of the 7 stages that run failed, so this is separable and was recorded
rather than fixed, on owner instruction.

**Open questions:** which side fails to signal; whether `ze-peer` in sink mode
ever exits on its own; and whether the hang is a consequence of changes landed
2026-08-08 in the reactor, the route-server plugin, or the plugin process
supervisor. Rebuild before trusting any verdict.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - orientation for the owning layer
  -> Decision: [fill at design time]
  -> Constraint: [fill at design time]

**Key insights:** (minimal context to resume after compaction)
- [fill at design time]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/runner_validate.go` - runner completion and timeout path
- [ ] `internal/component/plugin/process/process.go` - plugin process lifecycle and shutdown

**Behavior to preserve:**
- Every assertion the test makes today. They pass; only termination is broken.

**Behavior to change:**
- [fill at design time]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
[fill at design time]

### Transformation Path
[fill at design time]

### Boundaries Crossed

| From | To | What crosses |
|------|----|--------------|
| [fill at design time] | [fill at design time] | [fill at design time] |

### Integration Points
[fill at design time]

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| [fill at design time] | -> | [fill at design time] | [fill at design time] |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| [fill at design time] | [fill at design time] | the completion signal fires |

### Functional Tests
- [ ] `test/plugin/rib-graph.ci` must terminate on its own, run alone and unloaded

## Files to Modify

Not yet determined. The owning file follows from the diagnosis, which is
the first job of whoever takes this spec.

## Implementation Steps

1. Reproduce, then name the root cause at its producing function.
2. Fix at the owning layer, never at the symptom.
3. Prove the fix discriminates: red before, green after.

## Checklist

### Goal Gates (MUST pass)
- [ ] `test/plugin/rib-graph.ci` terminates without a raised timeout
- [ ] The root cause is named at its producing function, not inferred

### TDD
- [ ] Tests written before the fix
- [ ] Tests FAIL without the fix
- [ ] Tests PASS with the fix
- [ ] `make ze-verify` green before commit
