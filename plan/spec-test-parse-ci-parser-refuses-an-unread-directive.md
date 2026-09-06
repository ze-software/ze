# Spec: the test/parse .ci parser refuses a directive it does not read

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `plan/spec-ci-parser-refuses-an-assertion-key-it-does-not-read.md` (landed at `8c7f0a5bf2`, which set the vocabulary this one adopts) |
| Phase | in flight: `internal/test/runner/parsing.go` is rewritten in the working tree, UNCOMMITTED |
| Handoff | - |
| Updated | 2026-09-06 |

<!-- Backfilled. The work was commissioned straight from a journal row and
     skipped the spec step. Status is in-progress: the rewrite exists in the
     working tree and the migration is not finished. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The `test/parse` suite has its OWN `.ci` parser. Its line loop is a chain of
prefix matches with NO default, so a directive no arm matches is dropped and the
file still parses. Its vocabulary also disagrees with the generic parser: it
reads `expect=stdout:regex=` where the generic reads `pattern=`, and
`expect=stdout:not:contains=` where the generic reads `reject=stdout:contains=`.

Worse than absent: `not:contains=` means the OPPOSITE in the two parsers. The
generic `splitOnKeyBoundary` cuts at `:contains=` and drops the bare `not`, so
the generic parser reads a NEGATIVE assertion as a POSITIVE one.

Measured on 2026-09-06 over `test/parse/*.ci` and recorded in
`plan/journal/silent-fall-through.md`: 26 lines assert nothing. One of them,
`test/parse/config-dump-masks-bcrypt.ci`, is the whole proof that a bcrypt hash
is masked. The counts are in the journal row and are not restated here.

Goal: one vocabulary for one format, and a directive no arm reads fails the file.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the directive grammar the generic parser now enforces
  → Constraint: the negative on a stream is `reject=stdout:contains=`, and
    `pattern=` is the regex key. `8c7f0a5bf2` settled both.
- [ ] `ai/patterns/functional-test.md` - what a `.ci` author writes
  → Decision: an author should not have to know which suite a file belongs to in
    order to know what a directive means.

**Key insights:**
- A second implementation of one format is the defect. The disagreement is what
  makes a vacuous assertion invisible: each parser is self-consistent.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/parsing.go` - `parseCIFile` and the `ciCommand` type it fills
- [ ] `internal/test/runner/record_parse.go` - the generic parser's arms, for the vocabulary
- [ ] `internal/test/ci/ciformat.go` - `splitOnKeyBoundary`, which is why `not:contains=` inverts
- [ ] `internal/test/runner/accept_only.go` - the ratchet, which parses `test/parse` with the GENERIC parser
- [ ] `docs/architecture/testing/ci-format.md` - the published grammar

**Behavior to preserve:**
- The parse suite's assertion SCOPE is per-command: an assertion is checked
  against the stdout and stderr of the `cmd=` line that precedes it. That differs
  from the generic runner, where a stream assertion is file-level over one
  combined buffer, and the difference is deliberate.
- Every `test/parse` file that asserts something today still asserts it.

**Behavior to change:**
- The line loop gets a default that FAILS the file, naming the directive and the
  line.
- `regex=` is DELETED in favour of `pattern=`, and `not:contains=` in favour of
  `reject=stdout:contains=` (`ai/rules/no-layering.md`). No alias.
- The 30 affected lines are migrated.
- Whatever the 26 now-live assertions turn red is diagnosed. A red that says the
  PRODUCT is wrong is the finding.

## Data Flow (MANDATORY)

### Entry Point
- A `.ci` file under `test/parse/`, read by the parse suite. Entry format is one
  directive per line.

### Transformation Path
1. `parseCIFile` (`internal/test/runner/parsing.go`) reads the file's other lines.
2. A per-file parser state dispatches each line to its arm.
3. An unmatched directive fails the file, naming it.
4. Each arm fills the `ciCommand` bound to the preceding `cmd=` line.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` author ↔ the parse suite | the directive text | in flight; the default arm is the mechanism |
| parse suite ↔ generic runner | two parsers, one format | the vocabulary converges here; the two parsers do NOT merge, because the assertion scope genuinely differs |
| `test/parse` ↔ the accept-only ratchet | the ratchet parses these files with the GENERIC parser | Yes, and it currently names the three files carrying `regex=` and `not=` as unparseable |

