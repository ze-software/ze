# Spec: bfd-4-operator-ux

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-bfd-3-bgp-client |
| Phase | 1/1 |
| Updated | 2026-04-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec
2. `.claude/rules/planning.md`
3. `.claude/rules/plugin-design.md` -- YANG-driven RPC handler contract
4. `plan/learned/555-bfd-skeleton.md`, `556-bfd-1-wiring.md`, plus Stage 2 and Stage 3 learned summaries
5. `docs/guide/bfd.md` -- sketched "Observing state" section
6. Source files: `internal/plugins/bfd/engine/engine.go`, `internal/plugins/bfd/bfd.go`, `internal/component/telemetry/*`, `cmd/ze/cli/*` or wherever `show bgp` style commands live

## Task

Stage 4 gives operators a way to see what BFD is doing. Until now, the only observability is `ze.log.bfd` debug lines. Operators need:

1. **`show bfd sessions`** -- table of every live session: peer, mode, vrf, state, local/remote discriminator, RX/TX interval, detect time, last state change, clients holding refcounts.
2. **`show bfd session <peer>`** -- single-session detail: everything above plus last N state transitions (small ring buffer on the session).
3. **`show bfd profile [<name>]`** -- resolved profile parameters (after defaults applied).
4. **Prometheus metrics** (if `telemetry` is configured):
   - `ze_bfd_sessions{state,mode,vrf}` gauge
   - `ze_bfd_transitions_total{from,to,diag,mode}` counter
   - `ze_bfd_detection_expired_total{mode}` counter
   - `ze_bfd_tx_packets_total{mode}` / `ze_bfd_rx_packets_total{mode}` counter
5. **YANG RPCs** backing the CLI commands (`.claude/rules/plugin-design.md` -- every RPC needs YANG).

**Explicitly out of Stage 4 scope:**

