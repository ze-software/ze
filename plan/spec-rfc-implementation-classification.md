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
