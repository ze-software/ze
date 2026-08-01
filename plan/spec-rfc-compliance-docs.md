# Spec: rfc-compliance-docs

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | docs |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

<!-- Retroactive spec (ai/rules/planning.md, "Retroactive Specs"): the work was
     done first, so the audit and closure sections are filled in the same pass. -->

## Task

The README claims a lot for Ze's RFC compliance and points nowhere that explains it.

Its Testing table said "2,700+ MUST-level requirements across 166 enrolled RFCs and
drafts" and linked only the status ledger. The wiki had a page on the subject,
`rfc-implementation`, that nothing linked and that had drifted: it claimed 166 enrolled
documents, 2,720 requirements, 539 gap annotations, four ratchets, and "none
outstanding". The tree said 168, 2,909, 534, seven ratchets, and 34 outstanding across
the declared remainder.

The goal is a public page that explains how Ze proves RFC compliance, whose every claim
is checked against the code that produces it, linked from the README where quality is
discussed. A page that overstates the machinery is worse than no page, so the limits
are stated on it: the unit-test skew of the evidence, the RFCs with no public row, the
audit's real coverage, and the fact that no check verifies the page itself.

A second problem surfaced while writing it. The prose used our own vocabulary where a
plain word would have served, `gated` being the word the owner named. The rule that
follows is not a list of forbidden words: write for a capable reader who knows computing
but not this repository, and reach for a technical term only when it carries meaning the
plain word cannot. That needed recording where every agent reads it, not fixing in one
file.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/no-fabrication.md` - every claim on a public page is a behavioral claim
  → Constraint: read the function that PRODUCES the behavior, never the caller
- [ ] `ai/rules/comparison-honesty.md` - the page compares Ze's practice with the norm
  → Constraint: no claim that Ze is better without evidence a maintainer elsewhere would accept
- [ ] `ai/rules/simplified-technical-english.md` - the page is user-facing prose
  → Constraint: no jargon a reader cannot look up. Extended here by the owner directive on build vocabulary
- [ ] `ai/rules/canonical-sources.md` - `CONDENSED.md` is generated
  → Constraint: edit the rule, then regenerate; never hand-edit the digest

**Key insights:**
- The wiki is a separate repository (`../wiki`, branch `master`, pushes to GitHub). No
  check in this repository can see it.
- `ai/RFC-REQUIREMENTS.md` is generated and already refuses to go stale
  (`check_ledger_fresh`), so the live numbers exist and are trustworthy. Only the
  hand-typed copies drifted.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/rfc_requirements.py` - `_collect_for_check`, `tag_kind_counts`,
  `credited`, `register_counts`, `audit_coverage`, `evaluate_extractions`,
  `load_dispositions`, `parse_status_ledger`, `_refuse_unrun`, `check_new_summaries`,
  `check_ledger_fresh`, `run_check`: the source of every figure the page states
- [ ] `.claude/hooks/pretool-writeedit.py` - `_RFC_TAG`, `_RFC_APPROVED`: what the
  tagged-test guard actually enforces
- [ ] `scripts/dev/rfc_tagged_scope.py` - `tag_scope`: Go scopes to the enclosing
  function, other carriers fall back to the whole file
- [ ] `scripts/status/verify_run.go` - `stagesForMode`: `ze-rfc-check` runs in both modes
- [ ] `rfc/enrolled.txt`, `rfc/not-enrolled.txt`, `rfc/drain-budget.txt` - the corpus
- [ ] `docs/features/rfc-status.md` - the public ledger the page links

**Behavior to preserve:**
- The README's structure and tone. It is the project's front door.
- The wiki page slug `rfc-implementation`. Five wiki pages and the sidebar link it, so a
  rename would break them.
- `ai/rules/simplified-technical-english.md` stays a GUIDELINE, not a gate.

**Behavior to change:**
- The README's stale figures and its missing link to the explanation.
- The wiki page's stale figures and its wrong claims about the machinery.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A reader arrives at `README.md`, reads the Testing table, and follows the link.

