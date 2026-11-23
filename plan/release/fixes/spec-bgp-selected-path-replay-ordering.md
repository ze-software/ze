# Spec: bgp-selected-path-replay-ordering

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P2 |
| Release reason | Delayed single-path Adj-RIB-In replay responses must not overwrite, delete, or resurrect newer live or selected destination state on Route Server or Route Reflector peers |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-22 |

Split out of `spec-rfc7947-adj-rib-in-accepts-filtered-updates`
on 2026-11-22 (owner approval) so RFC 7947 Section 2.1 pre-policy Adj-RIB-In
retention lands cleanly without carrying an orthogonal RR/RS cold-worker
redesign.

## Task

Single-path replay (`ribevents.RelayReplayPath`, `adj_rib_in.replayPathCommand`)
reads one named source path and separately relays its answer to a destination
peer. When that query or response is delayed relative to newer live UPDATEs,
foreign/local takeovers, validation withdrawals, or peer retirement, an
uncoordinated response can overwrite newer bytes, act on stale ownership, or
cause absence ABA.

This spec captures the 2026-11-16 design investigation and waits for full
design review before implementation.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - reactor forwarding, ownership, and peer lifecycles
  → Constraint: preserve lock order (`aigp.dispatch` before `validationForwardMu`; `writeMu` before `adjOut.mu`); never hold locks across IPC/drain/export execution.
