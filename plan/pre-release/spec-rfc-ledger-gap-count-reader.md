# Spec: the RFC ledger reads a gap count only when it is spelled

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`checkGapCountAgreement` compares the gap count a summary CLAIMS against the
`{gap}` annotations its own checklist carries. It reads the claim only when it is
a SPELLED number sitting immediately before MUST or SHALL. Six `Support
remaining` cells write the count in digits, and the check skips every one of
them, including the two largest claims on the ledger.

The work, in the deferring spec's own words: give `checkGapCountAgreement` a
reader for a gap count written in DIGITS, or for one separated from MUST by
intervening words.

`rfc-ledger-single-declaration` deferred it on 2026-09-01 and measured it on
2026-09-03. The reason and the measurement, kept whole because the shard is being
deleted:

Out of scope and unchanged by that spec: the limit predates it, the generated
page states it in its own header, and widening the reader changes which rows the
check judges, which is a ledger decision rather than a migration one. That spec
moved where the `Remaining` cell is authored and did not touch how it is read.

**MEASURED 2026-09-03, and widening the regex is a TRAP: `gapCountRE` must not
simply gain `[0-9]+`.** The population is real: six `Support remaining` cells
write the count in digits and the check skips every one of them, including the
two largest claims on the ledger, rfc9012's "51 MUST-level gaps annotated" and
rfc9830's "20". But the cells are PROSE, and three of the six would go falsely red
under a naive widening. rfc6514 reads "declares all 133 MUST-level obligations:
132 have no producer in Ze", so the pattern takes 133 as the gap claim; another
cell reads "All 5 MUST-level requirements implemented and extracted", where 5 is
the opposite of a gap. The spelled-number restriction is not an oversight, it is a
smaller surface for the same hazard.

**The real answer is the one `ai/rules/principles.md` names:** a count parsed out
of English cannot be checked against a checklist, and the number should be
DERIVED from the `{gap}` annotations and rendered into the cell rather than
written by hand beside them. That is a ledger design change on a gated surface,
and it is the owner's to authorise.

So this spec carries a question to the owner before it carries a design. Widening
the reader is the smaller change and keeps the hand-written cell, with the three
false-red cells above as the evidence that the parse cannot be made reliable.
Deriving the number removes the parse and the disagreement together, and changes
what an author writes in the `Remaining` cell.

The current reader's own refusal text names its limit, so the state is disclosed
rather than hidden: "Only a spelled number sitting immediately before MUST or
SHALL is read as a gap count; a digit count, or a number further from the
keyword, is outside this check."

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the ratchets, and what `./le rfc check` refuses
  → Constraint: [fill at design time]
- [ ] `ai/rules/principles.md` - every fact declared once, every other surface derived from it
  → Constraint: a hand-written count beside a registry is a future disagreement with nothing to arbitrate it

