# Spec: Canonical In-Repo Documentation Entry Set

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-04-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `ai/rules/documentation.md` - source anchors, update checklist
3. `docs/` - current layout (architecture, guide, contributing, features, plugin-development)
4. `docs/DESIGN.md` - existing top-level architectural overview
5. Project wiki (if reachable) - to decide what to cherry-pick

## Task

The `docs/` tree already contains architecture documents, a user guide, a
features list, contributing notes, and plugin-development material. What it
does not have is a small, curated, **top-level entry set** that someone
landing on the repository on a code-browsing site can open first.

Today, a visitor sees `README.md` and a tree of folders. A new reader has to
guess which file explains the architecture at a glance, which file explains
the config syntax, which file explains how a BGP session is driven, and where
the protocol FSM lives. Many of these answers exist inside `docs/architecture`
or on the external wiki, but they are not surfaced as the canonical starting
points.

This spec covers establishing a small set of canonical, top-level markdown
files under `docs/` that function as the documented entry points. The content
is cherry-picked from existing sources (`docs/architecture/*`, `docs/DESIGN.md`,
the external wiki) rather than written from scratch.

The canonical set (target, not mandate):

- `docs/architecture.md` - one-page high-level architecture, linking deeper
- `docs/config-reference.md` - canonical config syntax reference
- `docs/bgp-fsm.md` - BGP FSM walk-through + state diagram
- `docs/plugin-overview.md` - how plugins register and talk to the core

Each file is short (hundreds of lines, not thousands). Depth lives in the
existing `docs/architecture/*` files, and this set links down into them.

Out of scope:
- Rewriting existing deep docs.
- Migrating the external wiki into the repo wholesale.
- Auto-generation of docs from code (separate future work).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - source of the architecture summary
- [ ] `docs/DESIGN.md` - existing top-level design doc (may merge or supersede)
- [ ] `docs/guide/configuration.md` - existing config guide (source material)
- [ ] `docs/guide/plugins.md` - existing plugin guide (source material)
- [ ] `ai/rules/documentation.md` - source anchors, every factual claim
  needs an HTML anchor comment

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - BGP FSM is sourced here

**Key insights:**
- This is a curation spec, not a writing spec. Every paragraph in the new
  files MUST have an HTML `<!-- source: ... -->` anchor pointing to the deeper
  doc or source file.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `docs/architecture/` - what exists at depth
- [ ] `docs/DESIGN.md` - may overlap with the new `architecture.md`; need to
  decide to merge or link
- [ ] `docs/guide/` - existing user-guide layout
- [ ] `docs/features.md` - feature list

**Behavior to preserve:**
- All existing docs remain. This spec only adds top-level entries.
- The external wiki keeps its role; the new in-repo files are the canonical
  source and the wiki becomes a derived mirror for pages in the canonical
  set. Non-canonical wiki pages are unaffected.

**Behavior to change:**
- Add new top-level docs.
- Decide the relationship with `docs/DESIGN.md`. Options:
  1. Replace `docs/DESIGN.md` with `docs/architecture.md` (delete-old, add-new).
  2. Keep `docs/DESIGN.md` as a historical archive and have `architecture.md`
     link to it.
  Record the decision during design.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

Not a code change. No runtime data flow. Documentation flow is:

1. Reader opens the repo
2. README points to `docs/architecture.md` as the starting door
3. `docs/architecture.md` briefly explains the shape and links to the deeper
   `docs/architecture/*` files
4. Similar pattern for config, FSM, plugins

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Visitor opens `README.md` | → | Link to `docs/architecture.md` | grep test: README links to the new entry doc |
| Visitor opens `docs/architecture.md` | → | Links to `docs/architecture/core-design.md` and companions | grep test: architecture.md contains expected link set |
| `make ze-doc-test` | → | Doc drift check | existing target passes |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ls docs/architecture.md docs/config-reference.md docs/bgp-fsm.md docs/plugin-overview.md` | All four exist |
| AC-2 | `grep -c "<!-- source:" docs/architecture.md` | At least one anchor per factual paragraph |
| AC-3 | `README.md` | Points to `docs/architecture.md` as the first link into the docs |
| AC-4 | `docs/architecture.md` | Fits on a single screen of scrolling (loose cap: ~300 lines); deeper detail is linked, not inlined |
| AC-5 | Every link in the new files | Resolves to an existing file |
| AC-6 | `make ze-doc-test` | Passes |
| AC-7 | Decision about `docs/DESIGN.md` | Documented in the spec's Deviations section |
| AC-8 | The external wiki | Has a pointer (`Source: main/docs/<file>.md`) on each page that corresponds to a canonical file, or a note in the learned summary if the wiki is not editable in this session |

## 🧪 TDD Test Plan

### Unit Tests
No Go unit tests.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-doc-test` | Makefile target | Doc drift check passes | |
| `docs-link-check` | `scripts/docs/link-check.go` (may need creating) | All internal links resolve | |