- [ ] `docs/architecture/plugin/rib-storage-design.md` - Adj-RIB-In storage and replay commands
  → Constraint: `replayPathCommand` reads one named source path and answers a withdrawal with no received generation when absent.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/bgp/ribevents` (`RelayReplayPath`), `internal/component/bgp/plugins/adj_rib_in/rib_replay_path.go` (`replayPathCommand`) - reads one named source path, then separately relays its answer; missing paths answer a withdrawal with no received generation.
- [ ] `internal/component/bgp/reactor/session_ownership.go` (`adjOutWrite.allow`), `internal/component/bgp/plugins/adj_rib_in/rib_eligibility.go` (`routeEligible`) - an old received MsgID can become an ineligible withdrawal; ordinary forwarded withdrawal ownership ignores that MsgID.
- [ ] `internal/component/bgp/reactor/receive_publication.go` (`receivePublicationCut.wait`), `internal/component/plugin/process/delivery.go` (`DrainEventsApplied`) - publication completion must precede applied-event drain; neither fences a delayed response's later enqueue.
- [ ] `internal/component/bgp/reactor/recovery_owed.go` (`recoveryAdmission.admit`, `adjOutOwed.noteAnnouncer`) - convergence contract is not a strict stale-response rejection guard.
- [ ] `internal/component/bgp/plugins/rs` (`routeServer.replayValidation`), `internal/component/bgp/plugins/rr` (`routeReflector.runValidation`) - RS runs the query synchronously on its live source worker; RR has a separate joined validation worker.

**Behavior to preserve:**
- Normal live forwarding FIFO and native RS pending gates (`fwdPool.dispatchOverflow`).
- Unrelated destination traffic and cold recovery progress while a single-path query is paused.

**Behavior to change:**
- Coordinate delayed selected-path replay answers against live source and destination state via short-lived per-query tickets and a dedicated coalescing RS validation worker.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Single-path replay | `adj_rib_in/rib_replay_path.go`, `replayPathCommand` | Queries one path and relays asynchronously | Guards response admission with source/destination tickets |
| RR validation worker | `plugins/rr`, `runValidation` | Separate joined validation worker | Reused as pattern for RS coalescing validation worker |
| Destination FIFO admission | `reactor/fwd_pool.go`, `dispatchOverflow` | Admits prepared groups into destination FIFO | Used under short ticket lock for guarded enqueue |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Single-path replay query issued by RS or RR validation/recovery worker against the live receive store.

### Transformation Path
1. Register a short-lived source/destination ticket (bounded to one active cold query per authenticated calling process) before sampling the publication cut.
2. Advance original received UPDATEs past the publication frontier before checking active tickets, invalidating matching tickets before import/projection/selection/forwarding (including unheld withdrawals and EOR retirement).
3. Wait for publication cut, run `drainReceiveStore`, and query the captured store owner without holding reactor/session/table locks.
4. Prepare through ordinary eligibility/export/ownership machinery; verify the live source ticket under its short publication lock and enqueue via `dispatchOverflow`.
5. Keep the destination ticket through final write admission (`writeMu` then `adjOut.mu`) with sticky invalidation on any intervening same-key decision (preventing announce/withdraw/absence ABA).
6. Unregister the exact ticket after locks are released on every outcome path; report explicit applied/superseded/refused/canceled outcomes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RS/RR cold worker → receive store | DirectBridge and forked RPC | No |
| Cold worker → reactor destination FIFO | `dispatchOverflow` under ticket check | No |

### Integration Points
- `internal/component/bgp/reactor/` - ticket lifecycle, frontier invalidation, guarded write admission
- `internal/component/bgp/plugins/rs/` - coalescing validation worker

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | Pending design approval |
| No unintended coupling | No | Pending design approval |
| No duplicated functionality | No | Pending design approval |
| Zero-copy preserved where applicable | No | Pending design approval |
| Registration over hardcoding | No | Pending design approval |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | One active cold query per authenticated calling process suffices for current RS/RR replay owners | Existing single cold worker per replay owner | Concurrent queries stall | Probe under concurrent callers | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Holding locks across drain/RPC deadlocks SDK callback loop | Race/timeout in DirectBridge or forked RPC | Separate coalescing worker; never wait on SDK callback loop |
| R-2 | Stale cleanup removes a newer ticket for the same key | Missing invalidation under hot-key churn | Unregister by exact ticket identity only |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Delayed replay answers can overwrite newer routes or resurrect withdrawn ones |
| How is it reverted? | Revert ticket admission and RS cold worker commit |
| Who else touches this path? | Closed `spec-rfc7947-adj-rib-in-accepts-filtered-updates` added import-filtered suppression to `replayPathCommand` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Delayed single-path replay query | → | Source/destination ticket invalidation | TBD on design approval |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Delayed old announcement or missing-path answer released after newer same-source replacement/withdrawal | No overwrite, deletion, or resurrection |
| AC-2 | Foreign or local takeover occurs while query response is held | Old replay spares the new winner's bytes and ownership |
| AC-3 | Validation withdrawal or another selected winner occurs with no new source UPDATE while response is held | Old source receipt does not restore withdrawn/superseded path |
| AC-4 | Absence ABA (absent → announce → withdraw → absent) while query is held | Old announcement stays absent |
| AC-5 | Query paused without forward-worker placeholder | Unrelated destination traffic and cold recovery complete |
| AC-6 | Cancellation, peer retirement, or store reconnection | Explicit incomplete outcomes, no late write, no retained ticket |
| AC-7 | DirectBridge and forked RPC transports | Identical callback, drain, delayed answer, and completion semantics |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Deterministic race/ordering probes for AC-1..AC-7 | `internal/component/bgp/reactor/` | All 7 ordering and lifecycle boundaries on both transports | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Active cold queries per calling process | 0..1 | 1 | N/A | 2 (explicit busy refusal) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| TBD on design approval | `test/plugin/` | Forked RPC delayed replay ordering | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Internal ordering guard; wire semantics unchanged | |

## Files to Modify
- `internal/component/bgp/reactor/` - ticket registry, source frontier invalidation, destination sticky invalidation
- `internal/component/bgp/plugins/rs/` - coalescing validation worker
- `internal/component/bgp/plugins/adj_rib_in/rib_replay_path.go` - truthful completion outcome contract

## Files to Create
- Deterministic ordering and transport test files in `internal/component/bgp/reactor/`

## Checklist

### Pre-Spec Verification
- [ ] Full design review and owner approval before moving to `ready`