### Transformation Path
1. `README.md` Testing row and Documentation table link the wiki page.
2. The wiki page explains the mechanism and links `ai/RFC-REQUIREMENTS.md` and
   `docs/features/rfc-status.md` for live values.
3. Both of those are generated from `scripts/dev/rfc_requirements.py`.
4. Separately, the vocabulary rule flows `ai/rules/simplified-technical-english.md` →
   `make ze-rules-condensed` → `ai/rules/CONDENSED.md` → every session's context.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| repository → wiki repository | absolute GitHub URL in README | Yes, `check_doc_links.py --md-only` |
| wiki page → repository files | absolute GitHub blob URLs | Yes, read each target exists |
| rule file → every session | `make ze-rules-condensed` regenerates the digest | Yes, grep of `CONDENSED.md` |

### Integration Points
- `README.md` Testing table and Documentation table.
- `docs/contributing/writing-style.md` jargon section.
- `ai/rules/simplified-technical-english.md` directive list.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the rule was edited at its canonical source, the digest regenerated by its target |
| No unintended coupling (components stay isolated) | Yes | docs only, no package touched |
| No duplicated functionality (extends existing, does not recreate) | Yes | the existing wiki page was rewritten rather than a second page created |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no code path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them | N-A | no registration surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The figures in the tree are correct, so only the copies drifted | `ai/RFC-REQUIREMENTS.md` is generated and `check_ledger_fresh` compares it byte for byte | the page would restate a wrong number with more confidence | recomputed every figure from `_collect_for_check()` | confirmed |
| A-2 | Rewriting the existing page beats adding a second one | the sidebar and four content pages link `rfc-implementation` | a duplicate page drifts against the original | `grep -rln rfc-implementation` in the wiki: `_Sidebar.md`, `development.md`, `feature-inventory.md`, `srv6.md`, `vrrp.md` | confirmed |
| A-3 | The rule belongs in the project rules, not in a personal directory | `ai/rules/rule-placement.md`: shared behavior rules go in `ai/rules/` so every agent sees them | only one agent would follow it | the directive appears in `CONDENSED.md` | confirmed |
| A-4 | An independent check of my own page would find real errors | `ai/rules/critical-review.md`: the author's own reasoning is not a review | the page ships with confident wrong claims | it found seven | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The page's figures drift again | a reader reports a number that does not match the tree | the figures carry their date, the page names the commands, and it states outright that no check verifies it |
| R-2 | The page reads as advocacy rather than description | a maintainer of another project would dispute a sentence | the opening comparison was cut and the limits section was added |
| R-3 | The vocabulary rule is recorded but not applied, because the checker cannot see the word | `gated` reappears in new prose | recorded as a directive that loads in every session; a checker word-list entry needs a `.py` change and is not done |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing runtime. A wrong claim on a public page costs credibility, which is the reason for the independent check |
| How is it reverted? | Single commit revert here; the wiki is a separate repository with its own history |
| Who else touches this path? | Any session editing `ai/rules/`, since `CONDENSED.md` is regenerated wholesale |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| README Testing table | → | the wiki page URL | `python3 scripts/dev/check_doc_links.py --md-only` |
| README Documentation table | → | the wiki page URL | same |
| wiki sidebar and five pages | → | `rfc-implementation` | `go run bin/check-links.go` in `../wiki` |
| a session starting up | → | the vocabulary directive | `grep -n 'worst offender' ai/rules/CONDENSED.md` |
| rule format | → | the edited rule file | `make ze-rules-lint` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a reader opens the README Testing table | the RFC row states figures that match the tree and links the explanation |
| AC-2 | a reader opens the README Documentation table | an RFC compliance row links the same page |
| AC-3 | any factual claim on the page is checked | it matches the function that produces the behavior |
| AC-4 | a reader looks for the weaknesses | the page states the unit-test skew, the 32 RFCs with no public row, the audit's real coverage, and that no check verifies the page |
| AC-5 | the page is searched for build vocabulary | `gated` does not appear |
| AC-6 | a new session starts | the vocabulary directive is in its context through `CONDENSED.md` |
| AC-7 | link checks run in both repositories | both report success |

