# Spec: chaos-syncing-state

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/chaos-web-dashboard.md` - dashboard layout and data flows
4. `cmd/ze-chaos/web/state.go` - PeerStatus enum, DashboardState, EOR tracking
5. `cmd/ze-chaos/web/dashboard.go` - ProcessEvent status transitions
6. `cmd/ze-chaos/web/render.go` - peer row and detail pane rendering
7. `cmd/ze-chaos/web/viz.go` - statusColor(), all-peers counting, timeline segments

## Task

Add a `PeerSyncing` status to the chaos web dashboard representing "initial route transfer in progress." Currently, peers jump directly from `PeerIdle` to `PeerUp` on `EventEstablished`. In reality, a BGP peer that has just established is not yet useful — it is still sending its initial routes. The new state makes this phase visible: `PeerIdle` → `PeerSyncing` (on Established) → `PeerUp` (on first EOR from that peer). This gives operators an at-a-glance view of which peers are still loading their RIB.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/chaos-web-dashboard.md` - dashboard design, SSE events, state management
  → Constraint: ProcessEvent() runs synchronously on the main event loop; must be fast
  → Constraint: Per-peer status enum drives dot color, timeline segments, and filters
- [ ] `docs/architecture/core-design.md` - BGP session lifecycle, EOR semantics
  → Decision: EOR (End-of-RIB) marks completion of initial route announcement per family

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4724.md` - Graceful Restart / End-of-RIB marker
  → Constraint: EOR = UPDATE with empty NLRI + empty Withdrawn; signals initial RIB transfer complete

**Key insights:**
- EOR is the natural boundary between "syncing" and "up" — it is already tracked per-peer via `EORSeen[]`
- The dashboard already counts `EORCount` and `SyncDuration` but doesn't use EOR as a status transition trigger
- `PeersUp` counter currently increments on Established; must now increment on EOR instead

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze-chaos/web/state.go` - PeerStatus enum: Idle(0), Up(1), Down(2), Reconnecting(3). PeerState struct. ConvergenceHistogram. DashboardState with EORSeen/EORCount/SyncDuration. statusColor() maps status → CSS color.
  → Constraint: PeerStatus is int enum; CSSClass() and String() are switch statements
  → Constraint: EORSeen is []bool indexed by peer index; EORCount is global aggregate
- [ ] `cmd/ze-chaos/web/dashboard.go` - ProcessEvent: EventEstablished → PeerUp + PeersUp++. EventEORSent → only updates EORSeen/EORCount/SyncDuration, no status change. EventDisconnected/Error → PeerDown + PeersUp-- (guarded by prevStatus == PeerUp).
  → Constraint: PeersUp-- is guarded by `if prevStatus == PeerUp`; must also guard for PeerSyncing
- [ ] `cmd/ze-chaos/web/render.go` - writePeerRows uses ps.Status.CSSClass() for dot. writePeerDetail uses CSSClass()+String(). writeRecentEvents uses eventTypeClass/eventTypeLabel.
- [ ] `cmd/ze-chaos/web/viz.go` - statusColor() returns hex colors for 4 states. All-peers tab counts totalUp/totalDown/totalReconn/totalIdle. Peer timeline renders segments using statusColor().
  → Constraint: statusColor is used in timeline segment rendering with inline style
- [ ] `cmd/ze-chaos/web/control.go` - writeControlPanel uses cssStatusUp/cssStatusDown/cssReconnecting for control status dots
- [ ] `cmd/ze-chaos/web/assets/style.css` - CSS classes: status-up (green), status-down (red), status-reconnecting (yellow), status-idle (gray)
- [ ] `cmd/ze-chaos/web/state_activeset.go` - PromotionPriorityForEvent: EventEstablished returns (0, false) — not auto-promoted
- [ ] `cmd/ze-chaos/web/handlers.go` - Filter handler: status filter values are "" (Relevant), "fault", "up", "down", "reconnecting", "idle"

