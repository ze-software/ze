# Spec: rename-0-umbrella -- retire the legacy package names

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | 0/3 |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/go-standards.md` ("Package-Naming Glossary") and `ai/rules/protocol.md`
4. `scripts/dev/protocol_skeleton_report.py` (LEGACY_EXCEPTIONS table)

## Task

The package-naming glossary (spec-layout-3) and the protocol skeleton
(spec-layout-4) documented four legacy package names as exceptions rather than
renaming them: `bgp/message`, `bgp/wireu`, `bgp/reactor`, `ike/wire`
(`scripts/dev/protocol_skeleton_report.py` LEGACY_EXCEPTIONS). The user
decided on 2026-07-08 to retire three of them. This umbrella coordinates the
set; each child is one atomic pure-rename (or merge) commit pair.

| Child | Spec | Change | Weight (measured 2026-07-08) | Ordering |
|-------|------|--------|------------------------------|----------|
| 1 | `spec-rename-1-ike-packet.md` | `ike/wire` -> `ike/packet` | 17 importer files (all inside `ike/engine`), ~408 qualifier lines, 2 doc anchors | ready now; no overlap with in-flight work |
| 2 | `spec-rename-2-bgp-packet.md` | `bgp/message` -> `bgp/packet` | 124 importer files (45 tests), ~1101 qualifier lines, 34 doc anchors | blocked on the rib-arch spec set (reworks the same trees) |
| 3 | `spec-rename-3-wireu-fold.md` | fold `bgp/wireu` into `bgp/packet` | 47 importer files (26 tests), ~193 qualifier lines, 17 doc anchors | blocked on child 2 (needs `packet` to exist) |

**`bgp/reactor` is deliberately excluded.** It is the open decomposition
candidate from spec-layout-0 (69 non-test files) and carries 331 of the 384
doc anchors. Renaming it before decomposing means churning 154 package
clauses and 331 anchors twice, and rib-arch-8 is editing its files now
(`plan/spec-rib-arch-8-nlri-rewrite.md` targets `reactor/filter_delta.go`).
The name dies with the decomposition, whichever destination that work gets.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/go-standards.md` - "Package-Naming Glossary" defines the target vocabulary
  → Decision: `packet` = protocol wire codec; `wire` = primitives/raw handoff, with `ike/wire` recorded as the exception these specs remove
  → Constraint: glossary rows for `wireu` and the `ike/wire` exception must be updated in the same commit as each rename
- [ ] `ai/rules/protocol.md` - probe table + exceptions list mirror LEGACY_EXCEPTIONS
  → Constraint: exceptions table, probe rows, and `protocol_skeleton_report.py` LEGACY_EXCEPTIONS + selftest fixtures must stay in sync per child
- [ ] `docs/architecture/wire/messages.md` - the `// Design:` anchor of both `message` and `wireu` files
  → Constraint: prose and source anchors in this doc change in children 2 and 3
- [ ] `ai/rules/git-safety.md` - commit mechanics
  → Constraint: each child is a pure rename/merge commit with no logic edits, so `git log --follow` stays usable

### RFC Summaries (MUST for protocol work)
- [ ] Not applicable: renames change no protocol behavior, no wire bytes, no RFC-covered semantics.

**Key insights:**
- All four names are documented exceptions today; removing one = rename + rule-surface sync + doc-anchor sweep + full verify.
- Zero aliased imports of any of the four packages exist (grep audit 2026-07-08), so qualifier rewrites are uniform.
- `pkg/` (external SDK) has zero references to any of the four; nothing user-visible carries these names (the ExaBGP topic string "bgp.reactor" at `internal/exabgp/topics/topics.go` is an independent compat API name, untouched).

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `scripts/dev/protocol_skeleton_report.py` - LEGACY_EXCEPTIONS at :56-61 lists the four (protocol, module) pairs; selftest asserts their classification
- [ ] `internal/component/bgp/message/message.go` - `package message`: Message interface, writeHeader; full codec for OPEN/UPDATE/NOTIFICATION/KEEPALIVE/ROUTE-REFRESH
- [ ] `internal/component/bgp/wireu/doc.go` - `package wireu` ("wire UPDATE"): lazy zero-copy UPDATE parsing; records the 2026-07-08 keep decision this umbrella supersedes
- [ ] `internal/component/bgp/wireu/split.go` - imports `bgp/message` (:9), proving fold direction has no import cycle
- [ ] `internal/component/ike/wire/doc.go` - `package wire` encodes and decodes IKEv2 messages, headers, payloads (a full codec, i.e. glossary `packet`)

**Behavior to preserve:**
- All wire formats, message semantics, exported API shapes (identifiers keep their names; only package name and import path change)
- All existing functional suites (`test/decode/`, `test/encode/`, `test/parse/ipsec-*.ci`) must pass unchanged
- Fuzz corpus `internal/component/bgp/wireu/testdata/fuzz/FuzzRewriteASPath` moves with its package

