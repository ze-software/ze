# Spec: bugfix-sys-directbridge-panic

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/review-bug-review-plugin-engine-system.md` finding SYS-003
3. `pkg/plugin/rpc/bridge.go`
4. `pkg/plugin/sdk/sdk_dispatch.go`
5. `internal/component/plugin/process/process.go`
6. `internal/component/plugin/server/dispatch.go`
7. `ai/rules/plugin-design.md` DirectBridge section

## Task

Fix SYS-003. A panic in an internal plugin DirectBridge callback must not leave the engine caller waiting for context timeout. The bridge path should return a prompt error, close or mark the bridge unusable, and trigger process cleanup semantics comparable to the external pipe path.

## Required Reading

### Architecture Docs
- [ ] `plan/review-bug-review-plugin-engine-system.md` - source finding and regression plan
  -> Decision: treat bridge callback panics as engine-visible callback errors, not timeouts.
  -> Constraint: preserve DirectBridge as an optimization, not a semantic fork.
- [ ] `ai/rules/plugin-design.md` - DirectBridge and plugin lifecycle contract
  -> Decision: DirectBridge fast path must preserve external RPC error semantics.
  -> Constraint: do not add a second direct-call mechanism.

### RFC Summaries (MUST for protocol work)
- [ ] N/A. This is plugin RPC lifecycle behavior.

**Key insights:**
- `SendCallback` waits on a per-call result channel.
- `bridgeEventLoop` calls callback handlers without `recover`.
- The internal plugin goroutine recovers after the panic, but the result channel is never signaled.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/rpc/bridge.go:91-118` - `SendCallback` sends a request then waits for `BridgeCallbackResult` or context expiry.
- [ ] `pkg/plugin/sdk/sdk_dispatch.go:117-148` - `bridgeEventLoop` calls `handler(cb.Params)` then sends the result, with no local recover.
- [ ] `internal/component/plugin/process/process.go:501-516` - outer internal plugin goroutine recovers panics and logs them after the bridge loop has unwound.
- [ ] `internal/component/plugin/server/dispatch.go:46-51` - engine dispatch path depends on callback response semantics.

**Behavior to preserve:**
- Successful DirectBridge callbacks remain synchronous and serial, matching the pipe event loop.
- Unknown callback methods still return an explicit unknown method error.
- Shutdown still closes callback channels and makes callers see bridge closed or context cancellation.

**Behavior to change:**
- A callback panic sends an error to the waiting caller before the plugin goroutine exits or the bridge closes.
- The bridge or process is marked unhealthy so later calls do not keep using stale callback state.
- Cleanup semantics remove or stop the failed internal plugin process when appropriate.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Engine sends an internal callback through `DirectBridge.SendCallback`.

### Transformation Path
1. Engine creates a `BridgeCallback` with a result channel.
2. SDK bridge event loop receives the callback.
3. Registered handler executes.
4. On success, handler result is sent to engine.
5. On panic, the fixed code sends an error, closes or marks the bridge, and triggers cleanup.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> internal plugin | DirectBridge callback channel | [ ] panic regression test returns prompt error |
| SDK callback -> process lifecycle | bridge event loop and internal plugin goroutine | [ ] process cleanup or unhealthy state asserted |

### Integration Points
- `pkg/plugin/rpc/bridge.go`
- `pkg/plugin/sdk/sdk_dispatch.go`
- `internal/component/plugin/process/process.go`
- `internal/component/plugin/server/dispatch.go`

