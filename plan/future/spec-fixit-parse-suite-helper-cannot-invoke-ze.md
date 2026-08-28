# Spec: fixit-parse-suite-helper-cannot-invoke-ze

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/finish-ci-coverage.md` |
| Updated | 2026-08-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Moved to `plan/future/` 2026-08-17.** Triage found nothing currently red. This adds a capability to the parse-suite rig (a helper being able to invoke `ze`), so it is an improvement rather than a release defect.

## Task

A helper script in a parse-suite `.ci` cannot invoke `ze`. It gets `ze: not
found`, so any parse-suite fixture whose scenario needs the binary has to be
written some other way or not written at all.

The orchestrated runner solves this and the parsing runner does not.
`internal/test/runner/runner_exec.go` puts a bare-name shim directory on the
child's PATH in two places, with the comment "so child processes resolve THIS
ze". `internal/test/runner/parsing.go` builds its child environment with
`childEnv(test.EnvVars...)` and never adds a PATH entry for the directory holding
`r.zePath`. Two runners, one facility, present in one.

**This is a live, unhomed obligation, which is why it gets a spec rather than a
row.** `plan/deferrals/finish-ci-coverage.md` records it as "half two" of a
2026-08-03 finding. Half one, the `mode=` bits, is half done and tracked in
`plan/journal/helper-bypassed-by-an-open-coded-copy.md`. Half two was named in
the Task of `spec-fixit-ci-peer-block-silent-directives` and given no acceptance
criterion, so it was neither implemented nor covered, and that spec has now
closed. An obligation named in a Task and covered by no AC is the exact shape
`ai/rules/completion.md` refuses to leave lying around.

**Why it matters beyond the one fixture.** A test rig limit does not announce
itself as a limit. It announces itself as "that scenario is awkward to test", and
the scenario quietly goes untested. `ai/rules/testing.md` maps config-option
changes to `test/parse/`, so this gap sits on the suite that owns config
behavior.

**The shape of the fix looks small and the spec must confirm that before
believing it.** The orchestrated runner already computes the shim directory; if
the parsing runner can reuse the same producer, this is one call and its test. If
the two runners derive the binary path differently, the honest fix is one
producer for both, because two answers to "where is ze" is the drift that put
them out of step.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` -- what a parse-suite `.ci` may
      declare, which this changes
- [ ] `ai/rules/testing.md` -- the change-type to test-directory mapping that
      routes config work to `test/parse/`
- [ ] `ai/rules/no-layering.md` -- if a second PATH mechanism is added rather than
      the first reused, that is the failure this rule names

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/runner_exec.go` -- the two sites that put the
      bare-name shim dir on PATH, and how the shim dir is derived
  → Constraint: the comment states the purpose is that child processes resolve
    THIS ze, not whatever is on the developer's PATH. Any fix keeps that.
- [ ] `internal/test/runner/parsing.go` -- `parsingRunner.setupWorkDir`,
      `childEnv`, and where `r.zePath` is set
  → Decision: whether `childEnv` is shared with the orchestrated path or is a
    second implementation decides whether this is a one-line fix or a merge.
- [ ] `internal/test/runner/record.go` -- what a parse-suite `Record` carries, so
      the fix has the binary path available where the environment is built

**Behavior to preserve:**
- The shim resolves the ze binary under test, never a system one.
- Every existing parse-suite fixture passes unchanged.
- The orchestrated path's PATH handling is untouched.

**Behavior to change:**
- A parse-suite helper script can invoke `ze` and gets the binary under test.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le functional parse`, or `bin/ze-test parse <selector>`, over a `.ci` whose
  `tmpfs=` helper script calls `ze`.

### Transformation Path
1. The parse runner materialises the fixture's tmpfs files into a work directory.
2. It builds the child environment with `childEnv(test.EnvVars...)`.
3. That environment carries no PATH entry for the directory holding `r.zePath`.
4. The helper runs and `ze` does not resolve.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| parse runner -> child process | the environment built by `childEnv` | Yes, recorded in the deferral shard and consistent with the absence of a PATH line in `parsing.go` |
| orchestrated runner -> child process | the shim dir added to PATH | Yes, read in `runner_exec.go` |
| runner -> binary under test | `r.zePath` | Not yet: whether both runners derive it the same way is the first read |

