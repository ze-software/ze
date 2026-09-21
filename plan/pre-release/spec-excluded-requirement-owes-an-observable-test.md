# Spec: excluded-requirement-owes-an-observable-test

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

Make "an excluded requirement Ze can observe still owes a test" a fact a gate
reads, instead of a sentence two documents state and nothing enforces.

`ai/rules/rfc-compliance.md` says a requirement met through a lower layer still
carries a test, and that the test asserts the thing Ze produces: the selector,
the state, or the option Ze installs for the layer below.
`docs/contributing/rfc-conformance-gates.md`, "Who implements the document",
repeats it for a document that leaves the count. Both are true and neither is
checked. A requirement can leave the gated population today and carry no test,
no declaration that it owes one, and no mark on the published page, and every
gate stays green.

The classification work landed the document-level half of this: a summary
declares whose code answers it, `readImplementation`
(`internal/le/rfc/meta.go`) refuses a kind that names no implementer, and
`Meta.CountsAgainstZe` takes `third-party` and `foundation` out of the
population `./le rfc check` counts. What left the count left the gate with it.
The obligation this spec owns is what an excluded row owes BACK: where Ze
installs the state the other layer acts on, the boundary Ze owns is testable,
and the test is the only evidence a reader outside this repository can check.

The question the design answers is which excluded rows are observable. Not
every one is: sixteen RFC 4302 obligations are excluded precisely because no
value Ze writes decides the field, which is what `{lower-layer}` was added for.
So the design owes three decisions. How an excluded site records whether Ze can
observe the behavior, and who decides it. What the gate refuses when an
observable exclusion carries no test, and whether that refusal is a ratchet over
the existing population or a red on the first run. What the published page shows
for an excluded row that carries a test, so the reader can tell a proven
delegation from an unproven one.

The precedent to read first is `binds-another-role`, which
`ai/rules/rfc-compliance.md` records as PRESUMED WRONG after it grew to 915
sites: a label that costs a sentence gets reached for, and a label that costs a
checkable fact does not. An observability flag a reviewer sets by hand, with
nothing checking it, is the same shape.

This spec changes no RFC ledger, tag, support claim, or publication status until
its design is approved.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the artifacts, the ratchets, and the two sections that state this obligation
- [ ] `ai/rules/rfc-compliance.md` - the 2026-08-31 whole-stack directive and the test it asks for at the boundary Ze owns

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4302.md` - the sixteen `{lower-layer}` rows, which are the case where no value Ze writes decides the field

**Key insights:**
- A label that costs a sentence is reached for, and one that costs a checkable fact is not
- Out of the count is not out of test, and only the page can show the difference

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/meta.go` - `readImplementation` refuses an implementation kind that names no implementer, and `Meta.CountsAgainstZe` decides whether the document's obligations count
- [ ] `internal/le/rfc/artifact.go` - the exclusion vocabulary and `ExclusionPresumedWrong`, the one kind the rule treats as suspect until justified
- [ ] `internal/le/site/rfccompliance.go` - `rfcImplementationHTML` publishes who implements each document, and the exclusion disclosure publishes what left the count

**Behavior to preserve:**
- A gap is an ISSUE and an exclusion is a DECISION, and neither is renamed into the other
- Every published share stays over the gated population (owner ruling, 2026-09-02)

**Behavior to change:**
- An excluded requirement Ze can observe declares the test it owes, and a gate reads that declaration

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The excluded requirement line in `rfc/short/<stem>.md`, and the summary's `## Meta` implementation row above it
- Format at entry: an annotation on one requirement, plus the document kind

### Transformation Path
1. The extraction artifact reader carries the exclusion and its kind
2. `./le rfc check` decides the population and would decide this obligation
3. The published page renders what left the count and, after this spec, what proves it anyway

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| summary text ↔ ledger | the annotation reader, refusing rather than defaulting | No |
| ledger ↔ published page | `RenderInput`, carrying no second copy of the fact | No |

### Integration Points
- `internal/le/rfc/artifact.go` - the closed exclusion vocabulary this would extend or read
- `internal/le/rfc/meta.go` - `implementationCounts`, the one declaration of which kinds leave the count

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
| An observability flag becomes the next `binds-another-role` | that kind reached 915 sites and the owner now presumes it wrong | the declaration must name the state Ze installs and the test that reads it, so the claim costs a fact |
| A first-run red over the existing excluded population stops every session | the enrolment ratchets were added for exactly that reason | the design says whether this is a ratchet over today's count or a red, and which sites are in the first population |

## Blast Radius

`./le rfc check`, `ai/RFC-REQUIREMENTS.md`, the published compliance pages, and
every summary carrying an excluded requirement.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| an observable excluded requirement carrying no test | → | the gate's refusal | `TestAnObservableExclusionWithoutATestIsRefused` |
| `./le rfc check` over a corpus holding one proven exclusion | → | the published exclusion disclosure | `TestAProvenExclusionShowsItsTestOnThePage` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | an excluded requirement whose behavior Ze can observe | it declares the test it owes, and the declaration names the state Ze installs |
| AC-2 | an observable excluded requirement carrying no test | the gate refuses it and names the requirement and the boundary |
| AC-3 | an excluded requirement no value Ze writes decides | it stays excluded with no test owed, and its reason says why |
| AC-4 | a proven exclusion | the published page shows it as proven, so a reader can tell it from an unproven one |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAnObservableExclusionDeclaresTheTestItOwes` | `internal/le/rfc/exclusion_test.go` | the declaration is read, and an absent one is refused | |
| `TestAnObservableExclusionWithoutATestIsRefused` | `internal/le/rfc/check_test.go` | out of the count is not out of test | |
| `TestAnUnobservableExclusionOwesNoTest` | `internal/le/rfc/check_test.go` | the RFC 4302 case stays legal | |
| `TestAProvenExclusionShowsItsTestOnThePage` | `internal/le/site/rfccompliance_test.go` | the reader can tell a proven delegation from an unproven one | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc-exclusion-proof` | `test/plugin/*.ci` | a reader asks the published ledger what proves an excluded requirement and gets the test | |

## Files to Modify

- `internal/le/rfc/artifact.go` - the exclusion vocabulary and what an excluded site declares
- `internal/le/rfc/check.go` - the refusal, and the population it runs over
- `internal/le/site/rfccompliance.go` - the proven exclusion on the published page
- `docs/contributing/rfc-conformance-gates.md` - state the gate rather than the rule

## Files to Create

- none beyond tests

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- an excluded site declares what it owes
   - Tests: `TestAnObservableExclusionDeclaresTheTestItOwes`
   - Files: `internal/le/rfc/artifact.go`
2. **Phase: the refusal** -- an observable exclusion with no test reds
   - Tests: `TestAnObservableExclusionWithoutATestIsRefused`, `TestAnUnobservableExclusionOwesNoTest`
   - Files: `internal/le/rfc/check.go`
3. **Phase: the page** -- a proven exclusion reads as proven
   - Tests: `TestAProvenExclusionShowsItsTestOnThePage`, `rfc-exclusion-proof`
   - Files: `internal/le/site/rfccompliance.go`

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
