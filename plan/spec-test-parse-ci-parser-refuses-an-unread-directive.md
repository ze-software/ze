# Spec: the test/parse .ci parser refuses a directive it does not read

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `plan/spec-ci-parser-refuses-an-assertion-key-it-does-not-read.md` (landed at `8c7f0a5bf2`, which set the vocabulary this one adopts) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-19 |

<!-- Backfilled after work commissioned from a journal row. The parser rewrite
     and migration are committed; acceptance and discrimination evidence remains
     to be reconciled before closure. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The `test/parse` suite has its own `.ci` parser. Before the September 6 repair,
its line loop was a chain of prefix matches with no default, so an unmatched
directive was dropped and the file still parsed. Its vocabulary also disagreed
with the generic parser: `regex=` meant `pattern=`, and
`expect=stdout:not:contains=` meant `reject=stdout:contains=`.

The retired `not:contains=` form also meant the opposite in the two parsers:
the generic splitter cut at `:contains=` and dropped the bare `not`, so a
negative assertion became a positive one.

Measured on 2026-09-06 over `test/parse/*.ci` and recorded in
`plan/journal/silent-fall-through.md`: 26 lines assert nothing. One of them,
`test/parse/config-dump-masks-bcrypt.ci`, is the whole proof that a bcrypt hash
is masked. The counts are in the journal row and are not restated here.

Goal: one vocabulary for one format, and a directive no arm reads fails the file.

The rewrite and migration landed in `d1e6e2d200` on September 6, with further
directive/key refusal work in `0897c2b951` on September 7. Current
`ciDirectives` dispatches the shared spellings and `ciFileParser.line` refuses
unknown directives. `checkExpectations` evaluates the recorded assertions per
command. This is an implemented parser with outstanding evidence, not an
uncommitted rewrite.

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
- The parse suite's assertion scope is per-command: each assertion reads the
  stdout or stderr of the preceding `cmd=`. The generic runner now also binds
  stdout and containment assertions to that command through `assertionTarget`;
  its stderr regex logging assertions remain file-level. Preserve the parse
  suite's existing semantics rather than assuming the old all-file contrast.
- Every `test/parse` file that asserts something today still asserts it.

**Implemented changes and remaining proof:**
- The dispatcher refuses an unread directive and quotes its source line.
- The retired `regex=` and `not:contains=` spellings have been migrated to the shared vocabulary, with no aliases.
- The original 30-line migration and 26 now-live assertions still need an evidence account showing that each intended assertion is exercised.
- Every red exposed by those assertions must be diagnosed at its producer. Migration and source inspection alone do not satisfy that obligation.

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
| `.ci` author ↔ the parse suite | the directive text | implementation committed: the dispatcher refuses unmatched directives and quotes the source line; discrimination evidence still needs reconciliation |
| parse suite ↔ generic runner | two parsers, one vocabulary | both now bind stdout/containment assertions to the preceding command; parse-suite stderr regex and legacy no-command negative cases retain their documented differences |
| `test/parse` ↔ the accept-only ratchet | the ratchet parses these files with the generic parser | `d1e6e2d200` records that the ratchet named no parse-suite file; no current run is claimed |

### Integration Points
- `internal/test/runner/accept_only.go` - the ratchet that reads the same files.
- `test/.accept-only-baseline` - which must be reconciled once the files parse.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | every line goes through the dispatcher, including the unmatched one |
| No unintended coupling | Yes | confined to `internal/test/runner` |
| No duplicated functionality | Partly | the vocabulary converges; the two parsers remain. Generic command scoping has since removed the historical all-file distinction, while stderr regex and legacy negative-test handling still differ |
| Zero-copy preserved where applicable | N-A | test tooling |
| Registration over hardcoding | No | the arms are a dispatch chain; the default is what makes an unlisted directive loud rather than silent |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | All 26 vacuous lines can be migrated to a live assertion | each was read during the measurement | a line has no equivalent in the shared vocabulary | the migration | UNVALIDATED |
| A-2 | The parse suite's existing per-command semantics must be preserved | its files depend on command-local assertions; the generic runner has since adopted command scoping too | the original argument against sharing parser machinery needs reassessment, without changing consumer meaning | compare the current documented dialects and their assertion producers | original all-file contrast superseded; semantic-preservation proof still owed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A now-live assertion turns red because the PRODUCT is wrong | the assertion fails after migration | that red is the finding; fix the product, never the assertion (`ai/rules/pre-release.md`) |
| R-2 | The accept-only baseline drifts while the files are unparseable to the ratchet | the ratchet names the three files | reconcile `test/.accept-only-baseline` in the same change |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the parse suite stops running, or keeps passing over assertions that check nothing |
| How is it reverted? | revert the parser and its consumer migrations together; the rewrite is committed in `d1e6e2d200` |
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
| the parser's default-arm refusal and each migrated key | `internal/test/runner/parsing_test.go` | AC-1, AC-2, AC-3, AC-4 | committed in `d1e6e2d200`; current execution and discrimination evidence not assessed here |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| assertions per `cmd=` | 0..n | n | 0, which is legal: a command may only be run | N/A |
| exit code | 0..255 | 255 | negative, refused as an invalid exit code | 256, refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the parse suite itself | `test/parse/*.ci` | an author writes a `test/parse` file and its assertions run | migration committed; corpus-wide assertion accounting and red diagnosis remain evidence obligations |
| `config-dump-masks-bcrypt` | `test/parse/config-dump-masks-bcrypt.ci` | `ze config dump` masks a bcrypt hash | live `reject=stdout:contains=UlwuiuH82Unfsq` since `8c7f0a5bf2`; AC-6 discrimination still owed |

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

## Original Implementation Steps (committed work; evidence reconciliation remains)

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
| Keep the parse suite's semantics while converging vocabulary | the original alternative was merging into `record_parse.go` | the September 6 design preserved per-command scope against a then-file-level generic runner. That historical distinction is now narrower: generic stdout/containment assertions are command-scoped too. This spec authorises no additional parser merger |
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

The uncommitted snapshot recorded when this spec was written is superseded.
`d1e6e2d200` committed the dispatcher, its tests, vocabulary migrations and the
per-command documentation. Its commit account reports that the accept-only
ratchet named no parse-suite file, and records the reds it exposed rather than
claiming a green corpus. `0897c2b951` subsequently extended directive/key
refusal. The bcrypt rejection was repaired in `8c7f0a5bf2`, before the
parse-specific rewrite.

Current source confirms the refusal and assertion-consumer paths, and the
retired stdout `regex=`/`not:contains=`/`not=` spellings were not found in the
current `test/parse/*.ci` source search. This is source and history evidence,
not a new parser, corpus or discrimination run.

| Item | State |
|------|-------|
| Parser rewrite and vocabulary | committed; `ciDirectives`, `ciFileParser.line` and `checkExpectations` remain the producers |
| Documentation | the shared vocabulary and per-command scope are described in `docs/architecture/testing/ci-format.md`, "The parse suite reads its own dialect" |
| Acceptance evidence | reconcile AC-1 through AC-7 individually; do not equate a migrated spelling with a discriminating assertion |
| Remains | account for every originally identified migration and exposed red; prove the bcrypt rejection discriminates; recover or repeat corpus and ratchet evidence and reconcile the baseline if needed; complete the closure sections only after those obligations are met |
