# Spec: interop-wire-capture -- observe the bytes on the wire in an interop scenario

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`test/interop/` can assert what a peer daemon PARSED. It cannot assert what ze
put on the wire. Nothing under `test/interop/` captures packets: no `tcpdump`,
no pcap, no BGP message log (measured 2026-08-05).

That gap makes one class of interop assertion unreachable. When a property is
visible only in the byte stream and not in the peer's parsed state, a scenario
over the peer's route dump passes whether or not the property holds. This is the
vacuity trap `ai/rules/interop-and-goal-validation.md` names: "a conforming peer
accepts the old and new wire equally".

**The case that created this spec.** RFC 4271 Section 5 asks a sender to order
path attributes by ascending type code. `(*announceAttrs).emit`
(`internal/component/bgp/reactor/announce_build.go`) produces that order, by
sorting the plan and merge-inserting each contribution into the base at the
first position that sorts after it. BIRD 2.15.1 accepts any order and
`birdc show route ... all` prints attributes in BIRD's own canonical order, so
reverting `emit` to the pre-convergence encoder leaves BIRD's route dump
byte-identical. The order is proven today only by
`test/plugin/wire-edit-api-origin-order.ci`, which pins the hex through the
daemon and never involves a second implementation.

Thomas ruled on 2026-08-05 to land the acceptance scenario first and home the
capture capability here, rather than grow the harness inside a scenario spec.

## What this spec owes

| Piece | Note |
|-------|------|
| A capture mechanism in the interop harness | Where it runs (peer container, ze container, or the bridge) is the design question. `test/interop/interop.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->, `Scenario.setup` starts every container and is the integration point |
| A helper a `check.py` can call | The check contract is `mod.check()` with no arguments (`Scenario.run_check`), so the helper is imported from `interop`, like `docker_exec` and the daemon classes |
| An assertion over a decoded UPDATE | Attribute type codes in the order they appear. Decoding belongs to the helper, never to each scenario |
| Proof it discriminates | Revert `emit`'s `sortByCode()` and the capture assertion MUST fail (`ai/rules/interop-and-goal-validation.md`). Without this the capability is unproven |

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity traps and the discrimination requirement
- [ ] `test/interop/interop.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - `Scenario.setup`, `Scenario.run_check`, `docker_exec`
- [ ] `docs/architecture/testing/interop.md` - the harness contract as documented

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; verify before trusting)

- [ ] `test/interop/interop.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> (the harness: container lifecycle and the check contract)
- [ ] `test/interop/run.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> (scenario discovery)
- [ ] `internal/component/bgp/reactor/announce_build.go` (`(*announceAttrs).emit`, the producer whose output a capture would observe)

**Behavior to preserve:** every existing scenario keeps passing unchanged. Capture
is opt-in per scenario, never a cost every scenario pays.

## Data Flow (MANDATORY)

### Entry Point
(fill during design)

### Transformation Path
(fill during design)

### Boundaries Crossed
| From | To | Format |
|------|----|--------|
| (fill during design) | (fill during design) | (fill during design) |

### Integration Points
| Point | Component |
|-------|-----------|
| (fill during design) | (fill during design) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A capture can run in the scenario containers without a privileged flag the harness does not already grant. | `test/interop/interop.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> starts containers with the options it sets today. | Capture needs a capability change on every scenario container, widening blast radius. | Read the container run options, then run one capture. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A capture that races the session start misses the UPDATE entirely and the assertion reads an empty file as a pass. | The helper returns no messages on a scenario known to send one. | The helper fails closed on an empty capture; never treat "no messages" as "nothing wrong" (`ai/rules/evidence.md`). |
| R-2 | Capture makes every scenario slower or flakier. | Interop suite runtime rises, or intermittent reds appear in unrelated scenarios. | Opt-in per scenario, off by default. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| A scenario asks the harness to capture, then asserts attribute order | -> | the capture helper in `test/interop/interop.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> | the first scenario to adopt it, named at design time |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A scenario enables capture and ze announces one route | The helper returns the UPDATE's path attribute type codes in wire order |
| AC-2 | `sortByCode()` is removed from `(*announceAttrs).emit`, on a route shape where a later contribution sorts EARLIER than an earlier one | The assertion FAILS, proving the capture discriminates |

