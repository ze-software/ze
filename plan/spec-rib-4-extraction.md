# Spec: BGP RIB Multi-Source Support

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-05-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/rib/rib.go` - RIBManager, ribInPool, bgpProtocolID
4. `internal/component/bgp/plugins/rib/rib_commands.go` - gatherCandidatesLocked
5. `internal/core/redistevents/registry.go` - ProtocolID type

## Task

The BGP RIB plugin (`internal/component/bgp/plugins/rib/`, registered as `"bgp-rib"`)
stores routes in `ribInPool map[string]*storage.PeerRIB`, keyed by bare peer address.
This prevents accepting BGP-wire-format routes from other sources (BMP Route Monitoring)
without peer address collisions.

This spec changes `ribInPool` to a two-level map keyed by ProtocolID then peer address:

```go
ribInPool map[redistevents.ProtocolID]map[string]*storage.PeerRIB
bgpPeers  map[string]*storage.PeerRIB // = ribInPool[bgpProtocolID], cached at init
```

The outer map holds one inner map per source protocol. `bgpPeers` is a cached reference
to `ribInPool[bgpProtocolID]`, set once at construction. BGP handlers use `r.bgpPeers`
directly, so the hot path has zero additional indirection compared to today. BMP (when
implemented) will cache its own reference similarly. Collision is impossible by
construction: each source writes to its own inner map.

Best-path selection (`gatherCandidatesLocked`) iterates only `r.bgpPeers` (not all
protocols). Monitor-only protocols are structurally excluded. Show commands iterate all
protocol slots when displaying routes.

The BGP RIB stays a BGP plugin (`"bgp-rib"`). The plugin name, package location,
command namespace, and YANG schema are unchanged.

### What This Spec Does NOT Do

| Excluded | Reason |
|----------|--------|
| Plugin rename (`"bgp-rib"` -> `"rib"`) | RIB stays a BGP plugin. `plugin-design.md` "Renaming a Registered Name": 6-day silent failure precedent. |
| Package move | No functional effect, high mechanical cost. |
| Command namespace change | Commands stay `bgp rib ...`. |
| Decoder registration | BMP uses the same BGP wire format. `design-principles.md`: "No premature abstraction." |
| `StaleLevelMonitorOnly` constant | Structural separation via outer map key makes per-entry flags unnecessary for protocol exclusion. |
| LG protocol source display | Belongs in the LG/locrib layer. |

### Design Context

Design review for `spec-bmp-6-looking-glass.md` identified that BMP routes need storage
for looking glass queries. BMP Route Monitoring carries literal BGP UPDATEs (same wire
format, same attributes, same NLRI families).

Research of open-source implementations (BIRD, FRR, GoBGP):
- All keep the BGP adj-RIB BGP-specific
- All use a separate central/unified RIB for cross-protocol aggregation
- All treat BMP as a read-only observer, not a route injector

Ze diverges intentionally: the BGP RIB accepts BGP-wire-format routes from multiple
sources. This reuses wire parsing, attrpool dedup, NLRI splitting, and show commands.
The two-level map keeps each source in its own namespace with zero hot-path overhead.

**Performance analysis:** string-prefix keys (`"1:10.0.0.1"`) cost one alloc+concat per
insert/lookup. Composite struct keys add struct hashing overhead. A separate named field
(`ribInMonitor`) is zero-cost but doesn't generalize. The `map[ProtocolID]map[string]*PeerRIB`
approach costs one integer map lookup (uint16 key) and generalizes to N sources.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RIB storage internals
- [ ] `ai/rules/design-principles.md` - YAGNI, no premature abstraction

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - BGP RIB concepts (Adj-RIB-In, Loc-RIB)

**Key insights:**
- `ribInPool map[string]*PeerRIB` is the data structure to change
- `bgpProtocolID` var already exists at `rib.go:432` via `redistevents.RegisterProtocol("bgp")`
- `redistevents.ProtocolID` is `uint16`, suitable as a map key
- BMP will register its own ProtocolID via `redistevents.RegisterProtocol("bmp")`
- `gatherCandidatesLocked` currently iterates the flat ribInPool; with the two-level map it iterates only the BGP slot

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `ribInPool map[string]*PeerRIB`, all access sites
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - gatherCandidatesLocked, injectRoute, withdrawRoute, inboundEmptyJSON, retainRoutesJSON, statusJSON
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` - handleReceivedStructured ribInPool access
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - bestCandidateNextHopAddr, lookupLabelsForBest, purgeBestPrevForPeer use ribInPool
- [ ] `internal/core/redistevents/registry.go` - ProtocolID type, RegisterProtocol

