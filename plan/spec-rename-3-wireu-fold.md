# Spec: rename-3-wireu-fold -- fold bgp/wireu into bgp/packet

| Field | Value |
|-------|-------|
| Status | blocked |
| Depends | spec-rename-2-bgp-packet.md |
| Phase | - |
| Updated | 2026-07-08 |

Umbrella: `spec-rename-0-umbrella.md`. Siblings: `spec-rename-1-ike-packet.md`, `spec-rename-2-bgp-packet.md` (prerequisite).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/go-standards.md` ("Package-Naming Glossary"), `ai/rules/protocol.md`
4. `internal/component/bgp/wireu/doc.go`, `internal/component/bgp/wireu/split.go`

## Task

`internal/component/bgp/wireu` ("wire UPDATE") holds the lazy-parsed,
zero-copy UPDATE view. Its doc.go records a keep-and-document decision from
the morning of 2026-07-08; the user superseded that decision the same day
after a clash audit came back clean: fold the package into `bgp/packet` (the
renamed `message`, spec-rename-2). One protocol, one codec package, per the
skeleton. This is a merge, not a pure rename: files move into the sibling
directory, `packet` becomes their package, and the two packages' halves
(eager codec + lazy UPDATE view) live together.

Measured scope (2026-07-08): 20 files in `wireu` plus
`testdata/fuzz/FuzzRewriteASPath` corpus; 47 importer files (26 tests); ~193
lines with the `wireu.` qualifier; 17 doc source anchors. Merge evidence:
zero identifier clashes with `message` across exported (60+81 vs 24+7) and
unexported (108 vs 34) top-level names, tests included; only `doc.go` and
`errors.go` collide by filename; `wireu` imports `message`
(`internal/component/bgp/wireu/split.go`), so the fold direction has no
import cycle.

**BLOCKED on spec-rename-2** (the destination package must exist as
`packet`). Rerun the clash audit at start: the packages may drift between the
audit snapshot and implementation.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. -->
- [ ] `ai/rules/go-standards.md` - "Package-Naming Glossary"
  → Decision: glossary `wireu` row ("kept") is removed by this spec; the keep rationale is superseded by the user's fold decision (2026-07-08)
  → Constraint: glossary edit ships in the same commit as the fold
- [ ] `ai/rules/protocol.md` - BGP probe row lists `wireu` in the pre-SDK codec pair
  → Constraint: after this spec the BGP row's codec exception reduces to history; only `reactor` remains a named exception
- [ ] `docs/architecture/wire/messages.md` - Design anchor for wireu files ("wire UPDATE lazy parsing")
  → Constraint: prose describing two packages becomes one-package prose; anchors updated
- [ ] `ai/rules/performance.md` and `ai/rules/performance.md` - the zero-copy discipline wireu implements
  → Constraint: the fold must not disturb zero-copy iterators; mechanical move only

### RFC Summaries (MUST for protocol work)
- [ ] Not applicable: no protocol behavior changes (RFC 4271/4760/7911 handling moves verbatim).

**Key insights:**
- The fold is directionally safe (wireu already depends on message) and name-safe (audit: zero clashes); the only real merge work is two filenames and one doc comment.
- After this spec the report's legacy set is `bgp/reactor` alone.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/wireu/doc.go` - names the package "wire UPDATE", records the keep decision this spec supersedes
- [ ] `internal/component/bgp/wireu/split.go` - SplitWireUpdate; imports `message` (:9); RFC 4271/4760/7911 comments move verbatim
- [ ] `internal/component/bgp/wireu/errors.go` - UPDATE parsing error values; filename collides with the destination's `errors.go` (wire-format errors) |
- [ ] `internal/component/bgp/message/errors.go` - destination collision partner; content is disjoint (ErrShortRead etc. vs ErrUpdateTruncated etc.)

**Behavior to preserve:**
- Zero-copy lazy UPDATE parsing, iterators, split logic: all identifiers keep their names, only the qualifier changes (`wireu.X` -> `packet.X`)
- Fuzz corpus `testdata/fuzz/FuzzRewriteASPath` moves with its test into the destination's `testdata/fuzz/`
- The one aliased-import check from the umbrella (zero aliases) re-verified at start

**Behavior to change:**
- Package membership only. None functional.

## Data Flow (MANDATORY)

### Entry Point
- Unchanged: UPDATE bytes -> lazy WireUpdate view (these files) -> reactor/RIB; split path on encode.

### Transformation Path
1. Same pipeline; the lazy-view stage moves into the codec package it already depends on.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None new | intra-tree merge; the message->wireu import edge becomes intra-package | [ ] |