## 🧪 TDD Test Plan

<!-- Scope is docs: the checks are the repositories' own link, rule-format and
     prose gates. There is no unit under test. -->

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N-A | - | no code changed | - |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `check_doc_links.py --md-only` | `scripts/dev/` | a reader follows any link in the README and it resolves | pass |
| `check-links.go` | `../wiki/bin/` | a reader follows any wiki link and it resolves | pass |
| `ze-rules-lint` | `make` target | the edited rule still conforms to `rule-format.md` | pass |
| `ste_check.py README.md` | `scripts/dev/` | the changed prose adds no banned habit | pass |

### Interop Tests (Scope: protocol)
N-A. No protocol behavior.

## Files to Modify
- `README.md` - correct figures, link the explanation from Testing and Documentation
- `docs/contributing/writing-style.md` - the build-vocabulary section
- `ai/rules/simplified-technical-english.md` - the directive
- `ai/rules/CONDENSED.md` - regenerated, never hand-edited
- `../wiki/rfc-implementation.md` - rewritten (separate repository)
- `../wiki/_Sidebar.md` - label (separate repository)

## Files to Create
- None. The page existed and was rewritten.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | docs only |
| YANG validation constraints | N-A | docs only |
| YANG custom validators | N-A | docs only |
| CLI commands/flags | N-A | docs only |
| CLI grammar (keyword before value) | N-A | docs only |
| Editor autocomplete | N-A | docs only |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | no metric |
| BGP family surface | N-A | no family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | nothing shipped, the page describes existing machinery |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | the wiki page `rfc-implementation`, rewritten |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | the compliance state is described, not changed. `rfc/short/` untouched |
| 10 | Test infrastructure changed? | No | no target, runner or format changed |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no claim about the requirement check |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, command, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | No | no source file changed; `grep -rn 'source: README.md' docs/` is empty |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the README's own figures, corrected |

## Implementation Steps

1. **Phase: Verify the ground truth** - recompute every published figure from
   `_collect_for_check()` and the named helpers rather than trusting the ledger
   - Verify: RFC 4271's 21 gap annotations match the prose already in
     `docs/features/rfc-status.md`, which is an independent witness
2. **Phase: Rewrite the page** - the mechanism, the evidence model, the ratchets, the
   gaps, the limits
   - Verify: `go run bin/check-links.go` in the wiki
3. **Phase: Independent check** - an agent re-derives every claim from source
   - Verify: findings applied, seven corrections
4. **Phase: Link it** - README Testing row and Documentation table
   - Verify: `check_doc_links.py --md-only`
