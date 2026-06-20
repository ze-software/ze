# Spec: bug-review-5 -- Verification and Fix Backlog

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-bug-review-4-bgp-plugins-and-protocol-codecs.md |
| Phase | - |
| Updated | 2026-06-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-bug-review-0-umbrella.md`
3. Child reports from specs 1 through 4
4. `skill://ze-review`, `skill://ze-review-deep`, `skill://ze-review-spec`, `skill://ze-hunt`, `skill://ze-find-alloc`
5. `ai/rules/planning.md`, especially Risks & Assumptions, Review Gate, Completion Checklist, and Spec Closure
6. `ai/rules/no-partial-completion.md`, `ai/rules/wiring-completeness.md`, `ai/rules/testing.md`, `ai/rules/tdd.md`
7. `plan/learned/RECURRING-PATTERNS.md`

## Task

Verify, deduplicate, prioritize, and route findings from the plugin/core bug-review children. The output is a final report and a fix backlog. Every accepted finding must either have a follow-up fix spec with a regression-test plan or be explicitly rejected with source proof.

This child does not fix bugs. It prevents review output from becoming a loose list of opinions.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-bug-review-0-umbrella.md` - parent ACs and child dependencies
  → Decision: final verification closes the review program only when every child output is accounted for.
  → Constraint: no accepted finding may remain without a route to a fix spec or explicit user-approved disposition.
- [ ] Child 1 report `plan/review-bug-review-inventory.md` - authoritative scope
  → Decision: final report compares child outputs to inventory coverage.
  → Constraint: uncovered in-scope inventory rows block final completion.
- [ ] Child 2 report `plan/review-bug-review-plugin-engine-system.md` - plugin engine/system findings
  → Decision: findings from plugin infrastructure and system plugins are verified against source before acceptance.
  → Constraint: a child assertion is candidate evidence, not final evidence.
- [ ] Child 3 report `plan/review-bug-review-bgp-engine.md` - BGP core findings
  → Decision: core findings need RFC/memory/path verification before fix specs.
  → Constraint: protocol claims require summaries and path evidence.
- [ ] Child 4 report `plan/review-bug-review-bgp-plugins.md` - BGP plugin findings
  → Decision: BGP plugin findings need family/registration/completeness verification before fix specs.
  → Constraint: missing-family-chain findings must copy bgp-family checklist into the fix spec.
- [ ] `skill://ze-review-spec` - spec vs implementation verification method
  → Decision: fix specs created from findings must be independently implementable and have end-to-end ACs.
  → Constraint: Task promises must map to ACs and wiring tests, not just unit checks.
- [ ] `ai/rules/testing.md` and `ai/rules/tdd.md` - regression test discipline
  → Decision: each accepted bug needs a failing regression test plan before implementation.
  → Constraint: no finding is considered ready to fix without the test that would have caught it.
- [ ] `ai/rules/no-partial-completion.md` and `ai/rules/wiring-completeness.md`
  → Constraint: the final report must not claim review completion if child scope, accepted findings, or fix specs remain unrouted.

### RFC Summaries (MUST for protocol work)
- [ ] Protocol summaries cited by child findings
  → Constraint: final verification reads the summaries for protocol findings before accepting them.

**Key insights:**
- Child findings are not final findings. They become final only after dedupe, source verification, trigger validation, and owner assignment.
- A finding without a regression-test plan is not ready for fixing.
- A rejected candidate is still useful if the proof is recorded. It prevents repeated false positives.
- The final report must separate confirmed, plausible, rejected, and clean areas.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `skill://ze-review` - quick review requires wiring first, functional coverage, docs, source read, history/blame for changed regions, removed-behavior audit, edge cases, security, allocation, logic, and RFC checks.
  → Constraint: final verification keeps wiring and trigger evidence as mandatory fields.
- [ ] `skill://ze-review-deep` - multi-agent review collects focused lenses and then dedupes and verifies findings before reporting.
  → Decision: use the same candidate -> dedupe -> verify -> report lifecycle for this review program.
- [ ] `skill://ze-hunt` - whole-tree hunt treats grep hits as candidates and requires code reading before classification.
  → Constraint: final report must never list grep hits as findings without source verification.
- [ ] `skill://ze-find-alloc` - allocation audit classifies hot-path encoding allocations and legitimate exceptions.
  → Constraint: performance findings need category, path hotness, target pattern, and reason.
- [ ] `skill://ze-review-spec` - verifies that specs and implementations match, including Task vs AC and reference comparison.
  → Decision: fix specs created from findings must pass spec completeness review before implementation.
