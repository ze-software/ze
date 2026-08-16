# Spec: fixit-stress-repro-counts-a-run-that-never-started

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`scripts/dev/stress-repro.py` reports `*** REPRODUCED on invocation 1` for a run
that started no test. The tool exists to answer "is this failure real or
load-dependent", and a false REPRODUCED is the one answer that ends the
investigation with the wrong verdict.

`usage_error_signature` guards against this and its guard is too narrow. It
returns a signature only when BOTH hold:

- the output contains `USAGE_BANNER`, which is `"\nCommands:\n"`, and
- the output contains one of three literal strings in `USAGE_SIGNATURES`:
  `unknown command:`, `unknown suite:`, `flag provided but not defined`.

A never-dispatched run whose message is none of those three passes the guard and
is counted as a reproduction.

**The code's own comment establishes the banner is the discriminating half.** It
reads: "ze-test prints the signature followed by 'Commands:' and the full command
list only when it never dispatched a suite; a run that reached a test never
prints it. That is a property of 'no test ran', not of ordering." That is the
sufficient condition, stated by the author.

The signature requirement was added for the opposite reason, also documented: the
three strings are NOT unique to a usage error, because `test/ui/root-namespace.ci`
and `test/ui/pipe-operators.ci` both assert them as expected output, and the
runner echoes the needle into a failure report. So the pairing exists to stop a
real failure being discarded as a typo.

**Both concerns are real and the current pairing serves only one.** Requiring the
non-unique half as well as the discriminating half converts a false positive into
a false negative, and the false negative is the more expensive direction here: a
discarded reproduction costs a rerun, a fabricated reproduction costs a wrong
conclusion about the product.

**Observed.** `stress-repro.py plugin` reported REPRODUCED on invocation 1 for a
run that started no test. Recorded in
`plan/journal/gate-excludes-part-of-its-population.md`. The exact message ze-test
emitted in that case is not yet captured and is the first deliverable.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/evidence.md` -- a guard that fails open in the permissive
      direction; here the permissive answer is "reproduced"
- [ ] `ai/rules/testing.md` -- the flaky-versus-deterministic distinction this
      tool exists to make
- [ ] `ai/INDEX.md` -- the dev-tools row naming this script, which must stay true

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/stress-repro.py` -- `usage_error_signature`,
      `USAGE_SIGNATURES`, `USAGE_BANNER`, `run_once`, and the reporting site that
      prints `*** REPRODUCED on invocation`
  → Constraint: the banner test is documented as a property of "no test ran".
    Any fix keeps that reasoning and does not weaken it.
  → Constraint: the three signatures are asserted as EXPECTED output by at least
    two `.ci` tests. A fix must not discard a genuine failure that quotes them.
- [ ] `internal/test/cli/` -- what ze-test actually prints when a suite token
      names something it cannot dispatch
  → Decision: whether the banner is emitted in every never-dispatched path is
    the fact the fix turns on. If it is, the banner alone is the guard.

**Behavior to preserve:**
- A real failure whose output happens to contain one of the three phrases is
  still counted as a reproduction.
- The parallel-completion property: the guard must not key on invocation
  ordinal, which the comment already rules out.

**Behavior to change:**
- A run that never dispatched a suite is never counted as a reproduction, whatever
  its message says.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `python3 scripts/dev/stress-repro.py <suite> [selector]`, run by an agent
  chasing a suspected flake.

### Transformation Path
1. `run_once` invokes `ze-test <suite> <sel> -v` with prebuilt binaries.
2. The combined output is passed to `usage_error_signature`.
3. If it returns a signature, the invocation is discarded as a usage mistake.
4. Otherwise a non-zero exit is counted as a reproduction and reported.
5. Step 3 is the defect: a never-dispatched run with an unlisted message reaches
   step 4.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| script <-> ze-test | the child's combined stdout and stderr | Yes, read in `run_once` |
| guard <-> reporting | the return of `usage_error_signature` | Yes |
| ze-test <-> its dispatch table | which messages accompany the banner | Not yet: this is deliverable 1 |