**Key insights:** (minimal context to resume after compaction)
- A naive `[0-9]+` widening reds three of the six cells falsely; the measurement is above
- The owner decides between widening the reader and deriving the number

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/rfc/check_status.go` - `gapCountRE` matches a spelled number from `spelledAlternation()` followed by optional whitespace and `MUST` or `SHALL`, case-insensitively. `spelledGapCount` returns the mapped integer and `false` when nothing matches. `checkGapCountAgreement` counts `AnnotationGap` requirements per RFC, then for each ledger row skips the comparison entirely when `spelledGapCount` returns `false`. A row whose count is in digits is never judged
- [ ] `internal/le/rfc/check.go` - `checkGapCountAgreement` runs over `collected.Requirements` and the ledger rows, and its findings are notes rather than errors
- [ ] `internal/le/rfc/render_ledger_test.go` - asserts the reader picks up a gap count from 58 of the ledger's rows, which is the current population and the number this spec moves

**Behavior to preserve:**
- The check stays silent rather than wrong: a cell it cannot read must not be judged
- The 58 rows it reads today must keep being read

**Behavior to change:**
- The six cells written in digits must stop being invisible to the check

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A developer runs `./le rfc check`, or the verification gate runs it, over the tree's `rfc/short/*.md` summaries.
- Format at entry: the `Support remaining` cell of each summary's Meta table, as free prose, and the `{gap}` annotations of the same summary's checklist.

### Transformation Path
1. `Collect` reads the summaries and their requirement annotations
2. `checkGapCountAgreement` (`check_status.go`) counts `AnnotationGap` per RFC stem
3. `spelledGapCount` parses the claim out of the `Remaining` cell
4. A disagreement becomes a finding naming both numbers

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Summary prose ↔ gate | a regular expression over the `Remaining` cell | No |

### Integration Points
- `renderLedger` (`internal/le/rfc/`) - if the number is derived, this is where it would be rendered
- `./le rfc check` - the command whose findings change

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Exactly six cells write the count in digits, and three of them would go falsely red | Measured 2026-09-03 by the deferring spec | The blast radius of a widening is larger than stated | Re-run the measurement over the tree at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A widened pattern reads a total, not a gap count: rfc6514's "all 133 MUST-level obligations" is 133 where the gap count is 132 | The check reds a correct summary | Do not widen the pattern; put the derived-number option to the owner |
| R-2 | Deriving the number changes what an author writes in a gated cell, so every summary's `Remaining` cell moves at once | A large diff over `rfc/short/` | The owner authorises the ledger change before the code |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The public RFC ledger publishes a gap count that disagrees with its own checklist, or the gate reds correct summaries |
| How is it reverted? | Single commit revert; nothing outside the repository depends on it |
| Who else touches this path? | Any spec working `internal/le/rfc/`, and the site session that renders the ledger |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over a summary whose `Remaining` cell writes the count in digits | → | the gap-count reader | [test name, fill at design time] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | rfc9012's cell, claiming 51 MUST-level gaps in digits | The check judges it against the checklist's `{gap}` count |
| AC-2 | rfc6514's cell, reading "declares all 133 MUST-level obligations: 132 have no producer in Ze" | The check does not read 133 as the gap claim |
| AC-3 | A cell reading "All 5 MUST-level requirements implemented and extracted" | The check does not read 5 as a gap claim |
| AC-4 | The 58 rows the reader reads today | Each is still read, with the same verdict |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs the RFC gate before a commit | `./le rfc check` → `checkGapCountAgreement` → findings | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [fill at design time] | `internal/le/rfc/check_test.go` | the reader over each of the six digit cells | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| rows the reader reads | 58 today | 58 | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [fill at design time] | `internal/le/rfc/` selftest | A developer sees the gate judge a digit-written claim | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Scope is tooling, no wire-visible change | |

## Files to Modify
- `internal/le/rfc/check_status.go` - the reader, or its replacement by a derived number
- `internal/le/rfc/check.go` - the caller, if the finding shape changes

## Files to Create
- [fill at design time]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | |
| YANG validation constraints | | |
| YANG custom validators | | |
| CLI commands/flags | | `./le rfc check` output |
| CLI grammar (keyword before value) | | |
| Editor autocomplete | | |
| Functional test for new RPC/API | | |
| Pipe completeness | | |
| Env var registration | | |
| Doctor check for runtime dependencies | | |
| Prometheus counters/metrics | | |
| BGP family surface (new SAFI / capability / attribute) | | N-A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | |
| 2 | Config syntax changed? | | |
| 3 | CLI command added/changed? | | |
| 4 | API/RPC added/changed? | | |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | | `docs/features/rfc-status.md` is generated from the Meta rows this check reads |
| 10 | Test infrastructure changed? | | `docs/contributing/rfc-conformance-gates.md` |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED: `./le spec citation anchors spec plan/pre-release/spec-rfc-ledger-gap-count-reader.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | | the generated page states the reader's limit in its own header |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- put the two options to the owner with the measurement, then wire the chosen one behind a failing test
   - Tests: [wiring test names]
   - Files: `check_status.go`
   - Verify: the six digit cells reach the check; the wiring test fails because none is judged yet
2. **Phase: [name]** -- [fill at design time]

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | Neither rfc6514's 133 nor the "All 5 implemented" cell is read as a gap claim |
| Rule: `ai/rules/principles.md` | If the number stays hand-written, say why deriving it was rejected |
| Rule: `ai/rules/evidence.md` | The check must not read an unreadable cell as agreement |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The population the reader reads | the assertion in `render_ledger_test.go`, updated with its rationale |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The cell is authored prose; a parse that cannot be sure must stay silent rather than guess |

### Failure Routing
| Failure | Route To |
|---------|----------|
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [fill at closure]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
