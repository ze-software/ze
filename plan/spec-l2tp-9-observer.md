# Spec: l2tp-9 -- Session Observer, Event Namespace, and CQM Sampler

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-l2tp-6c-ncp, spec-l2tp-7-subsystem |
| Phase | - |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-l2tp-0-umbrella.md` -- parent umbrella, "Event namespace" in-scope bullet
4. `internal/core/events/typed.go` -- typed event bus primitive
5. `internal/component/l2tp/session_fsm.go`, `tunnel_fsm.go`, `session.go`, `tunnel.go`
6. `internal/plugins/bfd/metrics.go` (precedent for subsystem-adjacent telemetry)

## Task

Provide the per-L2TP observability foundation: a typed event namespace for L2TP,
per-session event ring buffers, per-login CQM sample ring buffers, and a
long-lived DirectBridge observer that routes published events and samples into
those ring buffers. Sampler generates LCP Echo-Request on each established PPP
session at 1 Hz, aggregates raw echo results into 100-second min/avg/max/loss
buckets, and tags each bucket with session-state (established, negotiating,
down). All ring buffer memory is pre-allocated at subsystem start.

This spec executes the "Event namespace" in-scope bullet already promised by
`spec-l2tp-0-umbrella.md`. Consumers downstream are `spec-l2tp-10-metrics` and
`spec-l2tp-11-web`.

## Design Decisions (agreed with user, 2026-04-17)

| # | Decision |
|---|----------|
| D3 | Storage, sampler, and observer live in `internal/component/l2tp/` (adjacent to emitting FSM code). Precedent: `internal/plugins/bfd/metrics.go`, `internal/component/vpp/telemetry.go`. |
| D4 | Event transport: typed events via `internal/core/events/`. Observer subscribes via DirectBridge for zero-copy hot path. |
| D5 | Aggregated samples also flow through DirectBridge (uniform stream, fast lane), not direct ring-buffer writes. |
| D10 | Sample ring keyed by login (PPP username). Event ring keyed by session ID. |
| D11 | Bucket state enum distinguishes `established`, `negotiating`, `down`. Tx-limit and loss render as overlays, not states. |
| D12 | Retention: 24h at 100s resolution per login (864 buckets, ~34 KB/login). |
| D13 | Login identity: PPP username. |
| X | Cross-cutting: pre-allocate all rings at subsystem start based on `max-logins` config leaf. LRU eviction when full. Zero runtime allocation. |

## Scope

### In Scope

| Area | Description |
|------|-------------|
| Event namespace | Define `l2tp.*` event types: `tunnel-up`, `tunnel-down`, `session-up`, `session-down`, `lcp-up`, `lcp-down`, `ipcp-up`, `ipv6cp-up`, `auth-success`, `auth-failure`, `echo-timeout`, `tx-limit-hit`, `disconnect-requested`. Stable field set per type. |
| Event publisher wiring | Tunnel FSM, session FSM, PPP layer, RADIUS plugin, disconnect handler all publish via `core/events/` emit API. |
| Per-session event ring | Bounded ring buffer keyed by session ID. Pre-allocated. Size: TBD during DESIGN (target order of magnitude 200 events). |
| Per-login sample ring | Bounded ring buffer keyed by PPP username. 864 buckets. Pre-allocated. LRU eviction when `max-logins` reached. |
| CQM sampler | Long-lived worker that drives LCP Echo-Request per established session at 1 Hz, matches replies by Identifier, computes RTT, aggregates into 100s buckets, emits one sample event per bucket boundary. |
| DirectBridge observer | Long-lived worker subscribing to `l2tp.*` and sample stream. Routes each record into the matching ring. |
| YANG config | `max-logins`, `sample-retention-seconds` (default 86400), `event-ring-size-per-session`. Env var registration per `rules/go-standards.md`. |
| Subsystem lifecycle | Observer and sampler start/stop tied to L2TP subsystem Start/Stop. |

### Out of Scope (other specs / deferred)

| Area | Location |
|------|----------|
| Prometheus exposure of observer state | `spec-l2tp-10-metrics` |
| Web UI, JSON/CSV/SSE feeds, uPlot chart | `spec-l2tp-11-web` |
| Disconnect action | `spec-l2tp-11-web` |
| Generic `internal/core/cqm/` engine | Deferred until second non-L2TP probe consumer exists (3+ use-case rule) |
| Persistent archive to disk | Out. All state in-memory; restart clears. |

## Required Reading

### Architecture Docs
<!-- Filled during DESIGN phase when research starts. -->
- [ ] `docs/architecture/core-design.md` -- registration pattern, subsystem lifecycle
- [ ] `docs/architecture/api/architecture.md` -- typed event bus contract
- [ ] `plan/spec-l2tp-0-umbrella.md` -- parent umbrella scope and event namespace commitment

### RFC Summaries
- [ ] `rfc/short/rfc1661.md` -- PPP LCP Echo-Request/Reply format and semantics
- [ ] `rfc/short/rfc2661.md` -- L2TPv2 session state transitions

**Key insights:** (filled during DESIGN phase)

## Current Behavior (MANDATORY)

**Source files to read during DESIGN phase:**
- [ ] `internal/component/l2tp/session_fsm.go` -- where session-up/session-down events should be published
- [ ] `internal/component/l2tp/tunnel_fsm.go` -- where tunnel-up/tunnel-down events should be published
- [ ] `internal/core/events/events.go`, `typed.go` -- event bus API and DirectBridge mechanism
- [ ] `internal/plugins/bfd/metrics.go` -- subsystem-adjacent telemetry precedent
- [ ] `internal/component/l2tp/reactor.go` -- where the sampler worker is started and stopped

**Behavior to preserve:** (filled during DESIGN phase)

**Behavior to change:** Status quo: L2TP emits slog records only, no typed events. This spec introduces the typed event namespace and publisher call sites.

## Data Flow (MANDATORY)

### Entry Points
- FSM state transitions (session FSM, tunnel FSM)
- PPP negotiation completions (LCP, IPCP, IPv6CP)
- RADIUS plugin outcomes
- CQM sampler (LCP echo loop)

### Transformation Path
1. Publisher emits typed event via `events.EmitTyped(l2tp, <type>, payload)`
2. DirectBridge subscriber (observer worker) receives event without dispatch-path allocation
3. Observer hashes event to session ID or login, appends to matching ring
4. Ring buffers read by metrics exporter (`spec-l2tp-10`) and web feeds (`spec-l2tp-11`)

### Boundaries Crossed
| Boundary | How |
|----------|-----|
| L2TP subsystem to observer | Typed event emit via `core/events/` |
| Sampler to observer | DirectBridge sample stream |
| Observer to downstream consumers | Ring buffer read API (no re-emission) |

### Integration Points
- `spec-l2tp-7-subsystem` Start/Stop wires observer and sampler workers
- `spec-l2tp-6c-ncp` provides LCP Echo-Request send + reply match
- `spec-l2tp-8-plugins` RADIUS plugin becomes an event publisher

### Architectural Verification (filled during DESIGN phase)
- [ ] Zero-copy preserved on DirectBridge path
- [ ] Pre-allocation verified at Start (no runtime `make` in hot path)
- [ ] LRU eviction tested at `max-logins`

## Wiring Test (MANDATORY)

<!-- Filled during DESIGN phase with concrete .ci test names. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| LCP Echo-Request scheduled on established session | → | CQM sampler 100s bucket aggregation | `test/l2tp/observer-cqm-bucket.ci` |
| Session FSM enters Established | → | `l2tp.session-up` event published, observer appends to ring | `test/l2tp/observer-event-routing.ci` |
| `max-logins` reached | → | LRU eviction on new login arrival | `test/l2tp/observer-lru-eviction.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | PPP session reaches Established | One `l2tp.session-up` typed event is published; observer appends it to the per-session event ring |
| AC-2 | Session stays up for 100 seconds with all echoes succeeding | One aggregated sample appears in the per-login sample ring with state=`established`, loss=0, min/avg/max RTT populated |
| AC-3 | 10% of echoes in a 100s window time out | Bucket's loss field reflects 10; `echo-timeout` events appear in the per-session event ring at the time of each timeout |
| AC-4 | Session drops | Bucket covering the down window has state=`down`; session-ring remains keyed by that session ID and is retained until LRU reclaims it |
| AC-5 | Same login reconnects on a new session ID | Sample ring for that login continues appending (login-keyed continuity); event ring starts fresh for the new session ID |
| AC-6 | `max-logins` reached and a new login arrives | LRU login's ring is reclaimed; no runtime allocation; new login's ring is a pre-allocated slot |
| AC-7 | Subsystem Start | All rings pre-allocated (verifiable by observing zero allocation on sustained traffic) |
| AC-8 | Subsystem Stop | Observer and sampler workers exit cleanly; no goroutine leak |