### Integration Points
- `ai/INDEX.md` names the tool for agents chasing flakes.
- `plan/known-failures/` shards cite its verdicts.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | one guard, one reporting site |
| No unintended coupling (components stay isolated) | Yes | stays inside `scripts/dev/` |
| No duplicated functionality (extends existing, does not recreate) | Yes | the guard is extended, not replaced |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Python tooling |
| Registration over hardcoding | No | `USAGE_SIGNATURES` is a hardcoded list of three messages. Whether ze-test can be asked instead is worth one question during design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validation | Status |
|----|-----------|-------|----------|------------|--------|
| A-1 | ze-test prints the banner on EVERY never-dispatched path | the comment says so | the banner is not sufficient and the fix needs a different signal, such as the child's exit code or a machine-readable marker | drive ze-test with each bad-token shape and capture the output | unvalidated |
| A-2 | `stress-repro.py plugin` reproduced this | one observation, recorded in the journal | the symptom is something else and the guard is fine | rerun it and capture the exact output | unvalidated |
| A-3 | No `.ci` asserts the banner text `"\nCommands:\n"` as expected output | the comment's argument only names the three signatures | keying on the banner alone reintroduces the false negative the pairing prevents | grep the `.ci` corpus for the banner | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Widening the guard discards a real reproduction | a known-real failure stops reproducing under the tool | A-3 is the gate: prove no fixture asserts the banner before keying on it alone |
| R-2 | Adding a machine-readable marker to ze-test is a bigger change than the defect warrants | the diff grows past the script | `ai/rules/simplicity.md`: prefer the smallest fully correct answer, and A-1 decides whether one exists |

## Blast Radius

`scripts/dev/stress-repro.py` and, if A-1 is broken, whatever ze-test prints on a
bad suite token. No daemon code, no wire behavior, no test corpus change.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `stress-repro.py` over an output that carries the banner and no listed signature | -> | `usage_error_signature` | a case in `scripts/dev/stress_repro_test.py` |
| the same over a real failure quoting `unknown command:` with no banner | -> | the same | a second case, counted as a reproduction |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Output carrying the usage banner and a message not in `USAGE_SIGNATURES` | Discarded as never-dispatched. Not counted as a reproduction |
| AC-2 | Output from a genuine test failure that quotes `unknown command:` and carries no banner | Counted as a reproduction, unchanged |
| AC-3 | Output carrying the banner and a listed signature | Discarded, unchanged |
| AC-4 | A run where a later parallel invocation completes first | Verdict unchanged: the guard keys on no ordinal |
| AC-5 | `stress-repro.py plugin` | Reports that no test ran, not a reproduction |

## End-to-End User Stories

- An agent suspects a flake, runs the tool with a suite token the runner cannot
  dispatch, and is told no test ran rather than being told the failure
  reproduced.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| banner without a listed signature is a usage error | `scripts/dev/stress_repro_test.py` | AC-1 | |
| a listed signature without the banner is a reproduction | `scripts/dev/stress_repro_test.py` | AC-2 | |
| banner with a listed signature stays a usage error | `scripts/dev/stress_repro_test.py` | AC-3 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | The tool is a dev script with no daemon surface. Its own Python suite driven against real captured ze-test output is the end-to-end test, and capturing that output is deliverable 1 | |

## Files to Modify

- `scripts/dev/stress-repro.py` -- `usage_error_signature`, and
  `USAGE_SIGNATURES` if the list stops being the discriminator
- `scripts/dev/stress_repro_test.py` -- the failing case first
- `internal/test/cli/` -- only if A-1 is broken and ze-test must say so plainly

## Files to Create

- `scripts/dev/stress_repro_test.py`, if no sibling suite exists yet

## Implementation Steps

1. **Phase: Capture** -- run ze-test with each bad-token shape and record what it
   prints, including the `plugin` case
   - Verify: A-1 and A-2 confirmed or broken, with real output pasted
2. **Phase: Reproduce** -- the failing unit case
   - Verify: AC-1 is RED
3. **Phase: Fix** -- at whichever layer step 1 selects: the guard alone if the
   banner is sufficient, otherwise ze-test says so plainly
   - Verify: AC-1, AC-2, AC-3, AC-4
4. **Phase: Confirm the original**
   - Verify: AC-5
5. **Phase: `make ze-unit-pkg-test PKG=./scripts/dev`**
   - Verify: no sibling regressed

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] **Commit A:** code + tests + spec
- [ ] **Commit B:** `git rm plan/<spec>` only

## Known Limitations

- The tool still cannot distinguish a run that dispatched a suite and then
  crashed before any test from one that ran tests and failed. That is a
  different question and needs a different signal.
