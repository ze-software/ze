# Spec: reactor-split -- decompose the bgp/reactor god package

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-rib-arch-0-umbrella.md (closed, learned 1128) |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/go-standards.md` ("Package-Naming Glossary"), `ai/rules/protocol.md`
4. `internal/component/bgp/reactor/` file listing (the prefix clusters below)

## Task

`internal/component/bgp/reactor` is one package of 69 non-test files (154 with
tests, ~64k lines) plus one subpackage (`filter/`). It is the largest
structural divergence found by the 2026-07-08 comparative review
(spec-layout-0 umbrella, closed with this spec as the recorded destination for
its unscheduled candidate). Decompose it into glossary-named subpackages; the
legacy `reactor` name (documented exception,
`scripts/dev/protocol_skeleton_report.py`) dies as a side effect.

Points captured at skeleton creation (2026-07-08), to be re-verified at design:

- The package already clusters by filename prefix: `session_*` (11 files:
  per-connection read/write/negotiate/validate/health), `peer_*` (12 files:
  peer lifecycle, history, stats, BFD, static routes), `forward_*` (9 files:
  UPDATE build + pool/congestion/weight), `filter_*` (5 files + the existing
  `filter/` subdir), `reactor_*` (12 files: API, metrics, peer registry, wire,
  iface), plus singletons (`bufmux.go`, `delivery.go`, `listener.go`,
  `operation.go`, `update_group.go`, ...). These prefixes are the natural
  first-cut split lines.
- New packages take names from the glossary and skeleton rules
  (`ai/rules/go-standards.md`, `ai/rules/protocol.md`): an engine home for
  the runtime loop, `transport`-shaped socket I/O, per-peer state naming per
  the BGP RFC vocabulary (the `fsm` package already exists separately).
- 331 doc source anchors point into the package (measured 2026-07-08); the
  decomposition carries the largest doc sweep of the layout/rename efforts.
- `spec-rename-0-umbrella.md` deliberately excluded `reactor` from the
  legacy-name renames because this spec retires the name; cross-reference the
  exclusion decision there.
- **Blocked on the rib-arch set**: `spec-rib-arch-8-nlri-rewrite.md` edits
  `filter_delta.go` and `reactor_api_forward.go`; a grep across all nine
  rib-arch specs (2026-07-08) confirms none of them decomposes the package,
  so scope is complementary, not duplicate (layout-0 A-7 confirmed).
- Design must decide split boundaries by import/call analysis, not filename
  prefix alone; the clusters above are evidence, not the design.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. -->
- [ ] `ai/rules/go-standards.md` - "Package-Naming Glossary" (fill during design)
- [ ] `ai/rules/protocol.md` - target module vocabulary (fill during design)
- [ ] `docs/architecture/core-design.md` - reactor's documented role (fill during design)
- [ ] `ai/rules/architecture.md` - no tier moves; decomposition stays inside `internal/component/bgp/` (fill during design)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - only if design changes any behavior-adjacent seam (goal is behavior-neutral decomposition; fill during design)

**Key insights:** (fill during design)
- Skeleton note: the decomposition is re-architecture, not layout convention; it was deliberately kept out of the layout children (spec-layout-0 Key Design Decisions).

## Current Behavior (MANDATORY)

**Source files read:** (fill during design; must read BEFORE completing this spec)
- [ ] `internal/component/bgp/reactor/reactor.go` - (fill during design)
- [ ] `internal/component/bgp/reactor/peer.go` - (fill during design)
- [ ] `internal/component/bgp/reactor/session.go` - (fill during design)

**Behavior to preserve:**
- All runtime behavior; the decomposition is behavior-neutral (existing decode/encode/integration suites are the gate)

**Behavior to change:**
- Package structure only (fill during design)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- (fill during design; today: BGP session bytes and API commands enter the reactor loop)

### Transformation Path
1. (fill during design)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| (fill during design) | - | [ ] |

### Integration Points
- (fill during design; known: 15 importer files outside the package, measured 2026-07-08)

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved
- [ ] Registration over hardcoding

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The filename-prefix clusters map to low-coupling seams | file listing 2026-07-08 (prefix counts above) | split lines must come from call-graph analysis instead | import/call analysis during design | unvalidated |
| A-2 | rib-arch closes before design starts and its reactor edits are absorbed | Depends field; rib-arch-8 file list | rebase churn mid-design | `make ze-spec-status` at design start | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Internal cross-references force import cycles between the new subpackages | first extraction fails to compile without dependency inversion | design phase must produce a dependency-ordered extraction sequence before any move |
| R-2 | 331 doc anchors + prose make the sweep error-prone | `make ze-doc-verify` red tail | per-extraction anchor sweeps, not one big-bang sweep |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP session end to end (no new entry points; decomposition only) | → | relocated runtime packages | existing suites: `test/decode/*.ci`, `test/encode/*.ci`, `test/integration` pass unchanged (fill exact per-phase rows during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | after decomposition | `internal/component/bgp/reactor` no longer exists as a 69-file package; its concerns live in glossary-named subpackages (fill exact target list during design) |
| AC-2 | `scripts/dev/protocol_skeleton_report.py` | ("bgp", "reactor") LEGACY_EXCEPTIONS row removed; summary legacy count drops accordingly |
| AC-3 | behavior | full `make ze-precommit-verify` green; decode/encode/integration suites unchanged |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs BGP sessions across the decomposition | same wire/API paths through relocated packages | existing functional suites (fill exact rows during design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design; existing reactor tests move with their files) | - | - | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none expected | decomposition only | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing decode suite | `test/decode/bgp-flow-1.ci` (and siblings) | UPDATE handling unchanged across the split | |
| existing encode suite | `test/encode/attributes.ci` (and siblings) | UPDATE build unchanged across the split | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (fill during design) | `test/interop/scenarios/` | existing scenarios | behavior-neutrality against real daemons | |

### Future (if deferring any tests)
- (fill during design)

## Files to Modify
- `internal/component/bgp/reactor/**` - decomposed into subpackages (fill exact map during design)
- `scripts/dev/protocol_skeleton_report.py` - LEGACY_EXCEPTIONS ("bgp","reactor") removal
- `ai/rules/go-standards.md`, `ai/rules/protocol.md` - reactor exception rows retired
- `scripts/codegen/plugin_imports.go` - `internal/component/bgp/reactor/filter` row if that subdir moves
- docs holding the 331 anchors - sweep (fill during design)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | (fill during design; none expected) |
| CLI commands/flags | [ ] | (fill during design; none expected) |
| Doctor check | [ ] | (fill during design; none expected) |
| Prometheus counters | [ ] | (fill during design; metric registration moves with its file, names unchanged) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | No |
| 2 | Config syntax changed? | [ ] | No |
| 3 | CLI command added/changed? | [ ] | No |
| 4 | API/RPC added/changed? | [ ] | (fill during design) |
| 5 | Plugin added/changed? | [ ] | (fill during design) |
| 6 | Has a user guide page? | [ ] | No |
| 7 | Wire format changed? | [ ] | No |
| 8 | Plugin SDK/protocol changed? | [ ] | (fill during design) |
| 9 | RFC behavior implemented/changed? | [ ] | No |
| 10 | Test infrastructure changed? | [ ] | (fill during design) |
| 11 | Affects daemon comparison? | [ ] | No |
| 12 | Internal architecture changed? | [ ] | Yes - reactor architecture docs (331 anchors measured 2026-07-08) |
| 13 | Route metadata keys added/changed? | [ ] | No |
| 14 | Prometheus counters added/changed? | [ ] | No (names preserved) |
| 15 | Registered inventory changed? | [ ] | (fill during design) |
| 16 | Changed source referenced by doc anchors? | [ ] | Yes - the 331-anchor sweep |
| 17 | Docs show examples for this area? | [ ] | (fill during design) |

## Files to Create
- new subpackage directories under `internal/component/bgp/` (fill exact list during design)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| all stages | (fill during design; this is a skeleton capturing the destination and known points) |

### Implementation Phases
1. **Phase: design** — call-graph/import analysis over the 69 files; produce the target package map and a dependency-ordered extraction sequence; user approval before any move.
2. (fill during design)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| (fill during design) | - |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| (fill during design) | - |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| No behavior change | decomposition must not alter validation or error paths (fill specifics during design) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Import cycle during extraction | back to design: reorder the extraction sequence |
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
- (fill during design)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Standalone spec, blocked on rib-arch | fold into the rib-arch set; rename `reactor` -> `engine` now; drop the candidate | user decision 2026-07-08 at layout-0 closure: rib-arch has no decomposition scope (grep across its nine specs) and is another session's in-flight work; a rename-now would churn 154 package clauses + 331 anchors twice; dropping loses the layout effort's biggest finding |

## Known Limitations
- (fill during design)

## RFC Documentation
- (fill during design; goal is behavior-neutral, so no new RFC enforcement expected)

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
| behavior-neutral decomposition into glossary-named packages | functional test | (fill during implementation) |

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
