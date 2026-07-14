# Spec: rib-arch-6 -- Route-Server Fastpath: First Production locrib.OnChange Consumer

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-rib-arch-0-umbrella.md` - set context
4. `internal/component/bgp/plugins/rib/forward_observer.go` - the `OnChange` subscriber
5. `plan/learned/784-rib-rs-fastpath.md` - the producer wiring this consumes

## Task

The zero-copy forward-handle producer wiring exists (`plan/learned/784-rib-rs-fastpath.md`).
Its only consumer today is `observeForwardHandles`
(`internal/component/bgp/plugins/rib/forward_observer.go:37`), which registers a
`loc.OnChange(func(c locrib.Change) {...})` subscriber (:38) that merely **nil-checks**
`c.Forward` and emits a debug log line; per its own comment it "does NOT AddRef or read
Bytes" (`forward_observer.go:17`).

GAP: no production consumer reads the `Change.Forward` bytes. Build the **route-server
(RS/RR) fastpath state-tracker** as the first real subscriber: a consumer that AddRefs and
reads the zero-copy `Change.Forward` UPDATE bytes to maintain forwarding state, proving the
producer wiring end-to-end. The `forward_observer.go` comment names exactly this consumer
("RS/RR fast-path, sysrib mirroring, etc.", :23).

### Re-verification (2026-07-14)

- Gap real: no production consumer reads `Change.Forward` bytes. A tree-wide search finds
  only the debug observer referencing the field; every AddRef/Bytes read is test-only.
  Wording note: there are additional production `OnChange` subscribers on locrib (ospf
  `default.go:166`, sysrib `sysrib.go:852`), but only the observer touches `Change.Forward`,
  so "the only subscriber" here means "the only Forward-consuming subscriber".
- Assumption A-1 VALIDATED: `ribForwardHandle` (`forward_handle.go:31`) already exposes a
  sufficient reader API -- `AddRef` (`:54`, lazy-copy on first ref via `sync.Once`),
  `Release` (`:72`), and `Bytes` (`:88`). The missing piece is a consumer that calls them,
  not a handle-API change.

## Required Reading

### Architecture Docs
- [ ] `internal/component/bgp/plugins/rib/forward_observer.go` - the existing nil-check subscriber
  → Constraint: a real consumer MUST AddRef before reading `Change.Forward` bytes and release correctly, or it corrupts the zero-copy buffer pool.
- [ ] `plan/learned/784-rib-rs-fastpath.md` - the producer/handle design
  → Constraint: honour the AddRef/release contract and the unsubscribe-before-drop rule (`SetLocRIB` manages the subscription).
- [ ] `internal/component/bgp/plugins/rib/forward_handle.go` - `ribForwardHandle` (producer side)
  → Constraint: verify current behaviour against this source before designing.

**Key insights:**
- The producer + a debug observer already exist; this is the first consumer that actually reads bytes, so the AddRef/release lifecycle is the load-bearing design concern.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; anchors verified 2026-07-08)
- [ ] `internal/component/bgp/plugins/rib/forward_observer.go` - `observeForwardHandles` (:37): `loc.OnChange` subscriber (:38) that nil-checks `c.Forward` (:39) and debug-logs family/prefix/kind; explicitly does not AddRef or read Bytes (:17-18)
- [ ] `plan/learned/784-rib-rs-fastpath.md` - producer wiring for `Change.Forward`

**Behavior to preserve:**
- The existing debug observer (or its role) and the `OnChange`/unsubscribe contract; `SetLocRIB` manages subscription lifecycle.

**Behavior to change:**
- Add a production consumer that AddRefs and reads `Change.Forward` bytes to maintain RS/RR fastpath forwarding state.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Loc-RIB best-change delivers a `locrib.Change` with a non-nil `Forward` handle

### Transformation Path
1. Loc-RIB pipeline produces a `Change` carrying a zero-copy `Forward` handle (producer per learned 784)
2. The fastpath consumer's `OnChange` callback AddRefs the handle and reads the UPDATE bytes
3. It updates RS/RR forwarding state from those bytes
4. It releases the handle; `SetLocRIB` unsubscribes on rewire/teardown

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Loc-RIB → consumer | `loc.OnChange(func(c locrib.Change))` callback | [ ] |
| zero-copy buffer ↔ consumer | AddRef before read, release after (pool discipline) | [ ] |

### Integration Points
- `locrib.RIB.OnChange` (`forward_observer.go:38`) - the subscription API
- `Change.Forward` - the zero-copy UPDATE handle
- `forward_handle.go` `ribForwardHandle` - producer side

### Architectural Verification
- [ ] No bypassed layers (consumer subscribes via `OnChange`, not a RIB internal hook)
- [ ] No unintended coupling (state-tracker reads the public `Change`, not RIB internals)
- [ ] No duplicated functionality (extends the existing subscriber pattern)
- [ ] Registration over hardcoding - the consumer registers as an `OnChange` subscriber (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `Change.Forward` exposes an AddRef/release API sufficient for a real reader | learned 784 producer wiring | Need to extend the handle API | read `forward_handle.go` at design | **VALIDATED (2026-07-14)**: `ribForwardHandle` (`forward_handle.go:31`) exposes AddRef (:54) / Release (:72) / Bytes (:88); consumer just needs to call them |
| A-2 | Reading bytes under the RIB write lock is acceptable, or the handle can be read after unlock | observer runs the callback under the write lock (`forward_observer.go:28`) | Fastpath work must move off the lock | benchmark/read the lock scope at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Missing release leaks or corrupts the zero-copy buffer pool | pool counters imbalance; races under `-race` | strict AddRef/release pairing; race tests on the new consumer |
| R-2 | Heavy work under the RIB write lock stalls best-path processing | best-change latency rises | copy out under lock, process off-lock |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Loc-RIB change with a non-nil Forward handle | → | `forwardStateTracker` AddRefs, reads bytes, updates state | `TestForwardStateTracker_ReadsAndReleases` + `test/plugin/rs-fastpath.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Best-change delivers a Forward handle | consumer reads the UPDATE bytes and updates RS/RR state |
| AC-2 | Sustained churn | buffer pool counters stay balanced (no leak); `-race` clean |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForwardStateTracker_ReadsAndReleases` | `internal/component/bgp/plugins/rib/forward_tracker_test.go` | AddRef → read Bytes → Release lifecycle; state records prefix + byte length | PASS (RED→GREEN) |
| `TestForwardStateTracker_DisabledIsInert` | `forward_tracker_test.go` | disabled tracker never AddRefs (no copy-out cost), records no state | PASS |
| `TestForwardStateTracker_RemovePrunesState` | `forward_tracker_test.go` | ChangeRemove prunes the per-prefix state | PASS |
| `TestForwardStateTracker_NoLeakUnderChurn` | `forward_tracker_test.go` | AC-2: AddRef/Release balance under concurrent churn; `-race` clean | PASS (`-race` clean) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rs-fastpath` | `test/plugin/rs-fastpath.ci` | operator enables the fast path (`request bgp rib fastpath enable`), a peer announces a route, and `request bgp rib fastpath status` reports `forwarded > 0` | PASS (`ze-test bgp plugin rs-fastpath`) |

