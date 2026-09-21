# Spec: rfc-implementation-classification

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make "who implements this document" a fact each summary declares and a gate
reads, so the public ledger separates Ze's own conformance from a dependency's.
The owner ruled on 2026-09-21 that the ledger answers one question, whether ZE's
Go code does what the RFC says, and that a document Ze writes no Go for leaves
the count and names what implements it instead. The rule is recorded in
`ai/rules/rfc-compliance.md` and the procedure in
`docs/contributing/rfc-conformance-gates.md`, "Who implements the document".
Neither is enforced today, which is what this spec owns.

The vocabulary is four kinds and every summary declares exactly one: `ze` for a
document Ze implements in Go, `mixed` where Ze implements part and another layer
performs the rest, `third-party` where a layer under or beside Ze performs it
and Ze holds no Go code, `foundation` where the document defines or registers and
obliges no implementer. A `third-party` or `mixed` value names the implementing
component and its mechanism, and "the kernel" alone is refused.

`ParseMeta` (`internal/le/rfc/meta.go`) reads the `## Meta` table into the facts
this package needs, refusing rather than defaulting at every field, and
`refuseNearMiss` in the same file reds a label that names a known fact in a
spelling nothing reads. A label the reader does not know at all is accepted and
ignored, so an `Implementation` row added today would be a hand-kept fact no
surface derives from, which `ai/rules/principles.md` names as a future
disagreement with nothing to arbitrate it. The closed set therefore belongs
beside `enrolmentKinds` in that file, declared once, with `DispositionKinds`
(`internal/le/rfc/artifact.go`) as the shape to follow.

The count this changes is the one `./le rfc check` publishes and
`internal/le/site/rfccompliance.go` renders. A `third-party` or `foundation`
document leaves the gated population; a `mixed` document keeps the requirements
Ze's Go answers and excludes the rest. The requirement-level `{lower-layer}`
annotation is unchanged and stays counted, per the 2026-08-31 ruling: it governs
one obligation Ze meets by configuring a layer below, while this governs a
document Ze does not write Go for at all. A document Ze merely configures is
`mixed`, so the two never decide the same row.

Leaving the count never means proving nothing. Where Ze can observe the
behaviour, the requirement still carries a test asserting what Ze produced: the
selector, the security association, the socket option, the kernel counter. The
design must say how an excluded row records that it owes such a test, and how
the page shows an excluded row that has one.

Before implementation, complete the design and acceptance criteria using
`plan/TEMPLATE.md`. The design owes four decisions: the Meta label and its
refusals, where the excluded set leaves the arithmetic without flattering any
published share, what the rendered page calls each kind, and how the 195
existing summaries are classified without a hand-kept list. The 2026-09-02 owner
ruling against removing `{not-applicable}` from a denominator is the precedent
to read first: a subtraction that raises every share above it was rejected then,
and this exclusion must not reintroduce it by another name.

The classification data itself is separate work and this spec does not own it.
This skeleton changes no RFC ledger, tag, support claim, or publication status.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the artifacts, the ratchets, and the section this spec enforces
  → Constraint: a gap is an ISSUE and an exclusion is a DECISION, so an excluded document states which decision put it out of reach
- [ ] `ai/rules/rfc-compliance.md` - the owner directives that govern the count
  → Decision: the 2026-08-31 whole-stack ruling stays; this spec governs documents, not requirements

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4302.md` - the sixteen `{lower-layer}` rows that show where a document splits
  → Constraint: Ze installs the AH security association and Linux XFRM builds the packet, which is what `mixed` has to express

**Key insights:**
- The vocabulary is four kinds and the reader refuses anything else
- An unnamed implementer reads on the public page as work nobody owns

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/meta.go` - `ParseMeta` reads the Meta table, refusing rather than defaulting; `refuseNearMiss` reds a near-miss label, while a label it does not know at all is accepted and ignored
- [ ] `internal/le/rfc/ledger.go` - `enrolmentKinds` is the closed set of what a summary declares about its gating, declared once
- [ ] `internal/le/rfc/artifact.go` - `DispositionKinds` is the shape a second closed set follows
- [ ] `internal/le/site/rfccompliance.go` - the published page renders the counts this would change

