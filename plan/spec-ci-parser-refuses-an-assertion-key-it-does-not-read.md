# Spec: the generic .ci parser refuses an assertion key it does not read

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | implemented and landed at `8c7f0a5bf2`; the sibling parser is a spec of its own |
| Handoff | - |
| Updated | 2026-09-06 |

<!-- Backfilled. The work was commissioned straight from two journal rows and
     skipped the spec step. Status is in-progress: the product code exists, so
     the spec is past design, and closure has not run. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`parseExpect` in the generic `.ci` parser read one key per arm and dropped every
other key in silence, so `expect=stdout:not-contains=X` recorded NO assertion and
passed whatever the command printed. Sixteen assertions in the tree were vacuous
for that reason. The spelling gets written because `expect=file:` DOES accept
`not-contains=`, so two arms of one directive family disagreed and neither said
so.

Evidence is in `plan/journal/silent-fall-through.md` and
`plan/journal/green-that-could-not-have-been-red.md`, and the mechanism is in the
commit message of `8c7f0a5bf2`. Neither is restated here.

Goal: a `.ci` file naming a key the parser does not read FAILS the file, naming
the key, the accepted keys and the line. `ai/rules/principles.md`: a parser that
cannot answer says so.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the directive grammar and its key vocabulary
  → Constraint: one assertion vocabulary, not two. A negative on a stream is
    `reject=`, and `expect=stdout:!contains=` is DELETED rather than aliased
    (`ai/rules/no-layering.md`).
- [ ] `ai/patterns/functional-test.md` - what a `.ci` author writes
  → Decision: the refusal message must teach the replacement spelling, since an
    author learns the vocabulary from the refusal.

**Key insights:**
- A vacuous assertion is worse than a missing one: it reads as coverage.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/record_parse.go` - `parseExpect` and `parseReject`, the arms that read the keys
- [ ] `internal/test/runner/record_parse_keys.go` - `checkKeys` (`:29`), the refusal
- [ ] `internal/test/ci/ciformat.go` - `splitOnKeyBoundary`, which cuts a directive into pairs
- [ ] `docs/architecture/testing/ci-format.md` - the published grammar

**Behavior to preserve:**
- Every currently-passing `.ci` file still parses, once its spelling is migrated.
- `expect=file:not-contains=` keeps its meaning: the file arm always read it.

**Behavior to change:**
- Each `expect=` and `reject=` arm declares the keys it reads, and `checkKeys`
  fails the file on any other key.
- `not-contains=` and `!contains=` on a STREAM carry a named hint to their
  replacement, `reject=stdout:contains=`.
- `expect=stdout:!contains=` is deleted, and its 19 uses are migrated.

## Data Flow (MANDATORY)

### Entry Point
- A `.ci` file read by the functional test runner. Entry format is one directive
  per line, `expect=<stream>:<key>=<value>`.

### Transformation Path
1. `splitOnKeyBoundary` (`internal/test/ci/ciformat.go`) cuts the directive into
   key/value pairs.
2. `parseExpect` / `parseReject` (`record_parse.go`) select the arm by stream.
3. `checkKeys` (`record_parse_keys.go`) compares the pairs against the keys that
   arm declares, and returns an error naming the key and the line.
4. The arm reads its own keys and records the assertion.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` author ↔ runner | the directive text | Yes, the file now fails on an unread key |
| Generic parser ↔ the `test/parse` parser | two independent implementations of one format | NO. That disagreement is `plan/spec-test-parse-ci-parser-refuses-an-unread-directive.md` |

### Integration Points
- `internal/test/runner/accept_only.go` - the accept-only ratchet parses with
  this parser, so a newly-unparseable file is named by it.
