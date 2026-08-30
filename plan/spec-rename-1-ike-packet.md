# Spec: rename-1-ike-packet -- rename ike/wire to ike/packet

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

Umbrella: `spec-rename-0-umbrella.md`. Siblings: `spec-rename-2-bgp-packet.md`, `spec-rename-3-wireu-fold.md`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/go-standards.md` ("Package-Naming Glossary"), `ai/rules/protocol.md`
4. `internal/component/ike/wire/doc.go`, `internal/le/protocolskeleton/protocolskeleton.go`

## Task

`internal/component/ike/wire` is a full IKEv2 codec (parse + encode of
messages, headers, payloads) that the glossary names `packet`; its `wire`
name is one of the four documented legacy exceptions
(`internal/le/protocolskeleton/protocolskeleton.go`). Rename the package and
directory to `internal/component/ike/packet`, package identifier `wire` ->
`packet`, and retire the exception from every rule surface. Pure rename: no
logic edits, no API change, one atomic commit pair.

Measured scope (2026-07-08): 34 files in the package; 17 importer files, all
inside `internal/component/ike/engine/`; ~408 lines with the `wire.`
qualifier; 2 doc source anchors; zero aliased imports; zero local `packet`
identifiers in any importer.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. -->
- [ ] `ai/rules/go-standards.md` - "Package-Naming Glossary"
  → Decision: `packet` is the glossary term for a protocol wire codec; the `wire` row's "ike exception" clause is removed by this spec
  → Constraint: glossary edit ships in the same commit as the rename
- [ ] `ai/rules/protocol.md` - IKE probe row + exceptions
  → Constraint: after this spec, IKE maps cleanly with no exceptions; probe row and exceptions table both change
- [ ] `docs/architecture/bfd.md` is NOT affected; the 2 anchors into `ike/wire` live elsewhere - grep `source: internal/component/ike/wire` to locate them at implementation time
  → Constraint: `./le doc check verify` gates the anchor sweep (`internal/le/docstocode/codetodocs.go` does literal path-exists checks)

### RFC Summaries (MUST for protocol work)
- [ ] Not applicable: no protocol behavior changes; RFC 7296 semantics are untouched (rename only).

**Key insights:**
- All importers are inside one sibling package (`ike/engine`), so the blast radius is a single component.
- No interop or functional behavior can change; the gate is "everything that was green stays green."

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/wire/doc.go` - "Package wire encodes and decodes IKEv2 messages, headers, and payloads" (:1-2); a codec, i.e. glossary `packet`
- [ ] `internal/component/ike/wire/` listing - chain.go, header.go, message.go, payload_*.go + tests; 34 files, flat (no subdirs)
- [ ] `internal/component/ike/engine/register.go` and siblings - the 17 importers; qualifier form `wire.X` throughout (~408 lines)
- [ ] `internal/le/protocolskeleton/protocolskeleton.go` - LEGACY_EXCEPTIONS holds ("ike", "wire") (:60); selftest asserts `classify_module("ike", "wire") == "legacy-exception"` and `classify_module("ospf", "wire") == "domain"`

**Behavior to preserve:**
- Every exported identifier keeps its name; only the package qualifier changes
- `test/parse/ipsec-*.ci` suites and all `ike/...` unit tests pass unchanged
- The `ospf/wire` classification stays `domain` (that package is a raw handoff type, correctly named, not touched)

**Behavior to change:**
- Package path and identifier only. None functional.

## Data Flow (MANDATORY)

### Entry Point
- Unchanged: IKE datagrams enter via `ike/transport`, are decoded by this codec, and drive `ike/engine`.

### Transformation Path
1. Same pipeline before and after; only the codec package's name changes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None new | pure rename | [ ] |