**Behavior to preserve:**
- All existing BGP RIB functionality (adj-rib-in, adj-rib-out, best-path, locRIB mirror)
- All existing RIB CLI commands (output and semantics)
- All existing plugins that dispatch commands to the RIB (bgp-gr, bgp-rpki, bgp-rs)
- Plugin name, command prefix, package location, YANG schema all unchanged
- RouteEntry size unchanged

**Behavior to change:**
- `ribInPool` type changes from `map[string]*PeerRIB` to `map[ProtocolID]map[string]*PeerRIB`
- `bgpPeers` field caches `ribInPool[bgpProtocolID]`, set once in `NewRIBManager`
- BGP handlers use `r.bgpPeers[peerAddr]` (identical cost to today)
- `gatherCandidatesLocked` iterates `r.bgpPeers` only (not all protocols)
- Show commands iterate all protocol slots via helper (cold path)
- `updateMetrics` iterates all protocol slots for route counts (periodic path)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- BGP: StructuredEvent from reactor (existing path, unchanged)
- BMP: future spec will write to `ribInPool[bmpProtocolID]` (enabled by this spec)

### Transformation Path
1. BGP UPDATE arrives via StructuredEvent with peer address
2. RIB accesses `ribInPool[bgpProtocolID][peerAddr]`
3. Route stored in PeerRIB (unchanged)
4. Best-path selection iterates `ribInPool[bgpProtocolID]` only
5. Winner mirrors to locRIB (unchanged, bgpProtocolID hardcoded)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| BGP reactor -> RIB | StructuredEvent (unchanged) | [ ] |
| RIB -> locRIB | locRIB.InsertForward (unchanged) | [ ] |

### Architectural Verification
- [ ] BGP handlers use `r.bgpPeers` (cached pointer, zero indirection)
- [ ] gatherCandidatesLocked iterates only `r.bgpPeers`
- [ ] Show commands iterate all protocol slots via helper
- [ ] updateMetrics iterates all protocol slots
- [ ] NewRIBManager initializes outer map, BGP inner map, and caches `bgpPeers`

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| BGP UPDATE via StructuredEvent | -> | RIB stores in `ribInPool[bgpProtocolID]` | `TestBGPRouteStoredInProtocolSlot` |
| gatherCandidates | -> | Only iterates BGP slot | `TestGatherCandidatesOnlyBGP` |
| Show command | -> | Iterates all protocol slots | `TestShowIteratesAllProtocols` |
| Inject command | -> | Stores in BGP slot | `TestInjectUsesProtocolSlot` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | BGP UPDATE arrives | Route stored in `ribInPool[bgpProtocolID][peerAddr]` |
| AC-2 | Best-path selection | gatherCandidatesLocked iterates only `ribInPool[bgpProtocolID]` |
| AC-3 | `bgp rib show` | Shows routes from all protocol slots |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBGPRouteStoredInProtocolSlot` | `rib_test.go` | INSERT goes to `ribInPool[bgpProtocolID]` | |
| `TestGatherCandidatesOnlyBGP` | `rib_test.go` | Candidates come from BGP slot only, other slots ignored | |
| `TestShowIteratesAllProtocols` | `rib_test.go` | Show pipeline includes routes from all protocol slots | |
| `TestInjectUsesProtocolSlot` | `rib_commands_test.go` | `bgp rib inject` stores in BGP slot | |
| `TestWithdrawUsesProtocolSlot` | `rib_commands_test.go` | `bgp rib withdraw` reads from BGP slot | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ProtocolID (outer key) | 1..65535 | 65535 | 0 (Unspecified, no slot) | N/A (uint16 max) |

## Files to Modify
- `internal/component/bgp/plugins/rib/rib.go` - change ribInPool type, update all access sites, add helper to get/create protocol slot
- `internal/component/bgp/plugins/rib/rib_commands.go` - gatherCandidatesLocked iterates BGP slot, show/clear/retain/release iterate via helper, inject/withdraw use BGP slot
- `internal/component/bgp/plugins/rib/rib_structured.go` - handleReceivedStructured uses BGP slot
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - bestCandidateNextHopAddr, lookupLabelsForBest use BGP slot

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Unchanged |
| CLI commands/flags | No | Unchanged |
| Plugin registration | No | Unchanged |
| Functional test | No | Unit tests sufficient for internal type change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Internal refactor, invisible to users |
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
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/rib-storage-design.md` - note two-level map |