## 🧪 TDD Test Plan

### Unit Tests (filled during DESIGN phase)

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TBD | `internal/component/l2tp/observer_test.go` | event routing to correct ring | |
| TBD | `internal/component/l2tp/cqm_test.go` | 100s bucket aggregation math | |
| TBD | `internal/component/l2tp/cqm_test.go` | state enum transitions within bucket | |
| TBD | `internal/component/l2tp/observer_test.go` | LRU eviction at `max-logins` | |
| TBD | `internal/component/l2tp/observer_test.go` | pre-allocation at Start, no runtime `make` | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `max-logins` | 1-1000000 | 1000000 | 0 | 1000001 |
| `sample-retention-seconds` | 100-86400 | 86400 | 99 | 86401 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| TBD | `test/l2tp/observer-cqm-bucket.ci` | Session establishes, 100s elapses, one bucket lands with expected fields | |
| TBD | `test/l2tp/observer-event-routing.ci` | FSM transitions generate events in per-session ring | |
| TBD | `test/l2tp/observer-lru-eviction.ci` | Exceeding `max-logins` evicts LRU | |

## Files to Modify

- `internal/component/l2tp/reactor.go` -- start/stop observer and sampler workers
- `internal/component/l2tp/session_fsm.go` -- publish session-up/session-down typed events
- `internal/component/l2tp/tunnel_fsm.go` -- publish tunnel-up/tunnel-down typed events
- `internal/component/l2tp/schema/ze-l2tp-conf.yang` -- add `max-logins`, `sample-retention-seconds`, `event-ring-size-per-session` leaves
- `internal/component/l2tp/subsystem.go` -- subsystem start wires observer+sampler lifecycle

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | `internal/component/l2tp/schema/ze-l2tp-conf.yang` |
| Env vars for new leaves | [ ] | Per `rules/go-standards.md` env section |
| Functional test for end-to-end routing | [ ] | `test/l2tp/observer-*.ci` |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | [ ] | N/A (no CLI in this spec) |
| 4 | API/RPC added/changed? | [ ] | Event namespace docs in `docs/architecture/api/events.md` |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | `docs/guide/l2tp.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/l2tp.md` (new observer + CQM section) |

