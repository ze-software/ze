# Closure sections (append to the spec at /ze-close step 1)

<!-- NOT a standalone spec. Append this file's sections (everything below the
     horizontal rule) to plan/spec-<name>.md when implementation is done and you
     are heading for the audit, the review gate, and the two-commit closure.

     Why it is separate from plan/TEMPLATE.md: measured on 2026-07-25 across 161
     specs, sections copied at spec CREATION but only used at CLOSURE reached
     closure byte-identical to the template in 65-75% of in-progress specs
     (Files Exist 11/15, AC Verified 11/15, Wiring Verified 9/12, Final status
     11/17). The sections authors created when they needed them -- Goal
     Validation, Implementation Summary, Key Design Decisions -- were untouched
     in 0%. Distance from use, not difficulty, is what killed them. -->

---

## Implementation Summary

### What Was Implemented
- [actual changes made]

### Bugs Found/Fixed
- [bugs discovered along the way, each with the test that now covers it]

### Documentation Updates
- [docs updated, naming each source anchor, or "None" with the grep that proves it]
- [`make ze-doc-test` result if docs changed]

### Deviations from Plan
- [what differed from the spec and why]

## Mistake Log

<!-- One table, one place. Ship the `none` row and either replace it or leave it
     deliberately: three separate empty tables produced three separate 67-82%
     untouched rates, because an empty table asks nothing.
     Kind: assumption (a broken A-N) | approach (a route abandoned) | escalation
     (a mistake frequent enough to deserve a rule). -->
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

<!-- BLOCKING before the learned summary. See ai/rules/completion.md.
     Status: Done (with file:line) | Partial | Skipped | Changed.
     Partial and Skipped both require explicit user approval. -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (each needs user approval)
- **Skipped:** (each needs user approval)
- **Changed:** (recorded in Deviations)

## Goal Validation (BLOCKING)

<!-- Maps each goal from the Task section to proof it was achieved. "Tests pass"
     is not evidence for a goal; a named test with its output is.
     See ai/rules/interop-and-goal-validation.md for the required evidence per
     goal type, and for the vacuity traps: a test that would still pass with the
     behavior reverted proves nothing. -->
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| [what the feature is meant to achieve] | [interop / functional / benchmark / chaos test] | [test name, output, or file reference] |

## Deferrals Resolved

<!-- Closure must leave no dangling row: deferral_unassigned_problems in
     scripts/dev/commit_helper.py WARNS (it does not block) on a live row with no
     destination -- act on the warning here, because nothing else will.
     The spec's own shard is git rm'd at closure ONLY when every row in it is
     terminal; a shard still holding a live row outlives its source spec and
     deferral_shard_removal_problems blocks its removal
     (ai/rules/planning.md). Account for every row here.
     If resolving a row empties a FOREIGN shard (its last live row becomes
     terminal), that shard is now residue and this closure removes it too. -->
| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| [what was deferred] | done / cancelled / deferred | [spec that now owns it, or why it is closed] |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md). The review is INDEPENDENT: reviewer
     subagents or a fresh session over the actual diff, never your own inline
     reasoning about code you just wrote.

     The machine-checked artifact is the deliverable, not this table:
     scripts/dev/review_gate.py record --spec <spec> ... then check.
     commit_helper.py runs `review_gate.py check` on the closure commit and
     refuses without a fresh, hash-pinned, CLEAN artifact. Record the artifact
     first; this table exists only to carry what was FOUND and FIXED forward
     into the learned summary. -->

| Field | Value |
|-------|-------|
| Artifact | [path printed by `review_gate.py record`] |
| `review_gate.py check` | [clean / not run] |
| Reviewer lenses used | [e.g. logic+wiring, security+edge-cases, feature risk area] |

### Findings fixed
<!-- Only BLOCKER and ISSUE. NOTEs do not block: record them and proceed.
     Every fix is new code that needs a fresh pass, so re-run until clean. -->
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

## Pre-Commit Verification

<!-- BLOCKING. Do NOT trust the audit above: re-verify independently and paste
     the evidence. For each row run a command (ls, grep, go test -run) now.

     EVERY sub-table needs at least one data row: pre_commit_verification_gaps
     in scripts/dev/commit_helper.py checks them one by one and names the empty
     ones. A row in Files Exist is not evidence for AC Verified.
     Not acceptable: "already checked", "should work", a pointer to the audit. -->

### Files Exist (ls)
<!-- Every file in "Files to Create", and every .ci named in Wiring Test and
     Functional Tests. Paste the ls output. -->
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
<!-- Every AC-N, re-checked. Acceptable: test name + pass output, grep showing
     the call, ls showing the file. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
<!-- Every Wiring Test row: does the .ci exist AND exercise the claimed path?
     Read the file; do not infer it from its name. -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
<!-- Every A-N. `unvalidated` is not a valid final status. A broken assumption
     needs a Mistake Log row and a Deviations entry. -->
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
<!-- Every Yes in the Documentation checklist: verify the edited claim against
     source. Every No: paste the grep that proves no update was needed. -->
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Core Insight
<!-- Optional: the single most important design revelation from this work.
     Not every spec has one. Delete the section if nothing qualifies.
     Feeds the Decisions section of the learned summary. -->
