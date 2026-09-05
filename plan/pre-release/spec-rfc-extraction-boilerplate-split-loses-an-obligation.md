# Spec: rfc-extraction-boilerplate-split-loses-an-obligation

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

**The problem.** The RFC requirement extractor drops an obligation when the
obligation is stated BEFORE the RFC 2119 key-words paragraph inside one fused
chunk. `splitOffBoilerplate` (`internal/le/rfc/inventory.go`) searches for the
key-words paragraph and cuts only to the RIGHT of the match: the head it emits
is `rest[:end]`, which still carries the boilerplate match, so the caller
excludes the head whole. `sitesFor` in the same file is that caller, and it
skips every sentence for which `boilerplateRE.MatchString` is true. An
obligation sitting to the left of the match therefore leaves with the exclusion,
producing `obligation_keywords=0` and `sites=[]` for a paragraph that states a
MUST. The function's own comment states the limit: "WHAT IT DOES NOT REACH. An
obligation BEFORE the key-words paragraph is never cut, because the paragraph's
own keyword listing sits to the left of an 'interpreted as described in' match
and a left-hand cut would promote that listing to an obligation."

**The half that already landed, and must not be redone.** The 2026-08-08 review
recorded two cases. The first, a chunk with no `[.!?]` followed by whitespace
after the boilerplate match, was fixed on or before 2026-08-30: when
`boilerplateEnd` returns not-ok, `splitOffBoilerplate` cuts at the end of the
boilerplate match if `siteKeywordRE` still matches the tail, so the obligation
survives the exclusion. That is the inert fallback the reviewer named. Only the
mirror case is open.

**Why it is worth fixing.** The direction of the error is an UNDER-count, and an
under-count is the one direction nothing downstream can see: the caller drops the
sentence entire, the obligation never becomes a site, and the RFC reads as
asking for nothing. An over-count is visible to a reviewer, who deletes the row.
The gate cannot ask for evidence it never knew was owed.

**Why it is not urgent, and what that means for the spec.** The 2026-08-08
measurement found ZERO occurrences in the corpus of 184 sources, proven by an
independent sha over every site id and quote for both keyword registers. The
corpus has grown since, so the first task is to re-run that measurement over
today's `rfc/full/` and `rfc/drafts/` and report the count.

**The hard part, named by the reviewer.** A naive left-hand cut promotes the
key-words paragraph's OWN keyword listing ("MUST", "MUST NOT", "SHALL"...) to an
obligation, which is a false positive on every RFC that carries the paragraph.
Four rfc2890-class paragraphs are the known collision: their keywords precede
the match. The cut therefore has to distinguish a preceding SENTENCE that states
an obligation from the preceding LISTING that names the keywords, and the spec's
central design decision is what that test is.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - what the extractor feeds and which ratchets read it
  → Decision: [fill during research]
  → Constraint: [fill during research]

**Key insights:** (minimal context to resume after compaction)
- Only the preceding-obligation case is open; the no-terminator case landed and its fallback must survive the change.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/rfc/inventory.go` - `splitOffBoilerplate` loops on `boilerplateRE.FindStringIndex` and cuts at `boilerplateEnd(rest, loc[1])`, so every cut lands to the right of the match and each emitted head still carries it; `sitesFor` then skips any sentence matching `boilerplateRE`; `siteKeywordRE` is the MUST-level keyword test the no-terminator fallback already uses.

**Behavior to preserve:** (unless the user explicitly said to change it)
- A chunk that is only boilerplate is still dropped whole, and no keyword listing is promoted to an obligation.
- The no-terminator fallback added before 2026-08-30 keeps working.
- Every site id and quote the extractor produces today is unchanged, except for obligations the fix newly recovers.

**Behavior to change:** (only what the user asked for)
- An obligation stated before the key-words paragraph in a fused chunk becomes its own site.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The plain text of an RFC or draft, read from `rfc/full/<stem>.txt` or `rfc/drafts/`.
- The text enters section by section as paragraphs.

### Transformation Path
1. `sectionBodies` splits the document into sections.
2. `sentences` splits each paragraph and passes every chunk through `splitOffBoilerplate`.
3. `sitesFor` keeps the sentences that match the keyword register and drops every sentence that matches `boilerplateRE`.
4. The surviving sentences become sites, with an id of `<section>:<n>`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Extractor ↔ ledger | sites become rows in `rfc/short/<stem>.md` | No |
| Extractor ↔ gate | `./le rfc check` reads the extracted set | No |

### Integration Points
- `boilerplateEnd` and `siteKeywordRE` (`internal/le/rfc/inventory.go`) - the two helpers a left-hand cut has to reuse.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The corpus still carries zero occurrences | the 2026-08-08 sha measurement over 184 sources | the fix changes extracted counts and every affected summary needs a re-read | re-running the measurement over today's corpus | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A left-hand cut promotes the key-words listing to an obligation | new sites whose quote is the keyword list | require the head to hold a keyword OUTSIDE the listing, and re-run the sha over both registers before and after |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | every RFC summary gains false obligations, and the conformance gate demands evidence for text that asks for nothing |
| How is it reverted? | single commit revert; the extractor is a build-time tool |
| Who else touches this path? | the RFC gate specs under `plan/pre-release/` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a paragraph stating a MUST before the key-words paragraph | → | `splitOffBoilerplate` | `TestObligationBeforeTheKeyWordsParagraphSurvives` |
| an ordinary key-words paragraph with its keyword listing | → | `splitOffBoilerplate` | `TestKeyWordListingIsNotPromotedToAnObligation` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a fused chunk with an obligation before the boilerplate | the obligation becomes its own site with its own quote |
| AC-2 | a chunk that is only the key-words paragraph | it is still excluded whole |
| AC-3 | the whole corpus is extracted before and after the change | every site id and quote is identical apart from newly recovered obligations, proven by a sha over both registers |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestObligationBeforeTheKeyWordsParagraphSurvives` | `internal/le/rfc/inventory_test.go` | AC-1 | |
| `TestKeyWordListingIsNotPromotedToAnObligation` | `internal/le/rfc/inventory_test.go` | AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| corpus sha comparison | `./le rfc` action named at design | the extracted corpus does not move except where the fix recovers an obligation | |

## Files to Modify
- `internal/le/rfc/inventory.go` - the left-hand cut in `splitOffBoilerplate`, and the comment that records what it does not reach

## Files to Create
- [named at design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| Functional test for new RPC/API | | N-A: the extractor is a build-time tool with no RPC |
| CLI commands/flags | | [answered at design] |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 9 | RFC behavior implemented, changed, or newly proven? | | any `rfc/short/` summary whose extracted set moves |
| 10 | Test infrastructure changed? | | `docs/contributing/rfc-conformance-gates.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- measure the corpus, then write the two failing unit tests
   - Tests: the two names in the Wiring Test table
   - Files: `internal/le/rfc/inventory_test.go`
   - Verify: the preceding-obligation test fails and the listing test passes before any change
2. **Phase: [named at design]**

## Known Limitations
- [filled at design]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Current Behavior and Data Flow sections completed

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
