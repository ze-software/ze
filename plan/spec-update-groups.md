# Spec: update-groups

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-03-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/update-building.md` - three UPDATE paths, grouping
4. `docs/architecture/encoding-context.md` - ContextID, zero-copy forwarding
5. `internal/component/bgp/reactor/forward_pool.go` - per-peer fwdWorkers
6. `internal/component/bgp/rib/grouping.go` - two-level attribute grouping
7. `internal/component/bgp/rib/commit.go` - CommitService per-peer

## Task

Implement **automatic cross-peer update groups** — when multiple peers share the same outbound encoding context (ContextID) and the same outbound policy, compute the UPDATE once and fan out the wire bytes to all group members. This eliminates redundant per-peer UPDATE building for peers that would receive identical wire bytes.

Today Ze computes outbound UPDATEs independently per peer: for N peers with identical capabilities and policy, it builds N identical UPDATEs. Update groups reduce this to 1 build + N sends.

**Auto-grouping:** Peers are grouped automatically by the engine based on `sendCtxID` + policy equivalence. No user configuration needed (unlike FRR/VyOS peer-groups which require explicit assignment).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/update-building.md` — three UPDATE paths, grouped send
  → Decision: three paths (receive, build, forward) — update groups affect Build and Forward paths
  → Constraint: all grouped builders use `BuildGrouped*WithLimit()` with max message size
- [ ] `docs/architecture/encoding-context.md` — ContextID system
  → Decision: ContextID is uint16, deduped via FNV-64 hash of encoding params
  → Constraint: zero-copy only when `sourceCtxID == destCtxID`; re-encode otherwise
- [ ] `docs/architecture/pool-architecture.md` — attribute dedup pools
  → Constraint: RIB stores attribute refs, not full copies — UPDATE building reads from pools
- [ ] `.claude/rules/goroutine-lifecycle.md` — long-lived workers only
  → Constraint: no per-group goroutines in hot path; use channel + existing workers

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` — BGP UPDATE format
  → Constraint: all NLRIs in one UPDATE must share identical path attributes (§4.3)
- [ ] `rfc/short/rfc7911.md` — ADD-PATH
  → Constraint: ADD-PATH mode per-family per-direction affects NLRI encoding — peers with different ADD-PATH cannot share wire bytes
- [ ] `rfc/short/rfc8654.md` — Extended Message
  → Constraint: max message size differs (4096 vs 65535) — peers with different ExtMsg cannot share wire bytes

**Key insights:**
- ContextID already captures all encoding-relevant differences (ASN4, ADD-PATH, ExtMsg, ExtNH, iBGP/eBGP, ASN values)
- Peers with same `sendCtxID` produce bit-identical wire bytes for the same route set
- Today's policy is uniform (no per-peer route-maps) — so `sendCtxID` alone defines groups
- The forward pool (`fwdPool`) already has per-peer workers with batch drain — update groups sit above this

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_pool.go` — per-peer fwdWorker, batch drain pattern
  → Constraint: fwdPool dispatches pre-computed fwdItems per destination peer
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` — AnnounceNLRIBatch, per-peer loop
  → Constraint: iterates peers, builds UPDATE independently per peer (AS_PATH, next-hop, grouping)
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` — ForwardUpdate, context check
  → Constraint: checks srcCtxID == dstCtxID for zero-copy; otherwise re-encodes per peer
- [ ] `internal/component/bgp/rib/commit.go` — CommitService, one instance per peer
  → Constraint: each peer has own CommitService with own EncodingContext and groupUpdates flag
- [ ] `internal/component/bgp/rib/grouping.go` — GroupByAttributesTwoLevel
  → Constraint: groups routes by attributes then AS_PATH — produces ASPathGroups for UPDATE building
- [ ] `internal/component/bgp/reactor/peer.go` — Peer struct, sendCtx, sendCtxID
  → Constraint: each Peer has sendCtx/sendCtxID set at session establishment
- [ ] `internal/component/bgp/reactor/peer_send.go` — SendUpdate, session write
  → Constraint: session.writeMu per-peer exclusive write lock; flush per batch

