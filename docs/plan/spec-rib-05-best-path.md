# Spec: rib-05 — Best-Path Selection + Pool Review (DEFERRED)

**Status:** Skeleton — not ready for implementation. Complete research and design phases before implementing.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-rib-02-adj-rib-in.md` — prerequisite spec
3. `internal/plugins/bgp-rib/rib.go` - current bgp-rib (to be modified)
4. `docs/architecture/pool-architecture.md` - current pool design

## Task

Two deliverables:

**1. Best-path selection (Loc-RIB):** Modify existing `bgp-rib` to implement Loc-RIB with single best-path per prefix (RFC 4271 Section 9.1.2). Consumes route data from `bgp-adj-rib-in` via dispatch-command notifications (notified on route add/remove). Produces a single routing table with the best path per destination prefix.

**2. Pool architecture review:** The current pool implementation tries to do too much — it mixes NLRI pool handling and attribute pool handling in one design. Simplify: separate concerns, keep attribute dedup (valuable for common values like ORIGIN, LOCAL_PREF), reconsider NLRI pooling approach.

This is the fourth plugin in the RFC 4271 Section 3.2 architecture:

| Plugin | RFC concept | Role |
|--------|-------------|------|
| bgp-adj-rib-in (spec rib-02) | Adj-RIBs-In | Stores all received routes per source peer |
| bgp-rs (spec rib-03) | Route Server | Forward-all, uses adj-rib-in for replay |
| **bgp-rib (this spec)** | Loc-RIB | Single best-path per prefix |

**Depends on:** spec-rib-02-adj-rib-in.md only (parallel with spec rib-03, not sequential)
**Part of series:** rib-01 → rib-02 → rib-03 → rib-04 → rib-05 (this, deferred)

## Key Design Decisions (Pre-Approved)

| Decision | Rationale |
|----------|-----------|
| Single best-path per prefix (NOT per-destination-peer) | No export policy exists yet. Per-destination-peer is YAGNI. Revisit when export policy is added. |
| Keep pool-based attribute dedup in bgp-rib | Dedup is valuable for common attributes (ORIGIN, LOCAL_PREF, MED). Pool architecture is core Ze design. |
| Separate NLRI pools from attribute pools | Current pool mixes both concerns. NLRI storage has different access patterns than attribute dedup. Split into focused designs. |
| Consume from bgp-adj-rib-in via dispatch-command | Notification on route add/remove uses same inter-plugin mechanism as spec rib-03 |
| Reuse JSON event format for notifications | Do not invent a new serialization format. Reuse existing `shared.Event` or `shared.Route` JSON encoding. |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - storage patterns
- [ ] `docs/architecture/pool-architecture.md` - current pool design
  → Review: what works, what's too complex, what mixes NLRI and attribute concerns
- [ ] `docs/architecture/core-design.md` - plugin architecture

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - BGP-4 best-path selection
  → Constraint: Section 9.1.2 defines Decision Process Phase 2
- [ ] `rfc/short/rfc4456.md` - BGP Route Reflection
  → Constraint: ORIGINATOR_ID and CLUSTER_LIST affect tie-breaking

**Key insights:**
- (to be completed during research phase)

## Current Behavior (MANDATORY)

**Source files read:** (must complete before implementation)
- [ ] `internal/plugins/bgp-rib/rib.go` - RIBManager with ribInPool + ribOut
- [ ] `internal/plugins/bgp-rib/storage/routeentry.go` - RouteEntry with per-attribute pool handles
- [ ] `internal/plugins/bgp-rib/storage/familyrib.go` - per-family NLRI → RouteEntry
- [ ] `internal/plugins/bgp-rib/pool/attributes.go` - 13 per-attribute-type pools
- [ ] `internal/component/bgp/attrpool/` - generic pool implementation

**Behavior to preserve:**
- (to be documented after reading source files)

**Behavior to change:**
- (to be documented after reading source files)

## Data Flow (MANDATORY)

### Entry Point
- bgp-adj-rib-in sends notification via DispatchCommand on route add/remove

### Transformation Path
1. bgp-adj-rib-in receives UPDATE event from engine, stores in ribIn
2. bgp-adj-rib-in sends notification via DispatchCommand: route-add or route-del
3. Engine routes to bgp-rib via dispatch-command
4. bgp-rib updates candidate set for that prefix
5. bgp-rib runs best-path selection (pure function on candidate set)
6. If best path changed: bgp-rib sends updated route to peers via updateRoute

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| bgp-adj-rib-in → Engine | DispatchCommand RPC with notification | [ ] |
| Engine → bgp-rib | execute-command callback (dispatch-command routes to plugin) | [ ] |
| bgp-rib → Engine | updateRoute RPC for best-path changes | [ ] |

### Integration Points
- bgp-adj-rib-in handleReceived — sends notification after route insert/remove
- bgp-rib command handler — receives route-add/route-del notifications
- bgp-rib best-path — pure function on candidate set

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Scope Outline (to be expanded into full spec)

### Best-Path Selection Algorithm (RFC 4271 Section 9.1.2)

| Step | Criterion | Higher/Lower Wins | Notes |
|------|-----------|-------------------|-------|
| 1 | Local preference | Highest | Default 100 |
| 2 | AS-path length | Shortest | Count of AS numbers |
| 3 | Origin type | Lowest (IGP < EGP < INCOMPLETE) | |
| 4 | MED | Lowest | Only compare if same neighbor AS |
| 5 | eBGP over iBGP | eBGP preferred | |
| 6 | IGP cost to next-hop | Lowest | Deferred — requires IGP integration |
| 7 | Router ID | Lowest | Final tiebreak |

### Pool Architecture Review Scope

**Current problems:**
- Pool mixes NLRI storage (per-family, prefix-keyed) and attribute dedup (per-type, content-keyed)
- NLRI and attributes have different lifecycle and access patterns
- RouteEntry couples route identity (NLRI) with attribute storage (pool handles)

**Proposed separation:**

| Concern | Current | Proposed |
|---------|---------|----------|
| Attribute dedup | Per-attribute-type pools in `bgp-rib/pool/` | Keep, simplify API — focus on intern/release semantics |
| NLRI storage | Mixed into `storage/familyrib.go` | Separate — NLRI is a map key, not a pool entry |
| RouteEntry | Combines NLRI identity + attribute handles | Split: NLRI key + attribute reference |

### Future (explicitly out of scope)
- Per-destination-peer best-path (requires export policy)
- Export policy / route filtering
- IGP cost comparison (step 6 — requires IGP integration)
- Route dampening

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (to be defined) | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (to be defined) | | | |

## Files to Modify

- `internal/plugins/bgp-rib/rib.go` — remove ribInPool/ribOut, add candidate sets + best-path
- `internal/plugins/bgp-rib/rib_commands.go` — update commands, add notification handlers
- `internal/plugins/bgp-rib/register.go` — update command declarations
- `internal/plugins/bgp-adj-rib-in/rib.go` — add notification dispatch on route add/remove

## Files to Create

- `internal/plugins/bgp-rib/bestpath.go` — pure function best-path selection
- `internal/plugins/bgp-rib/bestpath_test.go` — extensive tests for all criteria

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Command registration | [x] | notification handlers in bgp-rib |
| bgp-adj-rib-in notification | [x] | handleReceived sends DispatchCommand |
| Functional tests | [x] | to be defined |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|

## Implementation Steps


### Failure Routing

| Failure | Route To |
|---------|----------|
| (to be defined) | |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights

- Best-path selection is a pure function on a candidate set — highly testable in isolation
- Notification from bgp-adj-rib-in to bgp-rib uses same dispatch-command mechanism as spec rib-03
- Loc-RIB does not need to store full wire bytes — only parsed attributes for comparison
- This spec depends only on spec rib-02 (bgp-adj-rib-in), NOT spec rib-03 (RS integration)
- Future: route policy/filtering and per-destination-peer selection are separate concerns
- Pool review: separate NLRI pooling from attribute dedup — different access patterns, different lifecycle

## Implementation Summary

### What Was Implemented
- (pending — spec is deferred)

### Bugs Found/Fixed
- (pending)

### Documentation Updates
- (pending)

### Deviations from Plan
- (pending)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] AC defined and demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] `make ze-lint` passes
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
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

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.