### Future (if deferring any tests)
- None.

## Files to Modify
- `README.md` - add link to `docs/architecture.md`
- `docs/DESIGN.md` - decide fate (delete or annotate)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test | Yes (doc tests) | `make ze-doc-test` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | (but `config-reference.md` is newly canonical) |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | (but `plugin-overview.md` is newly canonical) |
| 6 | Has a user guide page? | Yes | The four new files are the guide entry points |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | Optional | link checker under `scripts/docs/` if added |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create
- `docs/architecture.md` - top-level architecture entry
- `docs/config-reference.md` - config syntax reference
- `docs/bgp-fsm.md` - BGP FSM walk-through + state diagram
- `docs/plugin-overview.md` - plugin architecture summary
- `scripts/docs/link-check.go` - optional link checker (decide during design)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Source material in `docs/architecture/`, `docs/DESIGN.md`, `docs/guide/` |
| 3. Implement | Write the four entry files |
| 4. Full verification | `make ze-doc-test` + manual link check |
| 5. Critical review | Checklist |
| 9. Deliverables review | Checklist |
| 12. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: Decide DESIGN.md fate** - merge, supersede, or keep as archive.
   Record decision.
2. **Phase: Architecture entry** - `docs/architecture.md`. One page, links
   deep, HTML source anchors on every factual claim.
3. **Phase: Config entry** - `docs/config-reference.md`, cherry-picked from
   `docs/guide/configuration.md` and YANG module docs.
4. **Phase: FSM entry** - `docs/bgp-fsm.md`, referencing RFC 4271 and the FSM
   source file.
5. **Phase: Plugin entry** - `docs/plugin-overview.md`, cherry-picked from
   `docs/guide/plugins.md` and `ai/patterns/plugin.md`.
6. **Phase: README link** - point the README at `docs/architecture.md`.
7. **Phase: Link check** - run `make ze-doc-test`; add a link-check helper if
   not already present.
8. **Complete spec** - audit + learned summary.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Four entry files, all under the target size |
| Correctness | Every factual claim has a `<!-- source: -->` anchor pointing to a real file |
| Naming | Top-level entries use short, flat names (no `docs/architecture/top/...`) |
| No duplication | Content is a brief framing of the deeper docs, not a copy |
| Rule: documentation source anchors | Anchors present on every paragraph |
| Rule: no-layering | If `DESIGN.md` is replaced, it is deleted in the same commit, not left stale |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Four new files exist | `ls docs/architecture.md docs/config-reference.md docs/bgp-fsm.md docs/plugin-overview.md` |
| README links to entry | grep `architecture.md` in README |
| Anchors present | `grep -c "<!-- source:"` per file |
| `make ze-doc-test` passes | full run |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| No secrets leaked into docs | scan for tokens / private hostnames |
| No broken links that could leak referrer info | internal links only |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Doc drift check fails | Update anchors |
| Link check fails | Fix broken link |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights

## RFC Documentation

`docs/bgp-fsm.md` cites RFC 4271 Section 8 and links to `rfc/short/rfc4271.md`.

## Implementation Summary

### What Was Implemented
- (fill during /implement)

### Bugs Found/Fixed
- (fill during /implement)

### Documentation Updates
- (fill during /implement)

### Deviations from Plan
- (fill during /implement; record the DESIGN.md decision here)

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] `make ze-doc-test` passes
- [ ] README updated

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit > implicit
- [ ] Minimal coupling

### TDD
- [ ] Doc drift check run
- [ ] Link check run

### Completion
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-canonical-in-repo-docs.md`
- [ ] Summary included in commit
