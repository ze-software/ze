# Spec: wire-edit-5-fanout-dedup-deferred-fanout-ci -- finish the socket-level fan-out dedup functional test

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Updated | 2026-08-02 |

Deferral holder created at the closure of the wire-edit-5 fan-out dedup spec on 2026-08-02
(`ai/rules/planning.md`, "Creating the Deferral Spec"). The source spec
was removed by its closure commit, so the work below lives here.

## Task

The wire-edit-5 fan-out dedup spec (design record:
`docs/architecture/bgp/fanout-dedup.md`) shipped fan-out dedup with
mutation-verified Go coverage, including the cross-peer leak the design exists to
prevent. Its socket-level proof was recorded as unfinished at closure, in a
gitignored `test/draft/plugin/wire-edit-fanout-dedup.ci`. That file is absent
from the current checkout, and no live fixture of that name was found under
`test/`. Recover the draft if an existing copy is available; otherwise
reconstruct the fixture from the design and the acceptance criteria below.
Do not treat a missing local draft as implemented or executed coverage.

The closure record attributed two blockers to the draft's header. They remain
unresolved historical observations until the fixture and its stimulus are
available:

| Blocker |
|---------|
| `community { send none }` did not suppress in the fixture |
| the `contains=` value came back as a hex-decode error |

The Go evidence supports the deduplication implementation. It does not settle
whether the recorded community-suppression result came from the fixture or
the product. Establish A-1 at the forward producer before classifying the
remaining item as elective coverage, release evidence, or a suppression
defect. `reject=bgp` now expresses cross-peer wire negatives, as documented in
`docs/architecture/bgp/fanout-dedup.md`; retain the exact-byte and mutation
requirements when reconstructing the fixture.

Current source narrows A-1. `sendCommunitySuppression` maps `none` to all
community suppression bits, and the forward rail applies those operations
through `applyFactsSendCommunity`. `genericCommunityHandler`
(`internal/component/bgp/plugins/filter_community/handler.go`) now calls
`p.Drop()` when the last set-or-suppress operation is suppression. Its comment
records an earlier product defect that discarded those operations, and
`TestSendCommunitySuppressEmittedBytes` in
`internal/component/bgp/reactor/forward_send_community_test.go` asserts the
rebuilt bytes. That is reason to reject the assumption that the old fixture
was necessarily wrong. It does not identify the missing draft's input or
prove its socket result; A-1 remains unvalidated here.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

- [ ] `ai/rules/testing.md` - the draft-then-promote discipline
- [ ] `ai/patterns/functional-test.md` - `.ci` directive vocabulary

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; verify before trusting)

- [ ] `internal/component/bgp/reactor/forward_dedup.go` (the behavior under test)
- [ ] `internal/component/bgp/reactor/forward_dedup_test.go` (the Go coverage this must confirm end to end)

**Behavior to preserve:** the Go coverage stays; the `.ci` adds framing proof, it does not replace an assertion.

## Data Flow (MANDATORY)

### Entry Point
One route fanned out to peers in two policy groups over real sockets.

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
| A-1 | Current community suppression works for the reconstructed forward scenario, and any surviving failure is understood. | The current mask producer and community handler implement suppression; the missing draft recorded a failure without its root cause. | A reproduced suppression defect becomes the deliverable; coverage cannot hide it. | Recover or reconstruct the two-policy-group stimulus, trace its community policy through the forward producer, and compare the exact socket frames with suppression enabled and disabled. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Promoting a `.ci` whose assertion is a cumulative match would add a test that cannot fail. | (fill during design) | (fill during design) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| One route fanned out to peers in two policy groups | -> | one materialisation per group; each peer receives its own group's bytes | `test/plugin/wire-edit-fanout-dedup.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | One route to N peers in G groups over sockets | Each peer receives its group's exact bytes |
| AC-2 | The test with dedup disabled | It still passes, because dedup must be invisible on the wire |
| AC-3 | The test with the identity's base half removed | It fails, proving it discriminates |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (functional `.ci`, promoted from `test/draft/plugin/`) | `test/plugin/wire-edit-fanout-dedup.ci` | AC-1, AC-3 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `wire-edit-fanout-dedup` | `test/plugin/wire-edit-fanout-dedup.ci` | each peer receives its own policy group's bytes over a real socket | |

## Files to Modify
- `test/plugin/wire-edit-fanout-dedup.ci` (promoted from `test/draft/plugin/`)

- `internal/component/bgp/reactor/forward_dedup.go` - re-read only, unless the suppression blocker turns out to be a live defect

## Implementation Steps

1. Recover or reconstruct the draft and capture the exact community policy and `contains=` input behind the two historical blockers.
2. Resolve A-1 against the forward producer and classify any failure before changing the fixture or product.
3. Design the socket assertions against the current peer parser, including cross-peer rejection and the AC-3 base-identity mutation. Promote only after every AC is demonstrated.

## Checklist

### Goal Gates (MUST pass)
- [ ] Every AC demonstrated
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