## Files to Create

- `internal/component/l2tp/observer.go` -- observer worker, ring-buffer types, DirectBridge subscribe
- `internal/component/l2tp/cqm.go` -- sampler worker, bucket aggregator, state tagging
- `internal/component/l2tp/events.go` -- l2tp namespace registration and typed event definitions
- `internal/component/l2tp/observer_test.go`, `cqm_test.go`, `events_test.go`
- `test/l2tp/observer-*.ci` (multiple; filled during DESIGN phase)

## Implementation Steps

### /implement Stage Mapping (filled during DESIGN phase)

### Implementation Phases (filled during DESIGN phase)

Outline (rough, to be refined):
1. Event namespace definition and publisher wiring in FSM code
2. Per-session event ring, observer worker, DirectBridge subscription
3. Per-login sample ring with pre-allocation and LRU
4. CQM sampler: LCP echo loop + 100s aggregator + state tagging
5. Functional tests
6. Docs and learned summary

### Critical Review Checklist (filled during DESIGN phase)

### Deliverables Checklist (filled during DESIGN phase)

### Security Review Checklist (filled during DESIGN phase)

### Failure Routing (inherited from template)

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source, back to RESEARCH |
| Lint failure | Fix inline; if architectural, back to DESIGN |
| Functional test fails | Check AC; back to DESIGN or IMPLEMENT |
| 3 fix attempts fail | STOP. Report. Ask user. |

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

(LIVE -- written during DESIGN and IMPLEMENT phases)

## RFC Documentation

Add `// RFC 1661 Section 5.8` (LCP Echo-Request/Reply) near sampler code when implemented.

## Implementation Summary (filled during IMPLEMENT)

## Implementation Audit (filled during IMPLEMENT)

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

## Review Gate (filled during IMPLEMENT)

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above

## Pre-Commit Verification (filled during IMPLEMENT)

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table filled with concrete test names
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated in subsystem Start/Stop
- [ ] Documentation updated

### Quality Gates
- [ ] RFC 1661 Section 5.8 reference comment near sampler
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Pre-allocation verified at Start
- [ ] No runtime `make` on observer or sampler hot path
- [ ] DirectBridge zero-copy preserved
- [ ] Single responsibility per file (observer, cqm, events)

### TDD
- [ ] Tests written
- [ ] Tests FAIL first
- [ ] Tests PASS after implementation
- [ ] Boundary tests for `max-logins`, `sample-retention-seconds`
- [ ] Functional tests exercise end-to-end event routing

### Completion (BLOCKING before commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary at `plan/learned/NNN-l2tp-9-observer.md`
- [ ] Summary in same commit as code
