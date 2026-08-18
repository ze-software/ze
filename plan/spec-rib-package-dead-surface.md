# Spec: the rest of the dead surface in the bgp rib package

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`RouteStore` and the `internal/component/bgp/store` package were removed because
the Loc-RIB moved out of the engine into the `bgp-rib` plugin and pool handles
replaced the mutex-based typed stores. That deletion took the smallest coherent
unit. The same package still carries a larger dead surface, and this spec exists
so the question is answered deliberately rather than by whoever next trips over
it.

Verified by word-boundary search over `internal/`, `cmd/` and `pkg/`, excluding
test files:

| Symbol | File | Production callers |
|--------|------|--------------------|
| `IncomingRIB` and its methods | `internal/component/bgp/rib/incoming.go` | none; `newIncomingRIB` is unexported and called only from tests |
| `OutgoingRIB` and its methods | `internal/component/bgp/rib/outgoing.go` | none; `newOutgoingRIB` is unexported and called only from tests. One comment in `internal/component/bgp/reactor/reactor_api_batch.go` mentions the type and no code references it |
| `NewRoute` | `internal/component/bgp/rib/route.go` | none |
| `NewRouteWithWireCache` | `internal/component/bgp/rib/route.go` | none |
| `NewRouteWithWireCacheFull` | `internal/component/bgp/rib/route.go` | none |
| `Route.Acquire`, `Route.Release`, `Route.RefCount` | `internal/component/bgp/rib/route.go` | none once `RouteStore` is gone; it held the only two |

`NewRouteWithASPath` (`internal/component/bgp/rib/route.go`) IS live and the
reactor uses it, so `Route` itself stays. The transaction, commit and grouping
files in the same package need the same treatment before anything is removed:
this spec must establish, per file, which of them the reactor still reaches.

Goal: decide the disposition of each row above, then act. Deleting is the
default under `ai/rules/no-layering.md` and `ai/rules/simplicity.md`, and it is
not automatic: `OutgoingRIB` carries a transaction and rollback surface that no
other type in the tree provides, so if a planned consumer exists the answer is
to wire it rather than remove it. Establish that first.

One consequence to plan for: `plan/journal/refcount-released-outside-the-lock.md`
cites the deleted `releaseRoute`, and removing the refcount methods leaves that
row citing nothing. The row is a historical record and must survive; decide how
its citation is marked rather than deleting the row.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/DESIGN-HISTORY.md` - records that best-path lives in the bgp-rib plugin and the engine holds no Loc-RIB
  → Decision: <to be filled>
  → Constraint: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/rib/incoming.go` - Adj-RIB-In, no production caller
- [ ] `internal/component/bgp/rib/outgoing.go` - Adj-RIB-Out with a transaction surface, no production caller
- [ ] `internal/component/bgp/rib/route.go` - three unused constructors beside the live `NewRouteWithASPath`

**Behavior to preserve:**
- <to be filled>

**Behavior to change:**
- <to be filled>

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- <to be filled>

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | From | To |
|----------|------|-----|
| <to be filled> | <to be filled> | <to be filled> |

### Integration Points
- <to be filled>

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| <to be filled> | → | <to be filled> | <to be filled> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| <to be filled> | <to be filled> | <to be filled> |

### Functional Tests

No user-facing behavior changes: every symbol listed has no production caller,
so removing it cannot alter what the daemon does. The evidence that the removal
is safe is the build and the existing suites staying green, not a new test.

## Files to Modify

- `internal/component/bgp/rib/incoming.go` - <what changes>
- `internal/component/bgp/rib/outgoing.go` - <what changes>
- `internal/component/bgp/rib/route.go` - <what changes>

## Implementation Steps

1. <to be filled>

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] `make ze-precommit-verify` green

### Integration Checklist
- [ ] <to be filled>

### Documentation Update Checklist
- [ ] <to be filled>