- [ ] `ai/rules/planning.md:151-169` - Pre-Spec Verification requirements.
  → Constraint: generated fix specs need metadata, current behavior, data flow, ACs, risks/assumptions, required reading, and wiring tests.
- [ ] `ai/rules/planning.md:222-229` - Review Gate loop.
  → Constraint: final review program records review gate findings and reruns if the report artifacts have blocking issues.

**Behavior to preserve:**
- Review remains read-only.
- Candidate verification is grounded in source, not child authority.
- Fixes happen in separate specs with TDD, regression tests, and verification.
- Rejected candidates are retained with proof to avoid repeated hunts.

**Behavior to change:**
- Child findings become a prioritized, verified backlog.
- Accepted issues get concrete fix specs or explicit disposition.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Child reports, source files cited by findings, tests/docs/RFC summaries cited by findings, inventory report, active specs.

### Transformation Path
1. Load all child reports and inventory.
2. Normalize each candidate into one schema: child ID, source site, trigger, expected, actual, severity, owner, test plan, evidence status.
3. Deduplicate candidates that share root cause or source site.
4. Verify each survivor by reading source and tracing the user/peer/config/plugin path.
5. Classify each survivor as confirmed, plausible, rejected, duplicate, pre-existing accepted debt, or needs-more-evidence.
6. Prioritize accepted findings by severity, blast radius, fix risk, and testability.
7. Create or update fix specs for accepted findings.
8. Write final report with coverage, findings, rejected candidates, fix backlog, and remaining risks.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Child report -> normalized candidate | table extraction | [ ] required fields present |
| Candidate -> verified finding | source read and data-flow trace | [ ] file:line plus trigger proof |
| Finding -> fix spec | spec creation with TDD plan | [ ] fix spec path and AC row |
| Rejected candidate -> false-positive record | proof table | [ ] guard/invariant/source proof recorded |
| Final report -> user decision | prioritized backlog | [ ] every accepted issue has status |

### Integration Points
- `plan/review-bug-review-inventory.md`
- `plan/review-bug-review-plugin-engine-system.md`
- `plan/review-bug-review-bgp-engine.md`
- `plan/review-bug-review-bgp-plugins.md`
- `plan/review-bug-review-final.md`
- `plan/spec-bugfix-*.md` or more specific spec names created for accepted findings
- `rfc/short/`, `test/`, `docs/`, source files cited by findings

