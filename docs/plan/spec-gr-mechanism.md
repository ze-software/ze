# Spec: gr-mechanism

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc4724.md` — GR algorithm, timers, MUST requirements
4. `docs/architecture/core-design.md` — reactor, peer lifecycle, event dispatch
5. `internal/component/bgp/reactor/peer.go` — peer FSM callback, session lifecycle
6. `internal/component/bgp/reactor/reactor_notify.go` — PeerLifecycleObserver

## Task

Implement the Graceful Restart **mechanism** (RFC 4724) — the runtime behavior that uses the already-existing GR capability to retain routes, manage timers, and purge stale entries. The bgp-gr plugin today is capability-only (inject GR capability code 64 into OPEN). This spec adds the behavioral side: what happens when a GR-capable peer's session drops.

**Scope:** Receiving Speaker procedures only (Section 4.2). Ze does not yet need Restarting Speaker procedures (Section 4.1) — ze does not preserve forwarding state across its own restart.

**Why now:** Without the mechanism, GR capability is advertised but never acted on. When a GR-capable peer's session drops, routes are deleted immediately — identical to non-GR behavior. This defeats the purpose of advertising GR.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — reactor event loop, peer lifecycle
  → Constraint: Reactor owns peer sessions; plugin infrastructure routes events
  → Decision: PeerLifecycleObserver is the hook for session up/down
- [ ] `docs/architecture/behavior/fsm.md` — FSM states, transitions
  → Constraint: FSM is pure state machine; reactor handles I/O and timer management
- [ ] `docs/architecture/wire/capabilities.md` — capability negotiation
  → Constraint: SessionCaps.GracefulRestart holds negotiated GR state per session

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4724.md` — Graceful Restart mechanism
  → Constraint: NOTIFICATION terminates GR — no route retention
  → Constraint: Restart Timer range 0-4095 seconds, from peer's GR capability
  → Constraint: Stale routes purged on EOR receipt per AFI/SAFI
  → Constraint: No GR capability on reconnect → delete all stale routes immediately
  → Constraint: Missing AFI/SAFI or F-bit=0 on reconnect → delete stale for that family
  → Constraint: "previously marked as stale MUST be deleted" on consecutive restarts

