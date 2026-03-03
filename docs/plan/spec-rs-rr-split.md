# Spec: Split bgp-rs into RS (bgp-rs) + RR (bgp-rr) with Shared Forwarding

**Status:** Skeleton — not ready for implementation. Depends on spec-rib-04.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/plugins/bgp-rs/server.go` — current implementation to decompose
3. `internal/plugins/bgp-rs/worker.go` — worker pool to move
4. `internal/plugins/bgp-role/role.go` — role system to extend
5. `rfc/short/rfc4456.md` — Route Reflector (to be created)

## Task

Split the current `bgp-rs` plugin into two plugins with shared forwarding infrastructure:

- **bgp-rs** (Route Server) — current plugin, already renamed. Forward-all policy (RFC 7947).
- **bgp-rr** (Route Reflector) — new. Client/non-client selection (RFC 4456).

~60% of the code is generic forwarding infrastructure (worker pool, batching, async loops, flow control, event dispatch, peer tracking, withdrawal handling). Only target selection and the forward command format differ.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — plugin architecture, forwarding model
  → Constraint: Engine handles wire-level concerns (ORIGINATOR_ID, CLUSTER_LIST)
  → Decision: Plugin decides WHO gets route, engine handles HOW
- [ ] `.claude/rules/plugin-design.md` — registration, 5-stage protocol
  → Constraint: Each plugin gets own register.go, YANG schema

### RFC Summaries
- [ ] `rfc/short/rfc4456.md` — Route Reflector (MUST create before implementing)
  → Constraint: ORIGINATOR_ID set to source router-id if absent
  → Constraint: CLUSTER_LIST prepended with cluster-id
  → Constraint: Loop detection via cluster-id in CLUSTER_LIST
  → Constraint: §8 target selection rules (client→all, non-client→clients only)
  → Constraint: §3 route refresh forwarding respects roles
- [ ] `rfc/short/rfc7947.md` — Route Server
  → Constraint: Forward-all to all except source
- [ ] `rfc/short/rfc9234.md` — BGP Role
  → Constraint: Existing role values; rr-client/rr-server are local-only

**Key insights:**
- (to be completed during research phase)

## Current Behavior (MANDATORY)

**Source files read:** (must complete before implementation)
- [ ] `internal/plugins/bgp-rs/server.go` (1194L) — full forwarding server, to decompose
- [ ] `internal/plugins/bgp-rs/worker.go` (398L) — worker pool, move unchanged
- [ ] `internal/plugins/bgp-rs/peer.go` (34L) — PeerState, extend with Role
- [ ] `internal/plugins/bgp-role/role.go` — current role values
- [ ] `internal/plugins/bgp-role/config.go` — per-peer config parsing pattern
- [ ] `internal/plugins/bgp/wireu/aspath_rewrite.go` — pattern for wire rewrite
- [ ] `internal/plugins/bgp/reactor/reactor_api_forward.go:359` — pattern for ForwardReflectedUpdate
- [ ] `internal/plugins/bgp/reactor/received_update.go:91` — pattern for ReflectorWire

**Behavior to preserve:**
- All existing Route Server forwarding behavior
- Worker pool, batching, flow control, withdrawal tracking
- OPEN/state handling, peer tracking
- Replay mechanism

**Behavior to change:**
- ~~Rename current bgp-rr to bgp-rs (it's a Route Server)~~ Done — bgp-rs rename already applied
- Extract shared infrastructure to `internal/plugin/forward/`
- Create new bgp-rr with Route Reflector policy
- Add engine `cache reflect` command with ORIGINATOR_ID/CLUSTER_LIST handling

## Data Flow (MANDATORY)

### Entry Point
- BGP UPDATE received from peer → engine event → plugin

### Transformation Path (Route Server — existing)
1. Engine sends JSON event to bgp-rs plugin
2. bgp-rs SelectTargets: all peers except source
3. bgp-rs sends `bgp cache <ids> forward <selector>` command
4. Engine forwards wire bytes to selected peers

### Transformation Path (Route Reflector — new)
1. Engine sends JSON event to bgp-rr plugin
2. bgp-rr SelectTargets: client/non-client rules per RFC 4456 §8
3. bgp-rr sends `bgp cache <ids> reflect <selector> cluster-id <cid>` command
4. Engine checks CLUSTER_LIST for loop (cluster-id present → discard)
5. Engine sets ORIGINATOR_ID if absent (source peer's router-id)
6. Engine prepends cluster-id to CLUSTER_LIST
7. Engine forwards modified wire bytes to selected peers

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → Plugin | JSON event (same for RS and RR) | [ ] |
| Plugin → Engine (RS) | Text command: `bgp cache <ids> forward <sel>` | [ ] |
| bgp-rr → Engine (RR) | Text command: `bgp cache <ids> reflect <sel> cluster-id <cid>` | [ ] |
| Engine wire rewrite | `wireu.RewriteReflector()` modifies ORIGINATOR_ID, CLUSTER_LIST | [ ] |

### Integration Points
- `ForwardingPolicy` interface — strategy pattern for RS vs RR behavior
- `cache reflect` handler — new engine command for RR forwarding
- `wireu.RewriteReflector()` — wire-level attribute modification
- `ReceivedUpdate.ReflectorWire()` — lazy cached reflected wire version
- bgp-role YANG — extended with rr-client/rr-server local-only roles

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Architecture: Shared Package + Two Thin Plugins

### Package Layout

| Package | File | Purpose |
|---------|------|---------|
| `internal/plugin/forward/` | `server.go` | ForwardingServer, ForwardingPolicy interface |
| `internal/plugin/forward/` | `worker.go` | workerPool (moved from bgp-rs, unchanged) |
| `internal/plugin/forward/` | `peer.go` | PeerState (extended with Role) |
| `internal/plugin/forward/` | `event.go` | Event types, JSON parsing (extracted from server.go) |
| `internal/plugin/forward/` | `batch.go` | Batch accumulation, async release/forward loops |
| `internal/plugins/bgp-rs/` | `server.go` | ~50 lines: RS policy (forward-all) |
| `internal/plugins/bgp-rs/` | `register.go` | Registers as "bgp-rs" |
| `internal/plugins/bgp-rs/` | `server_test.go` | Existing tests (adapted) |
| `internal/plugins/bgp-rs/` | `propagation_test.go` | Existing propagation tests (adapted) |
| `internal/plugins/bgp-rr/` | `server.go` | ~80 lines: RR policy (rr-client/rr-server selection) |
| `internal/plugins/bgp-rr/` | `config.go` | ~60 lines: cluster-id parsing |
| `internal/plugins/bgp-rr/` | `register.go` | Registers as "bgp-rr" |
| `internal/plugins/bgp-rr/` | `server_test.go` | RR-specific tests |
| `internal/plugins/bgp-rr/` | `schema/ze-rr.yang` | YANG schema (cluster-id only; roles via bgp-role) |
| `internal/plugins/bgp-rr/` | `schema/embed.go` | Embed |

### ForwardingPolicy Interface

| Method | RS Implementation | RR Implementation |
|--------|-------------------|-------------------|
| `SelectTargets(source, families, peers)` | All except source | rr-client/rr-server rules (RFC 4456 §8) |
| `SelectRefreshTargets(source, peers)` | All except source with route-refresh cap | Same role rules as SelectTargets |
| `ForwardCommand(ids, selector)` | `bgp cache <ids> forward <sel>` | `bgp cache <ids> reflect <sel> cluster-id <cid>` |
| `OnConfigure(sections)` | No-op | Parse cluster-id from own config root |
| `SetPeerRole(addr, role)` | Store role (rs/rs-client) | Store role (rr-client/rr-server) |
| `PluginName()` | `"bgp-rs"` | `"bgp-rr"` |
| `Commands()` | `[rs status, rs peers]` | `[rr status, rr peers]` |

Everything else is identical: dispatch, workers, batching, flow control, peer state, withdrawal tracking, replay, OPEN/state handling.

Loop detection and ORIGINATOR_ID insertion are **engine-side** concerns (see RR Forwarding Mechanism below), NOT in the plugin interface.

### Peer Role Identification via bgp-role

Both RS and RR identify peers through the existing bgp-role plugin. New local-only roles extend the role table:

| Role | RFC 9234 Wire Value | On Wire? | Used By |
|------|---------------------|----------|---------|
| `provider` | 0 | Yes | bgp-role |
| `rs` | 1 | Yes | bgp-role, bgp-rs |
| `rs-client` | 2 | Yes | bgp-role, bgp-rs |
| `customer` | 3 | Yes | bgp-role |
| `peer` | 4 | Yes | bgp-role |
| **`rr-server`** | none | **No** | bgp-rr (non-client) |
| **`rr-client`** | none | **No** | bgp-rr (client) |

`rr-server` and `rr-client` are local-only policy roles — no capability announced on the wire.

### PeerState Extension

| Field | Type | Source | New? |
|-------|------|--------|------|
| `Address` | `string` | Existing | No |
| `ASN` | `uint32` | Existing | No |
| `Up` | `bool` | Existing | No |
| `Replaying` | `bool` | Existing | No |
| `ReplayGen` | `uint64` | Existing | No |
| `Capabilities` | `map` | Existing | No |
| `Families` | `[]string` | Existing | No |
| `Role` | `string` | From bgp-role config | **Yes** |

RouterID is NOT needed in PeerState. The engine handles ORIGINATOR_ID using its own peer state.

### RR Target Selection (RFC 4456 §8)

| Source Peer Role | Forward To |
|-----------------|------------|
| `rr-client` | All `rr-client` (except source) + all `rr-server` peers |
| `rr-server` (non-client) | `rr-client` peers only |

### RR Forwarding Mechanism

**New engine command:** `bgp cache <ids> reflect <selector> cluster-id <cid>`

The engine handles all wire-level concerns — the plugin only decides WHO gets the route:

| Step | Engine Action | Detail |
|------|---------------|--------|
| 1 | Loop detection | Scan CLUSTER_LIST in wire bytes for cluster-id. If found, return error (loop). JSON events do NOT include CLUSTER_LIST. |
| 2 | ORIGINATOR_ID | If absent in wire bytes, set to source peer's BGP router-id (from OPEN negotiation, PeerSettings). |
| 3 | CLUSTER_LIST prepend | Prepend cluster-id to CLUSTER_LIST (create attribute if absent). |

**Implementation pattern** follows `EBGPWire` / `RewriteASPath`:
- `wireu.RewriteReflector(dst, payload, originatorID, clusterID)` — wire-level attribute insertion/prepend
- `ReceivedUpdate.ReflectorWire(originatorID, clusterID)` — lazy cached reflected wire version
- `ForwardReflectedUpdate()` in reactor — same as `ForwardUpdate` but uses `ReflectorWire()`, looks up source peer router-id, checks loop

### RR Config

RR plugin needs one global setting: `cluster-id` (IPv4 address). Added to the RR plugin's own YANG schema as a leaf. Per-peer client/non-client identification comes from bgp-role config. No separate `route-reflector-client` boolean needed.

Plugin receives config in Stage 2 `OnConfigure`, extracts cluster-id from its own config root and reads peer roles from the bgp config section (same pattern as `bgp-role/config.go`).

### RR Route Refresh Handling (RFC 4456 §3)

RFC 4456 §3: "When an RR receives a ROUTE-REFRESH from an iBGP peer, it should forward to its clients." Current code forwards refresh to ALL peers except source. For RR, refresh forwarding must respect client/non-client roles via `SelectRefreshTargets`.

## Prerequisites

- Create `rfc/short/rfc4456.md` (Route Reflector summary — currently missing)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|

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

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|

## Files to Modify

- `internal/plugins/bgp-rs/server.go` (1194L) — decompose into shared + RS-specific
- `internal/plugins/bgp-rs/worker.go` (398L) — move unchanged to `forward/worker.go`
- `internal/plugins/bgp-rs/peer.go` (34L) — move + extend to `forward/peer.go`
- `internal/plugins/bgp/wireu/aspath_rewrite.go` — pattern for `reflector_rewrite.go`
- `internal/plugins/bgp/reactor/reactor_api_forward.go:359` — pattern for `ForwardReflectedUpdate`
- `internal/plugins/bgp/reactor/received_update.go:91` — pattern for `ReflectorWire`
- `internal/plugins/bgp-role/role.go` — extend with `rr-client` / `rr-server` local-only roles (used by bgp-rr)
- `internal/plugins/bgp-role/config.go` — pattern for per-peer config parsing

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] | `internal/plugins/bgp-rr/schema/ze-rr.yang` — cluster-id |
| bgp-role YANG | [x] | Extend with rr-client/rr-server role values |
| CLI commands/flags | [ ] | `rr status`, `rr peers` commands |
| Plugin SDK docs | [ ] | `.claude/rules/plugin-design.md` |
| Functional test | [x] | 3-peer RR topology + loop detection |

## Files to Create

- `internal/plugin/forward/server.go` — ForwardingServer, ForwardingPolicy interface
- `internal/plugin/forward/worker.go` — workerPool (moved from bgp-rs)
- `internal/plugin/forward/peer.go` — PeerState with Role
- `internal/plugin/forward/event.go` — Event types, JSON parsing
- `internal/plugin/forward/batch.go` — Batch accumulation, async loops
- `internal/plugins/bgp-rs/server.go` — RS policy
- `internal/plugins/bgp-rs/register.go` — Registration
- `internal/plugins/bgp-rr/server.go` — RR policy
- `internal/plugins/bgp-rr/register.go` — RR registration
- `internal/plugins/bgp-rr/config.go` — cluster-id parsing
- `internal/plugins/bgp-rr/schema/ze-rr.yang` — YANG schema
- `internal/plugins/bgp-rr/schema/embed.go` — Embed
- `internal/plugins/bgp/wireu/reflector_rewrite.go` — wire-level ORIGINATOR_ID/CLUSTER_LIST

## Implementation Steps

### Phase 1: Extract shared package (no behavior change)
1. Create `rfc/short/rfc4456.md`
2. Create `internal/plugin/forward/` with server, worker, peer, event, batch
3. Refactor `internal/plugins/bgp-rs/` to wrap ForwardingServer with RS policy
4. All existing tests pass via bgp-rs
5. Remove extracted code from bgp-rs (now in forward/)
6. Update `internal/plugin/all/all.go` (make generate)

### Phase 2: Engine reflect command
7. `wireu/reflector_rewrite.go` — wire-level ORIGINATOR_ID/CLUSTER_LIST insertion
8. `ReceivedUpdate.ReflectorWire()` — lazy cached
9. `cache reflect` handler + `ForwardReflectedUpdate()` (includes loop detection + ORIGINATOR_ID from peer map)
10. Unit tests for wire rewrite + loop detection

### Phase 3: RR plugin
11. Extend bgp-role YANG with `rr-client` / `rr-server` local-only role values
12. Create bgp-rr with RR policy, cluster-id config, YANG schema
13. RR target selection (rr-client/rr-server roles)
14. RR refresh target selection (RFC 4456 §3)
15. Unit + functional tests

### Failure Routing

| Failure | Route To |
|---------|----------|
| Existing tests fail after Phase 1 | Shared extraction broke behavior — fix forward package |
| Wire rewrite corrupts UPDATE | Check buffer offsets in RewriteReflector |
| Loop detection false positive | Verify CLUSTER_LIST parsing in wire bytes |
| RR target selection wrong | Verify role mapping from bgp-role config |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights

- Loop detection and ORIGINATOR_ID insertion are engine-side concerns, not plugin-side
- Plugin only decides WHO gets the route — engine handles HOW (wire modifications)
- ~60% code is shared forwarding infrastructure — strategy pattern is natural fit
- rr-client/rr-server are local-only roles — no wire capability needed
- ReflectorWire follows same lazy caching pattern as EBGPWire

## Implementation Summary

### What Was Implemented
- (pending — spec is deferred)

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
- [ ] Wiring Test table complete
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] `make ze-lint` passes
- [ ] RFC constraint comments added
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `docs/learned/`
- [ ] Summary included in commit

## Verification

- `make ze-verify` passes after Phase 1 (no behavior change, just code moved)
- Unit tests for `wireu.RewriteReflector` (round-trip, insert vs prepend, loop detection)
- Unit tests for RR target selection (client→all, non-client→clients only)
- Unit tests for RR refresh target selection (RFC 4456 §3)
- Functional tests: 3-peer RR topology with client/non-client peers
- Functional test: loop detection (UPDATE with own cluster-id in CLUSTER_LIST discarded)