### Integration Points
- `ike/engine` (17 files) - import path + qualifier rewrite
- `internal/le/protocolskeleton/protocolskeleton.go` - exception row removed, selftest updated
- `ai/rules/go-standards.md`, `ai/rules/protocol.md` - rows updated
- `ai/PACKAGE-MAP.md` - regenerated

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
| A-1 | All importers live under `internal/component/ike/` | repo-wide import grep 2026-07-08 listed 17 files, all in `ike/engine` | wider rewrite needed | rerun the import grep at implementation start | confirmed (2026-07-08 snapshot) |
| A-2 | No local `packet` identifier in any importer | grep audit 2026-07-08: zero declarations | compile errors after rewrite | `go build ./internal/component/ike/...` | confirmed (2026-07-08 snapshot) |
| A-3 | Exactly 2 doc source anchors point into `ike/wire` | anchor grep 2026-07-08 | doc-test failures reveal more | `./le doc check verify` after sweep | confirmed (2026-07-08 snapshot) |
| A-4 | No string literal, YANG node, metric, or CLI word depends on the package name | pkg/ grep zero refs; no quoted literals found carrying the name | user-visible break | repo grep for `ike/wire` after rename must return only history (plan/learned) | confirmed (2026-07-08 snapshot) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Another session has uncommitted edits under `ike/` when the rename lands | `git status` shows ike/ files modified before starting | check status first; land in a quiet window (umbrella R-1) |
| R-2 | A missed reference form (build tag file, generated code, script) escapes the grep | build or gate failure after rename | full `./le verify current mode full`; final repo-wide grep for the old path is an AC |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE exchange bytes from a peer via `ike/transport` | → | `ike/packet` decode -> `ike/engine` handlers (e.g. `engine/inbound.go`, `engine/responder.go`) | existing `go test ./internal/component/ike/...` suites (engine tests exercise decode/encode round-trips) pass unchanged after rename |
| IPsec/IKE config load | → | engine registration (`ike/engine/register.go`) | `test/parse/ipsec-eap-auth.ci`, `test/parse/ipsec-remote-access.ci` pass unchanged |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | after rename | `internal/component/ike/wire/` does not exist; `internal/component/ike/packet/` builds; package clause is `packet` in all 34 files |
| AC-2 | repo-wide grep for the old import path | zero hits in code, scripts, docs/ and ai/ (living surfaces); history under plan/learned/ exempt |
| AC-3 | `internal/le/protocolskeleton/protocolskeleton.go` | summary shows `legacy 3`; `--verbose` shows `ike: ... packet=canonical`; `--selftest` OK with ("ike", "wire") fixtures removed and `ospf/wire == domain` retained |
| AC-4 | `./le doc check verify` | green after the 2 anchors (and any prose mentions) are updated |
| AC-5 | rule surfaces | `ai/rules/go-standards.md` `wire` row no longer carries an ike exception; `ai/rules/protocol.md` IKE probe row says "none" under Exceptions |
| AC-6 | `./le verify current mode full` | green, including regenerated `ai/PACKAGE-MAP.md` |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Establishes an IKE/IPsec session | transport -> packet (renamed codec) -> engine FSM | existing `ike/engine` unit suites + `test/parse/ipsec-*.ci`, all unchanged |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| all existing `wire_test` files move with the package | `internal/component/ike/packet/*_test.go` | behavior identical post-rename | |
| report selftest (fail-first: flip the fixture expectations BEFORE editing LEGACY_EXCEPTIONS; red proves teeth) | `internal/le/protocolskeleton/protocolskeleton.go` | ("ike","wire") no longer an exception | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | no numeric inputs (rename only) | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing IPsec parse suites | `test/parse/ipsec-eap-auth.ci`, `test/parse/ipsec-remote-access.ci` | IKE config + engine behavior unchanged | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| not applicable | - | - | rename changes no wire bytes; justification per umbrella | |

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/component/ike/wire/` -> `internal/component/ike/packet/` (git mv; package clause in 34 files)
- `internal/component/ike/engine/*.go` - 17 files: import path + `wire.` -> `packet.` qualifiers (~408 lines)
- `ai/rules/go-standards.md` - glossary `wire` row: drop the ike exception clause
- `ai/rules/protocol.md` - IKE probe row exceptions -> none; exceptions table row removed
- `internal/le/protocolskeleton/protocolskeleton.go` - LEGACY_EXCEPTIONS ("ike","wire") removed; selftest fixtures updated
- docs with the 2 source anchors into `ike/wire` (locate by grep at implementation time) - anchor + prose sweep
- `ai/PACKAGE-MAP.md` - regenerated (`./le discovery-index update`)

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
| 7 | Wire format changed? | [ ] | No |
| 8 | Plugin SDK/protocol changed? | [ ] | No |
| 9 | RFC behavior implemented/changed/newly proven? | [ ] | No |
| 10 | Test infrastructure changed? | [ ] | No |
| 11 | Affects daemon comparison? | [ ] | No |
| 12 | Internal architecture changed? | [ ] | Yes - the docs holding the 2 `ike/wire` anchors get path + prose updates |
| 13 | Route metadata keys added/changed? | [ ] | No |
| 14 | Prometheus counters added/changed? | [ ] | No |
| 15 | Registered inventory changed? | [ ] | No |
| 16 | Changed source referenced by doc anchors? | [ ] | Yes - the 2 anchors; gated by `./le doc check verify` |
| 17 | Docs show examples for this area? | [ ] | No examples carry package paths |

## Files to Create
- None (rename only; learned summary at closure per template).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify; rerun A-1..A-4 greps |
| 3. Wiring phase | Wiring Test table (existing chains; no new entry points to register) |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `./le verify lint run && ./le test-unit  && ./le functional` |
| 6-9. Reviews + fixes | Critical Review Checklist below |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | two-commit closure |

### Implementation Phases
1. **Phase: report selftest fail-first** — flip the ("ike","wire") selftest expectations; run `--selftest`, paste the red; this proves the fixtures have teeth.
   - Tests: `internal/le/protocolskeleton/protocolskeleton.go --selftest`
   - Files: `internal/le/protocolskeleton/protocolskeleton.go`
   - Verify: selftest fails because LEGACY_EXCEPTIONS still holds the row
2. **Phase: the rename** — git mv the directory; rewrite package clause, import paths, qualifiers; remove the LEGACY_EXCEPTIONS row.
   - Tests: `go build ./...`, `go test ./internal/component/ike/...`, report `--selftest` green
   - Files: `internal/component/ike/packet/`, `internal/component/ike/engine/*.go`, `internal/le/protocolskeleton/protocolskeleton.go`
   - Verify: AC-1, AC-2 (code), AC-3
3. **Phase: rule + doc sweep** — go-standards.md, protocol.md, the 2 anchors + prose, regenerate PACKAGE-MAP.
   - Tests: `./le doc check verify`, `./le rules index-update` if rule headers changed
   - Files: per Files to Modify
   - Verify: AC-4, AC-5
4. **Full verification** — `./le verify current mode full`; AC-6.
5. **Complete spec** — audit tables, learned summary, two-commit closure (commit A: rename + rules + docs + spec + learned; commit B: git rm spec).

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N with fresh evidence; final repo grep for old path pasted |
| Correctness | diff contains ONLY mechanical hunks (path, package clause, qualifier); any logic hunk is a STOP |
| Naming | new package doc comment names the package `packet` and keeps the codec description |
| Rule: no-layering | no alias package, no forwarding shim left at the old path |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| renamed package builds | `go build ./internal/component/ike/...` |
| zero old-path references | `grep -rn "ike/wire" --include='*.go' .` returns nothing; docs/ai grep returns only history |
| report updated | run report; summary says `legacy 3` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| No behavior change | no validation, bounds check, or error path altered; mechanical-hunk review is the check |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compile error | missed reference; fix within the rename commit |
| Any test changes behavior | STOP: rename must be behavior-neutral; find the non-mechanical hunk |
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
| Target name `packet` | keep `wire` (documented exception) | the package is a full codec; `packet` is the glossary term; user approved retiring the exceptions 2026-07-08 |
| First child of the set | start with bgp/message | ike is conflict-free today (all importers in one component; no rib-arch overlap) |

## Known Limitations
- None beyond the umbrella's (history keeps old spellings).

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
- [ ] `./le verify worktree` passes (lint + all ze tests)
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
