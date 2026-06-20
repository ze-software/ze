# Spec: bugfix-bgp-reactor-startup-cleanup

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-06-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/review-bug-review-bgp-engine.md` finding BENG-004
3. `internal/component/bgp/reactor/reactor.go`
4. `internal/component/plugin/server/server.go`
5. `ai/rules/testing.md`
6. `ai/rules/no-workarounds-for-missing-behavior.md`

## Task

Fix BENG-004. If BGP reactor startup fails after listeners, subscriptions, cache scanners, or API resources have started, startup must abort through one cleanup path. No bound socket, event subscription, cache goroutine, plugin server resource, or canceled context ambiguity may remain after `StartWithContext` returns an error.

## Required Reading

### Source Finding
- [ ] `plan/review-bug-review-bgp-engine.md` - BENG-004 evidence and regression plan
  -> Decision: startup cleanup is a lifecycle correctness bug, not a protocol RFC issue.
  -> Constraint: the fix must prove cleanup on failures after partial startup.

### Architecture and Rules
- [ ] `docs/architecture/core-design.md:496-600` - BGP reactor lifecycle and forwarding architecture context
  -> Constraint: the reactor owns its resources and must release them when startup fails.
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md`
  -> Constraint: do not mask plugin server creation errors or skip API startup to make tests pass.
- [ ] `ai/rules/testing.md`
  -> Constraint: add a real failing path test, not a mock that only asserts a helper was called.

## Current Behavior

**Source files to read:**
- [ ] `internal/component/bgp/reactor/reactor.go:923-969` - `StartWithContext` starts cache scanner, subscriptions, listeners, and then calls `startAPIServer`.
- [ ] `internal/component/bgp/reactor/reactor.go:1111-1162` - `startAPIServer` returns plugin server creation errors directly.
- [ ] `internal/component/bgp/reactor/reactor.go:1200-1208` - `abortStartup` stops listeners and cancels context, but is not used for every late failure.
- [ ] `internal/component/plugin/server/server.go:173-177` - plugin server constructor failure can happen before a server is returned.

**Behavior to preserve:**
- Successful startup still starts listeners, cache scanner, API server, and peer sessions once.
- Early configuration validation errors still return without starting runtime resources.
- Existing stop semantics remain idempotent.

**Behavior to change:**
- Any error after runtime resource startup must call the same abort path.
- The abort path must stop recent update cache scanning or cancel the context that drives it.
- Listener factories and API server factories used in tests must observe cleanup.

## Data Flow

### Entry Point
- A caller invokes `StartWithContext` on the BGP reactor.

### Transformation Path
1. Config validation completes.
2. Reactor context and recent update cache scanner start.
3. Interface subscriptions and listeners start.
4. API/plugin server startup fails.
5. Fixed code invokes abort cleanup before returning the error.
6. Caller receives the original startup error, and all partially started resources are stopped.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| startup orchestration -> listener resources | `startMultiListeners`, listener handles | [ ] failure test observes `Stop` on started listener |
| startup orchestration -> cache scanner | `recentUpdates.Start` or equivalent loop | [ ] failure test observes scanner exits or `Stop` called |
| startup orchestration -> plugin server factory | `startAPIServer` | [ ] injected failure preserves wrapped error |

### Architectural Verification
- [ ] No bypassed layer: startup failures return through reactor lifecycle code, not package-level globals.
- [ ] No fake success path: plugin server failures remain errors.
- [ ] No duplicated cleanup logic: late startup failures share one abort helper.
- [ ] Stop remains safe after a failed startup.