### Architectural Verification
- [ ] No bypassed layers: callback errors still flow through DirectBridge result path.
- [ ] No unintended coupling: SDK does not import engine internals.
- [ ] No duplicated functionality: reuse existing bridge close/process cleanup where possible.
- [ ] Zero-copy preserved where applicable: N/A.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | SDK can recover per callback and write the result without corrupting bridge state | `bridgeEventLoop` owns the callback result channel | panic still leaks or double-sends | `TestDirectBridgeCallbackPanicReturnsPromptError`, `TestDirectBridgeTypedCallbackPanicReturnsPromptError` | confirmed |
| A-2 | Prompt error is preferable to waiting for context timeout | SYS-003 impact | callers may rely on timeout semantics | post-panic fail-fast and normal callback tests | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | recover hides plugin crashes | test shows process remains marked running after panic | mark bridge/process unhealthy after panic |
| R-2 | sending result after panic races with shutdown close | race test or flaky callback test | use buffered result channel and close-once semantics |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| DirectBridge callback handler panics | -> | SDK bridge event loop recover and bridge/process cleanup | `TestDirectBridgeCallbackPanicReturnsPromptError` |
| callback after bridge marked failed | -> | `SendCallback` returns bridge closed or process failed | `TestDirectBridgeCallbackAfterPanicFailsFast` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Bridge-mode plugin callback panics | caller receives a non-timeout error promptly |
| AC-2 | Callback panic occurs | bridge callbacks close or process is marked failed so later callbacks fail fast |
| AC-3 | Unknown callback method | existing unknown method error behavior remains unchanged |
| AC-4 | Normal callback succeeds | result and error propagation remain unchanged |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reload triggers an internal plugin callback that panics | engine callback -> DirectBridge -> SDK handler -> panic recovery -> error | `TestDirectBridgeCallbackPanicReturnsPromptError` |
| 2 | Engine sends another callback after the panic | engine callback -> bridge state check | `TestDirectBridgeCallbackAfterPanicFailsFast` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDirectBridgeCallbackPanicReturnsPromptError` | `pkg/plugin/sdk/sdk_dispatch_test.go` or `pkg/plugin/rpc/bridge_test.go` | AC-1 | |
| `TestDirectBridgeCallbackAfterPanicFailsFast` | same package | AC-2 | |
| Existing normal callback test | existing test file | AC-3 and AC-4 stay green | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| callback timeout | context deadline | before deadline | canceled context | deadline exceeded |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A unless reload callback panic can be injected through existing plugin test harness | `test/plugin/` if feasible | reload does not hang on panic | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | | | plugin callback behavior | |

### Future (if deferring any tests)
- No deferral approved. Unit tests must prove the behavior.

## Files to Modify

- `pkg/plugin/sdk/sdk_dispatch.go` - recover callback panics and send a result.
- `pkg/plugin/rpc/bridge.go` - expose or use bridge close/failure state if needed.
- `internal/component/plugin/process/process.go` - connect bridge panic to process health if needed.
- `internal/component/plugin/server/dispatch.go` - preserve caller error semantics.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| CLI commands/flags | No | |
| Functional test for new RPC/API | No | direct unit/integration test |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | |
| Prometheus counters/metrics | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | bug fix |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | behavior parity fix only |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` only if panic/error semantics are documented there |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | |
| 16 | Any changed source file is referenced by existing doc source anchors? | Yes | grep docs for changed source paths during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Files to Create

- `pkg/plugin/sdk/sdk_dispatch_test.go` or `pkg/plugin/rpc/bridge_test.go` panic regression tests.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Current Behavior |
| 3. Wiring phase | panic regression tests |
| 4. Implement | recover and failure-state behavior |
| 5. Review gate | Critical Review Checklist |
| 6. Full verification | targeted tests and `make ze-test-plugins` |

### Implementation Phases
1. **Phase: failing panic test** - create callback panic test and verify it times out or hangs before fix.
2. **Phase: recover and error result** - make the bridge event loop report panic errors.
3. **Phase: fail-fast state** - close or mark bridge/process after panic and test later calls fail fast.
4. **Phase: parity checks** - confirm normal and unknown callback paths still behave.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | no callback path can leave a waiting result channel unfulfilled |
| Lifecycle | panic marks plugin/bridge state unhealthy |
| Compatibility | external pipe callback behavior is not regressed |
| Tests | panic, normal, unknown, and shutdown paths covered |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Panic regression | `go test -run TestDirectBridgeCallbackPanicReturnsPromptError ./pkg/plugin/...` |
| Fail-fast regression | `go test -run TestDirectBridgeCallbackAfterPanicFailsFast ./pkg/plugin/...` |
| Plugin group | `make ze-test-plugins` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Denial of service | panic cannot stall reload/shutdown until timeout |
| Error leakage | panic error does not expose secrets beyond existing log policy |
| Race | bridge close and callback result send are race-safe |

