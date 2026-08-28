# Spec: fleet-5 -- Staged Rollout

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-fleet-2-config-templates, spec-fleet-4-inventory-health |
| Phase | - |
| Updated | 2026-05-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/plugin/server/managed.go` -- ManagedConfigService (config-changed dispatch)
4. `spec-fleet-0-umbrella.md` -- umbrella design decisions
5. `spec-fleet-1-device-registry.md` -- device registry
6. `spec-fleet-2-config-templates.md` -- config templates and groups (dependency)
7. `spec-fleet-4-inventory-health.md` -- health reporting (dependency)

## Task

Add staged rollout capability: push config changes to a percentage of devices in a group,
monitor ACK results, and automatically pause if the failure rate exceeds a threshold. The
operator can then inspect, resume, or abort the rollout.

Currently config-changed notifications go to all connected devices simultaneously. For a
group of 100 devices, a bad config breaks all 100 at once. Staged rollout limits blast
radius by notifying devices in batches.

This spec depends on config templates (fleet-2, for group targeting) and health reporting
(fleet-4, for rollout health gates). The client protocol is unchanged; the hub controls
which devices get notified and when.

### Key Design Decisions (from umbrella)

| Decision | Detail |
|----------|--------|
| Hub-side orchestration | Client protocol unchanged. Hub controls notification order and timing |
| State machine | States: pending, rolling, paused, complete, failed. Persisted in ZeFS for crash recovery |
| Failure threshold | Configurable (default 10%). If N% of a batch reject config, rollout pauses |
| Batch size | Percentage-based (e.g., 10% of group). Minimum 1 device per batch |
| Health gate | Optional: require all devices in batch to report healthy before proceeding to next batch |
| Manual control | Operator can pause, resume, abort, or force-complete a rollout |
| Connected-only targeting | Per the single-writer rule (`spec-fleet-6`), the hub cannot change a disconnected device's config, so a rollout targets only currently-connected devices. Offline devices are not staged; they pick up the current hub config when they reconnect (AC-14 already pauses a rollout with no connected devices) |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` -- config-changed notification flow
  -> Constraint: rollout controls when config-changed is sent, not how
- [ ] `internal/component/plugin/server/managed.go` -- BuildConfigChanged sends to one client
  -> Decision: rollout orchestrator calls BuildConfigChanged per device in batch order

**Key insights:**
- BuildConfigChanged(clientName) already targets a single device; rollout just controls the sequence
- config-ack carries ok/error; rollout uses this for pass/fail counting
- Rollout state must survive hub restart (persist in ZeFS)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed.go` -- BuildConfigChanged targets one device; no batching or sequencing
  -> Constraint: rollout wraps existing BuildConfigChanged, does not replace it
- [ ] `pkg/fleet/envelope.go` -- config-ack has OK bool and Error string
  -> Decision: rollout tracks per-device ack results for pass/fail counting
- [ ] `internal/component/plugin/server/registry.go` -- DeviceRegistry from fleet-1 (group queries)
  -> Decision: rollout targets devices via group membership from registry
- [ ] `internal/component/plugin/server/template.go` -- template rendering from fleet-2
  -> Decision: rollout triggers template-based config-changed for group

**Behavior to preserve:**
- Direct config-changed (non-rollout) still works for immediate pushes
- config-ack handling unchanged
- Client protocol unchanged

**Behavior to change:**
- New rollout orchestrator intercepts group-wide config-changed
- Operator initiates rollout via CLI: `fleet rollout start --group <g> --batch-percent <n> --failure-threshold <n>`
- Rollout state machine tracks progress, pauses on failure
- `show fleet rollout` shows active/recent rollouts with per-device status

## Data Flow (MANDATORY)

### Entry Point
- Operator runs `fleet rollout start --group region-west --batch-percent 20 --failure-threshold 10`
- Template change with rollout policy triggers automatic rollout

### Transformation Path
1. Operator initiates rollout for group
2. Rollout orchestrator queries DeviceRegistry for group members
3. Orchestrator partitions devices into batches (by percentage)
4. For batch 1: send config-changed to each device
5. Wait for config-ack from each device in batch (with timeout)
6. Count pass/fail. If failure rate > threshold: transition to paused
7. If batch passes: proceed to next batch
8. After all batches: transition to complete

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI to Rollout | `fleet rollout start` command | [ ] |
| Rollout to ManagedConfigService | BuildConfigChanged per device in batch | [ ] |
| config-ack to Rollout | Ack results tracked per device | [ ] |
| Rollout to ZeFS | State persisted for crash recovery | [ ] |

### Integration Points
- `ManagedConfigService.BuildConfigChanged` -- called per device by rollout
- `DeviceRegistry` from fleet-1 -- group membership for batch partitioning
- Health reports from fleet-4 -- optional health gate between batches
- CLI -- `fleet rollout start/pause/resume/abort/show`
- Web -- rollout progress on fleet dashboard

### Architectural Verification
- [ ] No bypassed layers (rollout uses existing config-changed path)
- [ ] No unintended coupling (non-rollout config-changed unaffected)
- [ ] No duplicated functionality (orchestrates existing primitives)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `fleet rollout start --group region-west` | -> | Rollout state machine creates, sends batch 1 | `test/managed/fleet-staged-rollout.ci` |
| All devices in batch ACK ok | -> | Rollout proceeds to next batch | `test/managed/fleet-staged-rollout.ci` |
| Device in batch rejects config (> threshold) | -> | Rollout pauses automatically | `test/managed/fleet-staged-rollout.ci` |
| `fleet rollout resume` | -> | Rollout continues from paused state | `test/managed/fleet-staged-rollout.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `fleet rollout start --group region-west --batch-percent 20` | Rollout created in pending state, first batch notified |
| AC-2 | All devices in batch 1 send config-ack ok | Rollout proceeds to batch 2 |
| AC-3 | 2 of 5 devices in batch reject config (failure-threshold 10%) | Rollout transitions to paused; no more batches sent |
| AC-4 | `fleet rollout resume` on paused rollout | Rollout retries current batch (or proceeds to next if operator chose to skip) |
| AC-5 | `fleet rollout abort` on active rollout | Rollout transitions to failed; no more notifications sent |
| AC-6 | All batches complete successfully | Rollout transitions to complete |
| AC-7 | `show fleet rollout` | Lists active and recent rollouts: group, progress (batch N/M), state, failure count |
| AC-8 | `show fleet rollout detail <id>` | Detail: per-device status (pending/ack-ok/ack-reject/timeout), batch boundaries |
| AC-9 | Hub restarts during active rollout | Rollout state loaded from ZeFS; resumes from last persisted batch |
| AC-10 | Device in batch does not respond within timeout (default 120s) | Counted as failure; timeout configurable |
| AC-11 | Health gate enabled: device reports degraded after ACK | Rollout pauses before next batch |
| AC-12 | Group has 3 devices, batch-percent 50 | Batch 1: 2 devices, Batch 2: 1 device (minimum 1 per batch) |
| AC-13 | Direct config-changed (no rollout) | Still works immediately for non-rollout pushes |
| AC-14 | Rollout for group with no connected devices | Rollout transitions to paused with "no connected devices" reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRolloutStateMachine` | `internal/component/plugin/server/rollout_test.go` | State transitions: pending -> rolling -> complete | |
| `TestRolloutPauseOnFailure` | `internal/component/plugin/server/rollout_test.go` | Pause when failure exceeds threshold | |
| `TestRolloutResume` | `internal/component/plugin/server/rollout_test.go` | Resume from paused state | |
| `TestRolloutAbort` | `internal/component/plugin/server/rollout_test.go` | Abort transitions to failed | |
| `TestRolloutBatchPartition` | `internal/component/plugin/server/rollout_test.go` | Correct batch sizing with minimum 1 | |
| `TestRolloutPersist` | `internal/component/plugin/server/rollout_test.go` | State survives reload from ZeFS | |
| `TestRolloutTimeout` | `internal/component/plugin/server/rollout_test.go` | Device timeout counted as failure | |
| `TestRolloutHealthGate` | `internal/component/plugin/server/rollout_test.go` | Degraded health pauses rollout | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-staged-rollout` | `test/managed/fleet-staged-rollout.ci` | Start rollout, devices ACK, rollout completes; also: device rejects, rollout pauses | |