**Behavior to preserve:**
- EOR-based sync tracking (EORSeen, EORCount, SyncDuration) continues to work as before
- Peers that disconnect while syncing must correctly decrement counters
- Timeline visualization shows state segments with correct colors
- Filter dropdown continues to work (with new "syncing" option added)
- All-peers tab counts remain accurate with new state
- Existing 4 states keep their current colors and behavior

**Behavior to change:**
- `EventEstablished` transitions to `PeerSyncing` instead of `PeerUp`
- `EventEORSent` triggers transition from `PeerSyncing` to `PeerUp`
- `PeersUp` increments on EOR (not Established); a new `PeersSyncing` counter tracks syncing peers
- Header and stats show syncing count
- New CSS class `status-syncing` with distinct color (cyan/blue)
- Filter dropdown gains "Syncing" option
- `EventDisconnected`/`EventError` must handle prevStatus == PeerSyncing (decrement PeersSyncing, not PeersUp)

## Data Flow (MANDATORY)

### Entry Point
- `peer.Event` with `Type: EventEstablished` arrives at `Dashboard.ProcessEvent()`
- Later, `peer.Event` with `Type: EventEORSent` arrives for the same peer

### Transformation Path
1. `ProcessEvent` receives `EventEstablished` → sets `ps.Status = PeerSyncing`, increments `PeersSyncing`
2. `ProcessEvent` receives `EventEORSent` → if `ps.Status == PeerSyncing`, sets `ps.Status = PeerUp`, decrements `PeersSyncing`, increments `PeersUp`
3. State transition recorded in `PeerTransitions` (existing logic, no change needed)
4. SSE broadcast picks up dirty peer → renders row with new CSS class
5. Browser receives updated HTML fragment → dot color changes from cyan to green

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Event → Dashboard state | ProcessEvent switch cases | [ ] |
| Dashboard state → HTML | CSSClass()/String()/statusColor() | [ ] |
| HTML → Browser | SSE peer-update event | [ ] |