## Files to Modify

- `internal/component/bgp/plugins/rib/forward_tracker.go` - new `forwardStateTracker` (production consumer) + `fastpathCommand`
- `internal/component/bgp/plugins/rib/rib.go` - `SetLocRIB` creates/stops the tracker; `request bgp rib fastpath` CommandDecl
- `internal/component/bgp/plugins/rib/rib_commands.go` - `request bgp rib fastpath` builtin registration
- `internal/component/bgp/plugins/rib/protocol_test.go` - command-count guard 18 → 19

## Implementation Steps

1. **Phase: design** - confirm the handle AddRef/release API (A-1) and lock scope (A-2).
2. **Phase: wiring** - failing test asserting the consumer reads bytes from a Forward handle.
3. **Phase: implement (TDD)** - AddRef/read/release; update RS/RR state; race tests.
4. **Functional test** - `.ci` proving the fastpath state update.
5. **Full verification** - `make ze-verify` including `-race` on the new tests.
6. **Complete spec** - audit, learned summary, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Production consumer reads `Change.Forward` bytes and updates state
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Race detector clean on the new consumer tests

## Design Verification (2026-07-14)

- **A-1 confirmed:** `ForwardHandle` (`internal/core/rib/locrib/forward_handle.go:33`) exposes
  `AddRef`/`Release`, and `ForwardBytes.Bytes()` (`:59`) returns the retained copy. First
  AddRef materialises an owned copy; the contract is "AddRef before reading Bytes, never
  read after the matching Release." The producer side is `ribForwardHandle`
  (`forward_handle.go`, rib plugin) landed by learned 784.