**Key insights:**
- GR Receiving Speaker behavior is triggered by TCP failure (not NOTIFICATION) when peer had GR capability
- The Restart Timer value comes from the **peer's** GR capability (their advertised restart time)
- EOR is already sent by ze (per-family, in `sendInitialRoutes`); received EOR detection is needed
- `RetainUpdate`/`ReleaseUpdate` in the reactor cache provide the building block for route retention
- Route retention is per-AFI/SAFI — families listed in peer's GR capability with F-bit set

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/bgp-gr/gr.go` (344L) — Capability-only plugin. `RunGRPlugin` uses SDK 5-stage protocol. `OnConfigure` extracts per-peer restart-time from config, encodes as hex capability code 64. Also provides CLI/IPC decode mode for hex→JSON/text.
- [ ] `internal/component/bgp/plugins/bgp-gr/register.go` (47L) — Registration: name=bgp-gr, CapabilityCodes=[64], Features="capa yang", ConfigRoots=["bgp"]
- [ ] `internal/component/bgp/capability/session.go` (28L) — `SessionCaps.GracefulRestart *GracefulRestart` — stores negotiated GR state per session
- [ ] `internal/component/bgp/capability/capability.go` — `GracefulRestart` struct: RestartTime, RestartFlags, Families (AFI/SAFI + ForwardingState). Parsed from OPEN, written with `WriteTo`
- [ ] `internal/component/bgp/message/eor.go` (43L) — `BuildEOR(family)` creates EOR UPDATE per RFC 4724. IPv4 unicast = empty UPDATE; others = MP_UNREACH_NLRI with AFI/SAFI only
- [ ] `internal/component/bgp/reactor/peer.go` (~900L) — FSM state callback at line ~875: when leaving Established, calls `notifyPeerClosed` then clears negotiated caps and encoding contexts. No GR check.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` (216L) — `PeerLifecycleObserver` interface: `OnPeerEstablished`, `OnPeerClosed`. `notifyMessageReceiver` dispatches received UPDATEs to plugins via cache + delivery channel.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` — `RetainUpdate(id)` and `ReleaseUpdate(id, plugin)` for cache-level route retention
- [ ] `internal/component/bgp/filter/filter.go` — `FilterResult.IsEOR()` detects EOR markers
- [ ] `test/plugin/graceful-restart.ci` — Functional test: routes resent after reconnect. Tests static route replay, NOT GR stale/timer behavior.

**Behavior to preserve:**
- GR capability injection via bgp-gr plugin (config→capability encode→OPEN)
- GR capability decode (CLI and IPC modes)
- EOR sending after initial routes (per negotiated family)
- Existing functional tests (graceful-restart.ci, graceful-restart-rib.ci)
- `PeerLifecycleObserver` interface (extend, don't break)

**Behavior to change:**
- Session teardown when peer had GR capability: currently deletes all routes → retain routes, mark stale, start restart timer
- Reconnect from GR-capable peer: currently normal → check F-bit per family, purge stale for missing/non-forwarding families
- Received EOR: currently no action → purge remaining stale routes for that family
- Restart timer expiry: no timer exists → purge all stale routes from peer

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- TCP connection fails for a peer whose negotiated capabilities include GR (code 64)
- The FSM transitions from Established to Idle/Connect
- `notifyPeerClosed` fires on all `PeerLifecycleObserver` instances

### Transformation Path
1. **Session down detected** — FSM callback in `peer.go` line ~875
2. **GR check** — Was this peer GR-capable? Check `SessionCaps.GracefulRestart` before clearing negotiated caps
3. **Route retention** — Mark peer's routes as stale (per AFI/SAFI from GR capability where F-bit set)
4. **Timer start** — Start restart timer using peer's advertised Restart Time
5. **Session re-establishment** — Peer reconnects, new OPEN exchange
6. **GR validation** — Check new OPEN's GR capability: present? Same families? F-bit set?
7. **Route replacement** — New UPDATEs replace stale routes
8. **EOR receipt** — Per-family: purge remaining stale routes
9. **Timer expiry (no reconnect)** — Purge all stale routes from peer

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ GR state | GR state lives in reactor (per-peer), consulted on session events | [ ] |
| FSM ↔ Reactor | FSM fires callback, reactor checks GR state | [ ] |
| Engine ↔ Plugin | Plugin receives peer-down/peer-up events; engine handles timers + stale marking | [ ] |

### Integration Points
- `PeerLifecycleObserver` — GR state manager registers as observer
- `SessionCaps.GracefulRestart` — Read before clearing on session down
- `Negotiated` capabilities on reconnect — Compare old vs new GR capability
- EOR detection in message receiver — Trigger stale purge per family
- Event dispatcher — Notify plugins of GR events (stale marking, purge)

### Architectural Verification
- [ ] No bypassed layers (GR hooks into existing lifecycle observer pattern)
- [ ] No unintended coupling (GR state manager is a separate component, not embedded in peer)
- [ ] No duplicated functionality (uses existing capability negotiation, extends peer lifecycle)
- [ ] Zero-copy preserved where applicable (route retention keeps cache entries, no copy)

## Design

### Where GR State Lives

GR state is **per-peer** and managed by a `GRStateManager` in the reactor. It is NOT in the bgp-gr plugin — the plugin only handles capability injection. The engine must own GR state because:
1. Timer management requires access to the reactor's clock and scheduler
2. Route retention requires access to the route cache
3. Session lifecycle events originate in the reactor

### GR State Per Peer

| Field | Type | Purpose |
|-------|------|---------|
| StaleRoutes | map of family → set of route IDs | Routes marked stale after session down |
| RestartTimer | timer reference | Fires when peer doesn't reconnect in time |
| PeerGRCap | saved GR capability | From previous session's negotiated caps |
| Active | bool | True between session-down and EOR-complete/timer-expiry |

### State Transitions

| Event | Condition | Action |
|-------|-----------|--------|
| Session down (TCP fail) | Peer had GR capability, reason is NOT NOTIFICATION | Mark routes stale, start restart timer |
| Session down (TCP fail) | Peer had no GR capability | Delete routes immediately (existing behavior) |
| Session down (NOTIFICATION) | Any | Delete routes immediately (RFC 4724 Section 4) |
| Restart timer expires | No reconnect | Purge all stale routes for peer |
| Peer reconnects | New OPEN has GR capability | Check per-family F-bit, purge stale for missing/non-F families |
| Peer reconnects | New OPEN lacks GR capability | Purge all stale routes immediately |
| UPDATE received | Matches stale route | Replace stale route (remove stale mark) |
| EOR received | For a specific family | Purge remaining stale routes for that family |

### Received EOR Detection

Ze already detects EOR via `FilterResult.IsEOR()` in the filter layer. This needs to be wired to the GR state manager when processing received UPDATEs. The reactor's message receiver path (`notifyMessageReceiver`) is the integration point.

### NOTIFICATION vs TCP Failure Distinction

The FSM callback currently passes reason as a string ("connection lost" vs "session closed"). For GR, we need to distinguish:
- **TCP failure without NOTIFICATION** → GR applies (retain routes)
- **NOTIFICATION received** → GR does NOT apply (delete routes per RFC 4724 Section 4)

The peer's `notifyPeerClosed` call needs to include whether a NOTIFICATION was involved.

### Plugin Notification

The bgp-gr plugin does not need to manage timers or stale routes. However, plugins observing peer events should be notified of GR state changes so they can adjust behavior (e.g., RIB plugin should know not to delete routes for a GR-active peer). This uses the existing event dispatch mechanism.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| TCP failure for GR-capable peer | → | Routes retained + timer started | `TestGRRouteRetentionOnSessionDown` |
| Restart timer expiry | → | Stale routes purged | `TestGRTimerExpiryPurgesStaleRoutes` |
| Peer reconnects with GR+F-bit | → | Stale routes kept until EOR | `TestGRReconnectPreservesForwardingState` |
| Peer reconnects without GR | → | Stale routes purged immediately | `TestGRReconnectWithoutCapPurgesStale` |
| EOR received for family | → | Stale routes for that family purged | `TestGREORPurgesStaleFamilyRoutes` |
| NOTIFICATION → session down | → | Routes deleted (no GR) | `TestGRNotificationBypassesRetention` |
| Config with GR + peer session cycle | → | End-to-end GR behavior | `test/plugin/gr-stale-purge.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GR-capable peer session drops (TCP fail, no NOTIFICATION) | Routes retained, marked stale, restart timer started |
| AC-2 | Restart timer expires without reconnect | All stale routes from peer deleted |
| AC-3 | Peer reconnects with GR capability and F-bit set for a family | Stale routes for that family kept until EOR or new UPDATE |
| AC-4 | Peer reconnects without GR capability | All stale routes deleted immediately |
| AC-5 | Peer reconnects with GR but F-bit=0 for a family | Stale routes for that family deleted immediately |
| AC-6 | Peer reconnects with GR but family missing from capability | Stale routes for that family deleted immediately |
| AC-7 | EOR received for a family | Remaining stale routes for that family deleted |
| AC-8 | UPDATE received replacing a stale route | Stale mark removed, route treated as fresh |
| AC-9 | NOTIFICATION triggers session down | Routes deleted immediately (standard BGP, no GR) |
| AC-10 | Consecutive restarts (peer drops again while stale routes exist) | Previously stale routes deleted, current routes marked stale |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGRStateManagerRouteRetention` | `internal/component/bgp/reactor/gr_state_test.go` | AC-1: stale marking on session down | |
| `TestGRStateManagerTimerExpiry` | `internal/component/bgp/reactor/gr_state_test.go` | AC-2: timer fires, stale purged | |
| `TestGRStateManagerReconnectWithFBit` | `internal/component/bgp/reactor/gr_state_test.go` | AC-3: F-bit preserves stale | |
| `TestGRStateManagerReconnectNoGR` | `internal/component/bgp/reactor/gr_state_test.go` | AC-4: no GR → purge all | |
| `TestGRStateManagerReconnectFBitZero` | `internal/component/bgp/reactor/gr_state_test.go` | AC-5: F-bit=0 → purge family | |
| `TestGRStateManagerReconnectMissingFamily` | `internal/component/bgp/reactor/gr_state_test.go` | AC-6: missing family → purge | |
| `TestGRStateManagerEORPurge` | `internal/component/bgp/reactor/gr_state_test.go` | AC-7: EOR triggers purge per family | |
| `TestGRStateManagerUpdateReplacesStale` | `internal/component/bgp/reactor/gr_state_test.go` | AC-8: fresh UPDATE unstales route | |
| `TestGRStateManagerNotificationBypass` | `internal/component/bgp/reactor/gr_state_test.go` | AC-9: NOTIFICATION = no retention | |
| `TestGRStateManagerConsecutiveRestarts` | `internal/component/bgp/reactor/gr_state_test.go` | AC-10: old stale deleted before new stale | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Restart Time | 0-4095 | 4095 | N/A (0 is valid) | N/A (capped by capability wire format) |
| Stale route count | 0-N | N routes | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gr-timer-expiry` | `test/plugin/gr-timer-expiry.ci` | Peer drops, timer expires, routes gone | |
| `gr-stale-purge-eor` | `test/plugin/gr-stale-purge-eor.ci` | Peer reconnects, sends EOR, stale purged | |
| `gr-notification-no-retain` | `test/plugin/gr-notification-no-retain.ci` | NOTIFICATION session down, routes deleted immediately | |