### Integration Points
- `PeerStatus.CSSClass()` — add `PeerSyncing` → `"status-syncing"` mapping
- `PeerStatus.String()` — add `PeerSyncing` → `"syncing"` mapping
- `statusColor()` in viz.go — add `PeerSyncing` → cyan hex color
- Filter handler — add `"syncing"` to accepted filter values
- All-peers tab counting — add `totalSyncing` counter
- Stats card — show syncing count alongside peers up count
- `syncStat()` function — syncing peers count is now directly visible via dots; sync stat may show both

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing enum, doesn't recreate)
- [ ] Zero-copy preserved where applicable (N/A — no wire encoding)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| ProcessEvent(EventEstablished) | → | ps.Status = PeerSyncing | `TestProcessEventSyncingState` |
| ProcessEvent(EventEORSent) | → | ps.Status = PeerUp | `TestProcessEventSyncingToUp` |
| ProcessEvent(EventDisconnected) while syncing | → | PeersSyncing-- | `TestProcessEventDisconnectWhileSyncing` |
| GET /peers?status=syncing | → | filter returns syncing peers | `TestPeerFilterSyncing` |
| PeerSyncing.CSSClass() | → | "status-syncing" | `TestPeerStatusCSSClass` |
| statusColor(PeerSyncing) | → | cyan hex | `TestStatusColor` |
| SSE peer-update for syncing peer | → | HTML with status-syncing class | `TestSSEPeerUpdateSyncingState` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer receives EventEstablished | Status becomes `PeerSyncing` (not PeerUp); `PeersSyncing` increments; `PeersUp` does NOT increment |
| AC-2 | Syncing peer receives EventEORSent | Status becomes `PeerUp`; `PeersSyncing` decrements; `PeersUp` increments |
| AC-3 | Syncing peer receives EventDisconnected | Status becomes `PeerDown`; `PeersSyncing` decrements; `PeersUp` unchanged |
| AC-4 | Syncing peer receives EventError | Status becomes `PeerDown`; `PeersSyncing` decrements; `PeersUp` unchanged |
| AC-5 | Syncing peer receives EventReconnecting | Status becomes `PeerReconnecting`; `PeersSyncing` decrements |
| AC-6 | Peer table renders syncing peer | Dot uses `status-syncing` CSS class; label shows "syncing" |
| AC-7 | Peer timeline for syncing peer | Timeline segment uses distinct cyan color |
| AC-8 | Filter dropdown with "Syncing" selected | Only syncing peers shown |
| AC-9 | Stats card | Shows "Syncing N" alongside "Peers N/M" when N > 0 |
| AC-10 | All-peers tab | Syncing peers counted separately with correct label and color |
| AC-11 | Peer reconnects after chaos (Established again while EORSeen is true) | Status goes to PeerSyncing; transitions normally to PeerUp on next EOR |
| AC-12 | Header peer count | Format changes to reflect syncing state (e.g., "peers: 47+3/50" where 3 are syncing) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerStatusString` | `cmd/ze-chaos/web/state_test.go` | PeerSyncing.String() returns "syncing" | |
| `TestPeerStatusCSSClass` | `cmd/ze-chaos/web/state_test.go` | PeerSyncing.CSSClass() returns "status-syncing" | |
| `TestStatusColor` | `cmd/ze-chaos/web/viz_test.go` | statusColor(PeerSyncing) returns cyan hex | |
| `TestProcessEventSyncingState` | `cmd/ze-chaos/web/dashboard_test.go` | EventEstablished → PeerSyncing, PeersSyncing++ | |
| `TestProcessEventSyncingToUp` | `cmd/ze-chaos/web/dashboard_test.go` | EventEORSent on syncing peer → PeerUp, PeersSyncing--, PeersUp++ | |
| `TestProcessEventDisconnectWhileSyncing` | `cmd/ze-chaos/web/dashboard_test.go` | EventDisconnected on syncing peer → PeerDown, PeersSyncing-- | |
| `TestProcessEventErrorWhileSyncing` | `cmd/ze-chaos/web/dashboard_test.go` | EventError on syncing peer → PeerDown, PeersSyncing-- | |
| `TestProcessEventReconnectWhileSyncing` | `cmd/ze-chaos/web/dashboard_test.go` | EventReconnecting on syncing peer → PeerReconnecting, PeersSyncing-- | |
| `TestProcessEventReEstablishAfterChaos` | `cmd/ze-chaos/web/dashboard_test.go` | Second Established after disconnect → PeerSyncing again | |
| `TestProcessEventEOROnNonSyncingPeer` | `cmd/ze-chaos/web/dashboard_test.go` | EOR on already-up peer is no-op for status (EOR tracking still works) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PeerSyncing enum value | 0-4 (5 statuses) | 4 (PeerReconnecting) | N/A (iota) | N/A (iota) |
| PeersSyncing counter | 0-PeerCount | PeerCount | 0 (can't go negative) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestSyncingStateRendering` | `cmd/ze-chaos/web/render_test.go` | Peer row HTML contains "status-syncing" class and "syncing" text | |
| `TestPeerFilterSyncing` | `cmd/ze-chaos/web/handlers_test.go` | GET /peers?status=syncing returns only syncing peers | |

## Files to Modify
- `cmd/ze-chaos/web/state.go` — Add PeerSyncing to enum; add PeersSyncing to DashboardState; update String(), CSSClass()
- `cmd/ze-chaos/web/dashboard.go` — ProcessEvent: Established→Syncing, EOR→Up transition; fix Disconnected/Error/Reconnecting guards for PeerSyncing
- `cmd/ze-chaos/web/viz.go` — statusColor() add PeerSyncing case; all-peers counting add totalSyncing
- `cmd/ze-chaos/web/render.go` — Stats card: show syncing count; header: update peer format
- `cmd/ze-chaos/web/handlers.go` — Filter: accept "syncing" status value
- `cmd/ze-chaos/web/assets/style.css` — Add `.status-syncing` / `.dot.status-syncing` with cyan color

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