### Architectural Verification
- [ ] No bypassed layers: final verification reads source and traces entry paths.
- [ ] No unintended coupling: fix owner is the deepest owner, not the reporter.
- [ ] No duplicated functionality: duplicate findings are merged, not fixed twice.
- [ ] Zero-copy preserved where applicable: performance/memory findings route to buffer-first fix patterns.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|----------------------------------|----------|--------------|--------|
| A-1 | Child reports exist and use stable finding IDs | specs 1 through 4 | final verification cannot map candidates | check files and ID tables before dedupe | unvalidated |
| A-2 | Accepted findings should become specs, not direct edits | umbrella contract and TDD rules | urgent defects are not fixed immediately | user can select a fix spec for implementation after review | unvalidated |
| A-3 | A plausible but not fully reproducible bug can be worth a fix spec if trigger path is realistic | ze-review false-positive filter | backlog includes speculative work | mark plausible, name missing evidence, and require proof in fix spec audit | unvalidated |
| A-4 | Some child candidates will be duplicates across lenses | deep review method | duplicate work or conflicting severities | dedupe table keeps highest severity and strongest evidence | unvalidated |
| A-5 | Existing active specs may be the right destination for some findings | current workspace has active specs and modified files | duplicate fix specs conflict with active work | final report records destination spec or new spec per finding | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | Final report becomes a summary without proof | finding rows omit source/trigger/test | block completion until required fields are filled |
| R-2 | Too many findings overwhelm prioritization | accepted list has no severity/owner/order | use severity plus blast radius plus testability table |
| R-3 | Fix spec duplicates an active spec | file under active spec already has relevant AC | route finding to active spec and note it in final report |
| R-4 | Accepted finding lacks regression test | test plan column empty | do not create fix-ready status; classify as needs-more-evidence |
| R-5 | Rejected candidate is dismissed too casually | rejection lacks guard or invariant | keep candidate as plausible until source proof exists |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| child report files | -> | normalized candidate table | `FinalReviewAllChildReportsLoaded` |
| normalized candidates | -> | deduped findings | `FinalReviewDedupHasRootCause` |
| accepted finding | -> | fix spec with regression AC | `FinalReviewAcceptedFindingsHaveFixSpecs` |
| rejected candidate | -> | rejection proof table | `FinalReviewRejectedCandidatesHaveProof` |
| inventory rows | -> | coverage summary | `FinalReviewInventoryCoverageZeroMissing` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Final verification starts | All child reports and inventory report exist or missing reports are blocking findings |
| AC-2 | Candidate lacks file:line, trigger, expected/actual, owner, severity, or regression plan | Candidate is marked incomplete and not accepted as a finding |
| AC-3 | Two candidates describe the same root cause | Final report merges them and records contributing child IDs |
| AC-4 | Candidate is accepted | Final report classifies it confirmed or plausible, assigns owner, severity, fix order, and fix spec path |
| AC-5 | Candidate is rejected | Final report quotes source proof, guard, invariant, or scope reason |
| AC-6 | Accepted finding touches protocol behavior | Relevant RFC summary read and cited in finding or fix spec |
| AC-7 | Accepted finding needs code changes | A follow-up fix spec exists with metadata, Current Behavior, Data Flow, Risks & Assumptions, Wiring Test, ACs, TDD plan, and regression test |
| AC-8 | Review program completes | Inventory coverage is zero-missing, all child outputs are accounted for, and final report lists remaining risks |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reads final bug-review output | child reports -> normalized table -> verified findings -> final report | `FinalReviewAllChildReportsLoaded` |
| 2 | Chooses a bug to fix | accepted finding -> prioritized backlog -> fix spec -> regression test plan | `FinalReviewAcceptedFindingsHaveFixSpecs` |
| 3 | Checks why a candidate was rejected | rejected candidate -> proof table -> source guard/invariant | `FinalReviewRejectedCandidatesHaveProof` |
| 4 | Checks coverage completeness | inventory rows -> child coverage -> final zero-missing table | `FinalReviewInventoryCoverageZeroMissing` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `FinalReviewAllChildReportsLoaded` | `plan/review-bug-review-final.md` | every child report named and loaded | |
| `FinalReviewDedupHasRootCause` | same report | duplicates merged with root-cause rationale | |
| `FinalReviewAcceptedFindingsHaveFixSpecs` | same report | each accepted finding maps to fix spec and regression test | |
| `FinalReviewRejectedCandidatesHaveProof` | same report | rejected candidates include source proof | |
| `FinalReviewInventoryCoverageZeroMissing` | same report | every inventory row covered or excluded | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Severity rank | NOTE/ISSUE/BLOCKER or low/medium/high/critical normalized | BLOCKER/critical | unknown low label | unknown label |
| Finding ID sequence per child | positive integer | highest child finding | 0 | duplicate ID |
| Fix priority order | positive integer | N accepted findings | 0 | duplicate priority |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `FinalReviewReportCompleteness` | `plan/review-bug-review-final.md` | user can act on final report without reading child transcripts | |
| `FixSpecRegressionPlanPresent` | each created fix spec | accepted bug has failing test plan before code changes | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Finding-specific scenario | `test/interop/scenarios/` | peer daemon named by finding | protocol bug can be reproduced or guarded in fix spec | |

### Future (if deferring any tests)
- No deferral in final review. A finding without a test plan is not accepted for fixing.

## Files to Modify

- No production code files.
- Read-only verification of child reports, cited source, tests, RFC summaries, and docs.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Review only |
| CLI commands/flags | No | Review only |
| Functional test for new RPC/API | No | Fix specs own tests |
| Env var registration | No | Review only |
| Doctor check for runtime dependencies | No | Review only |
| Prometheus counters/metrics | No | Review only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | Review only |
| 16 | Any changed source file is referenced by existing doc source anchors? | No | Review-only spec |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Files to Create

- `plan/review-bug-review-final.md` - final review report.
- `plan/spec-bugfix-FINDING-SLUG.md` or owner-specific `plan/spec-AREA-BUG.md` - one fix spec per accepted finding or tightly coupled finding cluster.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file plus child reports |
| 2. Audit | child report completeness and candidate fields |
| 3. Wiring phase | Wiring Test table, accepted finding -> fix spec route |
| 4. Implement (TDD) | create final report and fix specs, no production code |
| 5. /ze-review gate | review final report and fix spec completeness |
| 6. Full verification | report audits |
| 7-14 | standard completion |

### Implementation Phases

1. **Phase: Load and normalize** - read all child reports and normalize candidate fields.
   - Tests: `FinalReviewAllChildReportsLoaded`.
   - Files: child reports.
   - Verify: no missing child report or required field without blocking status.
2. **Phase: Dedupe and verify** - merge duplicates and verify survivors against source.
   - Tests: `FinalReviewDedupHasRootCause`, `FinalReviewRejectedCandidatesHaveProof`.
   - Files: cited source files, tests, RFC summaries.
   - Verify: each accepted/rejected candidate has proof.
