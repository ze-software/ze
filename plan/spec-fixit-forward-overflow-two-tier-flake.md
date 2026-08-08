# Spec: fixit-forward-overflow-two-tier-flake

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

`test/plugin/forward-overflow-two-tier.ci` failed ONCE, with an `ordered:`
needle against a 246-byte `fwdBucketMerge` UPDATE. It has not been reproduced
since and is not attributed to any change in the tree.

**This is unreproduced, so it does not yet meet the bar for a known-failures
shard either.** That path requires a reproduction attempt on the record
(`ai/rules/completion.md`). The first job is to establish whether it reproduces
at all: alone, and with `ZE_PLUGIN_PARALLEL` well above the core count, which
is the setting that reproduced other load-sensitive failures in this suite on
2026-08-08.

If it reproduces, fix the root cause. If a genuine effort fails to reproduce
it, record the attempt and the next step rather than closing silently.

**Found** 2026-08-08 during the repair of GitHub Actions run 31225029268. It
was not one of that run's failures.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - orientation for the owning layer
  -> Decision: [fill at design time]
  -> Constraint: [fill at design time]

**Key insights:** (minimal context to resume after compaction)
- [fill at design time]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer.go` - forward path and bucket merge
- [ ] `internal/test/peer/checker.go` - how an ordered needle is matched

**Behavior to preserve:**
- The existing ordered assertion. It is the thing under suspicion, not a thing to relax.

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
| [fill at design time] | [fill at design time] | the merge produces the ordered bytes |

### Functional Tests
- [ ] `test/plugin/forward-overflow-two-tier.ci` under repeated and oversubscribed runs

## Files to Modify

Not yet determined. The owning file follows from the diagnosis, which is
the first job of whoever takes this spec.

## Implementation Steps

1. Reproduce, then name the root cause at its producing function.
2. Fix at the owning layer, never at the symptom.
3. Prove the fix discriminates: red before, green after.

## Checklist

### Goal Gates (MUST pass)
- [ ] Either the root cause is fixed, or the reproduction attempt and next step are on the record
- [ ] No timeout was raised and no assertion was relaxed to reach green

### TDD
- [ ] Tests written before the fix
- [ ] Tests FAIL without the fix
- [ ] Tests PASS with the fix
- [ ] `make ze-verify` green before commit