### Integration Points
- `docs/architecture/testing/ci-format.md` documents the `.ci` surface.
- Every existing `test/parse/` fixture.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the environment builder is the right site |
| No unintended coupling (components stay isolated) | Yes | stays inside `internal/test/runner` |
| No duplicated functionality (extends existing, does not recreate) | To be decided | reusing the orchestrated shim producer satisfies this; a second one does not |
| Zero-copy preserved where applicable (refs, not copies) | N-A | test rig, not a hot path |
| Registration over hardcoding | N-A | no registration surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validation | Status |
|----|-----------|-------|----------|------------|--------|
| A-1 | The shim-dir producer in the orchestrated path is reusable from the parsing runner | both live in `internal/test/runner` | the fix is a merge of two environment builders, not a call | read both and name the shared producer, or the two | unvalidated |
| A-2 | No parse-suite fixture depends on `ze` being absent from PATH | nothing suggests it would | a fixture asserting `ze: not found` breaks | grep `test/parse/` for the message | unvalidated |
| A-3 | The parse runner has `r.zePath` in scope where the environment is built | it runs the binary | the path must be threaded and the change is larger | read `parsing.go` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | A second PATH mechanism is added rather than the first reused | the diff touches neither `runner_exec.go` nor a shared helper | A-1 is the gate: name the producer before writing the call |
| R-2 | Adding PATH changes an existing fixture's behavior | a `test/parse/` fixture goes red | A-2, and the full parse suite is the evidence, never a sample |

## Blast Radius

`test/parse/` fixtures and the parse runner's child environment. No daemon code,
no wire behavior.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| a `test/parse/` `.ci` whose `tmpfs=` helper runs `ze --version` | -> | the parse runner's child environment | a new `.ci` under `test/parse/` |
| the same, asserting the binary is the one under test and not a system one | -> | the shim dir | the same fixture, asserting the resolved path |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A parse-suite helper script invokes `ze` | It resolves and runs |
| AC-2 | The same, on a host with a different `ze` on PATH | The binary under test wins, matching the orchestrated runner's stated guarantee |
| AC-3 | Every existing `test/parse/` fixture | Passes unchanged, proven by the FULL suite and never a sample |
| AC-4 | The orchestrated suite | Unchanged, proven by `./le functional plugin` |
| AC-5 | `plan/deferrals/finish-ci-coverage.md` | Half two moves from `live` to a terminal state, and the shard is removed if that was its last live row |

## End-to-End User Stories

- A test author writes a `test/parse/` fixture whose helper drives `ze config
  validate` and it works, instead of discovering that the parse suite cannot do
  it and writing the scenario somewhere it fits less well.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| the parse runner's child environment carries the shim dir on PATH | `internal/test/runner/parsing_test.go` | AC-1 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| the shim dir precedes any inherited PATH entry | `internal/test/runner/parsing_test.go` | AC-2 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `parse-helper-invokes-ze.ci` | `test/parse/` | a helper script runs `ze` and the fixture asserts its output | |

## Files to Modify

- `internal/test/runner/parsing.go` -- the child environment
- `internal/test/runner/runner_exec.go` -- only if the shim producer is extracted
  so both runners share one
- `docs/architecture/testing/ci-format.md` -- the parse-suite surface, once it
  gains the facility
- `plan/deferrals/finish-ci-coverage.md` -- half two's disposition

## Files to Create

- `test/parse/parse-helper-invokes-ze.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
- `internal/test/runner/parsing_test.go`, if no sibling exists <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->

## Implementation Steps

1. **Phase: Read both runners** -- name the shim-dir producer and decide reuse
   versus extract
   - Verify: A-1 and A-3 confirmed or broken
2. **Phase: Reproduce** -- the fixture, red with `ze: not found`
   - Verify: AC-1 is RED
3. **Phase: Fix** -- at the producer the first phase named
   - Verify: AC-1, AC-2
4. **Phase: Full suites** -- `./le functional parse` and `./le functional plugin`, both
   entire
   - Verify: AC-3, AC-4
5. **Phase: Close the deferral row and the doc**
   - Verify: AC-5, `./le doc-check verify`

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify current mode full` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/specsession/review.go`
- [ ] **Commit A:** code + tests + spec
- [ ] **Commit B:** `git rm plan/<spec>` only

## Known Limitations

- Half one of the original finding, the `mode=` bits dropped on the orchestrated
  path, is NOT in this spec. It is recorded in
  `plan/journal/helper-bypassed-by-an-open-coded-copy.md` with the eight call
  sites it touches, and it is a runner-wide change that deserves its own spec.