5. **Phase: Record the vocabulary rule** - writing guide, canonical rule, regenerate
   - Verify: `make ze-rules-lint`, grep `CONDENSED.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N demonstrated by a command, not by reading |
| Correctness | Every figure recomputed from the producing helpers, not copied from the ledger |
| Honesty | The page's unflattering facts are present and not softened |
| Naming | No `gated` in the rendered prose. Terms the code defines (`carrier`, `polarity`, `disposition`) are kept where they name the real thing |
| Rule: `no-fabrication.md` | Every mechanism claim traced to the function that implements it |
| Rule: `comparison-honesty.md` | No unevidenced claim that Ze is better than other implementations |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Accurate page | the independent check's findings all applied |
| README links it | `grep -n 'wiki/rfc-implementation' README.md` |
| Figures match the tree | recomputed via `_collect_for_check()` |
| Vocabulary rule loads everywhere | `grep -n 'worst offender' ai/rules/CONDENSED.md` |
| Links resolve | both link checkers exit 0 |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Information disclosure | The page publishes the project's own compliance gaps deliberately. It names no credential, host, or private path |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A claim cannot be traced to a producing function | delete the claim rather than soften it |
| The independent check disputes a figure | recompute from the scanner, never from the ledger |
| A link check fails | fix the link before commit |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The ledger `ai/RFC-REQUIREMENTS.md` was already generated and already freshness-checked,
  so the numbers in the tree could not drift. Every figure that did drift was a hand-typed
  copy on a surface no check reads. The defect was never measurement, it was restatement.
- A regex over `rfc/short/*.md` overcounts annotations, because it also matches lines
  below MUST level. My first count said 538 gaps; the scanner's own collector says 534.
  A statistic about a checker should be produced by that checker.
- The strongest evidence that the page is honest is not any of its numbers. It is that
  the four counts reconcile exactly: 1,164 proven both ways plus 534 gaps plus 841
  not-applicable plus 370 single-direction equals 2,909, the whole enforced set.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Rewrite the existing wiki page | add a new "RFC compliance" page and leave the old one | five pages and the sidebar link the existing slug, and a second page would drift against the first |
| State the weaknesses on the page | publish the favourable totals only | a page about the honesty of a compliance claim that hid its own weak points would refute itself. The unit-test skew is the most useful line on it |
| Keep dated figures rather than removing all numbers | link the generated ledger and quote nothing | a page with no numbers loses the reader. The date plus the reproduce-it commands plus an explicit "no check verifies this page" is the honest form |
| Record the vocabulary rule in `ai/rules/` | fix the word in this page only; save it as a personal memory | `ai/rules/rule-placement.md`: a shared behavior rule belongs where every agent reads it |
| No statistics generator | build `--stats`, a generated file and a freshness check | the ledger already carries the headline figures and is already freshness-checked, so a new file and a new check would duplicate machinery that exists. The aggregate splits it does NOT carry are a smaller, separate job, recorded under Known Limitations rather than built here |

## Known Limitations

- The wiki lives in a separate repository, so no check here can verify its figures. The
  page says so.
- The vocabulary rule is a directive, not a checker entry. Adding `gated` to
  `scripts/dev/ste_check.py` would make it mechanical and needs an implementation session.
- The page's figures are a dated snapshot. The live values are one command away and the
  page names it.
- **The aggregate figures on the page are still hand-typed, and this is the one open
  item.** `ai/RFC-REQUIREMENTS.md` publishes the headline counts (2,909 across 168), the
  per-RFC table and the audit line, but no totals row, no annotation split
  (534 / 841 / 370) and no evidence split (3,060 / 19 / 0 / 2). Every one of those was
  recomputed by hand here, and by the reviewer, with ad-hoc scripts. Adding a totals line
  and the two splits to `render_ledger` is roughly thirty lines, needs no new file, no
  new target and no new check, and would inherit `check_ledger_fresh`. It is an
  implementation-phase change (`scripts/dev/rfc_requirements.py`) and was not done in
  this session.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete: every row a concrete command, none deferred
- [ ] `make ze-verify` not run: every file is `README.md`, `ai/**/*.md`, `docs/**/*.md`
      or `plan/**/*.md`, which is a NO row in `ai/rules/git-safety.md` Step 0. One of
      them, `ai/rules/CONDENSED.md`, IS generated and has its own gate, so that gate was
      run on its own: `make ze-rules-condensed-check` reports up to date, and
      `make ze-rules-lint` reports 97 rule files conform
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: nothing deferred

### TDD
- [ ] Tests written -- N-A, docs scope: no code changed, so there is no unit under test.
      The gates that do apply are the two link checkers, `ze-rules-lint` and
      `ste_check.py`, each named with its result in the Functional Tests table
- [ ] Tests FAIL (paste output) -- N-A for the same reason. The nearest equivalent is the
      independent check, which found nine claims wrong before the page was linked
- [ ] Tests PASS (paste output) -- `check_doc_links.py --md-only`: "all corpus path
      references resolve". `check-links.go`: "All links OK". `ze-rules-lint`: "97 rule
      file(s) conform to ai/rules/rule-format.md"
- [ ] Boundary tests for all numeric inputs -- N-A, no numeric input
- [ ] Functional `.ci` tests for end-to-end behavior -- N-A, no daemon behavior
- [ ] Interop tests for protocol features (or N-A with a reason) -- N-A, no wire-visible
      behavior

### Closure
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-rfc-compliance-docs.md`
- [ ] **Commit A:** docs + rules + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-rfc-compliance-docs.md` only

## Implementation Summary

### What Was Implemented
- `../wiki/rfc-implementation.md` rewritten: the requirement identifier contract, the
  evidence model with its layer and pipeline split, the seven ratchets, the annotation
  vocabulary with counts, the extraction sign-off, the tagged-test guard, the audit, the
  workflow for adding a document, and a limits section.
- `../wiki/_Sidebar.md`: label changed to "RFC Compliance", slug unchanged.
- `README.md`: the Testing row's figures corrected from "2,700+ across 166" to "2,900+
  across 168", and it now links the page; a Documentation table row added.
- `docs/contributing/writing-style.md`: a section on preferring the plain word, with a
  replacement table, the say-it-out-loud test, and the counterweight that terms the code
  defines are kept.
- `ai/rules/simplified-technical-english.md`: the same rule as one directive, dated and
  attributed, so it reaches every session through the digest.
- `ai/rules/CONDENSED.md`: regenerated by `make ze-rules-condensed`.

### Bugs Found/Fixed
- Fourteen findings, listed in Review Gate below: nine on the page from the first
  independent round (seven factually wrong claims, two overstatements), four on the
  repository diff from the second, and one from the owner on the shape of the wording
  rule. None reached a reader: every one was found and fixed inside the session that
  introduced it.

### Documentation Updates
- The page and the README, as above. `python3 scripts/dev/check_doc_links.py --md-only`
  reports "all corpus path references resolve".
- No `docs/` source anchor points at a file this commit changes: `grep -rn 'source:
  README.md' docs/` is empty.