**Behavior to preserve:**
- Every published share stays over the gated population; no annotation subtracts from a denominator (owner ruling, 2026-09-02)
- `{lower-layer}` keeps its requirement-level meaning and stays counted

**Behavior to change:**
- A summary declares who implements it, and `third-party` or `foundation` leaves the gated population

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The summary's `## Meta` table, one row per document, authored by the reviewer
- Format at entry: a label and one of four words, plus the implementer where the word is not `ze`

### Transformation Path
1. `ParseMeta` (`internal/le/rfc/meta.go`) reads the row and refuses a value outside the closed set
2. `Collect` carries the kind onto each requirement's population decision
3. `NewRenderInput` hands it to the published page and to `./le rfc check`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| summary text ↔ ledger | the Meta reader, refusing rather than defaulting | No |
| ledger ↔ published page | `RenderInput`, which carries no second copy of the kind | No |

### Integration Points
- `enrolmentKinds` (`internal/le/rfc/meta.go`) - the shape the second closed set follows, declared once
- `DispositionKinds` (`internal/le/rfc/artifact.go`) - the accessor pattern the page reads

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

| Risk | Basis | Validation |
|------|-------|------------|
| An exclusion flatters every share above it | the owner rejected exactly that for `{not-applicable}` on 2026-09-02 | the design states where the excluded set leaves the arithmetic, and a test holds the shares over a corpus with and without one |
| A reviewer reaches for `third-party` to make a red go away | `binds-another-role` grew to 915 sites under the same pressure | the implementer must be named and checkable, so the claim costs a fact rather than a sentence |

## Blast Radius

`./le rfc check`, `ai/RFC-REQUIREMENTS.md`, `docs/features/rfc-status.md` and the
published compliance pages all read the population this changes. Every summary
gains one Meta row.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a summary declaring `third-party` with no implementer named | → | the Meta reader's refusal | `TestThirdPartyWithoutAnImplementerIsRefused` |
| `./le rfc check` over a corpus holding one `foundation` summary | → | the gated population | `TestFoundationLeavesTheGatedPopulation` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a summary with no implementation row | the reader refuses it and names the four kinds |
| AC-2 | a summary declaring `third-party` naming only "the kernel" | the reader refuses it and asks for the component and the mechanism |
| AC-3 | a summary declaring `foundation` | its requirements leave the gated population and the page says which kind removed them |
| AC-4 | a summary declaring `mixed` | the requirements Ze's Go answers stay gated and the rest are named with their implementer |
| AC-5 | an excluded requirement Ze can observe | it carries a test over what Ze installs, and the page shows it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryKindTheVocabularyDeclaresIsReadable` | `internal/le/rfc/meta_test.go` | the closed set and the reader agree, over the real table | |
| `TestThirdPartyWithoutAnImplementerIsRefused` | `internal/le/rfc/meta_test.go` | "the kernel" alone does not pass | |
| `TestMixedKeepsTheRequirementsZeAnswers` | `internal/le/rfc/check_test.go` | a split document loses only the part Ze writes no Go for | |
| `TestFoundationLeavesTheGatedPopulation` | `internal/le/rfc/check_test.go` | an excluded kind leaves the denominator where the design says it does | |
| `TestExcludedRowStillDeclaresTheTestItOwes` | `internal/le/rfc/check_test.go` | out of the count is not out of test | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc-implementation-kind` | `test/plugin/*.ci` | a reader asks the published ledger who implements a document and gets the kind and the implementer | |

## Files to Modify