**Behavior to preserve:**
- Per-peer attribute grouping (two-level) within each UPDATE
- RFC 4271 §4.3 compliance: identical attributes per UPDATE
- Zero-copy forwarding when ContextIDs match (forward path)
- Per-peer next-hop resolution (RouteNextHop policy may differ)
- Per-peer AS_PATH manipulation (iBGP preserve vs eBGP prepend)
- Correct ADD-PATH encoding per peer's negotiated mode
- Message size splitting per peer's ExtendedMessage capability
- `group-updates` per-peer config knob
- Forward pool batch drain pattern (per-peer workers)
- Graceful degradation: if peers have unique contexts, behaves identically to today

**Behavior to change:**
- Build path: instead of building UPDATE N times for N peers in same group, build once and fan out
- Forward path: instead of dispatching N independent fwdItems, dispatch one shared payload to N peers
- Reactor: maintain update group membership, recompute on peer session up/down

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Route change arrives via: API command (AnnounceNLRIBatch), RIB commit, or received UPDATE (ForwardUpdate)
- Format: `[]*rib.Route` (batch) or `*api.WireUpdate` (forward)

### Transformation Path (Current → Proposed)

**Current (per-peer):**
1. Route batch arrives at reactor API
2. For each destination peer: resolve next-hop, build AS_PATH, group by attributes, build UPDATE(s)
3. Send UPDATE to peer via session

**Proposed (per-group):**
1. Route batch arrives at reactor API
2. Identify update groups from peer set (by `sendCtxID` + policy key)
3. For each update group: pick representative peer, build UPDATE once
4. For each peer in group: send the pre-built UPDATE bytes (fan-out)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor → fwdPool | fwdItem dispatch per peer (unchanged) | [ ] |
| CommitService → Peer | SendUpdate per peer (unchanged wire interface) | [ ] |
| Group management → Reactor | Group lookup on peer session events | [ ] |

### Integration Points
- `reactor.Reactor` — holds peer map, will also hold update group index
- `reactor.Peer` — `sendCtxID` used as group key component
- `forward_pool.go` — fwdPool workers unchanged, but receive shared payloads
- `rib/commit.go` — CommitService remains per-peer (context-specific), but can be shared per-group
- `reactor_api_batch.go` — main optimization target: loop over groups, not peers
- `reactor_api_forward.go` — secondary optimization: shared fwdItem per group

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Design

### Update Group Definition

An **update group** is the set of established peers that would receive bit-identical UPDATE wire bytes for the same route. Membership is determined by:

| Factor | Why |
|--------|-----|
| `sendCtxID` | Encodes all capability differences (ASN4, ADD-PATH, ExtMsg, ExtNH, ASN values, iBGP/eBGP) |
| Policy key | Future: per-peer outbound filters. Today: uniform (all peers share one policy key) |

**GroupKey:** `struct { ctxID ContextID; policyKey uint32 }` — policyKey is 0 for all peers today, extensible later.

### Group Lifecycle

| Event | Action |
|-------|--------|
| Peer session established | Compute GroupKey from `sendCtxID` + policy. Add to group. |
| Peer session closed | Remove from group. Delete group if empty. |
| Peer policy changed | Remove from old group, add to new group. |
| Peer capabilities renegotiated | Remove from old group, add to new group (sendCtxID changes). |

### Group Index Structure

```
Reactor:
  updateGroups map[GroupKey]*UpdateGroup

UpdateGroup:
  key     GroupKey
  members []*Peer              // established peers in this group
  ctx     *EncodingContext     // shared encoding context (from any member)
  ctxID   ContextID            // shared context ID
```

Maintained as a simple map in the reactor. No goroutines, no channels — just a lookup table updated on peer state changes.

### Optimization Points

#### 1. Build Path (AnnounceNLRIBatch / CommitService)

**Before:** `for each peer → build UPDATE → send`
**After:** `for each group → build UPDATE once → for each peer in group → send`

