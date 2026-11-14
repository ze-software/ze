# Spec: the functional runner refuses a stream expectation under a test-peer command

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Test tooling. A `.ci` author can write an assertion that can never pass, and its red reads as a product failure. No shipped behavior depends on it |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Created at the closure of `spec-bgp-session-write-wedge` (2026-11-14), whose
Phase 1 met this defect while writing `test/plugin/session-peer-stops-reading.ci`.

The defect, read at the producer on 2026-11-14:

- The functional runner ties an `expect=stdout:`, `expect=stderr:`,
  `reject=stdout:` or `reject=stderr:` line to the `cmd=` line above it
  (`Record.assertionTarget`, `internal/test/runner/record_parse.go`). It
  refuses a line with no command above it and a line under `cmd=stop`, and
  nothing else.
- A `le test peer` command's output is in no command span: the runner captures
  a test peer's stdout and stderr in that peer's own buffers
  (`runOrchestrated`, `internal/test/runner/runner_exec.go`, the `isPeerExec`
  arms), which feed the peer's verdict and the PEER OUTPUT section of a
  failure report.
- So a stream expectation under a peer's `cmd=` line reads none of what the
  peer printed. An `expect=` there can never pass and fails with "output does
  not contain" while the text is on screen under PEER OUTPUT. A `reject=`
  there can never fail.

`docs/architecture/testing/ci-format.md` states the rule since the source
spec. Nothing refuses the line.

Goal: the runner refuses, at parse time, a stream expectation or rejection
under a test-peer command, and its message names what to write instead: what a
peer received is asserted by its `expect=bgp` lines, and what the daemon did
about the peer is asserted on the daemon's log under the daemon's `cmd=` line.

Constraints already known:

- `plan/post-release/engineering/spec-harness-cannot-pass-on-its-own-failure.md`
  (Status `design`) changes the same parser and owns sibling rows of the same
  journal class. This item was not added to it at the source spec's closure,
  because that spec needs its design sections brought up to date before it
  takes any edit. Whoever designs either spec first decides whether this one
  is folded into it.
- `plan/journal/test-against-broken-path.md` holds a sibling row of
  2026-10-27: twelve `firewall-*.ci` tests whose `expect=stderr:` sits under a
  fixture command while ze logs the line. The refusal designed here is checked
  against that shape too.
- Every `.ci` under `test/` that the new refusal would reject is converted in
  the same work.

## Required Reading

- `ai/doors/system.md`, `ai/doors/le-tooling.md`
- `docs/architecture/testing/ci-format.md`, "Every stream assertion sits under a command"
- `docs/architecture/testing/runner-architecture.md`

## Current Behavior (MANDATORY)

- [ ] `internal/test/runner/record_parse.go` -> `Record.assertionTarget`: refuses no command above and `cmd=stop`, accepts a peer command
- [ ] `internal/test/runner/runner_exec.go` -> `runOrchestrated`: a peer's output goes to its own buffers (`peerOutputs`), never to the run's accumulators

## Existing Mechanisms (MANDATORY)

To be completed at design. Known today:

| Mechanism | Where | This spec |
|-----------|-------|-----------|
| Parse-time placement check of a stream assertion | `record_parse.go` `Record.assertionTarget` | Extends: one more refused placement |
| Peer command recognition | `runner_exec.go` `isPeerExec` | Extends: the parser asks the same predicate |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A `.ci` file with a stream expectation under a `le test peer` command line.

### Transformation Path
To be written at design.

### Boundaries Crossed
None expected: every step is inside `internal/test/runner`.

### Integration Points
To be written at design.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| To be designed | → | To be designed | To be designed |

## Acceptance Criteria

- AC-1: a `.ci` with `expect=stdout:`, `expect=stderr:`, `reject=stdout:` or `reject=stderr:` under a `le test peer` command is refused at parse time, with a message that names the line, why it can never read the peer's output, and what to write instead.
- AC-2: no `.ci` under `test/` is refused by the new check.

## Risks & Assumptions

To be written at design.

## 🧪 TDD Test Plan

### Unit Tests

| Test | Asserts |
|------|---------|
| To be designed | To be designed |

## Files to Modify

To be decided at design.

## Implementation Steps

To be decided at design.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `test-against-broken-path.md` | 2026-11-14 | `assertionTarget` accepts a stream expectation under a `le test peer` command |

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] The targeted tests pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/test/runner/` and `./le test functional runner`
