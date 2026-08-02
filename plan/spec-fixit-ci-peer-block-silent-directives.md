# Spec: fixit-ci-peer-block-silent-directives -- a reject= directive inside a stdin=peer block asserts nothing

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-02 |

Deferral holder created at the closure of `plan/spec-wire-edit-5-fanout-dedup.md` on 2026-08-02
(`ai/rules/deferral-tracking.md`, "Creating the Deferral Spec"). The source spec
was removed by its closure commit, so the work below lives here.

## Task

A `reject=bgp:` directive written inside a `stdin=peer` block of a `.ci` test is
a SILENT NO-OP. Neither the runner's peer-block parser nor the peer expectation
reader consumes it, so the line parses into nothing while reading as a guard: the
test appears to check a negative it never checks.

Three live sites carry one today:

| Site |
|------|
| `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci` |
| `test/plugin/filter-family-export-flowspec.ci` |
| `test/plugin/logging-level-filter.ci` |

The fix is a runner GUARD that hard-errors on `reject=` inside a peer block,
matching the precedent already in place for `option=env:`, plus an audit of the
three sites once the guard fires. Patching the three call sites alone would leave
the next one silent, which is the failure this spec exists to stop
(`ai/rules/fail-closed-guards.md`).

Found by an independent review of the wire-edit children on 2026-08-02. No RFC
claim rests on the dead lines: at each site the surrounding `expect=bgp:` framing
assertion still proves the behavior in the observed framing.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

- [ ] `ai/rules/fail-closed-guards.md` - a directive that neither denies nor speaks does not exist
- [ ] `ai/patterns/functional-test.md` - `.ci` directive vocabulary

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; verify before trusting)

- [ ] `internal/test/runner/record_parse.go` (the peer-block loop that already refuses `option=env:`)
- [ ] `internal/test/peer/expect.go` (the peer expectation reader)

**Behavior to preserve:** every currently-passing `.ci` must keep passing once the guard fires; a site that genuinely wants a rejection assertion gets one that works, not a deleted line.

## Data Flow (MANDATORY)

### Entry Point
A `.ci` file containing `reject=` between `stdin=peer:` and its terminator.

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
| A-1 | The three known sites are the only ones; a tree-wide scan finds no fourth. | Review scan of 2026-08-02 over `test/**/*.ci`. | The audit list grows; the guard is unchanged. | (fill during design) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The guard fires on a site whose author intended the assertion, turning a quiet gap into a red suite. That is the point, but it must be fixed at the same time, not left red. | (fill during design) | (fill during design) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| A `.ci` with `reject=` inside a peer block | -> | the runner's peer-block parser hard-errors | a fixture `.ci` under `test/draft/` that must fail to parse |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A `.ci` with `reject=` inside a `stdin=peer` block | The runner refuses the file with an error naming the directive and the line |
| AC-2 | The three known sites | Each either carries a working rejection assertion or has the dead line removed with a stated reason |
| AC-3 | The whole `.ci` corpus | No other site carries a silently dropped directive |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestPeerBlockRefusesRejectDirective | `internal/test/runner/record_parse_test.go` | AC-1 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| a draft `.ci` carrying `reject=` in a peer block | `test/draft/plugin/` | the runner refuses the file instead of dropping the directive | |

## Files to Modify
- `internal/test/runner/record_parse.go` - the guard that hard-errors on a dropped `reject=`
- `docs/architecture/testing/ci-format.md` - document that a peer block refuses `reject=`
- `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci`
- `test/plugin/filter-family-export-flowspec.ci`
- `test/plugin/logging-level-filter.ci`

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
