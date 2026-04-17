# Spec: rs-fastpath-2-adjrib -- adj-rib-in off the forwarding hot path

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-rs-fastpath-1-profile |
| Phase | - |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec
2. Umbrella: `spec-rs-fastpath-0-umbrella.md`
3. Sibling (completed): `plan/learned/NNN-rs-fastpath-1-profile.md`
4. `internal/component/bgp/plugins/adj_rib_in/rib.go`, `rib_commands.go`
5. `internal/component/bgp/plugins/rs/register.go` (hard dep declaration)
6. `internal/component/bgp/plugins/rs/server_handlers.go` (replay on peer up)
7. `internal/component/plugin/server/dispatch.go` (DirectBridge subscribe)
8. `plan/learned/068-spec-remove-adjrib-integration.md` — prior decoupling

## Task

Second child of the `rs-fastpath` umbrella. Two linked changes:

1. **adj-rib-in becomes an async side-subscriber.** Today every inbound UPDATE is delivered to adj-rib-in on the hot-path delivery goroutine, so BART insert sits in the forwarding latency. Change: adj-rib-in subscribes to the same DirectBridge event but stores asynchronously (worker goroutine + bounded queue), so the forward path does not wait.
2. **Relax `bgp-rs → bgp-adj-rib-in` from hard to soft dependency.** `rs/register.go` currently declares `Dependencies: ["bgp-adj-rib-in"]`, forcing auto-load. Change: rs runs forward-only without adj-rib-in; replay-on-new-peer becomes a no-op with a clear warning log when adj-rib-in is not loaded. Users who want replay keep loading adj-rib-in; users who want pure pass-through can opt out.

Both changes preserve correctness (per-source ordering, replay semantics when adj-rib-in is present, flow control, backpressure) and existing `.ci` tests.

Depends on Phase 1 of this umbrella landing the profile evidence and umbrella AC-1 target.

## Required Reading

### Architecture Docs

- [ ] `.claude/rules/design-principles.md`
- [ ] `.claude/rules/plugin-design.md`
- [ ] `plan/learned/068-spec-remove-adjrib-integration.md`

### RFC Summaries

- [ ] `rfc/short/rfc4271.md` — UPDATE processing.

**Key insights:** (filled during RESEARCH)

## Current Behavior

