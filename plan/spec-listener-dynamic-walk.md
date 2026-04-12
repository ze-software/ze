# Spec: listener-dynamic-walk

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-04-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/component/config/listener.go` -- the hardcoded `knownListenerServices` list
3. `plan/learned/566-iface-wireguard.md` -- context on why this was deferred

## Task

Replace the hardcoded `knownListenerServices` list in
`internal/component/config/listener.go` with a YANG-schema walk that
discovers every `ze:listener`-marked node dynamically. Today the
collector has a fixed Go slice of 8 services (web, ssh, mcp,
looking-glass, prometheus, plugin-hub, api-server-rest, api-server-grpc)
plus a dedicated `collectWireguardListeners` helper that walks
`interface.wireguard` because wireguard uses a flat `leaf listen-port`
instead of the `server` sub-list shape the other services use.

Every time a new service or interface kind adds `ze:listener` in its
YANG module, a Go developer must also add a new entry (or helper) to
`listener.go`. A dynamic walker would derive the collector list from
the schema itself at startup, eliminating the manual bookkeeping and
making `ze:listener` truly self-describing.

**Origin:** deferred from spec-iface-wireguard Phase 5 (D3 resolution).
Recorded in `plan/deferrals.md` 2026-04-12.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` -- listener conflict detection
- [ ] `.claude/rules/config-design.md` -- listener patterns

### Source Files Read
- [ ] `internal/component/config/listener.go` -- `CollectListeners`, `knownListenerServices`, `collectWireguardListeners`, `ListenerEndpoint`, `Protocol`
- [ ] `internal/component/config/yang_schema.go` -- YANG walker, `flattenChildren`
- [ ] `internal/component/config/schema.go` -- `LeafNode`, schema traversal helpers
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` -- `extension listener` definition

## Current Behavior (MANDATORY)

**Source files read:** see Required Reading above.

**Behavior to preserve:**
- All 8 existing TCP services are detected and produce `ListenerEndpoint` entries
- Wireguard UDP listen-port detection via `collectWireguardListeners`
- `Protocol` field (TCP/UDP) on `ListenerEndpoint` respected by `conflicts()`
- `ValidateListenerConflicts` error messages include protocol label

**Behavior to change:**
- `knownListenerServices` replaced by a dynamic list built from YANG schema walk at startup
- `collectWireguardListeners` folded into the generic walker (or kept as a shape-specific helper called by the walker when it encounters a non-`server`-sub-list listener)

## Data Flow (MANDATORY)

### Entry Point
- YANG schema loaded at startup via `config.YANGSchema()`
- Walker traverses schema tree looking for nodes marked `ze:listener`
- For each match: determine the tree path, protocol, and leaf structure

### Transformation Path
1. Schema walk: find all `ze:listener`-marked nodes
2. For each: build a `listenerService` (or equivalent) with the tree path + protocol
3. At config parse time: `CollectListeners` uses the dynamically-built list instead of the hardcoded one

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema -> listener collector | Schema walk at startup | [ ] |

### Integration Points
- `config.YANGSchema()` -- provides the schema tree to walk
- `LeafNode` / `ContainerNode` / `ListNode` -- schema node types to inspect for `ze:listener` extension
- `yang_schema.go` -- may need to expose `ze:listener` as a parsed annotation on schema nodes

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG schema with `ze:listener` on web + wireguard | -> | Dynamic walker produces `ListenerEndpoint` for both | `TestDynamicListenerWalk` in `listener_test.go` |
| YANG schema with a new `ze:listener`-marked service | -> | Walker picks it up without code changes | `TestDynamicListenerNewService` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Existing 8 TCP services with `ze:listener` in YANG | `CollectListeners` returns the same endpoints as the hardcoded list does today |
| AC-2 | Wireguard `ze:listener` with flat `listen-port` (no server sub-list) | Walker handles both shapes (server sub-list and flat leaf) |
| AC-3 | A new YANG module adds `ze:listener` on a container | `CollectListeners` picks it up at next schema load without any Go code change |
| AC-4 | `knownListenerServices` variable deleted | No compilation errors, no runtime regression |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDynamicListenerWalk` | `internal/component/config/listener_test.go` | AC-1 -- all existing services discovered | |
| `TestDynamicListenerWireguard` | `listener_test.go` | AC-2 -- wireguard flat-leaf shape handled | |
| `TestDynamicListenerNewService` | `listener_test.go` | AC-3 -- synthetic YANG module with ze:listener is auto-discovered | |

### Functional Tests
| Test File | End-User Scenario | Status |
|-----------|-------------------|--------|
| N/A -- internal refactor, no user-visible behavior change | | |

## Files to Modify

- `internal/component/config/listener.go` -- replace `knownListenerServices` with dynamic walker, generalize `collectWireguardListeners`
- `internal/component/config/listener_test.go` -- replace hardcoded-list tests with dynamic-walk tests
- `internal/component/config/yang_schema.go` -- expose `ze:listener` as a parsed annotation on schema nodes (if not already surfaced)
- `internal/component/config/schema.go` -- possibly add a `Listener bool` field to `ContainerNode` / `ListNode`

## Files to Create

- None expected (refactor of existing files)

## Implementation Steps

### Implementation Phases

1. **Phase 1: Schema annotation.** Expose `ze:listener` as a parsed flag on `ContainerNode` / `ListNode` in schema.go, populated by yang_schema.go during the YANG walk. Unit test that the flag is set on known listener nodes.
2. **Phase 2: Dynamic walker.** Replace `knownListenerServices` with a schema walk that builds the collector list from annotated nodes. Handle both the `server` sub-list shape and the wireguard flat-leaf shape. Delete `knownListenerServices`. Unit tests for AC-1/2/3.
3. **Phase 3: Cleanup.** Remove `collectWireguardListeners` if the generic walker handles it, or keep it as a shape-specific helper. Ensure AC-4 (no hardcoded list) passes. Full `make ze-verify`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every existing listener service still detected after the refactor |
| No regression | `TestValidateListenerConflicts_*` suite passes unchanged |
| Protocol handling | TCP/UDP distinction preserved for every discovered service |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `knownListenerServices` deleted | `grep -rn knownListenerServices internal/` returns nothing |
| All 8+1 existing services still detected | `TestDynamicListenerWalk` passes |
| New YANG `ze:listener` auto-discovered | `TestDynamicListenerNewService` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Schema walk only matches `ze:listener` extension, not arbitrary annotations |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Existing service not detected | Schema annotation missing or walker bug |
| 3 fix attempts fail | STOP. Report. Ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

N/A -- internal refactor.

## Implementation Summary

### What Was Implemented
(Fill at completion.)

### Bugs Found/Fixed
(Fill at completion.)

### Documentation Updates
(Fill at completion.)

### Deviations from Plan
(Fill at completion.)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| (filled at completion) | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| (filled at completion) | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| (filled at completion) | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| (filled at completion) | | |

### Audit Summary
- **Total items:** TBD
- **Done:** TBD
- **Partial:** TBD (all require user approval)
- **Skipped:** TBD (all require user approval)
- **Changed:** TBD (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| (filled at completion) | | |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| (filled at completion) | | |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| (filled at completion) | | |

## Checklist

### Goal Gates
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-listener-dynamic-walk.md`
