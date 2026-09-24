# Spec: RFC evidence strength 4 -- an independent second walk of the prose-register extractions

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | `plan/pre-release/spec-rfc-evidence-strength-0-umbrella.md` (decisions D-5, D-6) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`./le rfc extraction-status` reports 174 enrolled stems, all signed: 98 in the `rfc2119`
register, 73 in `prose`, 3 in `manual-walk`. A `prose` register means the document states
its gated obligations without capitalised RFC 2119 keywords, so the extractor's
keyword-visible sites cannot bound the requirement list. `ai/rules/rfc-compliance.md`
says a model-produced requirement list is a claim, and that two arithmetics must run:
forward (every normative site maps to a row or is excluded with a reason) and reverse
(every gated row maps to a real sentence of the RFC).

The reverse arithmetic is enforced only structurally today. `signoff.go` refuses a gated
row that no site maps to and that no section lists in `unsourced-ids`. An `unsourced-ids`
entry needs no quote: `sectionFromDecision` (`extraction_classify.go`) keeps the
sentence in the section's free-text `reason`, and nothing checks it against the RFC. So
a fabricated row passes the reverse arithmetic by being listed as unsourced.

Measured 2026-09-24 over the enrolled walks the ledger lists: the prose-register walks
hold 731 `unsourced-ids` entries in 71 of 72 stems, against 1130 mapped and 887 excluded
sites. 44 of those walks record the reviewer `claude`, and every other one names an
agent phase. The largest: rfc9830 (66 unsourced), rfc2181 (41), rfc9568 (39),
rfc8669 (21), rfc7858 (21).

This child delivers:

1. A quoted form for every unsourced requirement, checked verbatim against
   `rfc/full/<stem>.txt` inside the section that lists it.
2. A BLIND second walk for each of the 73 prose stems: a context that did not author the
   first walk, and that is not given it, classifies the derived skeleton and sources each
   gated row on its own.
3. A comparison tool that measures the agreement between the two walks, and a recorded
   adjudication of every disagreement.
4. A published measure: second-walked stems, agreement rate, and rows found without a
   source sentence.

The 794 unsourced entries in rfc2119-register walks are D-6 and are outside this child.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` "The extraction sign-off"
  → Constraint: only dispositions are authored; sites, sections, quotes and the register are DERIVED at check time
  → Constraint: `extraction-classify` transcribes a walk and performs none; a decision it applies names one site or one section
  → Constraint: a first sign-off is reviewed, not ratcheted; a changed exclusion count on a signed stem needs a `resign-reason` and a new `signed-off` date (`checkExtractionRatchet`)
- [ ] `rfc/extraction/README.md` - the artifact contract and "Applying a walk"
  → Constraint: an unknown key is refused (`artifact.go` closed key set); a new field changes the parser and the README together
- [ ] `ai/rules/rfc-compliance.md` - the two arithmetics; the published-figure correction rule
  → Constraint: a published figure that proves wrong is corrected on the surface that carried it, naming what was measured
- [ ] `ai/rules/planning.md` - independence is a property of the context
  → Decision: the second walker is a fresh agent context that receives the skeleton and the summary, never the first artifact's decisions
- [ ] `docs/architecture/core-design.md` - declared by `signoff.go`, `artifact.go`, `extraction_classify.go`
  → Constraint: the new command sits under `./le rfc`

**Key insights:**
- A quote check turns "read from indicative prose" from a claim into a checkable fact, and it catches the fabrication class `ai/rules/rfc-compliance.md` measured (RFC 8571 with four MUSTs and no keyword; RFC 7611 rows naming terms absent from the document)
- `checkRetiredRequirements` refuses a deleted id, so a row found fabricated needs D-5

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/signoff.go` - reverse arithmetic: gated row mapped or listed in `unsourced-ids`; an `unsourced-ids` id must exist in the summary
- [ ] `internal/le/rfc/artifact.go` - `unsourced-ids` is a string list; closed key set
- [ ] `internal/le/rfc/extraction_classify.go` - `sectionFromDecision` copies `UnsourcedIDs` and the free-text reason
- [ ] `internal/le/rfc/extraction_create.go` - the skeleton; carries forward authored decisions for an unchanged locator
- [ ] `internal/le/rfc/check_extraction.go` - `checkExtractionRatchet`

**Behavior to preserve:**
- a first walk's structure, the derived counts, the ratchet and the register derivation
- `extraction-create` still carries forward decisions for the FIRST walk; the second walk needs a skeleton without them (AC-3)