The UPDATE building (attribute packing, NLRI grouping, message splitting) happens once per group. The resulting `[]byte` is sent to each member peer. Since all members share the same `sendCtxID`, the wire bytes are identical.

**Constraints that prevent sharing:**
- Per-peer next-hop override: if RouteNextHop differs, next-hop in UPDATE differs → separate groups (add to policy key)
- Per-peer AS_PATH prepend: iBGP vs eBGP already separated by ContextID (contains IsIBGP + ASN values)

#### 2. Forward Path (ForwardUpdate)

**Before:** `for each peer → check context match → dispatch fwdItem`
**After:** `for each group → check context match once → dispatch shared fwdItem to all members`

The context comparison and potential re-encoding happens once per group, not per peer.

#### 3. Initial Route Sync (peer_initial_sync)

Lower priority. When a new peer joins an existing group, it could reuse wire bytes already computed for the group. But initial sync is infrequent — defer this optimization.

### What Does NOT Change

| Component | Why unchanged |
|-----------|---------------|
| `fwdPool` / `fwdWorker` | Still per-peer workers. They receive items, don't care if shared. |
| `CommitService` | Still per-peer instance (may share backing context in future). |
| `Session.SendUpdate()` | Writes bytes to TCP. Unchanged interface. |
| `GroupByAttributesTwoLevel()` | Route grouping within an UPDATE. Orthogonal. |
| Per-peer `writeMu` | TCP write serialization remains per-session. |
| `group-updates` config | Controls within-UPDATE NLRI grouping, not cross-peer grouping. |

### Naming

- **Update group:** cross-peer optimization (this spec)
- **Route grouping:** within-UPDATE NLRI packing (existing `group-updates` flag)

These are orthogonal. Both can be active simultaneously.

### Performance Expectations

| Scenario | Peers | Groups | Speedup |
|----------|-------|--------|---------|
| Route server (identical caps) | 100 | 1 | ~100x fewer builds |
| Route reflector (uniform clients) | 50 | 1 | ~50x fewer builds |
| Mixed iBGP + eBGP | 20 | 2 | ~10x fewer builds |
| All unique capabilities | N | N | 1x (no regression) |

The optimization is proportional to average group size. Worst case (all unique) degrades to current behavior with negligible overhead (one map lookup per peer on session up/down).

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer session established with same caps as existing peer | → | Reactor adds peer to existing update group | `TestUpdateGroupFormation` |
| AnnounceNLRIBatch to peers in same group | → | UPDATE built once, sent to all members | `TestUpdateGroupSharedBuild` |
| ForwardUpdate to peers in same group | → | fwdItem computed once per group | `TestUpdateGroupSharedForward` |
| Peer session closed | → | Peer removed from group, group deleted if empty | `TestUpdateGroupTeardown` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two peers established with identical capabilities | Both peers in same update group |
| AC-2 | Two peers with different ADD-PATH modes | Peers in different update groups |
| AC-3 | AnnounceNLRIBatch to 3 peers in same group | UPDATE built once (verified by build count), all 3 receive identical bytes |
| AC-4 | ForwardUpdate to 2 peers in same group with matching context | Context check + re-encode happens once, both receive same fwdItem payload |
| AC-5 | Peer session closes | Peer removed from group; group deleted when last member leaves |
| AC-6 | Peer renegotiates with different capabilities | Moves to new group (old group shrinks, new group grows or is created) |
| AC-7 | All peers have unique ContextIDs | Each group has 1 member; behavior identical to current (no regression) |
| AC-8 | Mixed: some peers grouped, some unique | Grouped peers share builds, unique peers build independently |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUpdateGroupKey` | `internal/component/bgp/reactor/update_group_test.go` | GroupKey equality and hashing | |
| `TestUpdateGroupAddRemove` | same | Add/remove peers, group creation/deletion | |
| `TestUpdateGroupFormation` | same | Peers with same sendCtxID join same group | |
| `TestUpdateGroupDifferentContexts` | same | Peers with different sendCtxID get separate groups | |
| `TestUpdateGroupTeardown` | same | Last peer removed deletes group | |
| `TestUpdateGroupRenegotiation` | same | Peer moves between groups on capability change | |
| `TestUpdateGroupSharedBuild` | same | AnnounceNLRIBatch builds once per group | |
| `TestUpdateGroupSharedForward` | same | ForwardUpdate computes once per group | |
| `TestUpdateGroupNoRegression` | same | All-unique-context case identical to current behavior | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Group member count | 1–65535 | 65535 peers per group | N/A (empty = deleted) | Limited by peer count |
| ContextID | 0–65535 | 65535 | 0 (unregistered) | N/A (uint16) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-update-groups-basic` | `test/plugin/update-groups-basic.ci` | Two peers with same config receive routes; verify both get identical UPDATEs | |
| `test-update-groups-mixed` | `test/plugin/update-groups-mixed.ci` | Mix of iBGP and eBGP peers; verify correct grouping | |