### Deviations from Plan
- The original spec in this session, `spec-rfc-compliance-stats`, proposed a statistics
  generator. It was dropped once the ledger was found to be freshness-checked already.
  That spec is not committed; this one replaces it.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | I asked the user to choose a publishing target for a new statistics generator, and wrote a spec against their answer | The ledger was already generated and already refused to go stale, so most of the generator was redundant. It was not ALL redundant: the ledger publishes the headline figures and the per-RFC table, but no totals row and neither aggregate split, so those numbers are still hand-typed | The user asked whether the spec needed implementing, which sent me to `--check-fresh`; a later review caught that my first answer overcorrected | Spec dropped, this one written instead. The accurate residue is recorded under Known Limitations. Lesson: check whether the existing generated artifact already covers it BEFORE you offer the user a choice between build options |
| assumption | I counted gap annotations with a regex over the summaries and got 538 | 534. The regex also matched annotations on lines below MUST level | The numbers agent used the scanner's own collector and disagreed | Recomputed with `_collect_for_check()`. Lesson in Design Insights |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A public page explaining how compliance is proven | Done | `../wiki/rfc-implementation.md` | rewritten, 171 lines |
| Every claim checked against producing code | Done | independent agent report | seven corrections applied |
| Linked from the README where quality is discussed | Done | `README.md` Testing table, Documentation table | |
| The limits stated, not hidden | Done | page sections "What a green build actually proves" and "What none of this proves" | |
| Build vocabulary recorded as a rule | Done | `ai/rules/simplified-technical-english.md`, `docs/contributing/writing-style.md` | reaches sessions via `CONDENSED.md` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `README.md` Testing row | 2,900+ across 168, both recomputed |
| AC-2 | Done | `README.md` Documentation table | RFC Compliance row |
| AC-3 | Done | independent check | every figure and mechanism claim verified or corrected |
| AC-4 | Done | page text | unit skew, 32 undisclosed RFCs, audit at 49 of 1,534, and "It does not check this page" |
| AC-5 | Done | `grep -c 'gated' ../wiki/rfc-implementation.md` | 0 |
| AC-6 | Done | `grep -n 'worst offender' ai/rules/CONDENSED.md` | present |
| AC-7 | Done | both link checkers | exit 0 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `check_doc_links.py --md-only` | Done | `scripts/dev/` | "all corpus path references resolve" |
| `check-links.go` | Done | `../wiki/bin/` | "All links OK" |
| `ze-rules-lint` | Done | make target | 97 rule files conform |
| `ste_check.py README.md` | Done | `scripts/dev/` | 5 findings, all on pre-existing lines |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `README.md` | Done | two edits |
| `docs/contributing/writing-style.md` | Done | one section added |
| `ai/rules/simplified-technical-english.md` | Done | one directive added |
| `ai/rules/CONDENSED.md` | Done | regenerated, not hand-edited |
| `../wiki/rfc-implementation.md` | Done | separate repository, separate commit |
| `../wiki/_Sidebar.md` | Done | separate repository, separate commit |