- `internal/test/cli/ci_parse_gate_test.go` - the gate that parses the corpus.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | every arm routes through `checkKeys` |
| No unintended coupling | Yes | change confined to `internal/test/runner` and `internal/test/ci` |
| No duplicated functionality | Partly | a SECOND parser for the same format still exists in `internal/test/runner/parsing.go`; that is the sibling spec |
| Zero-copy preserved where applicable | N-A | test tooling |
| Registration over hardcoding | No | each arm names its own key list, which is a per-arm declaration rather than a central one, and the compiler does not tie the list to the reads |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every vacuous assertion in the corpus is repairable to a real one | the 16 were each read and rewritten | a repaired assertion goes red and hides a product defect | the 16 repairs, each re-run | confirmed |
| A-2 | Deleting `expect=stdout:!contains=` breaks no caller outside the tree | grep over `test/` | a scenario stops parsing | grep plus the parse gate | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A repaired assertion turns red because the PRODUCT is wrong | the assertion fails after migration | that red is the finding; fix the product, never the assertion (`ai/rules/pre-release.md`) |
| R-2 | The `test/parse` suite's own parser still reads the deleted spellings | its three files become unparseable to the accept-only ratchet | named in the sibling spec, and recorded in `plan/journal/silent-fall-through.md` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | functional tests stop parsing, or an assertion silently keeps passing |
| How is it reverted? | single commit revert; `8c7f0a5bf2` touches 42 files, mostly `.ci` migrations |
| Who else touches this path? | every author of a `.ci` file, and the accept-only ratchet |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a `.ci` file carrying an unread key | → | `checkKeys` (`internal/test/runner/record_parse_keys.go`) | `internal/test/runner/record_parse_keys_test.go` |
| the whole `test/` corpus | → | `parseExpect` / `parseReject` (`record_parse.go`) | `internal/test/cli/ci_parse_gate_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `expect=stdout:not-contains=X` | the file FAILS to parse, and the message names the key, the accepted keys and the line |
| AC-2 | `expect=stdout:!contains=X` | the file fails, and the message names `reject=stdout:contains=` as the replacement |
| AC-3 | `expect=file:path=p:not-contains=X` | still parses and still asserts, unchanged |
| AC-4 | the whole `test/` corpus | parses with no unread key anywhere |
| AC-5 | each of the 16 repaired assertions | asserts what its author meant, and would go red if the behavior it names regressed |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| the key-refusal tests | `internal/test/runner/record_parse_keys_test.go` | AC-1, AC-2, AC-3 | landed with `8c7f0a5bf2` |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| keys per directive | 1..n | n | 0, which the arm reports as a missing key | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the corpus parse gate | `internal/test/cli/ci_parse_gate_test.go` | a `.ci` author writes an unread key and learns immediately | landed |
| the 16 repaired assertions | `test/**/*.ci` | each now asserts what its name claims | landed |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| none | - | - | test tooling, no wire-visible behavior | N-A |

## Files to Modify
- `internal/test/runner/record_parse.go` - each arm declares its keys
- `internal/test/runner/record.go`, `runner_exec.go`, `runner_output_assert.go`, `peer_contract.go`, `accept_only.go` - the deleted spelling's readers
- `docs/architecture/testing/ci-format.md`, `docs/functional-tests.md`, `ai/patterns/functional-test.md` - the vocabulary

## Files to Create
- `internal/test/runner/record_parse_keys.go` - `checkKeys`
- `internal/test/runner/record_parse_keys_test.go` - its tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | test tooling, no operator config |
| CLI commands/flags | No | no command surface changed |
| Functional test for new RPC/API | N-A | no RPC |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |
| Env var registration | N-A | none added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/ci-format.md`, both edited in `8c7f0a5bf2` |
| 16 | Any changed source file referenced by existing doc anchors? | Yes | `docs/architecture/testing/ci-format.md` is the anchor for the runner's parse files, and it was updated |
| 1-9, 11-15, 17 | - | No | no operator-facing surface changed |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - add `checkKeys` and call it from one
   arm; write the refusal test and observe it red.
   - Files: `record_parse_keys.go`, `record_parse_keys_test.go`
2. **Phase: Every arm** - give each `expect=` and `reject=` arm its key list.
   - Files: `record_parse.go`
3. **Phase: Vocabulary** - delete `expect=stdout:!contains=`, add the hint, and
   migrate its 19 uses to `reject=stdout:contains=`.
4. **Phase: Repair** - run the corpus, and repair each assertion the refusal
   exposes. A red that says the PRODUCT is wrong is the finding.
5. **Phase: Documentation** - state the one vocabulary in the three pages.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every arm calls `checkKeys`, none is left reading a key list it does not declare |
| Correctness | the repaired assertions assert the author's meaning, not the current output |
| Naming | one negative spelling on a stream, `reject=`, and no alias beside it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| no unread key in the corpus | `go test ./internal/test/cli/ -run CIParseGate` |
| the deleted spelling is gone | `grep -rn 'expect=stdout:!contains=' test/` answers nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the refusal message quotes the offending key, so a crafted key cannot be mistaken for parser text |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A repaired assertion fails | the PRODUCT is wrong; fix it at the source |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- Two arms of one directive family that disagree about a key teach the wrong
  word, and the author who learned it has no way to find out.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Delete `expect=stdout:!contains=` | alias it to `reject=stdout:contains=` | `ai/rules/no-layering.md`; the alias is what taught the wrong word |
| Per-arm key declaration | one central key table | the arm and its list sit together, so a reviewer sees both in one screen. The cost is that the compiler does not tie the list to the reads, which is the Architectural Verification "No" above |

## Known Limitations
- The `test/parse` suite has its OWN parser for the same format, and it still
  drops a directive no arm matches. `not:contains=` means the OPPOSITE there.
  That is `plan/spec-test-parse-ci-parser-refuses-an-unread-directive.md`.
- `splitOnKeyBoundary` folds a key whose lead byte is not a letter into the
  previous value, so a second assertion can vanish before `checkKeys` sees it.
  Recorded in `plan/journal/silent-fall-through.md`, no live instance in the tree.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated, not library-only
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean

## Current Condition and What Remains

| Item | State |
|------|-------|
| Product code | LANDED at `8c7f0a5bf2`. Reachability checked: `git rev-list HEAD \| grep -c '^8c7f0a5bf2'` answers 1 |
| Documentation | landed in the same commit |
| Journal rows | `plan/journal/silent-fall-through.md` and `plan/journal/green-that-could-not-have-been-red.md` |
| PROVEN | AC-1, AC-2 and AC-3, by the tests in `record_parse_keys_test.go`. AC-4 by the corpus parse gate |
| ASSERTED, not proven | AC-5. The 16 assertions were repaired and pass, and no discrimination walk forced each one red against the behavior it names. A repaired assertion that was never observed red is exactly the shape `plan/journal/green-that-could-not-have-been-red.md` counts |
| Remains | the AC-5 discrimination walk over the 16, and the closure sections |