**AC-2's route shape is load-bearing, found at the round 1 review of
`spec-wire-edit-4-api-origin-deferred-bird-interop` (closed 2026-08-07 in `2cc75ab5f`), 2026-08-05.** On a
plain IPv4-unicast announce the contributions are ORIGIN, AS_PATH and NEXT_HOP,
which arrive in naturally ascending order, so removing `sortByCode()` is a NO-OP
and the output is byte-identical. A capture scenario built on that shape would
inherit exactly the vacuous-evidence defect the sibling spec hit. Pick a shape
where the sort has work to do: MP_REACH (14) contributed before LOCAL_PREF (5),
or AS4_PATH (17) before NEXT_HOP (3).
| AC-3 | A scenario that does not enable capture | Runs unchanged, at unchanged cost |
| AC-4 | Capture is enabled but no BGP message arrives | The helper raises, never returns an empty result that reads as a pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | | | |

### Functional Tests
<!-- Tooling scope: this spec changes no daemon Go, so its driving surface is the
     harness runner itself, not a `.ci`. The daemon-level byte proof already exists
     and stays where it is (`test/plugin/wire-edit-api-origin-order.ci`). -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/interop/run.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->, driving the first scenario to adopt capture | `test/interop/scenarios/` | a live peer receives attributes in ascending type-code order | |
| `test/interop/run_test.go` | `test/interop/` | the runner still fails closed when Docker is absent | |

## Files to Modify
- `test/interop/interop.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - the capture helper and the container lifecycle it hooks
- `docs/architecture/testing/interop.md` - the harness contract a scenario author reads; a capability nobody can discover is a capability nobody uses
- `docs/functional-tests.md` - test infrastructure changed (Documentation Update Checklist row 10)

## Files to Create
- (fill during design)

### Documentation Update Checklist (BLOCKING)

<!-- Answered at skeleton for the surfaces already known. Rows the design will
     settle say so, rather than being left blank: an unanswered row and a
     forgotten one look identical. -->

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | **Yes** | `docs/architecture/testing/interop.md` (the harness contract a scenario author reads) and `docs/functional-tests.md`. This spec adds a new test TOOL, which is exactly what row 10 covers |
| 12 | Internal architecture changed? | No | The harness is test infrastructure, not daemon architecture |
| 9 | RFC behavior newly proven? | Decided at design | If the first adopting scenario carries an `RFC requirement:` tag for RFC 4271 Section 5, then `rfc/short/rfc4271.md` and the `docs/features/rfc-status.md` row move with it. `ai/rules/rfc-compliance.md` governs; do not tag a suite nothing executes |
| 1-8, 11, 13-17 | - | N-A | No user feature, config, CLI, API, plugin, guide, wire format, SDK, comparison, metadata, metric, inventory or example surface is touched |

## Implementation Steps

1. (fill during design)

## Known Limitations

- Scope is the interop harness. The daemon-level byte proof stays with
  `test/plugin/wire-edit-api-origin-order.ci`; this spec adds a second,
  independent observation against a live peer, and does not replace the first.

## Checklist

### Goal Gates (MUST pass)
- [ ] Every AC demonstrated
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Feature code integrated, not library-only

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `wire-edit-4-api-origin-deferred-bird-interop.md`, 2026-08-05

Deferred by spec-wire-edit-4-api-origin-deferred-bird-interop.

Live-peer proof of ascending attribute type-code order (RFC 4271 Section 5) for an API-originated route. The property is produced by `(*announceAttrs).emit` (`internal/component/bgp/reactor/announce_build.go`) and pinned at the byte level by `test/plugin/wire-edit-api-origin-order.ci`, but no interop scenario can observe it
