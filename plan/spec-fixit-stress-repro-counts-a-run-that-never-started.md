# Spec: fixit-stress-repro-counts-a-run-that-never-started

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make the stress reproducer distinguish a dispatched suite from an invocation
that never reached one. A never-dispatched invocation must report a setup
failure and must never produce a REPRODUCED verdict, whatever its diagnostic
text says. Require positive evidence of suite dispatch before applying the
existing crash and `any-failure` classification.

The current Go producer is `run` in `internal/le/stressrepro/run.go`.
`usageErrorSignature` recognises only the conjunction of `usageBanner`
(`"\nCommands:\n"`) and one of `unknown command:`, `unknown suite:` or
`flag provided but not defined`. If neither matches, classification continues
to `crashSignature` and the `AnyFailure` nonzero-exit check. No positive
suite-dispatch condition gates that verdict.

A usage banner is not a dispatch marker. A real functional test can assert
help output: `test/ui/help-parent-node.ci` asserts `Commands:` in the help
page of `ze show bgp help`. A failure report quoting that expected output
must remain eligible as a genuine reproduction once the suite dispatched.

This is development tooling. The 2026-08-19 disposition placed it in the
optional backlog; this correction does not turn it into a release defect.

## Historical evidence and corrected diagnosis

The original observation was that the retired `stress-repro.py plugin`
reported `*** REPRODUCED on invocation 1` although no test started. The
occurrence is recorded in `plan/journal/gate-excludes-part-of-its-population.md`.
The subsequent correction recorded that the captured output contained no
usage banner at all. The original proposal to drop only the signature half
of the guard could therefore not fix that observation.

The earlier assumption that no real test could print the banner was also
false. Both negative-text heuristics are superseded by the positive-dispatch
contract above. The historical invocation is not a claim about the current
Go command's result, and no fresh run was made for this reconciliation.

## Required Reading

- [ ] `docs/functional-tests.md`: current suite invocation and draft workflow.
- [ ] `docs/architecture/testing/runner-architecture.md`: suite dispatch,
      discovery and reporting boundaries.
- [ ] `ai/rules/evidence.md`: a verdict requires evidence for its population.
- [ ] `ai/rules/testing.md`: genuine failures must remain visible.

## Current Behavior (MANDATORY)

- [ ] `internal/le/stressrepro/run.go`: `run`, `usageErrorSignature`,
      `crashSignature` and `Report.Text`.
- [ ] `internal/le/stressrepro/process.go`: `realProcessRunner.Invoke` and
      `runCommand`, the argv and combined-output producer.
- [ ] `internal/test/cli/dispatch.go` and the suite entry points it registers:
      select the producer of trustworthy dispatch evidence during design.

Preserve crash-signature classification, the explicit `any-failure` mode,
parallel completion independent of invocation ordinal, and captured child
output. A failure from a dispatched suite stays eligible even when its output
contains a usage banner or any of the old usage signatures.

Change the prerequisite for classification: absent positive dispatch evidence
means the invocation cannot establish a reproduction. A zero exit without that
evidence must not be reported as a successful test run either.

## Data Flow (MANDATORY)

### Entry Point

`./le stress-repro run suite <suite> [test <selector>]`.

### Transformation Path

1. `realProcessRunner.Invoke` starts the selected ze-test command.
2. The suite execution boundary produces positive dispatch evidence through
   a channel the parent can distinguish from fixture output. Design must
   establish whether a suitable existing marker can be reused.
3. The reproducer checks that evidence for each completed invocation.
4. Without it, report a never-dispatched/setup outcome and retain the capture.
5. With it, apply the existing crash and `any-failure` verdict rules.

### Boundaries Crossed

| Boundary | Contract | Evidence owed |
|----------|----------|---------------|
| ze-test dispatch -> reproducer | Positive evidence comes from dispatch, not a quoted fixture assertion | Producer and non-spoofing argument at design time |
| Child output -> diagnostic report | Preserve the output even when no suite dispatched | Captured invalid invocation |
| Parallel invocation -> verdict | Each invocation carries its own dispatch state | Later invocation completes first |

### Integration Points

The existing stress-repro process runner and ze-test suite entry points own
this flow. No daemon behaviour or new test-file directive is required.

### Architectural Verification

