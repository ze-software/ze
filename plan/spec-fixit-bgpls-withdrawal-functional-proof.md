# Spec: fixit-bgpls-withdrawal-functional-proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` |
| Updated | 2026-08-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The route-server opaque (BGP-LS) withdrawal fix landed 2026-08-08 is proven
by UNIT tests only.

`test/plugin/rfc7606-54-bgpls-override-propagates.ci` CANNOT catch the defect on
any platform. It was run with the fix disabled and still passed, because its
assertions stop at the announcement and no route-server `.ci` ever closes a
peer. Only a running daemon proves the daemon
(`ai/rules/interop-and-goal-validation.md`).

The wire-visible defect itself is CLOSED: without the fix, BGP-LS routes a
departing peer contributed were never withdrawn from the other route-server
clients, and the unit tests that now cover it are proven to discriminate. What
is missing is functional proof.

**Two traps an earlier attempt paid for, worth inheriting:** the `.ci` phase
counter is SHARED by every connection of one `ze-peer`, so an assertion at
seq=2 holds the phase below an `action=close` at seq=3 and the test deadlocks.
On darwin the two peers need 15s or more to establish because of the local bind
of 127.0.0.2, which is plausibly the same asymmetry that made this class of
failure appear only on linux CI.

RFC 9552 Section 5.2 requires unknown Link-State NLRI to be handled as opaque
objects and to be preserved and propagated, so the new test should carry the
matching `RFC requirement:` tag.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - orientation for the owning layer
  -> Decision: [fill at design time]
  -> Constraint: [fill at design time]

**Key insights:** (minimal context to resume after compaction)
- [fill at design time]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rs/server_inventory.go` - records the NLRI, now as hex for opaque families
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` - emits the batched withdrawal command

**Behavior to preserve:**
- The landed fix and its unit tests. This spec adds proof, it does not revisit the fix.

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
| [fill at design time] | [fill at design time] | opaque withdrawal reaches the surviving client |

### Functional Tests
- [ ] A new `.ci` that closes a peer and asserts the surviving client receives the BGP-LS withdrawal
- [ ] It must be shown RED with the fix disabled and GREEN with it restored

## Files to Modify

Not yet determined. The owning file follows from the diagnosis, which is
the first job of whoever takes this spec.

## Implementation Steps

1. Reproduce, then name the root cause at its producing function.
2. Fix at the owning layer, never at the symptom.
3. Prove the fix discriminates: red before, green after.

## Checklist

### Goal Gates (MUST pass)
- [ ] A functional test proves the withdrawal against a running daemon
- [ ] The test is proven to discriminate, not merely to pass
- [ ] The RFC 9552 Section 5.2 requirement carries a tagged test

### TDD
- [ ] Tests written before the fix
- [ ] Tests FAIL without the fix
- [ ] Tests PASS with the fix
- [ ] `make ze-verify` green before commit