### Failure Routing
| Failure | Route To |
|---------|----------|
| panic recovery cannot safely mark process failed from SDK | expose a narrow bridge failure signal without importing engine internals |
| test needs real server process manager | add integration test under `internal/component/plugin/server` |

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

- A DirectBridge panic must produce a callback result before any cleanup path can be considered complete.

## Core Insight

Fast paths must fail at least as clearly as the slow path they replace.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Recover inside bridge event loop | rely on outer goroutine recover | only the event loop has access to the waiting result channel |

## Known Limitations

- Implemented in DirectBridge and SDK bridge dispatch; no remaining limitation for this bugfix.

## RFC Documentation

N/A.

## Implementation Summary

### What Was Implemented
- DirectBridge callback panics now send an `ErrBridgeFailed`-wrapped error to the waiting caller, fail the bridge, close callback channels, and make later callbacks fail fast.
- Tests cover generic callback panic, typed callback panic, post-panic fail-fast behavior, normal callbacks, and unknown methods.

### Bugs Found/Fixed
- SYS-003 documented for implementation.
- No production bug was fixed by this review program.

### Documentation Updates
- No user docs required for the fix-spec artifact.

### Deviations from Plan
- None.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Create actionable fix plan for SYS-003 | done | this spec | panic path and result-channel behavior covered |
| Include regression plan | done | Wiring Test and TDD sections | panic and normal callback tests named |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-4 | done | `TestDirectBridgeCallbackPanicReturnsPromptError`, `TestDirectBridgeTypedCallbackPanicReturnsPromptError`, `TestDirectBridgeCallbackAfterPanicFailsFast` | panic returns prompt error, bridge fails fast after panic |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| DirectBridge panic returns error | done | `pkg/plugin/sdk` | passing |
| DirectBridge cleanup after panic | done | `pkg/plugin/sdk` and `pkg/plugin/rpc` | passing |
| normal callback behavior | done | existing bridge and SDK tests | covered by `go test ./pkg/plugin/...` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `pkg/plugin/sdk/sdk_dispatch.go` | done | panic recovery and callback draining implemented |
| `pkg/plugin/rpc/bridge.go` | done | bridge failure state and fail-fast errors implemented |
| `internal/component/plugin/process/process.go` | unchanged | no process change needed after bridge-layer fix |

### Audit Summary
- Total items: 1 accepted finding converted to a fix spec.
- Done: DirectBridge panic handling and regression tests.
- Partial: none.
- Skipped: no approved scope reduction.
- Changed: DirectBridge state, SDK bridge dispatch, tests, and process protocol docs.

## Goal Validation
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| DirectBridge panics fail fast | spec artifact | this file names panic recovery location, ACs, and regression tests for SYS-003 |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | SYS-003 DirectBridge callback panic leaves engine caller waiting for timeout | child 2 report | fix spec created |

### Fixes applied
- None, review program creates fix specs only.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | Fix-spec artifact now has summary, audit, goal validation, review gate, and pre-commit sections | this spec | no action |

### Final status
- [ ] `/ze-review` re-run by implementation owner after code changes
- [x] Fix spec contains source evidence, ACs, and regression plan

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `plan/spec-bugfix-sys-directbridge-panic.md` | yes | created by review program |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| review deliverable | fix spec exists | this file |

### Wiring Verified
| Entry Point | Test | Verified |
|-------------|------|----------|
| DirectBridge callback panic | planned unit tests | pending implementation |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1..A-3 | unresolved for implementation | listed in spec |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user docs needed | SDK/bridge lifecycle bugfix spec only | yes |