- **A-2 addressed:** the `OnChange` handler runs under the RIB write lock
  (`change.go:89-98`), so the consumer does the bounded copy-out (AddRef) under the lock and
  processes off-lock in a worker — no heavy work under the lock.

## Implementation Summary

- Added `forwardStateTracker` (`forward_tracker.go`): the first production consumer of
  `Change.Forward`. Its `onChange` AddRefs under the RIB write lock and enqueues; a worker
  reads `Bytes()` off-lock, records per-prefix forwarding state (last UPDATE byte length),
  and Releases. A full (backpressure) queue releases the handle and counts the drop, so the
  buffer pool never leaks.
- Wired into `SetLocRIB` alongside the existing debug observer; stopped and re-created on
  Loc-RIB rewire. Inert until `request bgp rib fastpath enable`, so a binary that never
  enables it pays only a single atomic load per change.
- Observability: `request bgp rib fastpath <enable|disable|status>` (rib_commands.go builtin
  + CommandDecl in rib.go). The `show bgp rib` verb greedily parses trailing tokens as
  filters, so a `show bgp rib fastpath` subcommand would need a YANG grammar container; the
  `request` verb has no such shadow, so the diagnostic lives there.
- **AC-1 met:** the tracker reads `Change.Forward` bytes and updates state
  (`TestForwardStateTracker_ReadsAndReleases`; the `.ci` proves it end-to-end from a real
  received route). **AC-2 met:** `TestForwardStateTracker_NoLeakUnderChurn` runs `-race`
  clean with a balanced AddRef/Release count under sustained concurrent churn.

## Review Gate

Self-review of the diff (forward_tracker.go + rib.go SetLocRIB/CommandDecl + rib_commands.go + tests + .ci):
- Lifecycle: AddRef strictly inside the handler (source valid); Release strictly after the
  off-lock Bytes read; `Stop()` drains the queue and does a final non-blocking drain to
  release a handle enqueued by an onChange that raced the stop.
- No lock inversion: `onChange` never takes the tracker mutex (only AddRef + enqueue); all
  `t.mu` use is in the worker/snapshot, off the RIB write lock.
- Inert by default: a single `enabled.Load()` gates all work.
Findings: 0 BLOCKER, 0 ISSUE.

## Pre-Commit Verification

Re-verified 2026-07-14:

| Item | Evidence |
|------|----------|
| AC-1 verified | `TestForwardStateTracker_ReadsAndReleases` PASS; `.ci` PASS (`ze-test bgp plugin rs-fastpath`) |
| AC-2 verified | `TestForwardStateTracker_NoLeakUnderChurn` PASS under `CGO_ENABLED=1 go test -race` (balance == 0) |
| RED captured | disabling the accumulator/tracker read makes the byte-reading tests fail; the byte-read tests assert `forwarded`/`bytes` from the handle |
| No regression | full `./internal/component/bgp/plugins/rib/` suite PASS, normal and `-race` |
| Structural gates | `ze-lint-changed` 0 issues; `ze-cli-grammar-check` OK (260 commands) |
| A-1/A-2 resolved | confirmed — `forward_handle.go:33/59`, `change.go:89-98` read this session |

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `spec-rib-arch-0-umbrella.md`.