3. **Phase: Prioritize and route** - assign severity, owner, fix priority, destination spec.
   - Tests: `FinalReviewAcceptedFindingsHaveFixSpecs`.
   - Files: final report and fix specs.
   - Verify: accepted findings have fix specs with regression plans.
4. **Phase: Coverage closure** - compare final report to inventory and child outputs.
   - Tests: `FinalReviewInventoryCoverageZeroMissing`.
   - Files: inventory and final report.
   - Verify: uncovered in-scope count is zero.
5. **Phase: Review artifacts** - run `/ze-review-spec` style check on each created fix spec.
   - Tests: `FixSpecRegressionPlanPresent`.
   - Files: fix specs.
   - Verify: fix specs can be implemented by another session.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every child report loaded, every inventory row covered, every accepted finding routed |
| Feature completeness | final report includes confirmed, plausible, rejected, duplicate, clean-area, and remaining-risk sections |
| Correctness | accepted findings verified by source and trigger path, not child assertion alone |
| Naming | stable finding IDs and fix spec slugs are unique |
| Data flow | every finding traces user/peer/config/plugin entry to failing behavior |
| Testability | each fix spec has a regression test plan that would catch the bug |
| Prioritization | severity and fix order reflect impact, blast radius, and verification confidence |
| Rule compliance | no direct production edits under review spec |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Final report | read `plan/review-bug-review-final.md` |
| Child report accounting | final report child coverage table |
| Deduped findings table | final report confirmed/plausible findings |
| Rejected candidates table | final report rejected section with proof |
| Fix specs | one path per accepted finding in final report |
| Regression plans | each fix spec TDD table names failing test |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|------------------|
| Sensitive data | final report and fix specs do not paste secrets from configs/env/certs |
| Exploit details | enough trigger detail for reproduction without exposing private data |
| False negatives | rejected security candidates have source proof, not intuition |
| Resource use | no broad expensive commands without scoped need |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Child report missing | return to that child spec before final closure |
| Candidate incomplete | mark incomplete and request/perform source verification |
| Accepted finding lacks test | cannot be fix-ready; create evidence task or reject |
| Duplicate findings conflict on severity | keep highest severity until source proof lowers it |
| Fix spec would overlap active spec | route finding to active spec and record cross-reference |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The final review is an evidence reducer. It should reduce candidate volume while increasing confidence, not produce another unverified list.

## Core Insight

A review finding is only useful when it is already one step away from TDD: reproduce, watch test fail, fix, watch it pass.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Normalize child findings before dedupe | manually merge prose | stable fields make missing evidence visible |
| Create fix specs for accepted findings | leave findings in final report | specs enforce ownership, ACs, wiring, and tests |
| Keep rejected candidate proof | omit false positives | prevents repeated review churn |
| Treat plausible findings as actionable when trigger is realistic | accept only confirmed findings | races, resource exhaustion, and rare malformed inputs are often realistic before a reproducer exists |

## Known Limitations

- This child closes the review program, not the bug fixes.
- A plausible finding may still need a reproducer during its fix spec audit before code changes.

## RFC Documentation

Fix specs for protocol findings must add or verify RFC comments at enforcement sites.

## Implementation Summary

### What Was Implemented
- Created `plan/review-bug-review-final.md`.
- Loaded inventory and all area reports.
- Deduplicated accepted findings, preserved plausible and rejected candidates, and mapped every accepted finding to a fix spec.
- Created four final bugfix specs in addition to the four created earlier during review execution.

### Bugs Found/Fixed
- Accepted 10 findings into fix specs.
- Did not change production code under the review spec.
- Did not promote SYS-004, SYS-005, BPLUG-P1, or BPLUG-P2 because final evidence did not prove an end-to-end bug or product decision.

### Documentation Updates
- Created final review report and bugfix specs only.
- No user documentation required for review-only artifacts.