**Behavior to change:**
- Package names and import paths only, per the children table. No functional change anywhere.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- No new data enters. The affected packages sit on existing paths: BGP wire bytes -> `message`/`wireu` codecs -> reactor/RIB; IKE wire bytes -> `ike/wire` codec -> `ike/engine`.

### Transformation Path
1. Renames relocate the codec stages in the import graph without changing any transformation.
2. Child 3 merges two stages of the same path (eager message codec + lazy UPDATE view) into one package; call graph is unchanged.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None new | pure rename/merge; no boundary is added, moved, or removed | [ ] |

### Integration Points
- `scripts/dev/protocol_skeleton_report.py` LEGACY_EXCEPTIONS - one row removed per child
- `ai/rules/go-standards.md` glossary + `ai/rules/protocol.md` exceptions/probe - rows updated per child
- `ai/PACKAGE-MAP.md` - regenerated (`make ze-discovery-index-update`) per child

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — nothing new registers; renames only

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No identifier clashes between `message` and `wireu` (fold is a clean merge) | comm audit 2026-07-08: 0 clashes across exported (60+81 vs 24+7) and unexported (108 vs 34) top-level names, tests included | fold needs renames first; child 3 redesign | audit rerun at child-3 start (packages may drift before then) | confirmed (2026-07-08 snapshot; re-check at child-3 start) |
| A-2 | No local identifiers named `packet` shadow the new qualifier in any importer | grep audit 2026-07-08 over all importer files of the three packages: zero `packet :=` / `var packet` / `packet []byte` declarations | qualifier rewrite produces compile errors; rename locals first | `go build ./...` after each rename | confirmed (2026-07-08 snapshot; recompile validates) |
| A-3 | No external surface carries the legacy names | `pkg/` grep: zero references; no quoted string literal ties telemetry/config/API to the package names; ExaBGP topic "bgp.reactor" (`internal/exabgp/topics/topics.go`) names a topic, not the package | a rename would break users | grep audit rerun per child | confirmed |
| A-4 | The rib-arch spec set owns the BGP trees until it closes | `plan/spec-rib-arch-*.md` uncommitted in another session; rib-arch-8 lists `reactor` and NLRI-path files | children 2-3 conflict with in-flight branches | `make ze-spec-status` shows rib-arch set closed before starting child 2 | satisfied (rechecked 2026-07-22: rib-arch set fully closed -- no `spec-rib-arch-*.md` remains in plan/, learned 1128 + children 1123-1154 on disk; child 2 flipped to ready the same day) |
| A-5 | Only `doc.go` and `errors.go` collide by filename between `message` and `wireu` | comm over both directory listings 2026-07-08 | more file merges needed in child 3 | rerun listing comm at child-3 start | confirmed (2026-07-08 snapshot) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Concurrent sessions have uncommitted work touching renamed files; a big rename conflicts with everything in flight | `git status` shows other-session modifications in the affected trees | Land each child as one atomic commit pair in a quiet window; children 2-3 stay blocked until rib-arch closes |
| R-2 | Doc sweep fixes anchors (gated by `make ze-doc-verify`) but misses prose mentions of old paths | prose still says `bgp/message` after anchors are green | Per child, grep docs/ and ai/ for the old path in BOTH anchor and prose form; the AC requires zero hits |
| R-3 | Mixing logic edits into a rename commit destroys `git log --follow` usability and bloats review | diff shows non-mechanical hunks | Pure rename/merge commits only; any discovered logic fix goes in a separate commit before or after |
| R-4 | Historical records (plan/learned/, rfc-may-decisions, learned summaries) mention old paths | grep hits under plan/learned/ | Leave history untouched; it describes the past accurately. Only living docs (docs/, ai/) are updated |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| No new entry points (umbrella; renames only) | → | children keep every existing chain intact | existing suites: `test/decode/*.ci`, `test/encode/*.ci`, `test/parse/ipsec-*.ci` pass unchanged per child |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | All three children closed | `scripts/dev/protocol_skeleton_report.py` summary shows `legacy 1` (only `bgp/reactor` remains) |
| AC-2 | Any child closed | repo-wide grep for that child's old import path returns zero hits (code, docs anchors, docs prose in living docs) |
| AC-3 | Umbrella closure | reactor exclusion recorded here with its destination (the spec-layout-0 decomposition decision), per no-deferral-without-destination |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers send BGP UPDATEs / IKE exchanges before and after each rename | identical wire path, relocated packages | full `make ze-precommit-verify` per child; suites named per child spec |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| existing package tests move with each package | per child spec | rename did not change behavior | |
| report selftest updated per child | `scripts/dev/protocol_skeleton_report.py` | LEGACY_EXCEPTIONS row removal (fail-first: update the selftest expectation before the table) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | no numeric inputs (renames only) | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing decode suite | `test/decode/bgp-flow-1.ci` (and siblings) | BGP UPDATE decode unchanged after children 2-3 | |
| existing encode suite | `test/encode/attributes-encode.ci` (and siblings) | BGP UPDATE encode unchanged after children 2-3 | |
| existing IPsec parse suite | `test/parse/ipsec-eap-auth.ci` (and siblings) | IKE config + engine unchanged after child 1 | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| not applicable | - | - | renames change no wire behavior; existing interop scenarios remain the proof | |