- `internal/le/rfc/meta.go` - the closed set, the Meta label, and the two refusals
- `internal/le/rfc/check.go` - the gated population, and the line naming the excluded set
- `internal/le/site/rfccompliance.go` - the published split and its wording
- `docs/contributing/rfc-conformance-gates.md` - replace the "not enforced today" paragraph with the gate's own refusals

## Files to Create

- none beyond tests

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the Meta label reaches the reader
   - Tests: `TestEveryKindTheVocabularyDeclaresIsReadable`, `TestThirdPartyWithoutAnImplementerIsRefused`
   - Files: `internal/le/rfc/meta.go`
   - Verify: a summary declaring a kind is read, and one declaring nothing is refused
2. **Phase: the population** -- an excluded kind leaves the gated set
   - Tests: `TestFoundationLeavesTheGatedPopulation`, `TestMixedKeepsTheRequirementsZeAnswers`
   - Files: `internal/le/rfc/check.go`
3. **Phase: the page** -- the split is published with its implementer
   - Tests: `rfc-implementation-kind`
   - Files: `internal/le/site/rfccompliance.go`
4. **Phase: the corpus** -- every summary declares its kind, from the classification pass rather than from a list

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Tests written
- [ ] Tests FAIL before the implementation
- [ ] Tests PASS after it
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Architectural Verification table filled, including registration over hardcoding

## Implementation Summary

### What Was Implemented
- `| Implementation |` and `| Implementation reason |` become parsed Meta facts, with `implementationKinds` as the one closed set and `readImplementation` (`internal/le/rfc/meta.go`) refusing an absent row, a value outside the four kinds, a kind with no reason, and a `third-party` or `mixed` reason naming no component (`namesAnImplementer`)
- `Meta.Enrolled` gates a summary only where `implementationCounts` holds of its kind, so a `third-party` or `foundation` document leaves the population `./le rfc check` counts: 3226 gated MUSTs across 174 enrolled RFCs, from 3322 across 184
- `internal/le/site/rfcledger.go` carries the pair onto each ledger stem, and `rfcImplementationHTML` and `rfcImplementationMirror` publish the split with the stems each kind holds
- All 201 summaries declare a kind, `ai/rules/rfc-compliance.md` and `docs/contributing/rfc-conformance-gates.md` carry the rule, and `ai/skills/ze-rfc.md` carries it into the authoring skill

### Bugs Found/Fixed
- The declined index published the literal word `enrolled` as the reason ten summaries are not enrolled. `Meta.Disposition` read the enrolment cell alone after `Enrolled` became a conjunction of two facts, so `rfcDeclinedIndexHTML` printed "no meaning is recorded for this kind" on the public page and `TestEveryDispositionKindOnTheIndexSaysWhatItMeans` was red. Fixed at the producer: `Meta.Disposition` answers the implementation kind where the enrolment row says `enrolled`, and `dispositionKinds` (`internal/le/rfc/artifact.go`) publishes the sentence `third-party` and `foundation` mean. `plan/journal/field-carries-two-meanings.md` carries the occurrence
- `CountsAgainstZe` was exported with no caller anywhere. It is now `implementationCounts`, the one declaration of the division, read by `Enrolled` and by `Disposition`

### Documentation Updates
- `docs/contributing/rfc-conformance-gates.md`, "Who implements the document": the closing paragraph said the classification is a reviewer's judgement no gate reads, and cited this spec as the owner of building it. Both are now false. It states the four refusals, the gated population, the published section and the disposition, with a `<!-- source: internal/le/rfc/meta.go -- readImplementation, Meta.Enrolled, implementationCounts -->` anchor
- `ai/rules/rfc-compliance.md` and `ai/rules/points/rfc-compliance/directives/name-who-implements-each-rfc.md` carry the directive, committed in `7a7aff6449`

