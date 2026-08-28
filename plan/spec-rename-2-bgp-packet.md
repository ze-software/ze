# Spec: rename-2-bgp-packet -- rename bgp/message to bgp/packet

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-rib-arch-0-umbrella.md (closed, learned 1128) |
| Phase | - |
| Updated | 2026-07-22 |

Unblocked 2026-07-22: the entire rib-arch set this spec was blocked on has
closed (learned 1123, 1125-1128, 1150-1152, 1154). The 2026-07-08 scope
measurements (124 importers, 34 anchors) predate the `ze_bgp` feature-gate
work (commit c4038def0), so the re-measure step this spec already mandates
at start-of-work is now doubly important.

Umbrella: `spec-rename-0-umbrella.md`. Siblings: `spec-rename-1-ike-packet.md`, `spec-rename-3-wireu-fold.md` (which depends on this spec).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/go-standards.md` ("Package-Naming Glossary"), `ai/rules/protocol.md`
4. `internal/component/bgp/message/message.go`, `internal/le/protocolskeleton/protocolskeleton.go`

## Task

`internal/component/bgp/message` is the eager BGP codec (OPEN, UPDATE,
NOTIFICATION, KEEPALIVE, ROUTE-REFRESH parse + encode; Message interface and
writeHeader in `message.go`). The glossary names a protocol codec `packet`;
`message` is a documented legacy exception
(`internal/le/protocolskeleton/protocolskeleton.go`). Rename directory and
package to `internal/component/bgp/packet`, retire the exception. Pure
rename: no logic edits, one atomic commit pair. Sibling spec-rename-3 then
folds `wireu` into the renamed package.

Measured scope (2026-07-08): 42 files in the package; 124 importer files (45
tests), including outside the BGP tree (`internal/perf`,
`internal/chaos/peer`, `test/integration`); ~1101 lines with the `message.`
qualifier; 34 doc source anchors; zero aliased imports; zero local `packet`
identifiers in any importer; hardcoded fixture paths in
`internal/le/` and `:232`.

**BLOCKED until the rib-arch spec set closes:** rib-arch reworks the NLRI and
decode paths that import this package (`plan/spec-rib-arch-8-nlri-rewrite.md`
and siblings); a 124-file rename would conflict with every in-flight branch.
Check the retired `ze-spec-status` (current: `./le spec-status`) before starting.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. -->
- [ ] `ai/rules/go-standards.md` - "Package-Naming Glossary"
  → Decision: `packet` = protocol wire codec; the glossary's BGP-legacy row (`message` kept as historical) is updated by this spec
  → Constraint: glossary edit ships in the same commit as the rename
- [ ] `ai/rules/protocol.md` - BGP probe row lists `message`+`wireu` as the pre-SDK codec pair
  → Constraint: probe row + exceptions table change here (message) and again in spec-rename-3 (wireu)
- [ ] `docs/architecture/wire/messages.md` - the `// Design:` anchor for this package's files
  → Constraint: prose + anchors updated in the same commit; `./le doc-check verify` gates
- [ ] `ai/rules/performance.md` - the encoding architecture this package implements
  → Constraint: rename must not disturb WriteTo(buf, off) patterns; mechanical hunks only

### RFC Summaries (MUST for protocol work)
- [ ] Not applicable: no protocol behavior changes (RFC 4271 semantics untouched; rename only).