Reuse an existing dispatch signal only if its producer and all supported
suite paths satisfy the contract. A help banner, exit status or invocation
ordinal alone cannot answer whether a suite dispatched.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | An existing positive suite-dispatch signal can be reused | The runner already reports suite execution, but suitability has not been established | Add evidence at the owning dispatch boundary | Trace each supported suite path and capture its result | unvalidated |
| A-2 | The historical never-dispatched output can be recovered or reconstructed against the current CLI | The August correction records the absence of a banner | Use a current invalid invocation that demonstrates the same verdict error, without claiming the old spelling still reproduces | Recover the capture, then compare with current argv and dispatch | unvalidated |
| A-3 | Fixture output cannot impersonate the selected dispatch evidence | The producer/channel is not yet designed | A quoted marker could reintroduce the false reproduction | Exercise a fixture whose output quotes the marker | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | The guard rejects a genuine test failure quoting help text | A dispatched failure is classified as setup failure | Classify by dispatch evidence; keep the banner-and-signature failure case |
| R-2 | A supported suite never emits the marker | Valid invocations report never-dispatched | Identify every supported dispatch path before implementation |

## Blast Radius

`internal/le/stressrepro/` and, if needed, the existing ze-test dispatch or
execution reporting boundary. No product or wire behaviour change.

## Wiring Test (MANDATORY)

| Entry Point | Feature Code | Required proof |
|-------------|--------------|----------------|
| Invalid current ze-test invocation through stress-repro | Dispatch check before reproduction verdict | No dispatch marker, nonzero exit, no REPRODUCED |
| Dispatched failing fixture that quotes help text | Existing crash/any-failure verdict after dispatch check | Genuine failure remains eligible |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A failed invocation without positive dispatch evidence, with or without a banner or listed signature | Report never-dispatched/setup failure; never count it as a reproduction |
| AC-2 | A dispatched genuine failure quoting a listed usage signature | Apply the existing reproduction rules |
| AC-3 | A dispatched genuine failure quoting the usage banner, including one that also quotes a listed signature | Apply the existing reproduction rules; do not discard it as a usage error |
| AC-4 | A later parallel invocation completes first | Verdict follows that invocation's dispatch evidence and result, independent of ordinal |
| AC-5 | The historical never-dispatched scenario reconstructed for the current CLI | Retain the capture and report that no suite dispatched, without a REPRODUCED verdict |

## End-to-End User Stories

An agent chasing a flake with an invalid suite invocation gets a setup error.
A genuine failure of a help-output fixture can still reproduce under stress.

## TDD Test Plan

### Unit Tests

| Case | Location | Validates |
|------|----------|-----------|
| No marker, failed exit, both banner-present and banner-absent captures | `internal/le/stressrepro/stressrepro_test.go` | AC-1 |
| Marker present, failure quotes banner and signatures | `internal/le/stressrepro/stressrepro_test.go` | AC-2, AC-3 |
| Parallel completion order differs from launch order | `internal/le/stressrepro/stressrepro_test.go` | AC-4 |

### Functional Tests

Exercise the actual current stress-repro command over the invalid invocation
and a discriminating failing fixture. This proves the dispatch marker reaches
the classifier; a fabricated unit result alone does not.

## Files to Modify

- `internal/le/stressrepro/run.go` and its existing tests.
- `internal/le/stressrepro/process.go` if the evidence channel requires it.
- The existing ze-test dispatch/reporting producer selected during design.
- `docs/functional-tests.md` if the tool's documented result contract changes.

## Files to Create

None selected before design; prefer the existing producer and test files.

## Implementation Steps

1. Recover the historical capture and identify a current never-dispatched input.
2. Identify the positive dispatch producer and resolve A-1/A-3 for every supported suite path.
3. Prove the current classifier misreports the input, then change the dispatch prerequisite and exercise AC-1 through AC-4.
4. Confirm AC-5 through the actual command and retain the output.
5. Complete review and the worktree verification gate.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 through AC-5 demonstrated.
- [ ] Wiring table names the concrete tests and executed command evidence.
- [ ] Every assumption resolved.
- [ ] `./le verify worktree` passes.

### TDD
- [ ] Regression fails before the change and passes afterwards.
- [ ] Genuine dispatched failures remain reproducible.

### Closure
- [ ] Append and complete `plan/TEMPLATE-CLOSURE.md`.
- [ ] Independent review recorded through `./le spec session review record`.
- [ ] Commit A preserves code, tests and this spec; commit B removes the spec.

## Known Limitations

Suite dispatch alone does not prove that an individual test started before a
crash. This spec preserves that distinction; it does not infer test execution
from a dispatch marker.
