# Spec: ze-api-callback-pump

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Reconcile the callback-pump improvement against the native fixture framework
before proposing any implementation or closure. The original Python helper was
retired on 2026-08-28 in `eae282592`. Its thirteen open-coded loops were the
2026-08-16 migration population, not a current list of files to change. Restoring
that helper or adding a second pump beside the SDK is outside this task.

The requirement remains that a fixture waiting for a route or an RPC result
continues answering filter callbacks, so the daemon observes the fixture's
verdict rather than its `on-error` fallback. Existing fixture behaviour and
mutation discrimination must survive any change. The original scope required
one proven helper user; it did not authorise a wholesale fixture rewrite.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - multiplexed RPC and callback delivery
- [ ] `docs/plugin-development/README.md` - the supported SDK surface
- [ ] `docs/functional-tests.md` - compiled fixture registration and execution
- [ ] `ai/patterns/functional-test.md` - fixture conventions

## Current Behavior (MANDATORY)

Source read on 2026-09-19:

- `internal/test/fixture/fixture.go`, `observeConfigured`: callbacks are installed
  before startup; `OnAllPluginsReady` starts the scenario in a goroutine and
  returns, while `plugin.Run` continues serving callbacks.
- `internal/test/fixture/plugin_fixture_12_filters.go`, `p12FilterDriver`:
  the redistribution filter fixtures share one driver. It installs
  `OnFilterUpdate`, runs the scenario in a goroutine after readiness, and calls
  `plugin.Run` on the serving path.
- `pkg/plugin/sdk/sdk_dispatch.go`, `eventLoop`: reads inbound callbacks,
  selects their registered handler and writes its result or error.
- `pkg/plugin/rpc/mux.go`, `NewMuxConn`: starts the background RPC reader;
  responses and inbound requests have separate destinations.

This source reading establishes that the old single-threaded Python polling
recipe is no longer the implementation target. It does not establish a current
functional pass or prove that breaking callback service makes the chosen fixture
fail. Those are the remaining evidence obligations.

**Behaviour to preserve:** all existing native fixture outcomes, callback error
handling and the option for a deliberately negative fixture to exercise an
unanswered callback. No daemon or wire change is planned.

**Behaviour to change:** none identified from the retired-helper finding. If the
native evidence exposes a surviving callback starvation defect, identify its
producing function before designing a repair within this goal.

## Data Flow (MANDATORY)

### Entry Point
A `.ci` invokes a registered `le-test fixture` driver.

### Transformation Path
1. The driver creates an SDK plugin and registers its filter callback.
2. `Plugin.Run` completes startup and serves incoming callbacks.
3. The readiness callback launches the scenario separately, so condition polling
   and dispatch RPCs do not occupy the callback-serving loop.
4. The registered filter handler returns the route verdict while the scenario
   observes the result.

### Boundaries Crossed
| Boundary | Current producer | Proof still owed |
|----------|------------------|------------------|
| Fixture wait and callback service | `observeConfigured`, `p12FilterDriver` | Functional verdict during an overlapping wait |
| SDK callback and daemon filter verdict | `eventLoop`, `OnFilterUpdate` | Mechanism-broken run fails rather than accepting fallback |

### Integration Points
`internal/test/fixture`, `pkg/plugin/sdk`, and the native `test/plugin` fixtures.
The retired Python helper and its unit-test file are historical provenance only.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | The native drivers satisfy the original callback-during-wait requirement | Scenario goroutines are separate from `Plugin.Run` | A current defect remains despite the Python retirement | Exercise a filter fixture through a wait and break callback service | source-supported; runtime unvalidated |
| A-2 | The original thirteen-copy migration is obsolete | Python retirement and the shared native redistribution driver | A surviving equivalent needs a named owner | Trace the original fixture obligations to current native users before closure | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Helper retirement is mistaken for proof that callbacks are serviced correctly | Closure cites source shape alone | Require the positive and mechanism-broken functional outcomes |
| R-2 | A replacement pump duplicates SDK service | Proposed helper reads the same callback stream as `Plugin.Run` | Repair the existing serving path if a defect is demonstrated |

## Blast Radius

Planning and closure evidence for native plugin fixtures. No product change is
authorised by this reconciliation.

## Wiring Test

| Entry Point | Feature path | Evidence |
|-------------|--------------|----------|
| A native filter fixture waits while Ze requests a verdict | Scenario goroutine plus SDK callback loop | Select a current `test/plugin/redistribute-*.ci` and record both normal and mechanism-broken results before closure |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behaviour |
|-------|-------------------|--------------------|
| AC-1 | A native fixture waits for its condition | Filter callbacks are answered throughout the wait |
| AC-2 | Callback service or the filter mechanism is broken | The selected fixture fails; its pass cannot come from `on-error` fallback |
| AC-3 | Existing fixture population corresponding to the historical thirteen users | Behaviour is preserved; no compulsory corpus migration is introduced |
| AC-4 | A fixture intentionally exercises an unanswered callback | The negative scenario remains possible; no helper hides or forbids it |
| AC-5 | Current fixture/SDK documentation | Explains which path serves callbacks and what an unanswered callback means |

## End-to-End User Stories

A fixture author can wait for a route while the registered filter determines its
verdict, without copying a manual callback loop.

## Test Plan

The original `ze_api_test.py` pump and completion tests are superseded by the
native path. At pickup, inspect existing SDK and fixture tests for the same
observable obligations before adding any test. The functional proof must use an
existing native filter fixture and show that its assertion fails when the
relevant callback mechanism is broken. Run the affected plugin population after
any repair; source inspection alone cannot close AC-1 through AC-4.

## Files to Modify

- This spec: record the native evidence and the disposition of each original AC.
- `internal/test/fixture/fixture.go`, `plugin_fixture_12_filters.go` or the SDK
  callback producer only if evidence finds a surviving defect in this scope.
- Existing native fixture documentation if AC-5 has no current explanation.

## Files to Create

None currently required. No retired Python helper is to be recreated.

## Implementation Steps

1. Map the historical fixture obligations to their current native drivers.
2. Prove callback service during a wait and the selected fixture's discrimination.
3. If a defect survives, repair its producer without changing filter semantics;
   otherwise record that the native framework satisfies the improvement.
4. Complete normal review and closure only after every original AC is accounted for.

## Checklist

- [ ] AC-1 through AC-5 have current evidence or an explicit owner-approved disposition
- [ ] No retired helper is restored and no duplicate callback consumer is added
- [ ] `./le verify worktree` passes after any implementation change
- [ ] Independent review and normal closure requirements are met

## Known Limitations

This reconciliation ran no tests and does not close the spec. The original
thirteen-copy count is dated evidence, and the current source reading does not
assert that every historical user has been exercised.