### Integration Points
- 47 importer files - import path + qualifier rewrite to `packet`
- former `message.` qualifiers inside wireu files (e.g. split.go) - dropped (now intra-package)
- `scripts/dev/protocol_skeleton_report.py`, `ai/rules/go-standards.md`, `ai/rules/protocol.md`, `ai/PACKAGE-MAP.md` - rule-surface sync

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (the dependency already existed; it becomes internal)
- [ ] No duplicated functionality
- [ ] Zero-copy preserved (mechanical move)
- [ ] Registration over hardcoding — nothing new registers

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Zero identifier clashes between the two packages | comm audit 2026-07-08 (exported 60+81 vs 24+7; unexported 108 vs 34; tests included) | clash needs a rename first; fold design holds otherwise | rerun both comm audits at start | confirmed (2026-07-08 snapshot; MUST rerun at start) |
| A-2 | Only doc.go and errors.go collide by filename | directory-listing comm 2026-07-08 | more file renames in the move | rerun listing comm at start | confirmed (2026-07-08 snapshot) |
| A-3 | wireu -> message is the only cross-import between the pair | import grep 2026-07-08 (split.go; no reverse edge) | cycle would block the fold | grep both directions at start | confirmed (2026-07-08 snapshot) |
| A-4 | 17 doc anchors point into `bgp/wireu` | anchor grep 2026-07-08 | doc-test reveals more | `make ze-doc-test` after sweep | confirmed (2026-07-08 snapshot) |
| A-5 | spec-rename-2 closed; destination is `bgp/packet` | Depends field | fold target absent | `make ze-spec-status` + `ls internal/component/bgp/packet` at start | unvalidated (checked at start) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Packages drift after the audit snapshot (new identifiers collide) | rerun audit differs | the start-of-work audit rerun is phase 1; a clash means STOP and pick a rename for the colliding symbol with the user |
| R-2 | Merged package's file set mixes two concerns without a map | reviewers can't tell eager codec from lazy view | keep the moved files' `// Design:` headers; extend the destination doc.go with a two-paragraph structure note (eager codec files vs lazy UPDATE view files) |
| R-3 | Fuzz corpus path silently detaches from its test | fuzz test can't find corpus post-move | move `testdata/fuzz/` with the package; run the fuzz target's seed pass in verification |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP UPDATE bytes from a peer | → | lazy WireUpdate view (now in `bgp/packet`) -> reactor/RIB | `test/decode/bgp-flow-1.ci` (and siblings) pass unchanged |
| Oversized outbound UPDATE | → | split logic (SplitWireUpdate, now in `bgp/packet`) | existing split unit tests move with the file; `test/encode/*.ci` pass unchanged |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | after fold | `internal/component/bgp/wireu/` does not exist; its files live in `internal/component/bgp/packet/` with package clause `packet`; `errors.go` collision resolved (e.g. `errors_update.go`); doc.go content merged into the destination doc.go |
| AC-2 | repo-wide grep for the old import path and `wireu.` qualifier | zero hits in code and living docs (plan/learned history exempt) |
| AC-3 | `scripts/dev/protocol_skeleton_report.py` | summary `legacy 1` (only `bgp/reactor`); `--selftest` OK with ("bgp","wireu") fixtures removed |
| AC-4 | `make ze-doc-test` | green after the 17-anchor + prose sweep |
| AC-5 | rule surfaces | go-standards.md glossary `wireu` row removed (decision recorded as superseded); protocol.md BGP row updated |
| AC-6 | fuzz | `FuzzRewriteASPath` seed corpus found and passing from its new location |
| AC-7 | `make ze-verify` | green, including regenerated `ai/PACKAGE-MAP.md` |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Receives and forwards UPDATEs, including ones requiring split | wire -> packet (single codec package) -> reactor -> peers | `test/decode/`, `test/encode/` suites unchanged |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| all wireu tests move with their files | `internal/component/bgp/packet/*_test.go` | behavior identical post-fold | |
| report selftest (fail-first: flip ("bgp","wireu") fixture expectations before the table edit) | `scripts/dev/protocol_skeleton_report.py` | exception retired | |
| fuzz seed pass | `FuzzRewriteASPath` from new corpus path | corpus moved correctly | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | no numeric inputs (move only) | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing decode suites | `test/decode/bgp-flow-1.ci` (and siblings) | lazy UPDATE parse unchanged | |
| existing encode suites | `test/encode/attributes.ci` (and siblings) | encode + split unchanged | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| not applicable | - | - | fold changes no wire bytes; justification per umbrella | |

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/component/bgp/wireu/*.go` -> `internal/component/bgp/packet/` (git mv; package clause; `message.` qualifiers dropped; `errors.go` -> collision-free name; doc.go merged)
- `internal/component/bgp/wireu/testdata/` -> `internal/component/bgp/packet/testdata/` (fuzz corpus)
- 47 importer files - import path + `wireu.` -> `packet.` qualifiers (~193 lines)
- `ai/rules/go-standards.md` - glossary `wireu` row removed
- `ai/rules/protocol.md` - BGP probe row updated
- `scripts/dev/protocol_skeleton_report.py` - LEGACY_EXCEPTIONS ("bgp","wireu") removed; selftest fixtures updated
- docs holding the 17 anchors (incl. `docs/architecture/wire/messages.md`) - anchor + prose sweep
- `ai/PACKAGE-MAP.md` - regenerated

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | not needed: no config change |
| CLI commands/flags | [ ] | not needed |
| Doctor check | [ ] | not needed: no new runtime dependency |
| Prometheus counters | [ ] | not needed |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | No |
| 2 | Config syntax changed? | [ ] | No |
| 3 | CLI command added/changed? | [ ] | No |
| 4 | API/RPC added/changed? | [ ] | No |
| 5 | Plugin added/changed? | [ ] | No |
| 6 | Has a user guide page? | [ ] | No |
| 7 | Wire format changed? | [ ] | No bytes change; wire architecture doc prose updated to one-codec-package structure |
| 8 | Plugin SDK/protocol changed? | [ ] | No |
| 9 | RFC behavior implemented/changed/newly proven? | [ ] | No |
| 10 | Test infrastructure changed? | [ ] | No |
| 11 | Affects daemon comparison? | [ ] | No |
| 12 | Internal architecture changed? | [ ] | Yes - `docs/architecture/wire/messages.md` and the other anchor-holding docs describe the merged package |
| 13 | Route metadata keys added/changed? | [ ] | No |
| 14 | Prometheus counters added/changed? | [ ] | No |
| 15 | Registered inventory changed? | [ ] | No |
| 16 | Changed source referenced by doc anchors? | [ ] | Yes - 17 anchors; gated by `make ze-doc-test` |
| 17 | Docs show examples for this area? | [ ] | No examples carry package paths |

## Files to Create
- None net-new (files move; collision renames happen inside the destination).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | rerun A-1..A-5 (clash audits are phase 1, BLOCKING) |
| 3. Wiring phase | Wiring Test table (existing chains) |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6-9. Reviews + fixes | Critical Review Checklist below |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | two-commit closure |

### Implementation Phases
1. **Phase: audit rerun (BLOCKING)** — rerun identifier-clash comms (both visibility levels, tests included), filename comm, cross-import grep. A clash = STOP, present the colliding symbols to the user.
   - Tests: paste audit outputs into this spec
   - Files: this spec (annotations)
   - Verify: A-1, A-2, A-3 re-confirmed; A-5 confirmed
2. **Phase: report selftest fail-first** — flip ("bgp","wireu") fixtures; paste the red.
   - Tests: `scripts/dev/protocol_skeleton_report.py --selftest`
   - Files: `scripts/dev/protocol_skeleton_report.py`
   - Verify: red proves fixture teeth
3. **Phase: the fold** — git mv files + testdata; package clause; drop `message.` qualifiers inside moved files; resolve doc.go/errors.go collisions; rewrite the 47 importers; remove the exceptions row.
   - Tests: `go build ./...`, `make ze-unit-test`, fuzz seed pass, report `--selftest` green
   - Files: per Files to Modify
   - Verify: AC-1, AC-2 (code), AC-3, AC-6
4. **Phase: rule + doc sweep** — go-standards.md, protocol.md, 17 anchors + prose, PACKAGE-MAP regen.
   - Tests: `make ze-doc-test`
   - Files: per Files to Modify
   - Verify: AC-4, AC-5
5. **Full verification** — `make ze-verify`; AC-7.
6. **Complete spec** — audit tables, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N with fresh evidence; final greps pasted |
| Correctness | moved files' hunks are mechanical (package clause, dropped qualifiers, filename); any logic hunk is a STOP |
| Data flow | the former inter-package edge is now internal; no import cycles introduced elsewhere |
| Naming | destination doc.go explains the two halves (eager codec / lazy view) per R-2 |
| Rule: no-layering | no alias package or re-export shim left at `bgp/wireu` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| single codec package builds | `go build ./internal/component/bgp/...` |
| zero old references | repo grep for `bgp/wireu` and `wireu.` returns only history |
| report updated | run report; summary says `legacy 1` |
| fuzz corpus attached | run the fuzz target seed pass from the new path |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| No behavior change | no validation, bounds check, or error path altered; zero-copy iterators untouched |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Clash found at audit rerun | STOP: present colliding symbols; agree renames before the fold |
| Compile error | missed reference or collision; fix within the fold commit |
| Any test changes behavior | STOP: fold must be behavior-neutral |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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
- (fill during implementation)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fold into `packet` | rename to `update` (collides with a ubiquitous local identifier); rename to `wireupdate` (clunky, off-glossary); keep + document (the earlier 2026-07-08 decision, recorded in wireu/doc.go) | user approved the fold 2026-07-08 after the clash audit; one protocol, one codec package per the skeleton. This spec supersedes the doc.go keep record |
| Separate spec from the rename | fold during spec-rename-2 | keeps the 124-file rename a pure rename; the fold is a semantic merge reviewed on its own |
| errors.go collision -> rename the incoming file | merge both error files into one | the two error sets are disjoint concerns (header/wire-format vs UPDATE-parse); a mechanical filename change is lower-risk than merging var blocks |

## Known Limitations
- After this spec the merged `bgp/packet` is a large package (roughly 62 files). Accepted: BGP is the platform archetype, and the skeleton values one discoverable codec home over package-count aesthetics.

## RFC Documentation
Not applicable: existing `// RFC 4271/4760/7911` comments move verbatim with their code.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

## Implementation Audit

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| one codec package, zero behavior change | functional test | (fill: `make ze-verify` output + old-path greps + report summary) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (fill during implementation)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