### Deviations from Plan
- Promoted BENG-005 into an allocation-confirming fix spec because the trigger is concrete and the spec requires allocation proof before code changes.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Load child reports | done | final report source artifacts table | all four child artifacts consumed |
| Deduplicate accepted findings | done | accepted findings table | root-cause fix specs named |
| Preserve rejected proof | done | plausible and rejected tables | no unverified candidate promoted |
| Create fix specs | done | fix spec ledger | every accepted finding mapped |
| Record verification tests | done | final report audit tests | report audits pass |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | source artifacts table | child reports loaded |
| AC-2 | done | accepted findings table | dedupe and severity preserved |
| AC-3 | done | fix spec ledger | accepted findings have specs |
| AC-4 | done | plausible/rejected tables | non-promoted items have reason |
| AC-5 | done | audit tests table | final report audit complete |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| FinalReviewAllChildReportsLoaded | pass | final report audit tests | manual report audit |
| FinalReviewDedupHasRootCause | pass | final report accepted findings table | manual report audit |
| FinalReviewAcceptedFindingsHaveFixSpecs | pass | final report fix spec ledger | manual report audit |
| FinalReviewRejectedCandidatesHaveProof | pass | final report plausible/rejected tables | manual report audit |
| FinalReviewInventoryCoverageZeroMissing | pass | final report inventory closure | manual report audit |
| FixSpecRegressionPlanPresent | pass | eight fix specs | manual report audit |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `plan/review-bug-review-final.md` | created | final report |
| `plan/spec-bugfix-bgp-reactor-startup-cleanup.md` | created | BENG-004 |
| `plan/spec-bugfix-bgp-next-hop-alloc.md` | created | BENG-005 |
| `plan/spec-bugfix-bgp-nlri-strictness.md` | created | BPLUG-001 |
| `plan/spec-bugfix-bgp-srpolicy-encode.md` | created | BPLUG-002 |

### Audit Summary
- **Total items:** 18
- **Done:** 18
- **Partial:** 0
- **Skipped:** 0
- **Changed:** final report and four fix specs created

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Verified final bug-review backlog | Final report and fix specs | `plan/review-bug-review-final.md` plus fix specs named in accepted findings table |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | SYS-001, BENG-001, BENG-002 accepted | final report | fix specs created |
| 2 | ISSUE | remaining accepted SYS, BENG, and BPLUG findings | final report | fix specs created |
| 3 | NOTE | plausible findings not promoted | final report | recorded with reason and future route |

### Fixes applied
- None during review spec execution. Follow-up bugfix specs own production code.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | ArtifactReview found final closure and earlier fix-spec audits incomplete | review artifacts | closure sections and fix-spec audits completed |

### Final status
- [x] Critical review of final artifacts records no open review-artifact BLOCKER or ISSUE after closure edits
- [x] Fix specs contain source evidence, ACs, and regression plans
- [x] All NOTEs recorded above or explicitly none

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `plan/review-bug-review-final.md` | yes | created by child 5 |
| eight accepted-finding fix specs | yes | final report fix spec ledger |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | child reports loaded | final report source artifacts table |
| AC-2 | dedupe complete | final report accepted findings table |
| AC-3 | fix specs created | final report fix spec ledger |
| AC-4 | rejected/plausible candidates documented | final report plausible and rejected tables |
| AC-5 | report audits pass | final report audit tests table |

### Wiring Verified (end-to-end)
| Entry Point | Report Audit | Verified |
|-------------|--------------|----------|
| child reports | FinalReviewAllChildReportsLoaded | yes |
| accepted finding | FinalReviewAcceptedFindingsHaveFixSpecs | yes |
| rejected candidates | FinalReviewRejectedCandidatesHaveProof | yes |
| inventory scope | FinalReviewInventoryCoverageZeroMissing | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | every child report loaded |
| A-2 | confirmed | accepted findings table and fix spec ledger |
| A-3 | confirmed | rejected and plausible candidate tables |
| A-4 | confirmed | final report risk register and active spec overlap table |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user docs needed | review-only final report and fix specs | yes |
| RFC docs/comments needed later | protocol fix specs state requirement | yes |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-N all demonstrated
- [x] End-to-End User Stories all have report evidence
- [x] Wiring Test table complete
- [x] Critical review gate clean for final report artifacts
- [x] `make ze-spec-status` passes
- [x] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass, defer with user approval)
- [x] RFC constraint comments required in fix specs where applicable
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed

### Design
- [x] No premature abstraction
- [x] No speculative features
- [x] Single responsibility
- [x] Explicit behavior
- [x] Minimal coupling

### TDD
- [x] Final report audits written or manual evidence recorded
- [x] Regression test named for every accepted bug
- [x] Boundary tests named for numeric/protocol inputs
- [x] Functional or interop tests named where peer/user behavior is affected
- [x] Goal Validation table filled

### Completion (BLOCKING, before ANY commit)
- [x] Critical Review passes
- [x] Partial/Skipped items have user approval or are not applicable
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [x] Write learned summary to `plan/learned/943-bug-review-5-verification-and-fix-backlog.md`
- [x] **Commit A script prepared:** spec + final report + fix specs + learned summary + counter bump in `tmp/commit-f32fa560.sh`
- [x] **Commit B script prepared:** remove `plan/spec-bug-review-5-verification-and-fix-backlog.md` only after final state is preserved