- Historical session dump (persistence across restarts) -- not planned.
- Per-session packet capture -- would be a separate feature.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/bfd.md`
- [ ] `docs/architecture/api/commands.md` -- RPC conventions
- [ ] `.claude/rules/plugin-design.md` -- YANG + proximity principle
- [ ] `.claude/rules/self-documenting.md`
- [ ] `docs/guide/command-reference.md` -- how existing `show` commands are documented

### Source files

- [ ] `internal/plugins/bfd/engine/engine.go` -- need a read-only snapshot API on Loop
- [ ] `internal/plugins/bfd/bfd.go` -- plugin RPC handlers live here
- [ ] `internal/component/telemetry/*` -- Prometheus registry, how other subsystems register metrics
- [ ] existing `show bgp peers` command path for structure and formatting

## Current Behavior (MANDATORY)

**Source files read:** (filled during /implement)

**Behavior to preserve:**

- BFD plugin lifecycle and engine internals stay untouched -- Stage 4 is purely observational.
- Existing `.ci` tests continue to pass.

**Behavior to change:**

- New read-only snapshot method on `engine.Loop` (e.g., `Snapshot() []SessionState`).
- New RPC handlers under `internal/plugins/bfd/` registered via YANG.
- New Prometheus metric registration guarded by the telemetry config.

## Data Flow

### Entry Point

- Operator types `show bfd sessions` in the CLI or submits the equivalent JSON-RPC.
- Prometheus scraper GETs the telemetry endpoint.

### Transformation Path

1. CLI → dispatcher → BFD plugin RPC handler
2. Handler calls `engine.Loop.Snapshot()` which returns a copy of session state under `mu`
3. Handler formats as JSON (CLI rendering) or text table (terminal rendering)
4. Prometheus: a gauge callback reads the snapshot periodically (or on-demand per scrape)

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ Plugin | Dispatch RPC, YANG-defined | [ ] |
| Plugin ↔ Engine | `Loop.Snapshot()` under `mu` | [ ] |
| Telemetry ↔ Plugin | prometheus gauge registered with current BFD metrics | [ ] |

### Integration Points

- `engine.Loop.Snapshot()` new method
- `internal/plugins/bfd/handlers_show.go` new file for RPC handlers
- `internal/plugins/bfd/metrics.go` new file registering Prometheus metrics
- YANG RPCs in `ze-bfd-conf.yang` (or a new `ze-bfd-api.yang`)

### Architectural Verification

- [ ] Snapshot returns a copy, not live pointers
- [ ] No long-held lock: Snapshot copies then releases `mu`
- [ ] RPC handlers in `internal/plugins/bfd/` (proximity principle)
- [ ] Metrics registered via the same telemetry pattern as other subsystems

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| CLI `show bfd sessions` | → | `bfd.handleShowSessions` → `engine.Loop.Snapshot` | `test/plugin/bfd-show-sessions.ci` |
| CLI `show bfd session <peer>` | → | `bfd.handleShowSession` | `test/plugin/bfd-show-session.ci` |
| CLI `show bfd profile` | → | `bfd.handleShowProfile` | `test/plugin/bfd-show-profile.ci` |
| Prometheus scrape | → | `bfd.metrics` gauge populated from snapshot | `test/plugin/bfd-metrics.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with two single-hop sessions + `show bfd sessions` | Output lists both, one row each, with correct state/mode/vrf/discriminator |
| AC-2 | `show bfd session 192.0.2.1` | Per-session detail including last 5 transitions |
| AC-3 | `show bfd session 198.51.100.9` (unknown peer) | Clear "not found" message, exit code 1 |
| AC-4 | `show bfd profile fast` | Resolved profile including defaults |
| AC-5 | `show bfd profile` (no name) | Lists all profiles |
| AC-6 | Prometheus scrape with 2 Up + 1 Down session | `ze_bfd_sessions{state="up"} 2` + `ze_bfd_sessions{state="down"} 1` |
| AC-7 | Session flaps Up→Down→Up | `ze_bfd_transitions_total{from="up",to="down"}` increments by 1; same for Down→Up |
| AC-8 | Detect time expires on a session | `ze_bfd_detection_expired_total{mode="single-hop"}` increments by 1 |
| AC-9 | Snapshot under concurrent load | No deadlock, no missed sessions, no duplicated entries |
| AC-10 | YANG validate accepts the new RPC definitions | `bin/ze config validate <the YANG module>` passes |
| AC-11 | `plan/deferrals.md` row `spec-bfd-4-operator-ux` | Marked done pointing to learned summary |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLoopSnapshotEmpty` | `internal/plugins/bfd/engine/snapshot_test.go` | Snapshot returns empty slice on empty Loop | |
| `TestLoopSnapshotTwoSessions` | `internal/plugins/bfd/engine/snapshot_test.go` | Two sessions returned in deterministic order | |
| `TestLoopSnapshotConcurrent` | `internal/plugins/bfd/engine/snapshot_test.go` | `go test -race` clean with concurrent EnsureSession / ReleaseSession during Snapshot calls | |
| `TestHandleShowSessions` | `internal/plugins/bfd/handlers_show_test.go` | Handler returns JSON matching schema | |
| `TestHandleShowSessionNotFound` | `internal/plugins/bfd/handlers_show_test.go` | Unknown peer returns structured "not found" | |
| `TestMetricsRegistered` | `internal/plugins/bfd/metrics_test.go` | After plugin Start, metrics appear in the telemetry registry | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `state` enum | up/down/init/admin-down | "admin-down" | unknown string | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bfd-show-sessions` | `test/plugin/bfd-show-sessions.ci` | Two pinned sessions; `show bfd sessions` output contains both | |
| `bfd-show-session` | `test/plugin/bfd-show-session.ci` | Single pinned session; `show bfd session <peer>` returns detail | |
| `bfd-show-profile` | `test/plugin/bfd-show-profile.ci` | Profile `fast` defined; `show bfd profile fast` returns resolved values | |
| `bfd-metrics` | `test/plugin/bfd-metrics.ci` | Two sessions; curl the telemetry endpoint; assert gauge values | |

### Future
- None.

## Files to Modify

- `internal/plugins/bfd/engine/engine.go` or new `internal/plugins/bfd/engine/snapshot.go` -- `Snapshot()` + `SessionState` struct
- `internal/plugins/bfd/schema/ze-bfd-conf.yang` -- new RPCs (or new `ze-bfd-api.yang`)
- `internal/plugins/bfd/bfd.go` -- register RPC handlers
- `internal/plugins/bfd/handlers_show.go` (new) -- handler code
- `internal/plugins/bfd/metrics.go` (new) -- Prometheus wiring
- `plan/deferrals.md` -- close Stage 4 row
- `docs/guide/bfd.md` -- real "Observing state" section
- `docs/guide/command-reference.md` -- add `show bfd *`
- `docs/features.md` -- BFD observability
- `docs/comparison.md`

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG RPCs | [ ] Yes | `ze-bfd-conf.yang` or new `ze-bfd-api.yang` |
| CLI commands | [ ] Yes (automatic via YANG + dispatch handler) | - |
| Editor autocomplete | [ ] Yes (automatic from YANG) | - |
| Functional test | [ ] Yes | four `.ci` tests |
| Prometheus metrics | [ ] Yes | `internal/component/telemetry` pattern |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File |
|---|----------|----------|------|
| 1 | New user-facing feature? | [ ] Yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] No | - |
| 3 | CLI command added? | [ ] Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added? | [ ] Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin changed? | [ ] Yes | `docs/guide/plugins.md` |
| 6 | User guide page? | [ ] Yes | `docs/guide/bfd.md` |
| 7 | Wire format? | [ ] No | - |
| 8 | Plugin SDK/protocol? | [ ] No | - |
| 9 | RFC behavior? | [ ] No (operational surface) | - |
| 10 | Test infrastructure? | [ ] No | - |
| 11 | Daemon comparison? | [ ] Yes | `docs/comparison.md` |
| 12 | Internal architecture? | [ ] Yes | `docs/architecture/bfd.md` |
| 13 | Route metadata? | [ ] No | - |

## Files to Create

- `internal/plugins/bfd/engine/snapshot.go`
- `internal/plugins/bfd/engine/snapshot_test.go`
- `internal/plugins/bfd/handlers_show.go`
- `internal/plugins/bfd/handlers_show_test.go`
- `internal/plugins/bfd/metrics.go`
- `internal/plugins/bfd/metrics_test.go`
- `test/plugin/bfd-show-sessions.ci`
- `test/plugin/bfd-show-session.ci`
- `test/plugin/bfd-show-profile.ci`
- `test/plugin/bfd-metrics.ci`

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files tables |
| 3. Implement | Implementation Phases |
| 4. Verify | `make ze-verify` |
| 5. Critical review | Critical Review Checklist |
| 6. Fix issues | - |
| 7. Re-verify | - |
| 8. Repeat | Max 2 passes |
| 9. Deliverables | - |
| 10. Security review | - |
| 11. Re-verify | - |
| 12. Present summary | - |

### Implementation Phases

1. **Phase: Snapshot API** -- `engine.Loop.Snapshot()` returns `[]SessionState` copy.
   - Tests: `TestLoopSnapshotEmpty`, `TestLoopSnapshotTwoSessions`, `TestLoopSnapshotConcurrent`
2. **Phase: YANG RPCs** -- `show-bfd-sessions`, `show-bfd-session`, `show-bfd-profile` definitions.
3. **Phase: Handlers** -- `handlers_show.go` wires RPCs to snapshot.
   - Tests: `TestHandleShowSessions`, `TestHandleShowSessionNotFound`
4. **Phase: Metrics** -- `metrics.go` registers gauges/counters; state-change notify increments counters; snapshot populates gauges.
   - Tests: `TestMetricsRegistered`
5. **Phase: Functional tests** -- four `.ci` tests.
6. **Phase: Docs** -- update guide, comparison, features.
7. **Phase: Close spec** -- audit, learned summary, deferral row.

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Completeness | Every AC has implementation at file:line |
| Correctness | Snapshot is a deep enough copy that mutation in the loop after Snapshot returns does not race the reader |
| Naming | `Snapshot`, `SessionState`, `handleShowSessions` |
| Data flow | CLI → RPC → handler → Snapshot → formatted output |
| Rule: proximity | Handlers in `internal/plugins/bfd/`, not in a separate `handler/` |
| Rule: YANG required | Every RPC has a YANG definition |

### Deliverables Checklist

| Deliverable | Verification |
|-------------|--------------|
| Snapshot API | `go test ./internal/plugins/bfd/engine/...` passes |
| YANG RPCs | `bin/ze schema lint` (or equivalent) passes |
| Handlers wired | `bin/ze-test plugin bfd-show-sessions` passes |
| Metrics | `bin/ze-test plugin bfd-metrics` passes |
| Docs updated | each file in the table has a diff |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | `show bfd session <peer>` validates peer is a valid IP before lookup |
| Resource exhaustion | Snapshot allocation bounded by session count (config-limited); no unbounded memory |
| Information disclosure | Profile output does not leak auth secrets (Stage 5 concern, sanity-check here) |
| Concurrency | Snapshot holds `mu` briefly, copies, releases; no reader starvation |

### Failure Routing

| Failure | Route to |
|---------|----------|
| Race detector trips on Snapshot | Rework to copy under lock, not read-through |
| RPC handler panics on unknown peer | Return structured error, not panic |
| 3 fix attempts fail | STOP |

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

- Counters (`ze_bfd_transitions_total`) update on every state change via the existing notify path. Gauges populate from Snapshot at scrape time.

## RFC Documentation

- Not applicable (operational surface, no RFC MUST).

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
- [ ] AC-1..AC-11 demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes (includes `make ze-test` -- lint + all ze tests)
- [ ] Feature code integrated
- [ ] Functional tests pass
- [ ] Docs updated
- [ ] Critical Review passes

### Quality Gates
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Snapshot is copy, not pointer
- [ ] Proximity principle respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests
- [ ] Functional tests

### Completion
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written
- [ ] Summary in commit