### Integration Points
- `internal/test/runner/accept_only.go` - the ratchet that reads the same files.
- `test/.accept-only-baseline` - which must be reconciled once the files parse.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | every line goes through the dispatcher, including the unmatched one |
| No unintended coupling | Yes | confined to `internal/test/runner` |
| No duplicated functionality | Partly | two parsers remain, by design, because the assertion scope differs. Only the VOCABULARY converges, and that divergence is now stated in the type's own comment |
| Zero-copy preserved where applicable | N-A | test tooling |
| Registration over hardcoding | No | the arms are a dispatch chain; the default is what makes an unlisted directive loud rather than silent |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | All 26 vacuous lines can be migrated to a live assertion | each was read during the measurement | a line has no equivalent in the shared vocabulary | the migration | UNVALIDATED |
| A-2 | The per-command assertion scope is deliberate, not an accident of the second parser | the suite's own files depend on it | merging the parsers would be correct after all | asserted from the files; the rewrite states it in the type comment | UNVALIDATED |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A now-live assertion turns red because the PRODUCT is wrong | the assertion fails after migration | that red is the finding; fix the product, never the assertion (`ai/rules/pre-release.md`) |
| R-2 | The accept-only baseline drifts while the files are unparseable to the ratchet | the ratchet names the three files | reconcile `test/.accept-only-baseline` in the same change |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the parse suite stops running, or keeps passing over assertions that check nothing |
| How is it reverted? | not yet landed; a single commit revert once it is |
| Who else touches this path? | the accept-only ratchet, and every author of a `test/parse` file |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a `test/parse` file carrying an unread directive | → | the default arm of the line dispatcher (`internal/test/runner/parsing.go`) | `internal/test/runner/parsing_test.go` |
| `test/parse/config-dump-masks-bcrypt.ci` | → | the `reject=stdout:contains=` arm | the parse suite run itself |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a `test/parse` file carrying a directive no arm reads | the file FAILS to parse, naming the directive and the line |
| AC-2 | `expect=stdout:pattern=` | parses and asserts a regex, with the same key the generic parser uses |
| AC-3 | `reject=stdout:contains=` | parses and asserts NON-containment, with the same meaning the generic parser gives it |
| AC-4 | `expect=stdout:regex=` or `expect=stdout:not:contains=` | the file fails, because both spellings are deleted |
| AC-5 | the whole `test/parse` corpus | parses with no unread directive, and every assertion is live |
| AC-6 | `test/parse/config-dump-masks-bcrypt.ci` | the bcrypt-masking assertion is live and would go red if the hash were published |
| AC-7 | the accept-only ratchet | parses every `test/parse` file with the generic parser and names none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| the parser's default-arm refusal and each migrated key | `internal/test/runner/parsing_test.go` | AC-1, AC-2, AC-3, AC-4 | written, uncommitted |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| assertions per `cmd=` | 0..n | n | 0, which is legal: a command may only be run | N/A |
| exit code | 0..255 | 255 | negative, refused as an invalid exit code | 256, refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the parse suite itself | `test/parse/*.ci` | an author writes a `test/parse` file and its assertions run | in flight; 26 lines are being made live |
| `config-dump-masks-bcrypt` | `test/parse/config-dump-masks-bcrypt.ci` | `ze config dump` masks a bcrypt hash | the assertion exists and is vacuous today |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| none | - | - | test tooling, no wire-visible behavior | N-A |