### Future (if deferring any tests)
- Restarting Speaker procedures (RFC 4724 Section 4.1) — ze restart preserving forwarding state. Requires separate spec when ze supports process restart with state preservation.
- Selection Deferral Timer — only relevant for Restarting Speaker, deferred with above.

## Files to Modify
- `internal/component/bgp/reactor/peer.go` — Save GR capability before clearing on session down; distinguish NOTIFICATION vs TCP failure
- `internal/component/bgp/reactor/reactor_notify.go` — Extend `PeerLifecycleObserver` or add GR-specific hooks
- `internal/component/bgp/reactor/reactor.go` — Register GR state manager as observer

## Files to Create
- `internal/component/bgp/reactor/gr_state.go` — GR state manager (per-peer stale tracking, timers)
- `internal/component/bgp/reactor/gr_state_test.go` — Unit tests for GR state manager
- `test/plugin/gr-timer-expiry.ci` — Functional: timer-based stale purge
- `test/plugin/gr-stale-purge-eor.ci` — Functional: EOR-based stale purge
- `test/plugin/gr-notification-no-retain.ci` — Functional: NOTIFICATION bypasses GR

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No — GR mechanism is engine-internal | |
| RPC count in architecture docs | [ ] No | |
| CLI commands/flags | [ ] No | |
| CLI usage/help text | [ ] No | |
| API commands doc | [ ] No | |
| Plugin SDK docs | [ ] No — plugin behavior unchanged | |
| Editor autocomplete | [ ] No | |
| Functional test for new RPC/API | [x] Yes — functional tests in test/plugin/ | `test/plugin/gr-*.ci` |

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests for GRStateManager** → Table-driven tests for all 10 ACs. Review: edge cases? Consecutive restarts?
2. **Run tests** → Verify FAIL (paste output). Fail for RIGHT reason (type/function not found)?
3. **Implement GRStateManager** → Per-peer stale tracking, timer management, lifecycle observer hooks. Minimal code to pass.
4. **Run tests** → Verify PASS (paste output). All pass? Any flaky?
5. **Wire into reactor** → Modify `peer.go` to save GR caps before clearing, add NOTIFICATION distinction. Register GR state manager.
6. **Wire EOR detection** → Connect received EOR events to GR state manager for stale purge.
7. **RFC refs** → Add `// RFC 4724 Section X.Y` comments above enforcing code
8. **Functional tests** → Create `.ci` tests for timer expiry, EOR purge, NOTIFICATION bypass
9. **Verify all** → `make ze-test`
10. **Critical Review** → All 6 checks from `rules/quality.md`
11. **Complete spec** → Fill audit tables, write learned summary

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 (fix syntax/types) |
| Test fails wrong reason | Step 1 (fix test) |
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

## RFC Documentation

Add `// RFC 4724 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: timer constraints (Restart Time from peer's capability), stale marking trigger (TCP fail only, not NOTIFICATION), purge triggers (EOR per family, timer expiry, reconnect validation), consecutive restart handling.

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
- [ ] AC-1..AC-10 all demonstrated
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
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