## Files to Modify
- `internal/component/plugin/server/managed.go` -- rollout-aware config-changed dispatch
- `internal/component/plugin/server/registry.go` -- group membership queries for batch partitioning
- `cmd/ze/hub/main.go` -- wire rollout orchestrator

## Files to Create
- `internal/component/plugin/server/rollout.go` -- Rollout type, state machine, batch orchestration, persistence
- `internal/component/plugin/server/rollout_test.go` -- unit tests
- `test/managed/fleet-staged-rollout.ci` -- functional test

## Implementation Steps

### Implementation Phases

1. **Phase: Rollout state machine** -- states, transitions, batch partitioning
   - Tests: `TestRolloutStateMachine`, `TestRolloutBatchPartition`, `TestRolloutPauseOnFailure`, `TestRolloutResume`, `TestRolloutAbort`
   - Files: `rollout.go`, `rollout_test.go`
   - Verify: unit tests pass

2. **Phase: Config-ack tracking** -- per-device ack/reject/timeout tracking
   - Tests: `TestRolloutTimeout`
   - Files: `rollout.go`, `managed.go`
   - Verify: ack results drive state transitions

3. **Phase: Persistence + crash recovery** -- rollout state in ZeFS
   - Tests: `TestRolloutPersist`
   - Files: `rollout.go`
   - Verify: rollout survives hub restart

4. **Phase: Health gate** -- optional health check between batches
   - Tests: `TestRolloutHealthGate`
   - Files: `rollout.go`
   - Verify: degraded health pauses rollout

5. **Phase: CLI + Web** -- `fleet rollout` commands, web progress view
   - Tests: `fleet-staged-rollout.ci`
   - Files: CLI commands, web page extensions
   - Verify: rollout lifecycle manageable via CLI

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-N have implementation |
| Correctness | Failure counting uses percentage, not absolute count; batch minimum is 1 |
| Naming | CLI uses `fleet rollout` prefix; states use lowercase |
| Data flow | CLI -> orchestrator -> BuildConfigChanged per device -> ack tracking -> state transition |
| Rule: no-partial-completion | Rollout with persistence means crash recovery works |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Auth scope | `fleet rollout start` requires admin authorization |
| Concurrent rollouts | Only one active rollout per group at a time |
| Resource exhaustion | Rollout state bounded (max 100 recent rollouts retained) |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify current mode full` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