**Source files read:**
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` — BART insertion, per-peer storage.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` — `adj-rib-in replay` handler.
- [ ] `internal/component/bgp/plugins/rs/register.go` — hard dep declaration.
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` — `handleStateUp` dispatches `adj-rib-in replay` and runs convergent delta loop.
- [ ] `internal/component/plugin/server/dispatch.go` — DirectBridge `subscribeEvents` / `emitEvent` / `deliverEvent`.
- [ ] `internal/component/plugin/registry/*.go` — how `Dependencies: [...]` is resolved at startup.

**Behavior to preserve:**
- Replay-on-new-peer produces the same routes, in the same AFI/SAFI order, as today when adj-rib-in is present.
- Per-source ordering of forwarded UPDATEs.
- Backpressure (pause-source) still fires on the rs worker pool.
- All existing `.ci` tests pass unchanged, assuming adj-rib-in is still auto-loaded by default.

**Behavior to change:**
- Ingress hot-path event delivery to adj-rib-in is asynchronous (delivery goroutine returns before BART insert).
- Replay command (`adj-rib-in replay <peer> <index>`) waits on storage drain before replying, so replay content is still correct.
- `rs/register.go` declares adj-rib-in as a soft/optional dependency. Resolver logic: if adj-rib-in is loaded, rs uses it for replay; if not, rs logs a one-shot warning and skips replay (new peer gets only the routes received after it connected).

## Data Flow

### Entry Point

- Inbound UPDATE bytes on TCP arrive at a peer session.

### Transformation Path

1. Session `Run()` reads wire bytes.
2. Reactor publishes UPDATE event via DirectBridge.
3. Hot-path subscriber (rs plugin) dispatches forwarding.
4. Side-path subscriber (adj-rib-in) enqueues the event to a bounded storage queue; a dedicated worker goroutine drains the queue and inserts into BART.
5. On peer up, rs dispatches `adj-rib-in replay <peer> <index>` via the existing command path. Replay handler waits for the storage queue to drain to `index` before replying, guaranteeing snapshot consistency.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ adj-rib-in | DirectBridge side-subscription | [ ] |
| adj-rib-in async storage | bounded channel + worker goroutine | [ ] |
| rs ↔ adj-rib-in | command dispatch (unchanged) | [ ] |
| Plugin resolver ↔ rs registration | "soft dep" semantics | [ ] |

### Integration Points

- `internal/component/plugin/registry/*.go` — add "optional dependency" concept (or equivalent) that does not block plugin startup when absent.
- `internal/component/bgp/plugins/adj_rib_in/rib.go` — add storage queue + worker; subscribe as side-subscriber.
- `internal/component/bgp/plugins/rs/register.go` — declare dep as optional.
- `internal/component/bgp/plugins/rs/server_handlers.go` — if adj-rib-in not loaded, log warning and skip replay body.

### Architectural Verification

- [ ] No bypassed layers.
- [ ] No unintended coupling.
- [ ] No duplicated functionality.
- [ ] Zero-copy preserved where applicable (storage-queue item is a reference, not a copy).

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Two peers + bgp-rs + bgp-adj-rib-in; sender streams; third peer joins | → | async side-subscription + replay drain wait | `test/plugin/bgp-rs-replay-mid-stream.ci` |
| Two peers + bgp-rs (no bgp-adj-rib-in); sender streams; third peer joins | → | soft-dep fallback + warning log | `test/plugin/bgp-rs-no-adjrib.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 100k IPv4 route bench, adj-rib-in loaded | Forwarding throughput matches Phase 1's profile-validated target for this stage (set into this row once Phase 1 ships). |
| AC-2 | Peer joins mid-stream; adj-rib-in has N routes stored | New peer receives N routes via replay; replay content byte-identical to pre-change. |
| AC-3 | adj-rib-in storage queue filled to high-water mark | Back-pressure: new events wait or are dropped with a counter; documented behaviour, no silent loss. |
| AC-4 | Ze starts without `bgp-adj-rib-in` plugin loaded | Ze starts successfully; one warning log at startup ("bgp-rs: adj-rib-in not loaded; replay disabled"). Forwarding works for all subsequent peers. |
| AC-5 | Ze starts without adj-rib-in; peer joins mid-stream | Replay is skipped (no-op). New peer receives only routes received after it connected. |
| AC-6 | All existing `.ci` tests | Pass unchanged. |
| AC-7 | `make ze-test`, `make ze-verify-fast`, `make ze-race-reactor` | All clean. |
| AC-8 | Memory-bounded storage queue | Unit test asserts the queue is bounded and drains on Stop. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAdjRibInAsyncStore` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | DirectBridge deliver returns before storage completes; stored entry appears after worker drains. | |
| `TestAdjRibInQueueBounded` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | Queue capacity is bounded; overflow behaviour documented and tested. | |
| `TestAdjRibInReplayDrainsQueue` | `internal/component/bgp/plugins/adj_rib_in/rib_commands_test.go` | Replay handler waits for queue drain before responding; no lost routes. | |
| `TestRSSoftDepSkipsReplay` | `internal/component/bgp/plugins/rs/server_handlers_test.go` | When adj-rib-in is absent, replay dispatch is skipped; warning logged once. | |
| `TestRegistryOptionalDepStart` | `internal/component/plugin/registry/registry_test.go` | Plugin declaring an optional dependency starts when dep is missing. | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Storage queue capacity | 1..max | TBD set in design | 0 | TBD |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-replay-mid-stream` | `test/plugin/bgp-rs-replay-mid-stream.ci` | Two peers + bgp-rs + adj-rib-in; sender streams 1000 routes; third peer joins; third peer receives all 1000 via replay. | |
| `bgp-rs-no-adjrib` | `test/plugin/bgp-rs-no-adjrib.ci` | Two peers + bgp-rs only (adj-rib-in absent); sender streams; third peer joins; third peer gets only post-join routes; one warning log at startup. | |

### Future

- None.

## Files to Modify

- `internal/component/bgp/plugins/adj_rib_in/rib.go` — subscribe as side-subscriber; add bounded queue + worker goroutine.
- `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` — replay handler waits for queue drain.
- `internal/component/bgp/plugins/rs/register.go` — declare adj-rib-in as optional dep.
- `internal/component/bgp/plugins/rs/server_handlers.go` — graceful no-op when adj-rib-in absent.
- `internal/component/plugin/registry/registry.go` (or equivalent) — optional-dependency resolution.
- `.claude/rules/plugin-design.md` — document optional-dependency semantics.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | — |
| CLI commands | [ ] | — |
| Editor autocomplete | [ ] | — |
| Functional test for new behavior | [ ] | `test/plugin/bgp-rs-replay-mid-stream.ci`, `test/plugin/bgp-rs-no-adjrib.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` — rs-without-adj-rib-in is a new operational mode |
| 2 | Config syntax changed? | [ ] | — (no new config) |
| 3 | CLI command added/changed? | [ ] | — |
| 4 | API/RPC added/changed? | [ ] | — |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` — rs dependency relaxed |
| 6 | Has a user guide page? | [ ] | `docs/guide/<route-server>.md` — note replay requires adj-rib-in |
| 7 | Wire format changed? | [ ] | — |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` — optional-dependency semantics |
| 9 | RFC behavior implemented? | [ ] | — |
| 10 | Test infrastructure changed? | [ ] | — |
| 11 | Affects daemon comparison? | [ ] | — (umbrella owns the final numbers) |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` — side-subscriber concept |

## Files to Create

- `test/plugin/bgp-rs-replay-mid-stream.ci`
- `test/plugin/bgp-rs-no-adjrib.ci`
- `plan/learned/NNN-rs-fastpath-2-adjrib.md` (on completion)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. `/ze-review` gate | Review Gate |
| 5. Full verification | `make ze-test`, `make ze-verify-fast`, `make ze-race-reactor` |
| 6–9. Critical review + fixes | Critical Review Checklist |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | Re-run stage 5 |
| 13. Executive Summary | Per `rules/planning.md` |

### Implementation Phases

1. **Phase 1 — Async storage.** Add bounded storage queue + worker goroutine to adj-rib-in. Switch ingress subscription to side-subscriber. Replay handler waits for queue drain.
   - Tests: `TestAdjRibInAsyncStore`, `TestAdjRibInQueueBounded`, `TestAdjRibInReplayDrainsQueue`, `bgp-rs-replay-mid-stream.ci`.
   - Files: `adj_rib_in/rib.go`, `rib_commands.go`.
   - Verify: bench shows hot-path delivery time no longer scales with BART insert cost; replay content byte-identical.
2. **Phase 2 — Optional dependency.** Add optional-dep concept to the plugin registry. Update `rs/register.go` to declare adj-rib-in as optional. Update `rs/server_handlers.go` to skip replay with a warning when adj-rib-in is absent.
   - Tests: `TestRegistryOptionalDepStart`, `TestRSSoftDepSkipsReplay`, `bgp-rs-no-adjrib.ci`.
   - Files: `plugin/registry/*.go`, `rs/register.go`, `rs/server_handlers.go`, `.claude/rules/plugin-design.md`.
   - Verify: ze starts both with and without adj-rib-in; functional tests pass.
3. **Functional tests** → included in phases 1 and 2.
4. **Full verification** → `make ze-verify-fast`, `make ze-race-reactor`.
5. **Complete spec** → audit tables, learned summary.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has test + file:line. |
| Correctness | Replay content byte-identical to pre-change when adj-rib-in is present. |
| Rule: no-layering | Synchronous storage code path fully removed, not co-existing. |
| Rule: goroutine-lifecycle | Worker is long-lived; stopped on plugin shutdown; no per-event goroutines. |
| Rule: plugin-design | Optional-dep semantics documented in rules/plugin-design.md. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Async storage merged | `grep -n "side-subscrib\|async" internal/component/bgp/plugins/adj_rib_in/rib.go` |
| Optional-dep declared | `grep -n "Optional\|Soft" internal/component/bgp/plugins/rs/register.go` |
| `.ci` tests pass | `bin/ze-test plugin -p bgp-rs-replay-mid-stream`, `... bgp-rs-no-adjrib` |
| `rules/plugin-design.md` updated | `git diff .claude/rules/plugin-design.md` shows optional-dep section |
| Learned summary | `ls plan/learned/*rs-fastpath-2-adjrib*.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | Replay command args still validated (peer address, index). |
| Resource exhaustion | Storage queue bounded; overflow behaviour documented and tested. |
| Error leakage | Warning log when adj-rib-in absent is informational, not an error path. |
| Concurrency | Race detector clean; storage worker does not hold locks during BART insert. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Replay content diverges from pre-change | Fix in Phase 1; drain-wait semantics wrong |
| Optional-dep breaks auto-load ordering for other plugins | Fix registry in Phase 2; verify with `make ze-inventory` |
| `ze-race-reactor` flags new race | Fix in the phase that introduced it |
| 3 fix attempts fail | STOP. Ask user. |

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

<!-- LIVE -->

## RFC Documentation

- RFC 4271 — adj-rib-in per-peer inbound snapshot semantics; async storage must preserve.

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

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

- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-race-reactor` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)

- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design

- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit
- [ ] Minimal coupling

### TDD

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)

- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-rs-fastpath-2-adjrib.md`
- [ ] Summary included in commit