### Future (if deferring any tests)
- None.

## Files to Modify
- Per child spec. Umbrella-owned shared surfaces, touched once per child:
- `ai/rules/go-standards.md` - glossary rows (`wireu`, `wire` exception)
- `ai/rules/protocol.md` - probe rows + exceptions table
- `scripts/dev/protocol_skeleton_report.py` - LEGACY_EXCEPTIONS + selftest fixtures
- `ai/PACKAGE-MAP.md` - regenerated per child (`make ze-discovery-index-update`)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | not needed: no config surface changes |
| CLI commands/flags | [ ] | not needed: no CLI changes |
| Env var registration | [ ] | not needed |
| Doctor check for runtime dependencies | [ ] | not needed: no new runtime dependency |
| Prometheus counters/metrics | [ ] | not needed: no observable-state change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | No - internal rename |
| 2 | Config syntax changed? | [ ] | No |
| 3 | CLI command added/changed? | [ ] | No |
| 4 | API/RPC added/changed? | [ ] | No |
| 5 | Plugin added/changed? | [ ] | No |
| 6 | Has a user guide page? | [ ] | No |
| 7 | Wire format changed? | [ ] | No bytes change; `docs/architecture/wire/messages.md` prose/anchors updated in children 2-3 |
| 8 | Plugin SDK/protocol changed? | [ ] | No (`pkg/` has zero references) |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] | No |
| 10 | Test infrastructure changed? | [ ] | No |
| 11 | Affects daemon comparison? | [ ] | No |
| 12 | Internal architecture changed? | [ ] | Yes - per child: architecture docs whose anchors point into renamed dirs (34 + 17 + 2 anchors measured) |
| 13 | Route metadata keys added/changed? | [ ] | No |
| 14 | Prometheus counters added/changed? | [ ] | No |
| 15 | Registered plugin/event/send/command/capability inventory changed? | [ ] | No |
| 16 | Any changed source file referenced by existing doc source anchors? | [ ] | Yes - the per-child anchor sweep IS the doc work; `make ze-doc-verify` gates it |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] | No examples carry package paths |

## Files to Create
- `plan/spec-rename-1-ike-packet.md`, `plan/spec-rename-2-bgp-packet.md`, `plan/spec-rename-3-wireu-fold.md` (this set)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| all stages | executed per child spec; the umbrella only tracks ordering and shared decisions |

### Implementation Phases
1. **Child 1** (`spec-rename-1-ike-packet.md`) - can start immediately.
2. **Child 2** (`spec-rename-2-bgp-packet.md`) - after the rib-arch set closes.
3. **Child 3** (`spec-rename-3-wireu-fold.md`) - after child 2; rerun the clash audit first.
4. **Umbrella closure** - when children close and the reactor exclusion has its recorded destination.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | per child: old path gone everywhere; rule surfaces synced; report count dropped |
| Rule: no-layering | no compatibility shims, no forwarding aliases left behind |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| three closed children | `make ze-spec-status` shows none of the rename children open |
| legacy count 1 | run `scripts/dev/protocol_skeleton_report.py`, read summary line |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| No behavior change | renames must not alter validation, bounds checks, or error paths; diff review confirms mechanical-only hunks |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compile error after rename | fix the missed reference in the same commit; it is part of the rename |
| Functional suite fails | STOP: a rename must not change behavior; find the non-mechanical hunk |
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
| Fold `wireu` into `packet` (child 3) | rename to `update` (collides with a very common local identifier); rename to `wireupdate` (clunky, still off-glossary); keep + document (the 2026-07-08 morning decision) | user approved the fold 2026-07-08 after the clash audit came back clean; two codec packages for one protocol is itself a skeleton divergence. This supersedes the keep decision recorded in `internal/component/bgp/wireu/doc.go` |
| Exclude `bgp/reactor` from this set | rename it to `engine` now (only 15 importers) | 331 doc anchors + 154 package clauses would churn twice; rib-arch-8 edits its files now; the decomposition (spec-layout-0 open candidate) retires the name as a side effect |
| One pure rename/merge commit pair per child | batch all renames in one commit | atomic pure renames keep `git log --follow` usable and keep conflict windows short |
| Child ordering 1 -> (rib-arch) -> 2 -> 3 | start with the biggest (message) | child 1 is conflict-free today; 2 and 3 collide with rib-arch's trees |

## Known Limitations
- `bgp/reactor` keeps its name until the decomposition lands; the report's legacy count floor for this set is 1, not 0.
- Historical records (plan/learned/, closed-spec history) keep old path spellings; only living docs are swept.

## RFC Documentation
Not applicable: no RFC-covered behavior changes.

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
| retire three legacy names with zero behavior change | functional test | (fill: per-child `make ze-precommit-verify` runs + report summary line) |

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
- [ ] `make ze-standard-test` passes (lint + all ze tests)
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