## Files to Modify
- `internal/test/runner/parsing.go` - the line dispatcher, its default arm, and the `ciCommand` fields
- `test/parse/*.ci` - the 30 affected lines
- `test/.accept-only-baseline` - reconcile once the files parse to the ratchet
- `docs/architecture/testing/ci-format.md` - state that the parse suite's scope is per-command and its vocabulary is the shared one

## Files to Create
- `internal/test/runner/parsing_test.go` - the parser's own tests

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
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/ci-format.md` and `docs/functional-tests.md` |
| 16 | Any changed source file referenced by existing doc anchors? | Yes | `docs/architecture/testing/ci-format.md` is the `// Design:` anchor of `parsing.go`, and it MUST state the per-command scope that distinguishes this parser from the generic one |
| 1-9, 11-15, 17 | - | No | no operator-facing surface changed |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - move the line loop onto a dispatcher
   with a default that fails, and write the refusal test. Observe it red.
2. **Phase: Vocabulary** - delete `regex=` and `not:contains=`, add `pattern=`,
   `reject=stdout:contains=` and the stderr regex arm.
3. **Phase: Migration** - rewrite the 30 affected lines in `test/parse/*.ci`.
4. **Phase: Diagnosis** - run the suite and diagnose every red the 26 now-live
   assertions produce. A product red is the finding, not an obstacle.
5. **Phase: Ratchet** - reconcile `test/.accept-only-baseline`.
6. **Phase: Documentation** - the two pages.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | no arm reads a key the file's grammar does not admit, and no directive falls through |
| Correctness | `reject=stdout:contains=` means non-containment in BOTH parsers |
| Naming | one spelling per assertion, shared with `record_parse.go` |
| Data flow | the per-command scope is preserved and documented, not silently changed |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| no unread directive in `test/parse` | run the parse suite |
| the two deleted spellings are gone | `grep -rn 'regex=\|not:contains=' test/parse/` answers nothing |
| the ratchet names no file | `go test ./internal/test/... -run AcceptOnlyLint` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the refusal quotes the offending directive, so a crafted line cannot be mistaken for parser text |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A now-live assertion fails | the PRODUCT is wrong; fix it at the source |
| A migrated line has no equivalent spelling | A-1 is broken; report it and ask which way |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- Two parsers for one format do not merely duplicate work: they can disagree
  about a directive's MEANING, and each one is self-consistent, so neither can
  see the disagreement. `not:contains=` was negative in one and positive in the
  other, and no test in either suite could tell.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Keep two parsers, converge the vocabulary | merge into `record_parse.go` | the assertion SCOPE genuinely differs: per-command here, file-level there. Merging would change what every `test/parse` file asserts |
| Delete rather than alias | alias `regex=` to `pattern=` | `ai/rules/no-layering.md`; the alias is what let the dialect persist |

## Known Limitations
- `splitOnKeyBoundary` still folds a key whose lead byte is not a letter into the
  previous value, so a second assertion can vanish before any refusal sees it.
  That is a separate row in `plan/journal/silent-fall-through.md`, with no live
  instance in the tree, and it needs a rule the format does not yet state.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
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

**IN FLIGHT.** `internal/test/runner/parsing.go` is rewritten in the working tree
and `internal/test/runner/parsing_test.go` is new and untracked. Neither is
committed, so no SHA can be cited. Meanwhile the accept-only ratchet parses
`test/parse` with the GENERIC parser, so the three files carrying `regex=` and
`not=` are unparseable to it and it names them.

| Item | State |
|------|-------|
| Parser rewrite | in the working tree, UNCOMMITTED |
| PROVEN | nothing yet. The rewrite has its own tests, and no discrimination walk has forced the default arm red |
| ASSERTED, not proven | AC-1 through AC-4, from the rewritten dispatcher; AC-5, AC-6 and AC-7 depend on the migration, which is not finished |
| Remains | (1) the 30-line migration in `test/parse/*.ci`; (2) diagnosing every red the 26 now-live assertions produce, including `config-dump-masks-bcrypt`; (3) the accept-only baseline reconciliation; (4) the documentation of the per-command scope; (5) landing it; (6) closure sections |
