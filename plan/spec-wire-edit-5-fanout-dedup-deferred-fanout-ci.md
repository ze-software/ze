# Spec: wire-edit-5-fanout-dedup-deferred-fanout-ci -- finish the socket-level fan-out dedup functional test

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Updated | 2026-08-02 |

Deferral holder created at the closure of `plan/spec-wire-edit-5-fanout-dedup.md` on 2026-08-02
(`ai/rules/deferral-tracking.md`, "Creating the Deferral Spec"). The source spec
was removed by its closure commit, so the work below lives here.

## Task

`plan/spec-wire-edit-5-fanout-dedup.md` shipped fan-out dedup with
mutation-verified Go coverage, including the cross-peer leak the design exists to
prevent. Its socket-level proof did not ship: `wire-edit-fanout-dedup.ci` is
unfinished and sits in `test/draft/plugin/`, which is gitignored, so it is in no
commit and no suite.

Two blockers are written into the draft's own header and must be resolved before
it can be promoted (`ai/rules/testing.md`, "Draft a Functional Test Before It Is
Live"):

| Blocker |
|---------|
| `community { send none }` did not suppress in the fixture |
| the `contains=` value came back as a hex-decode error |

The missing piece is end-to-end framing, not design. The behavior itself is
already proven at the Go level.

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
| A-1 | `community { send none }` suppresses on the forward rail and the fixture was simply wrong. | The draft header records the observation but not a root cause. | A live suppression defect on the forward rail, which becomes the real deliverable. | (fill during design) | unvalidated |

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

1. (fill during design)

## Checklist

### Goal Gates (MUST pass)
- [ ] Every AC demonstrated
- [ ] `make ze-verify` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
