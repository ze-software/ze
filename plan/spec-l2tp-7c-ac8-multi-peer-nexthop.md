# Spec: l2tp-7c-ac8-multi-peer-nexthop

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-l2tp-7c-redistribute (done) |
| Phase | 1/1 |
| Updated | 2026-04-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/learned/641-l2tp-7c-redistribute.md` -- predecessor decisions
3. `internal/component/bgp/plugins/redistribute/format.go` -- `nhop self` emission
4. `internal/component/bgp/reactor/peer.go:572` -- `resolveNextHop()`
5. `test/plugin/redistribute-l2tp-announce.ci` -- single-peer precedent

## Task

Add a .ci functional test proving that when two BGP peers have distinct
local session addresses, each peer's UPDATE for a redistributed L2TP
subscriber route carries that peer's own local address as NEXT_HOP.
This closes AC-8 from spec-l2tp-7c-redistribute.

The NEXT_HOP resolution logic already works (redistribute emits
`nhop self`; reactor's `resolveNextHop` substitutes per-peer
`settings.LocalAddress`). What is missing is a test that proves it
end-to-end with two peers.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/641-l2tp-7c-redistribute.md` -- fakel2tp pattern, per-family emission
  -> Constraint: fakel2tp emits via dispatch-command; reuse same pattern
- [ ] `docs/guide/l2tp.md` Redistribute section -- config shape
  -> Constraint: `redistribute { import l2tp { family [...]; } }`

### Source Files
- [ ] `internal/component/bgp/plugins/redistribute/format.go:58` -- emits `nhop self`
  -> Decision: redistribute always requests `nhop self` for subscriber routes
- [ ] `internal/component/bgp/reactor/peer.go:572-596` -- `resolveNextHop()`
  -> Constraint: `nhop self` resolves to `p.settings.LocalAddress` per peer
- [ ] `test/plugin/redistribute-l2tp-announce.ci` -- single-peer test structure
  -> Constraint: reuse same config shape, add second peer + second ze-peer

**Key insights:**
- `nhop self` is hardcoded in redistribute format.go for subscriber routes
- `resolveNextHop` reads `p.settings.LocalAddress` which comes from the peer's `connection { local { ip ... } }` config
- Two ze-peer instances needed, each on its own port, each expecting a different NEXT_HOP

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/plugin/redistribute-l2tp-announce.ci` -- single peer, local 127.0.0.1, expects NEXT_HOP 127.0.0.1
- [ ] `internal/component/bgp/plugins/redistribute/format.go` -- always `nhop self`

**Behavior to preserve:**
- Single-peer announce test continues to pass
- `nhop self` resolution path unchanged

**Behavior to change:**
- None -- this is a test-only spec

## Data Flow (MANDATORY)

### Entry Point
- fakel2tp dispatch-command emits RouteChangeBatch on EventBus

### Transformation Path
1. bgp-redistribute receives batch, builds `"update text origin incomplete nhop self nlri ipv4/unicast add 10.0.0.1/32"`
2. Dispatches `UpdateRoute(ctx, "*", cmd)` -- selector `*` means all peers
3. Reactor fans out to each peer; each peer's `resolveNextHop()` converts `nhop self` to its own `LocalAddress`
4. Each peer sends its own UPDATE with its own NEXT_HOP

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| EventBus -> redistribute plugin | Typed event subscription | redistribute-l2tp-announce.ci |
| redistribute -> reactor | UpdateRoute RPC with `nhop self` | redistribute-l2tp-announce.ci |
| reactor -> per-peer egress | Fan-out with per-peer resolveNextHop | **This spec** |

### Integration Points
- `resolveNextHop()` at `peer.go:572` -- already implemented, needs test coverage

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| fakel2tp emit -> redistribute -> reactor fan-out | -> | `resolveNextHop()` per peer | `redistribute-l2tp-multi-peer-nexthop.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two BGP peers: peer1 local 127.0.0.1, peer2 local 127.0.0.2; fakel2tp emits add ipv4/unicast 10.0.0.1/32 | peer1 receives UPDATE with NEXT_HOP=127.0.0.1; peer2 receives UPDATE with NEXT_HOP=127.0.0.2 |

## TDD Test Plan

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `redistribute-l2tp-multi-peer-nexthop` | `test/plugin/redistribute-l2tp-multi-peer-nexthop.ci` | Two peers with different local addresses both receive UPDATE, each with own NEXT_HOP | |

## Files to Modify
- None (test-only spec)

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create
- `test/plugin/redistribute-l2tp-multi-peer-nexthop.ci` -- two-peer NEXT_HOP test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Check redistribute-l2tp-announce.ci exists as template |
| 3. Implement | Write .ci test (Phase 1 below) |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-verify-fast` |
| 13. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: two-peer .ci test** -- write redistribute-l2tp-multi-peer-nexthop.ci
   - Config: two BGP peers with distinct `connection { local { ip } }` values
   - Two ze-peer background processes, each on its own port
   - Python observer: fakel2tp emit add ipv4/unicast 10.0.0.1/32
   - ze-peer 1 expects UPDATE with NEXT_HOP=127.0.0.1
   - ze-peer 2 expects UPDATE with NEXT_HOP=127.0.0.2
   - Verify: `bin/ze-test bgp plugin redistribute-l2tp-multi-peer-nexthop`
2. **Full verification** -- `make ze-verify-fast`

### Design Notes

The .ci test needs two ze-peer processes. The test runner supports
multiple `cmd=background` directives with different `seq` values. Each
ze-peer binds its own `$PORT` (auto-allocated). The ze config declares
two peers, each connecting to its respective ze-peer port.

The key config difference between the two peers:
- peer1: `connection { local { ip 127.0.0.1 } remote { ip 127.0.0.1 } }`
- peer2: `connection { local { ip 127.0.0.2 } remote { ip 127.0.0.2 } }`

Each ze-peer's `expect=json` assertion checks for its own NEXT_HOP value.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Two-peer .ci functional test | Done | `test/plugin/redistribute-l2tp-multi-peer-nexthop.ci` | Passes in 6.1s |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `redistribute-l2tp-multi-peer-nexthop.ci` expect=json assertions: peer1 NEXT_HOP=127.0.0.1, peer2 NEXT_HOP=127.0.0.2 | Both ze-peer instances receive correct per-peer NEXT_HOP |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| redistribute-l2tp-multi-peer-nexthop | Done | `test/plugin/redistribute-l2tp-multi-peer-nexthop.ci` | `bin/ze-test bgp plugin redistribute-l2tp-multi-peer-nexthop` passes |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `test/plugin/redistribute-l2tp-multi-peer-nexthop.ci` | Created | Two-peer NEXT_HOP test |

### Audit Summary
- **Total items:** 4
- **Done:** 4
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/redistribute-l2tp-multi-peer-nexthop.ci` | Yes | Created this session |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Two peers get distinct NEXT_HOP | `bin/ze-test bgp plugin redistribute-l2tp-multi-peer-nexthop` -- pass 1/1 100.0% 6.1s |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| fakel2tp emit -> redistribute -> reactor fan-out | `redistribute-l2tp-multi-peer-nexthop.ci` | Yes -- expect=json checks NEXT_HOP per peer |

## Review Gate
<!-- Fill after /ze-review -->