**Key insights:**
- Importers extend beyond `internal/component/bgp/` into perf, chaos, and integration-test trees; the qualifier rewrite is repo-wide.
- Two dev-script fixtures hardcode file paths inside the package and are invisible to the Go build; they fail their own checks, not compilation.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/message/message.go` - `package message`; Message interface embedding `bgpctx.WireWriter`; `writeHeader` (RFC 4271 Section 4.1); Detail headers list the per-message files
- [ ] `internal/component/bgp/message/errors.go` - wire format error values (ErrShortRead etc.); note: `wireu/errors.go` also exists, relevant to sibling spec 3
- [ ] `internal/le/protocolskeleton/protocolskeleton.go` - LEGACY_EXCEPTIONS holds ("bgp", "message") (:57)
- [ ] `internal/le/` - fixture rows name `internal/component/bgp/message/pack.go` (:151) and `.../p.go` (:232)

**Behavior to preserve:**
- Every exported identifier keeps its name; only package name and import path change
- `test/decode/*.ci`, `test/encode/*.ci`, integration and perf suites pass unchanged
- The glossary's `message` term row: after this spec it describes a retired name (historical note), not a live package

**Behavior to change:**
- Package path and identifier only. None functional.

## Data Flow (MANDATORY)

### Entry Point
- Unchanged: BGP session bytes -> header/message parse (this package) -> reactor/FSM dispatch; encode path mirrors it outbound.

### Transformation Path
1. Same pipeline before and after; only the codec package's name changes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None new | pure rename | [ ] |

### Integration Points
- 124 importer files across bgp, perf, chaos, integration trees - import path + qualifier rewrite
- `internal/le/` fixtures - path strings updated
- `internal/le/protocolskeleton/protocolskeleton.go`, `ai/rules/go-standards.md`, `ai/rules/protocol.md`, `ai/PACKAGE-MAP.md` - per-child rule-surface sync

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable
- [ ] Registration over hardcoding — nothing new registers

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | rib-arch set closed before this starts | Depends field; user ordering decision 2026-07-08 | rename conflicts with in-flight branches | the retired `ze-spec-status` (current: `./le spec-status`) at start | unvalidated (checked at start) |
| A-2 | No local `packet` identifier in any of the 124 importers | grep audit 2026-07-08: zero declarations | compile errors after rewrite | `go build ./...` | confirmed (2026-07-08 snapshot; re-grep at start) |
| A-3 | 34 doc source anchors point into `bgp/message` | anchor grep 2026-07-08 | doc-test reveals more | `./le doc-check verify` after sweep | confirmed (2026-07-08 snapshot) |
| A-4 | No string literal, YANG node, metric, or CLI word depends on the package name | quoted-literal grep 2026-07-08 found none; `pkg/` zero refs | user-visible break | post-rename repo grep AC | confirmed (2026-07-08 snapshot) |
| A-5 | Only the historical hook-parity producer hardcoded paths into this package | 2026-07-08 snapshot | a native hook check breaks post-rename | run `./le hook-check unit` at the start and `./le verify current mode full` after the rename | confirmed (2026-07-08 snapshot) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Largest rename of the set (124 files); long-lived branches conflict | `git status` / open sessions on bgp trees | land in a quiet window; the Depends gate exists for exactly this |
| R-2 | Importers drift between the 2026-07-08 audit and implementation | importer count differs at start | re-measure at start; the numbers here are scope estimates, not ACs |
| R-3 | Qualifier rewrite touches a string or comment that coincidentally says `message.` | diff review shows non-import `message.` hunks in prose/comments | rewrite via gopls package rename (semantic) rather than blind sed; review comment hunks |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP UPDATE bytes from a peer | → | `bgp/packet` parse -> reactor dispatch | `test/decode/bgp-flow-1.ci` (and decode siblings) pass unchanged |
| Route announcement outbound | → | reactor -> `bgp/packet` encode -> wire | `test/encode/attributes-encode.ci`, `test/encode/addpath-encode.ci` pass unchanged |
| Integration harness drives a session | → | `test/integration` imports the renamed package directly | `test/integration/session_test.go` and siblings compile + pass |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | after rename | `internal/component/bgp/message/` does not exist; `internal/component/bgp/packet/` builds; package clause `packet` in all files |
| AC-2 | repo-wide grep for the old import path | zero hits in code, scripts, docs/ and ai/ living surfaces (plan/learned history exempt) |
| AC-3 | `internal/le/protocolskeleton/protocolskeleton.go` | summary `legacy 2` (after spec-1's 3); `--verbose` shows `bgp: ... packet=canonical`; `--selftest` OK with fixtures updated |
| AC-4 | `./le doc-check verify` | green after the 34-anchor + prose sweep |
| AC-5 | rule surfaces | go-standards.md `message` row marked retired/historical; protocol.md BGP row no longer lists `message` |
| AC-6 | `internal/le/` | passes with fixture paths updated to `bgp/packet/` |
| AC-7 | `./le verify current mode full` | green, including regenerated `ai/PACKAGE-MAP.md` |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a BGP session end to end (open, update, keepalive, notification) | wire -> packet codec -> reactor -> RIB and back | `test/decode/`, `test/encode/`, `test/integration` suites unchanged |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| all existing package tests move with the package | `internal/component/bgp/packet/*_test.go` | behavior identical post-rename | |
| report selftest (fail-first: flip fixture expectations before the table edit) | `internal/le/protocolskeleton/protocolskeleton.go` | ("bgp","message") no longer an exception | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | no numeric inputs (rename only) | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing decode suites | `test/decode/bgp-flow-1.ci`, `test/decode/bgp-evpn-1.ci` (and siblings) | UPDATE decode unchanged | |
| existing encode suites | `test/encode/attributes-encode.ci`, `test/encode/addpath-encode.ci` (and siblings) | UPDATE encode unchanged | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| not applicable | - | - | rename changes no wire bytes; existing interop scenarios remain valid unchanged | |

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/component/bgp/message/` -> `internal/component/bgp/packet/` (git mv; package clause in 42 files)
- 124 importer files (list via import grep at start) - import path + `message.` -> `packet.` qualifiers (~1101 lines), including `internal/perf/`, `internal/chaos/peer/`, `test/integration/`
- `internal/le/` - fixture paths at :151 and :232
- `ai/rules/go-standards.md` - glossary `message` row -> retired/historical
- `ai/rules/protocol.md` - BGP probe row + exceptions table
- `internal/le/protocolskeleton/protocolskeleton.go` - LEGACY_EXCEPTIONS ("bgp","message") removed; selftest fixtures updated
- `docs/architecture/wire/messages.md` and the other docs holding the 34 anchors - anchor + prose sweep
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
| 7 | Wire format changed? | [ ] | No bytes change; `docs/architecture/wire/messages.md` prose/anchors updated |
| 8 | Plugin SDK/protocol changed? | [ ] | No (`pkg/` zero references) |
| 9 | RFC behavior implemented/changed/newly proven? | [ ] | No |
| 10 | Test infrastructure changed? | [ ] | No |
| 11 | Affects daemon comparison? | [ ] | No |
| 12 | Internal architecture changed? | [ ] | Yes - all docs holding the 34 anchors; prose sweep included |
| 13 | Route metadata keys added/changed? | [ ] | No |
| 14 | Prometheus counters added/changed? | [ ] | No |
| 15 | Registered inventory changed? | [ ] | No |
| 16 | Changed source referenced by doc anchors? | [ ] | Yes - 34 anchors; gated by `./le doc-check verify` |
| 17 | Docs show examples for this area? | [ ] | No examples carry package paths |

## Files to Create
- None (rename only; learned summary at closure per template).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | rerun A-1..A-5 validations; re-measure importer set |
| 3. Wiring phase | Wiring Test table (existing chains; nothing new to register) |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `./le verify-lint run && ./le test-unit  && ./le functional` |
| 6-9. Reviews + fixes | Critical Review Checklist below |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | two-commit closure |

### Implementation Phases
1. **Phase: gate check + re-measure** — confirm rib-arch closed (`./le spec-status`); rerun the importer/anchor/literal greps; update scope numbers here if drifted.
   - Tests: paste grep evidence into this spec
   - Files: this spec (annotations)
   - Verify: A-1 confirmed; scope current
2. **Phase: report selftest fail-first** — flip ("bgp","message") selftest expectations; paste the red.
   - Tests: `internal/le/protocolskeleton/protocolskeleton.go --selftest`
   - Files: `internal/le/protocolskeleton/protocolskeleton.go`
   - Verify: red proves fixture teeth
3. **Phase: the rename** — git mv; package clause; semantic rewrite of import paths + qualifiers across all importers; LEGACY_EXCEPTIONS row removal; hook-parity fixture paths.
   - Tests: `go build ./...`, `./le test-unit`, report `--selftest` green, hook-parity check green
   - Files: per Files to Modify
   - Verify: AC-1, AC-2 (code), AC-3, AC-6
4. **Phase: rule + doc sweep** — go-standards.md, protocol.md, 34 anchors + prose, PACKAGE-MAP regen.
   - Tests: `./le doc-check verify`
   - Files: per Files to Modify
   - Verify: AC-4, AC-5
5. **Full verification** — `./le verify current mode full`; AC-7.
6. **Complete spec** — audit tables, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N with fresh evidence; final repo grep for old path pasted |
| Correctness | diff is mechanical-only (path, package clause, qualifier); any logic hunk is a STOP |
| Data flow | no import order/initialization change; `register.go` blank-import chains intact |
| Rule: no-layering | no alias or forwarding package left at the old path |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| renamed package builds repo-wide | `go build ./...` |
| zero old-path references | repo grep for `bgp/message` returns only history (plan/learned) |
| report updated | run report; summary says `legacy 2` |
| hook-parity fixtures updated | run `internal/le/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| No behavior change | no validation, bounds check, or error path altered; mechanical-hunk review is the check |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compile error | missed reference; fix within the rename commit |
| Any test changes behavior | STOP: rename must be behavior-neutral |
| rib-arch still open at start | STOP: the Depends gate holds; do not start |
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
| Target name `packet` | keep `message` (documented exception) | glossary term for a protocol codec; user approved retiring the exceptions 2026-07-08 |
| Rename before the wireu fold | fold wireu into `message` then rename both | the pure rename stays pure (history-friendly); the fold (spec-rename-3) is a separate semantic change on top |
| Blocked on rib-arch | land now | 124 importer files across the trees rib-arch is rewriting; conflict cost exceeds any urgency |

## Known Limitations
- Scope numbers (124 importers, ~1101 lines, 34 anchors) are 2026-07-08 measurements; re-measure at start (R-2).

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
| rename with zero behavior change | functional test | (fill: `./le verify current mode full` output + old-path grep) |

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
- [ ] `./le verify current mode full` passes (lint + all ze tests)
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