## Files to Create
- None

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files, tests |
| 3. Implement (TDD) | Phases below |
| 4. /ze-review gate | Review Gate |
| 5. Full verification | `make ze-lint && make ze-unit-test` |
| 6. Critical review | Checklist |
| 7. Fix issues | - |
| 8. Re-verify | Stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Checklist |
| 11. Security review | Checklist |
| 12. Re-verify | Stage 5 |
| 13. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: Type change + cached pointer** - change `ribInPool` type, add `bgpPeers`
   cached pointer. Add `protocolPeers(pid)` helper for cold-path access (show, metrics).
   Update `NewRIBManager`. Write failing tests.
   - Tests: `TestBGPRouteStoredInProtocolSlot`
   - Files: `rib.go`

2. **Phase: Update all BGP access sites** - every `r.ribInPool[peerAddr]` becomes
   `r.bgpPeers[peerAddr]`. Update handlers, commands, bestchange lookups.
   - Tests: `TestInjectUsesProtocolSlot`, `TestWithdrawUsesProtocolSlot`
   - Files: `rib.go`, `rib_commands.go`, `rib_structured.go`, `rib_bestchange.go`

3. **Phase: gatherCandidatesLocked** - iterate only `r.bgpPeers`.
   - Tests: `TestGatherCandidatesOnlyBGP`
   - Files: `rib_commands.go`

4. **Phase: Show commands + metrics** - show pipeline and metrics iterate all protocol
   slots via `protocolPeers` helper.
   - Tests: `TestShowIteratesAllProtocols`
   - Files: `rib_commands.go` (show pipeline), `rib.go` (metrics)

5. **Phase: Test fixup** - update existing tests that access ribInPool directly.

6. **Full verification** - `make ze-verify`

7. **Complete spec** - audit + learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each AC has file:line |
| Correctness | All existing bgp-gr/rpki/rs dispatch still work |
| Naming | `protocolPeers` helper, consistent access pattern |
| Data flow | BGP path functionally unchanged, one extra map indirection |
| Rule: no-layering | Direct type change, no wrapper |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Two-level ribInPool | grep `map\[redistevents.ProtocolID\]` rib.go |
| bgpPeers cached pointer | grep `bgpPeers` rib.go (field + NewRIBManager init) |
| BGP handlers use bgpPeers | grep `r\.bgpPeers` rib.go rib_commands.go rib_structured.go rib_bestchange.go |
| gatherCandidates BGP-only | grep `r\.bgpPeers` rib_commands.go (in gatherCandidatesLocked) |
| Show iterates all | grep `protocolPeers` in show pipeline |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Nil inner map | protocolPeers helper must initialize inner map before write |
| Concurrent access | peerMu still protects all ribInPool access (outer and inner) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Existing tests break | Update to access via `ribInPool[bgpProtocolID]` |
| Metrics labels change | Verify metric labels still use bare peer address |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| RIB should become protocol-agnostic | locrib is already the protocol-agnostic layer | Design review + BIRD/FRR/GoBGP research | Avoided rename, move, decoder registry |
| Protocol-keyed string keys needed | Two-level map with ProtocolID outer key is faster and simpler | Design review: performance analysis of key schemes | No string alloc, no key parsing |
| StaleLevelMonitorOnly needed for exclusion | Structural separation via outer map key makes per-entry flags unnecessary | Design review: "do we need ProtocolID if routes have StaleLevelMonitorOnly?" | One fewer constant, one fewer check per candidate |

## Design Insights

- BIRD, FRR, GoBGP all keep the BGP adj-RIB BGP-specific. Ze's locrib is the
  protocol-agnostic layer (like BIRD's `rtable`, FRR's zebra).
- Ze diverges: BMP routes share the BGP RIB storage (same wire format).
- `map[ProtocolID]map[string]*PeerRIB` is the fastest collision-safe key scheme:
  one uint16 map lookup vs string concat or struct hashing.
- Caching `bgpPeers = ribInPool[bgpProtocolID]` at init makes the hot-path cost
  identical to today (bare `map[string]*PeerRIB` lookup). The outer map is only
  touched by show commands (cold path) and metrics (periodic).
- Structural exclusion (iterate only the BGP slot) is simpler and faster than
  per-entry flag checks (`StaleLevelMonitorOnly`).
- The two-level map generalizes to N BGP-wire-format sources without code changes.

## RFC Documentation

RFC 4271 Section 3.2: Adj-RIB-In is per-peer BGP storage.
Ze extends the storage to per-source-per-peer via the outer ProtocolID key.

## Implementation Summary

### What Was Implemented
- (fill during /implement)

### Bugs Found/Fixed
- (fill during /implement)

### Documentation Updates
- (fill during /implement)

### Deviations from Plan
- (fill during /implement)

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

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit > implicit
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for numeric inputs

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-rib-4-extraction.md`
- [ ] Summary included in commit