### Future (if deferring any tests)
- Benchmark comparing N-peer UPDATE throughput with/without groups (performance, not correctness)
- Initial sync reuse from existing group (optimization, not MVP)

## Files to Modify
- `internal/component/bgp/reactor/reactor.go` — add `updateGroups` map, group management methods
- `internal/component/bgp/reactor/reactor_api_batch.go` — loop over groups instead of peers
- `internal/component/bgp/reactor/reactor_api_forward.go` — shared fwdItem per group
- `internal/component/bgp/reactor/peer.go` — group join/leave on session events

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A (auto-grouping, no user config) |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A (internal optimization) |

## Files to Create
- `internal/component/bgp/reactor/update_group.go` — UpdateGroup type, GroupKey, index management
- `internal/component/bgp/reactor/update_group_test.go` — unit tests
- `test/plugin/update-groups-basic.ci` — functional test
- `test/plugin/update-groups-mixed.ci` — functional test

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests** for UpdateGroup type (add/remove/lookup) → Review: edge cases? Boundary tests?
2. **Run tests** → Verify FAIL (paste output). Fail for RIGHT reason?
3. **Implement UpdateGroup type** in `update_group.go` — GroupKey, index, add/remove → Minimal code to pass. Simplest solution?
4. **Run tests** → Verify PASS (paste output). All pass?
5. **Write unit tests** for group-aware AnnounceNLRIBatch (verify single build per group)
6. **Run tests** → Verify FAIL
7. **Modify AnnounceNLRIBatch** to iterate groups → build once per group → fan out
8. **Run tests** → Verify PASS
9. **Write unit tests** for group-aware ForwardUpdate
10. **Run tests** → Verify FAIL
11. **Modify ForwardUpdate** to compute fwdItem once per group
12. **Run tests** → Verify PASS
13. **Wire group join/leave** into peer session lifecycle (session established / closed)
14. **Write functional tests** — multi-peer scenarios
15. **RFC refs** → Add `// RFC 4271 Section 4.3` comments where relevant
16. **Verify all** → `make ze-test` (lint + all ze tests including fuzz + exabgp)
17. **Critical Review** → All 6 checks from `rules/quality.md`
18. **Complete spec** → Fill audit tables, write learned summary

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 or 7 or 11 (fix syntax/types) |
| Test fails wrong reason | Step 1, 5, or 9 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |

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
<!-- LIVE — write IMMEDIATELY when you learn something -->

- ContextID already encodes all wire-format-relevant differences. No need for a separate encoding comparison.
- iBGP vs eBGP is captured in ContextID (via IsIBGP + ASN values), so AS_PATH prepend behavior is automatically group-separated.
- RouteNextHop policy (per-peer next-hop override) is the main candidate for future policy key differentiation.
- `group-updates` (within-UPDATE NLRI grouping) and update groups (cross-peer build sharing) are orthogonal.

## RFC Documentation

Add `// RFC 4271 Section 4.3: "all path attributes [...] apply to all destinations"` above group-aware build code.
MUST document: identical attributes per UPDATE (§4.3), ADD-PATH mode per peer (RFC 7911), message size per peer (RFC 8654).

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
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
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