### Deviations from Plan
- Three planned test names were written as two: `TestEveryKindTheVocabularyDeclaresIsReadable` kept its name, `TestThirdPartyWithoutAnImplementerIsRefused` landed as `TestAnImplementationDeclarationIsRefusedWhenItSaysTooLittle` covering all four refusals, and `TestFoundationLeavesTheGatedPopulation` plus `TestMixedKeepsTheRequirementsZeAnswers` landed as one table-driven `TestADocumentZeWritesNoGoForLeavesTheGatedPopulation` over all four kinds
- The planned `.ci` scenario `rfc-implementation-kind` was not written. The published split is asserted by `TestThePublishedPageSaysWhoImplementsEachDocument` (`internal/le/site/rfccompliance_test.go`) over the real corpus, which reaches the same producers a `.ci` would reach through the site build. `le rfc check` and the site renderer carry no daemon path a `.ci` could exercise
- `TestExcludedRowStillDeclaresTheTestItOwes` was not written, because AC-5 was not built. See Work Not Done

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | `Meta.Enrolled` became a conjunction of two facts and `Meta.Disposition` kept reading one of them | a summary can declare `enrolled` and still not be gated, so the disposition has two possible sources | `TestEveryDispositionKindOnTheIndexSaysWhatItMeans` red, and the journal row that recorded it | `Disposition` names whichever fact removed the summary; `plan/journal/field-carries-two-meanings.md` holds the occurrence |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| a summary declares one of four kinds and a reader refuses anything else | Done | `readImplementation`, `implementationKinds` (`internal/le/rfc/meta.go`) | the closed set sits beside `enrolmentKinds`, declared once |
| `third-party` or `mixed` names the implementing component, and "the kernel" alone is refused | Done | `namesAnImplementer`, `vagueImplementerRE` (`internal/le/rfc/meta.go`) | derived from the shape of a name, since the named thing lives outside this repository |
| a `third-party` or `foundation` document leaves the gated population | Done | `Meta.Enrolled`, `implementationCounts` (`internal/le/rfc/meta.go`) | 3226 gated MUSTs across 174 RFCs |
| a `mixed` document keeps the requirements Ze's Go answers | Done | `implementationCounts` (`internal/le/rfc/meta.go`) | `{lower-layer}` keeps its requirement-level meaning and stays counted |
| the page names each kind and its implementer | Done | `rfcImplementationHTML`, `rfcImplementationMirror` (`internal/le/site/rfccompliance.go`) | the section renders before the buckets, so a share is never read before whose work it measures |
| every summary declares its kind, without a hand-kept list | Done | 201 files under `rfc/short/`, commit `7a7aff6449` | each kind carries its own reason, so no list holds the classification |
| an excluded requirement Ze can observe still owes a test | Not done | - | rule only, no gate. See Work Not Done |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestAnImplementationDeclarationIsRefusedWhenItSaysTooLittle` | `readImplementation` names the four kinds in the refusal |
| AC-2 | Done | `TestAnImplementationDeclarationIsRefusedWhenItSaysTooLittle` | `namesAnImplementer` strips "the kernel" and asks for the component |
| AC-3 | Done | `TestADocumentZeWritesNoGoForLeavesTheGatedPopulation`, `TestThePublishedPageSaysWhoImplementsEachDocument` | the page names the kind, its meaning and its summaries |
| AC-4 | Done | `TestADocumentZeWritesNoGoForLeavesTheGatedPopulation` | `mixed` stays gated, and the page names its summaries with their implementer |
| AC-5 | Skipped | - | in-scope and not built; homed in `plan/pre-release/spec-excluded-requirement-owes-an-observable-test.md`, whose run is the owner's decision |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestEveryKindTheVocabularyDeclaresIsReadable` | Done | `internal/le/rfc/meta_implementation_test.go` | over the real Meta table |
| `TestThirdPartyWithoutAnImplementerIsRefused` | Changed | `internal/le/rfc/meta_implementation_test.go` | landed as `TestAnImplementationDeclarationIsRefusedWhenItSaysTooLittle`, covering all four refusals |
| `TestMixedKeepsTheRequirementsZeAnswers` | Changed | `internal/le/rfc/meta_implementation_test.go` | folded into `TestADocumentZeWritesNoGoForLeavesTheGatedPopulation` |
| `TestFoundationLeavesTheGatedPopulation` | Changed | `internal/le/rfc/meta_implementation_test.go` | same table-driven test |
| `TestExcludedRowStillDeclaresTheTestItOwes` | Skipped | - | AC-5, homed in the pre-release spec named above |
| `rfc-implementation-kind` (`.ci`) | Changed | `internal/le/site/rfccompliance_test.go` | landed as `TestThePublishedPageSaysWhoImplementsEachDocument` over the real corpus |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/rfc/meta.go` | Done | the closed set, the two Meta labels, the refusals, the population, the disposition |
| `internal/le/rfc/check.go` | Changed | the population is decided by `Meta.Enrolled`, so `check.go` needed no edit; `check_baseline.go` carries the retired-ledger baseline |
| `internal/le/site/rfccompliance.go` | Done | the published split and its wording |
| `docs/contributing/rfc-conformance-gates.md` | Done | the "not enforced today" paragraph is replaced by the gate's own refusals |

### Audit Summary
- **Total items:** 23
- **Done:** 17
- **Partial:** 0
- **Skipped:** 2 (AC-5 and its test, homed in `plan/pre-release/spec-excluded-requirement-owes-an-observable-test.md`; the owner decides whether that spec runs)
- **Changed:** 4 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| "who implements this document" is a fact each summary declares and a gate reads | functional | `./le rfc check` exits 0 reading it: "3226 gated MUST-level requirement(s) across 174 enrolled RFC(s)", against 3322 across 184 before the change |
| the public ledger separates Ze's own conformance from a dependency's | functional | `TestThePublishedPageSaysWhoImplementsEachDocument` renders the real corpus and reports 14 summaries naming an implementer beside Ze, each named on the page with its kind and its meaning |
| a document Ze writes no Go for leaves the count and names what implements it | unit | `TestADocumentZeWritesNoGoForLeavesTheGatedPopulation` over all four kinds, and `TestAnImplementationDeclarationIsRefusedWhenItSaysTooLittle` for the reason that names nobody |
| no share is taken over a subtracted denominator (owner ruling, 2026-09-02) | unit | the excluded documents leave the population entirely, so their requirements are in no numerator and no denominator; `./le rfc check` prints the population it used, and `internal/le/site` is green including `provenshare_test.go` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| AC-5: an excluded requirement Ze can observe declares the test it owes, a gate refuses one carrying none, and the page shows a proven exclusion | the document-level classification landed without it. The obligation is stated in `ai/rules/rfc-compliance.md` and in `docs/contributing/rfc-conformance-gates.md` and no gate reads it, and the design owes three decisions this spec never took: how a site records observability, what the gate refuses over the existing population, and what the page shows for a proven exclusion | `plan/pre-release/spec-excluded-requirement-owes-an-observable-test.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfc-implementation-classification-d526f23a-b540-4e9d-abf4-8e34ca349a2d.md` |
| `./le spec session review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | wiring and reachability, functional test coverage, documentation drift, removed-behavior audit, logic correctness, project rules cross-check including the `docs/contributing/ze-go-style.md` pass |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | ten summaries reach the declined index carrying the word `enrolled` as their disposition, which the vocabulary does not know, so the public page prints "no meaning is recorded for this kind" and the site package is red | `Meta.Disposition` (`internal/le/rfc/meta.go`), `rfcDeclinedIndexHTML` (`internal/le/site/rfccompliance.go`) | `Disposition` answers the implementation kind where the enrolment row says `enrolled`, and `dispositionKinds` publishes what the two kinds mean |
| 2 | ISSUE | `CountsAgainstZe` is exported and has no caller in any package, which `./le repository check` reports | `internal/le/rfc/meta.go` | replaced by `implementationCounts`, the one declaration of the division, read by `Enrolled` and `Disposition` |
| 3 | ISSUE | the published split had no test: no site test named `rfcImplementationHTML` or its mirror, so the page could stop naming a kind with nothing red | `internal/le/site/rfccompliance.go` | `TestThePublishedPageSaysWhoImplementsEachDocument` over the real corpus |
| 4 | NOTE | `./le repository check` reports `SiteDispositions` with no cross-package non-test caller | `internal/le/rfc/artifact.go` | pre-existing and already decided: `plan/journal/unwired-feature.md`, row 2026-09-02, records that it stays exported because `internal/le/site/rfcdetail_test.go` builds its vocabulary from it |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/rfc/meta_implementation_test.go` | Yes | `git show --stat 7a7aff6449` lists it at 106 lines |
| `plan/pre-release/spec-excluded-requirement-owes-an-observable-test.md` | Yes | written in commit A of this closure |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | a summary with no implementation row is refused, naming the four kinds | `readImplementation` returns a `parseErr` when the row is absent, and `go test ./internal/le/rfc/` is green apart from the fixture seal noted below |
| AC-2 | "the kernel" alone is refused | `namesAnImplementer` strips `vagueImplementerRE` before matching `namedComponentRE`, so a bare gesture leaves nothing to match |
| AC-3 | a `foundation` document leaves the gated population and the page says so | `./le rfc check`: 3226 gated MUSTs across 174 enrolled RFCs; `TestThePublishedPageSaysWhoImplementsEachDocument` logs 14 summaries named with their implementer |
| AC-4 | a `mixed` document keeps the requirements Ze's Go answers | `implementationCounts` holds of `mixed`, asserted for all four kinds by `TestADocumentZeWritesNoGoForLeavesTheGatedPopulation` |
| AC-5 | not built | homed in `plan/pre-release/spec-excluded-requirement-owes-an-observable-test.md` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| a summary declaring `third-party` with no implementer named | none; `internal/le/rfc/meta_implementation_test.go` | Yes: `ParseMeta` is the entry point every reader of `rfc/short/` goes through, and the refusal is asserted over the real Meta table |
| `./le rfc check` over the corpus holding `foundation` summaries | none; the command was run | Yes: it exits 0 and reports the population it now counts |
| the published page | none; `internal/le/site/rfccompliance_test.go` | Yes: `TestThePublishedPageSaysWhoImplementsEachDocument` renders the real ledger through `rfcImplementationHTML` and its mirror |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| R-1, an exclusion flatters every share above it | confirmed prevented | the document leaves the gated population entirely, so its requirements are in no numerator and no denominator, and `./le rfc check` prints the population it used |
| R-2, a reviewer reaches for `third-party` to make a red go away | confirmed costed | `namesAnImplementer` makes the claim cost a checkable fact, and `TestAnImplementationDeclarationIsRefusedWhenItSaysTooLittle` holds the refusal |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/contributing/rfc-conformance-gates.md`, "Who implements the document" | the paragraph now states `readImplementation`'s four refusals and `Meta.Enrolled`, read against `internal/le/rfc/meta.go` | Yes |
| RFC status rows (`docs/features/rfc-status.md`) | generated by `./le rfc index-update`; no `Support status`, `Support coverage` or `Support remaining` row changed in this work | Yes, no update owed |
| CLI reference, config syntax, plugin SDK, wire format | no CLI surface, config option, plugin or wire byte changed: the diff is `internal/le/rfc`, `internal/le/site`, `rfc/short/` and prose | Yes, no update owed |
| doctor checks | no runtime dependency added: no file path, socket, port, binary or certificate | Yes, no update owed |

## Core Insight

A gate that reads one field can be made to read two, and the readers derived from the first field do not follow. `Meta.Enrolled` became a conjunction and `Meta.Disposition` stayed a projection of one half, so the public page published the word `enrolled` as the reason a summary is not enrolled. The lesson is routed to `plan/journal/field-carries-two-meanings.md`, which is the class it belongs to.