**Behavior to change:**
- an unsourced requirement carries a quote the check verifies
- a second walk is recorded and published

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc extraction-create stem <stem> blind` writes an unclassified skeleton to scratch
- the second walker writes a decisions file; `./le rfc extraction-compare stem <stem> decisions <path>` compares it with `rfc/extraction/<stem>.json`
- the adjudicated result is applied with `./le rfc extraction-classify`

### Transformation Path
1. skeleton without carried-forward decisions
2. second walker's decisions, including a quote for each unsourced row
3. comparison report: agreements and disagreements per site and per requirement
4. adjudication, written into the artifact's second-walk record

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| first artifact and second walker | the second walker's brief omits the artifact | No |
| artifact and RFC text | verbatim quote check over `rfc/full/<stem>.txt` | No |

### Integration Points
- `signoff.go` gains the quote check; `check.go` prints the second-walk figures; `sections.go` renders them on "Extraction sign-off"

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the artifact is still written only by `extraction-classify` or `extraction-create` |
| No unintended coupling (components stay isolated) | Yes | `internal/le/rfc` |
| No duplicated functionality (extends existing, does not recreate) | Yes | the compare tool reads the same derived inventory as `signoff.go` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling |
| Registration over hardcoding, outbound | Yes | `extraction-compare` registers in the existing `rfc` action table (`actions.go`) |
| Registration over hardcoding, inbound | Yes | no stem list; the population is every artifact whose derived register is `prose` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Most unsourced rows can be tied to one verbatim sentence of their section | the section reason field is required to carry it | many rows need D-5 | the quote check run over the pilot stems rfc2181 and rfc9830 | unvalidated |
| A-2 | A fresh agent context without the first artifact is independent enough | `ai/rules/planning.md` defines independence by context | the second walk repeats the first walker's errors | the owner decides whether one stem per tranche is walked by a person (umbrella D-8) | unvalidated |
| A-3 | Verbatim match after whitespace collapse is robust to the RFC text's line wrapping and page headers | `rfc/full/*.txt` carries form feeds and page footers | valid quotes fail | a unit test with a quote spanning a page break | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The quote check reds every signed stem at once | `./le rfc check` fails over the whole corpus when the rule lands | change-scoped: a stem must meet it when its artifact changes; the backlog is published |
| R-2 | Second walks disagree mostly on wording, not on substance | agreement below 50% with no fabricated row | the compare tool keys on disposition and target id, never on free text |
| R-3 | A second walk re-signs a stem another spec is walking | `spec-rfcgate-6` lists the stem | check its stem list before each tranche |
| R-4 | A fabricated row found here has passing tests and a proof record | the row is gated and proven | D-5; the proof records die with the row's retraction, and the retraction is published on the ledger |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the public ledger's bound; a false refusal blocks every session's verify |
| How is it reverted? | a single commit revert per phase |
| Who else touches this path? | `spec-rfcgate-6-supported-extraction-signoff`, the extraction tooling |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over a fixture whose changed artifact lists an unsourced row with a quote absent from the RFC | → | quote check in `signoff.go` | `TestSignoffRefusesUnsourcedQuoteAbsentFromSection` in `internal/le/rfc/extraction_test.go` |
| `./le rfc extraction-compare stem <stem> decisions <path>` | → | comparison | `TestExtractionCompareCountsAgreementPerSiteAndRow` |
| `./le rfc extraction-create stem <stem> blind` | → | skeleton without carried decisions | `TestExtractionCreateBlindCarriesNoDecision` in `internal/le/rfc/extraction_create_test.go` |
| `./le rfc check` summary | → | second-walk figures | `TestCheckPrintsSecondWalkFigures` in `internal/le/rfc/check_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a changed artifact names an unsourced row whose quote is absent from its section of `rfc/full/<stem>.txt` | `./le rfc check` refuses it, naming the stem, the section, the row and the first 70 characters of the quote |
| AC-2 | an unchanged artifact with bare `unsourced-ids` | still passes; `./le rfc check` counts it in an "unquoted unsourced" backlog figure |
| AC-3 | `./le rfc extraction-create stem <stem> blind` on a signed stem | writes a skeleton to scratch with every site and section unclassified, even where a decision would carry forward |
| AC-4 | `./le rfc extraction-compare` on two walks | prints, per stem: sites where both agree, sites where the disposition differs, sites where the mapped id differs, gated rows sourced by only one walk, and rows neither walk could source |
| AC-5 | a gated row neither walk can tie to a sentence | reported as "no source sentence"; handled under D-5 in the same commit as the second-walk record |
| AC-6 | each of the 73 prose stems | its artifact carries a second-walk record: walker, date, agreement counts, and one adjudication line for each disagreement; every unsourced row carries a verified quote |
| AC-7 | `./le rfc check` and `ai/RFC-REQUIREMENTS.md` "Extraction sign-off" | publish: second-walked stems of the prose register, agreement rate, rows retracted or rewritten, unquoted unsourced backlog |
| AC-8 | a second-walk record whose walker equals the first walk's reviewer field | refused, as the same named context |
| AC-9 | a published figure the second walks change | corrected on `ai/RFC-REQUIREMENTS.md` and on `docs/features/rfc-status.md` by regeneration, and the change names what was measured |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSignoffRefusesUnsourcedQuoteAbsentFromSection` | `internal/le/rfc/extraction_test.go` | AC-1 | |
| `TestSignoffAcceptsQuoteAcrossPageBreak` | same | A-3 | |
| `TestSignoffGrandfathersBareUnsourcedIDs` | same | AC-2 | |
| `TestExtractionCreateBlindCarriesNoDecision` | `internal/le/rfc/extraction_create_test.go` | AC-3 | |
| `TestExtractionCompareCountsAgreementPerSiteAndRow` | `internal/le/rfc/extraction_test.go` | AC-4, AC-5 | |
| `TestSecondWalkRecordRefusesSameWalker` | same | AC-8 | |
| `TestCheckPrintsSecondWalkFigures` | `internal/le/rfc/check_test.go` | AC-7 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| quote length | 24 characters and up (the level-correction minimum) | 24 | 23 | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le rfc selftest` rows for the quote check and the compare tool | `internal/le/rfc/selftest_state.go` | an author lists an unsourced row with an invented sentence and the gate refuses it | |

## Files to Modify
- `internal/le/rfc/artifact.go` - quoted unsourced entries; the second-walk record; closed keys
- `internal/le/rfc/signoff.go` - quote check; walker check
- `internal/le/rfc/extraction_create.go` - `blind` keyword
- `internal/le/rfc/extraction_classify.go` - carries quotes and the second-walk record
- `internal/le/rfc/check.go`, `internal/le/rfc/sections.go` - published figures
- `internal/le/rfc/actions.go` - `extraction-compare` action; `blind` keyword
- `internal/le/rfc/selftest_state.go` - fixtures
- `rfc/extraction/<stem>.json` - 73 prose stems, one tranche per commit
- `rfc/short/<stem>.md`, `rfc/corrections/<stem>.md` - rows rewritten or retracted under D-5
- `rfc/extraction/README.md`, `docs/contributing/rfc-conformance-gates.md` - the quote rule, the blind walk, the compare tool
- `docs/architecture/core-design.md` - declared; checked
- `ai/RFC-REQUIREMENTS.md`, `docs/features/rfc-status.md` - regenerated

## Files to Create
- `internal/le/rfc/extraction_compare.go` - the comparison, beside `extraction_classify.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling |
| YANG validation constraints | N-A | none |
| YANG custom validators | N-A | none |
| CLI commands/flags | Yes | `./le rfc extraction-compare`, `blind` keyword; `internal/le/rfc/actions.go` |
| CLI grammar (keyword before value) | Yes | `stem <stem> decisions <path>`, per `ai/rules/cli.md` |
| Editor autocomplete | N-A | le action |
| Functional test for new RPC/API | N-A | `./le rfc selftest` fixtures |
| Pipe completeness | N-A | le action output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | none |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | ze CLI untouched |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | summaries changed under D-5; regenerated pages |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/rfc-conformance-gates.md`, `rfc/extraction/README.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/INDEX.md` dev-tools row for `extraction-compare` (`ai/rules/repo-maintenance.md`) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/pre-release/spec-rfc-evidence-strength-4-prose-second-walk.md` at implementation start |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the command table in `docs/contributing/rfc-conformance-gates.md` "The extraction sign-off" |

## Implementation Steps

1. **Phase: Wiring** -- the quote check fixture, red
2. **Phase: Quote check** -- AC-1, AC-2, A-3
3. **Phase: Blind skeleton and compare** -- AC-3, AC-4, AC-5, AC-8
4. **Phase: Publish** -- AC-7
5. **Phase: Pilot** -- rfc2181 and rfc9830 (107 unsourced entries); measure the agreement rate and the no-source rate; write both into Design Insights and the umbrella
6. **Phase: Tranches** -- the other 71 stems, largest unsourced count first, a tranche per commit; each walk by a fresh agent given the skeleton, the summary and `rfc/full/<stem>.txt`, never the first artifact (AC-6, AC-9)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | the quote is searched inside the section that lists the row, not the whole document |
| Correctness | the compare tool reports a row that NEITHER walk sourced, not only disagreements |
| Rule: `ai/rules/principles.md` | a stem whose source text is missing is an error, never a pass with zero sites |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| 73 second walks | `./le rfc check` second-walk figure equals the prose count from `./le rfc extraction-status` |
| No unquoted unsourced row in a prose stem | the "unquoted unsourced" figure for the prose register reads 0 |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Independence leak | the second walker's brief names no path under `rfc/extraction/` for its stem |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A row without a source sentence | D-5 |
| A second walk finds an obligation the summary missed | add the row in the same commit; it is gated at once, since the stem is enrolled |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The reverse arithmetic already exists as structure. What it lacks is a checkable tie from an unsourced row to text, and a quote is the smallest such tie.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Blind second walk plus comparison | A reviewer reading the first walk | a reviewer who reads the first walk is anchored by it; the comparison makes agreement a number |
| Change-scoped quote rule | Refuse every unquoted row at once | 1525 unquoted entries across the corpus would red the tree; the second walks drain the prose share |

## Known Limitations

- The 794 unsourced entries in rfc2119-register walks (D-6). If the owner rules them in, they become their own spec in `plan/pre-release/`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: tooling; `./le rfc selftest` fixtures instead)
- [ ] Interop tests for protocol features (N-A: no protocol behavior)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** code + tests + artifacts + docs + edited spec
- [ ] **Commit B:** `remove plan/pre-release/spec-rfc-evidence-strength-4-prose-second-walk.md` only