## Risks and Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | Cache scanner shutdown can be observed through existing Stop or context cancellation | BENG-004 source read | add a test seam for cache scanner lifecycle | startup failure test | unvalidated |
| A-2 | Plugin server creation can be injected without global mutation leaking across tests | constructor currently called directly | add narrow factory field reset by tests | unit test cleanup | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Cleanup runs twice and panics | Stop-after-failed-start test panics | make abort path idempotent |
| R-2 | Test seam becomes production abstraction bloat | exported factory or broad interface added | keep seam unexported and scoped to reactor tests |
| R-3 | Error wrapping changes user-visible diagnostics | tests comparing exact string fail | assert `errors.Is` or substring with original cause |

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `StartWithContext` with plugin server creation failure after listeners start | -> | abort cleanup path | `TestStartWithContextCleansUpAfterAPIServerFailure` |
| `Stop` after failed startup | -> | idempotent cleanup | `TestStopAfterFailedStartupIsSafe` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | API server creation fails after at least one listener started | `StartWithContext` returns the creation error and the listener is stopped |
| AC-2 | cache scanner or reactor context started before the failure | scanner exits or explicit stop is called before test completes |
| AC-3 | caller invokes `Stop` after failed startup | no panic, no double close, no stale listener |
| AC-4 | successful startup path | unchanged resource startup order and running state |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStartWithContextCleansUpAfterAPIServerFailure` | `internal/component/bgp/reactor/reactor_startup_test.go` | AC-1, AC-2 | |
| `TestStopAfterFailedStartupIsSafe` | same | AC-3 | |
| existing successful startup test | existing reactor tests | AC-4 | |

### Boundary Tests
| Field | Last Valid | Invalid / Failure | Expected |
|-------|------------|-------------------|----------|
| listener count | 1 started listener | plugin server creation failure after listener | listener stopped |
| startup state | `running=false` before success | failure after context creation | context canceled, no running state |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| targeted reactor startup unit is sufficient | Go unit | daemon startup fails cleanly before serving BGP | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Not required | - | - | lifecycle cleanup, not peer protocol | |

## Files to Modify

- `internal/component/bgp/reactor/reactor.go` - route late startup errors through abort cleanup.
- `internal/component/bgp/reactor/reactor_startup_test.go` or existing startup test file - regression tests.
- Possibly a narrow unexported test seam in `reactor.go` if plugin server construction cannot be injected otherwise.

## Files to Create

- `internal/component/bgp/reactor/reactor_startup_test.go` if no suitable test file exists.

## Implementation Steps

1. Add failing startup cleanup tests using fake listeners and plugin server creation failure.
2. Introduce the smallest internal seam needed to force `startAPIServer` failure.
3. Make all errors after resource startup call one abort helper before returning.
4. Ensure abort cleanup is idempotent and safe if `Stop` is called afterward.
5. Run targeted reactor tests and `make ze-lint-changed` after code changes.

## Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Correctness | every late `return err` path calls cleanup |
| Resource lifecycle | listeners, context, cache scanner, API resources stopped |
| Maintainability | no broad factory interfaces or exported test-only hooks |
| Tests | failure injection proves real cleanup, not only branch coverage |

## Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Late startup failure cleanup | `go test -run TestStartWithContextCleansUpAfterAPIServerFailure ./internal/component/bgp/reactor` |
| Stop-after-failure safety | `go test -run TestStopAfterFailedStartupIsSafe ./internal/component/bgp/reactor` |
| Lint | `make ze-lint-changed` |

## Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Port/resource leak | failed startup cannot leave listening sockets |
| Duplicate event handling | failed startup cannot leave subscriptions active |
| Error visibility | operator still sees the true startup failure |

## Failure Routing

| Failure | Route To |
|---------|----------|
| cache scanner lacks observable stop | add narrow lifecycle hook or context assertion in reactor package |
| plugin server constructor cannot be injected safely | add unexported factory field with test reset |

## Design Insights

- Startup success is a cutover point. Before success, every allocated runtime resource is provisional and must be released on any later error.

## Core Insight

Every post-resource startup error must behave like a failed transaction: preserve the original error and roll back provisional runtime state.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One abort path for late failures | cleanup at each failure site | one path prevents future leaks when startup grows |
| Keep constructor seam unexported | package-level global factory | avoids API surface and cross-test contamination |

## Known Limitations

- This spec does not implement the fix.

## Implementation Summary

### What Was Implemented
- Fix spec only. Production code is unchanged.

### Bugs Found/Fixed
- BENG-004 documented for implementation.

### Documentation Updates
- No user docs required by this fix spec.

### Deviations from Plan
- None.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Create actionable fix plan for BENG-004 | Done | this spec | Generated from accepted review finding |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-4 | Planned | tests listed above | To be satisfied by implementation spec owner |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Startup cleanup tests | Planned | `internal/component/bgp/reactor` | Not run by review program |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/reactor.go` | Planned | implementation target |

### Audit Summary
- Total items: 1 accepted finding converted to a fix spec.
- Done: fix spec created.
- Partial: implementation pending by design.
- Skipped: no production code changes in review program.
- Changed: new spec file.

## Goal Validation

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| BENG-004 has implementable remediation path | spec artifact | this file names source, tests, ACs, and cleanup invariants |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | generated from BENG-004 | `plan/review-bug-review-bgp-engine.md` | fix spec created |

### Fixes applied
- None, review program creates fix specs only.

### Final status
- [ ] `/ze-review` re-run by implementation owner after code changes
- [x] Fix spec contains source evidence, tests, ACs, and rollback invariant

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `plan/spec-bugfix-bgp-reactor-startup-cleanup.md` | yes | created by review program |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| review deliverable | fix spec exists | this file |

### Wiring Verified
| Entry Point | Test | Verified |
|-------------|------|----------|
| `StartWithContext` failure | planned unit tests | pending implementation |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1, A-2 | unresolved for implementation | listed in spec |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user docs needed | lifecycle cleanup only | yes |