## Files to Create
- None (all changes are modifications to existing files)

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Add PeerSyncing to enum** — Insert between PeerIdle and PeerUp in state.go. Update String(), CSSClass(). Add PeersSyncing counter to DashboardState. → Review: Does inserting break iota ordering for existing values? (Yes — PeerSyncing must go after PeerReconnecting to avoid changing existing enum values, OR we assign explicit values)
2. **Write unit tests for enum** — TestPeerStatusString, TestPeerStatusCSSClass for all 5 values. → Review: Cover all branches?
3. **Run tests** → Verify FAIL (new status not yet handled)
4. **Update ProcessEvent transitions** — Established→Syncing, EOR→Up, fix disconnect/error/reconnecting guards. → Review: All counter arithmetic correct? No double-decrement?
5. **Write ProcessEvent unit tests** — All 10 tests from TDD plan. → Review: Edge cases covered (EOR on non-syncing, re-establish after chaos)?
6. **Run tests** → Verify PASS
7. **Add CSS class** — `.status-syncing` / `.dot.status-syncing` with cyan color (`var(--accent)` or `#58a6ff`). → Review: Visually distinct from all 4 existing colors?
8. **Update statusColor()** — Add PeerSyncing case. → Review: Used in timeline segments?
9. **Update viz.go all-peers counting** — Add totalSyncing. → Review: Sum still equals PeerCount?
10. **Update render.go stats card** — Show syncing count. Update header format. → Review: Readable when syncing=0?
11. **Update handlers.go filter** — Accept "syncing". Add to filter dropdown options. → Review: Dropdown has 7 options now — still fits?
12. **Write rendering tests** — TestSyncingStateRendering, TestPeerFilterSyncing. → Review: Assert exact CSS class?
13. **Verify all** → `make ze-chaos-test`
14. **Critical Review** → All 6 checks from `rules/quality.md`
15. **Complete spec** → Fill audit tables, move to done/

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 4 (fix enum/counter references) |
| Test fails wrong reason | Step 2 or 5 (fix test expectations) |
| Existing tests fail (PeersUp count changed) | Step 4 (existing tests expect PeersUp++ on Established — must update) |
| Lint failure | Fix inline |
| Timeline colors wrong | Step 8 (check statusColor mapping) |

## Design Insights

### Enum Value Ordering
PeerSyncing should NOT be inserted between existing values — that would change the iota values of PeerDown and PeerReconnecting, breaking any code that compares or stores these as integers. Instead, append PeerSyncing after PeerReconnecting.

### Counter Arithmetic
The key invariant: `PeersUp + PeersSyncing + (down) + (reconnecting) + (idle) = PeerCount`. Every transition that leaves a state must decrement its counter; every transition that enters a state must increment. The disconnect/error/reconnecting handlers currently only guard `prevStatus == PeerUp` — they must also guard `prevStatus == PeerSyncing`.

### Color Choice
Cyan/blue (`#58a6ff` — the existing `--accent` variable) is ideal because:
- Distinct from green (up), red (down), yellow (reconnecting), gray (idle)
- Already in the palette as the accent color
- Semantically neutral-to-positive (syncing = progressing, not failing)

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## RFC Documentation

Add `// RFC 4724: End-of-RIB marker signals initial route transfer complete` above the EOR→PeerUp transition.

## Implementation Summary

### What Was Implemented
- (to be filled after implementation)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] `make ze-chaos-test` passes
- [ ] Feature code integrated (`cmd/ze-chaos/web/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated (chaos-web-dashboard.md status table)
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
- [ ] Spec moved to `docs/plan/done/NNN-chaos-syncing-state.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.