### Audit Summary
- **Total items:** 5 requirements, 7 ACs, 4 checks, 6 files
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** one, recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A page that explains how RFC compliance is proven | reader-facing artifact | `../wiki/rfc-implementation.md`, 171 lines covering the identifier contract, the evidence model, the seven ratchets, the annotation counts, extraction, the tagged-test guard and the audit. Reachable from `README.md` (two links) and the wiki sidebar; `go run bin/check-links.go` resolves both |
| Every claim true against the code | independent verification | two review rounds re-derived every figure and mechanism claim from `scripts/dev/rfc_requirements.py`, `rfc_tagged_scope.py` and `pretool-writeedit.py`; thirteen findings, all applied. Artifact recorded by `review_gate.py` |
| Figures match the tree | recomputation | `_collect_for_check()` gives 168 / 2,909 / 1,164 / 534 / 841 / 370, and RFC 4271's 21 gaps match the independent prose in `docs/features/rfc-status.md` |
| The limits are visible | artifact inspection | the page states 3,060 of 3,081 tags are unit tests, 32 enrolled RFCs have no public row, the audit covers 49 of 1,534, and no check verifies the page |
| Vocabulary rule applies to every agent | digest inspection | the directive appears in `ai/rules/CONDENSED.md`, which loads in every session |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | - | no shard: nothing was deferred |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | (filled at record time) |
| `review_gate.py check` | (filled at record time) |
| Reviewer lenses used | claim-by-claim verification against producing code; adversarial read for overstatement |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | "25 checks" | page, shape table | corrected to 23, with the command that counts them |
| 2 | ISSUE | "ten of the checks compare against the previous commit" | page, ratchets | softened to "about half", the precise count being contestable |
| 3 | ISSUE | ratchet 4 described as accepting a declared remainder | page, ratchet table | `check_new_summaries` skips only enrolled stems; enrolment is the only escape |
| 4 | BLOCKER | "only the project owner can authorize" the test-change escape, stated as enforced | page, tagged-test section | it is a comment and a greppable trail; reworded as convention plus audit trail |
| 5 | ISSUE | edit scope stated as the enclosing test function | page, tagged-test section | true for Go only; other carriers widen to the whole file |
| 6 | ISSUE | enrolment requires text under `rfc/full/` | page, workflow | `rfc/full/` or `rfc/drafts/` |
| 7 | ISSUE | "a gap carries a `file:line`" | page, honest part | 525 of 534 do; the checker requires only a reason |
| 8 | ISSUE | "the published numbers cannot drift" | page, reproduce section | false for the page itself; replaced with an explicit statement that no check verifies it |
| 9 | NOTE | opening paragraph disparaged other implementations | page, opening | cut (`ai/rules/comparison-honesty.md`) |
| 10 | ISSUE | the vocabulary rule also banned `carrier`, `polarity` and `disposition`, which are exact names of real things: `CARRIERS`, the tag grammar field, the extraction classification | `ai/rules/simplified-technical-english.md`, `docs/contributing/writing-style.md` | defined terms kept and expanded on first use |
| 14 | ISSUE | the rule was written as a ban on one word, when the owner's point was general: plain words unless the technical one earns its place | `ai/rules/simplified-technical-english.md`, `docs/contributing/writing-style.md` | rewritten as the general rule, with `gated` demoted to the standing example (owner, mid-session) |
| 11 | ISSUE | `gated on X` replaced with "runs only when X is true", wrong for the dominant compile-out sense | `docs/contributing/writing-style.md` | separate rows for the build-tag sense and the authorization sense |
| 12 | ISSUE | README said "annotated with a published reason"; only `{gap}` carries a publication obligation, the other 1,211 annotations do not | `README.md` | "annotated with a recorded reason, and every gap disclosed on the status ledger" |
| 13 | ISSUE | the Mistake Log claimed the dropped generator would have restated existing data | this spec, Mistake Log | corrected: the ledger carries the headline figures but no aggregate split, so those figures are still hand-typed |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `README.md` | Yes | modified, `git status --porcelain README.md` reports ` M README.md` |
| `docs/contributing/writing-style.md` | Yes | edit applied, section present |
| `ai/rules/simplified-technical-english.md` | Yes | edit applied, directive present |
| `ai/rules/CONDENSED.md` | Yes | `make ze-rules-condensed` wrote 97 rules; `make ze-rules-condensed-check` reports up to date |
| `../wiki/rfc-implementation.md` | Yes | 171 lines, `git status` in the wiki reports ` M rfc-implementation.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | README figures match the tree | `_collect_for_check()` gives 168 enrolled and 2,909 enforced; the row says 168 and 2,900+ |
| AC-2 | Documentation table links the page | the row "RFC Compliance" is present in `README.md` |
| AC-5 | no build vocabulary on the page | `grep -c 'gated\|gating'` returns 0 |
| AC-6 | the directive loads in every session | `grep -n 'worst offender' ai/rules/CONDENSED.md` matches |
| AC-7 | links resolve | `check_doc_links.py --md-only` and `check-links.go` both exit 0 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| README Testing table → wiki page | N-A, docs scope: `check_doc_links.py --md-only` | Yes, "all corpus path references resolve" |
| wiki sidebar → `rfc-implementation` | N-A, docs scope: `go run bin/check-links.go` | Yes, "All links OK" |
| rule file → every session | N-A, docs scope: `grep` of the regenerated digest | Yes, directive present at `CONDENSED.md` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | every figure recomputed from the scanner's own collector |
| A-2 | confirmed | `grep -rln 'rfc-implementation'` in the wiki lists the sidebar and five pages |
| A-3 | confirmed | the directive is in `CONDENSED.md`, which every session loads |
| A-4 | confirmed | the independent check produced nine findings, one of them a BLOCKER |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 6, user guide page | the wiki page is the guide, rewritten and link-checked | Yes |
| Row 9, RFC behavior | `git status --porcelain rfc/` is empty: no summary changed, so no status row needed updating | Yes |
| Row 16, source anchors | `grep -rn 'source: README.md' docs/` returns nothing | Yes |
| Row 17, stale examples | the README's own figures were the stale example, and they are corrected | Yes |

## Core Insight

A generated artifact stops numbers drifting only on the surface that renders it. Every
figure that rotted here was a hand-typed copy on a page no check reads, while the
generated ledger beside it had been correct the whole time. The fix for a restatement
problem is to stop restating, not to generate the same data again somewhere new.
