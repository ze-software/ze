# Learned: Canonical In-Repo Documentation Entry Set

**Spec:** `spec-docs-1-canonical`
**Date:** 2026-05-13
**Status:** Complete

## What Was Done

Created four canonical top-level entry documents under `docs/`:

| File | Lines | Source Anchors | Purpose |
|------|-------|---------------|---------|
| `docs/architecture.md` | 184 | 27 | One-page architecture overview with component map |
| `docs/config-reference.md` | 215 | 15 | Config syntax reference (JUNOS-like, YANG-driven) |
| `docs/bgp-fsm.md` | 192 | 17 | BGP FSM walk-through with state diagram |
| `docs/plugin-overview.md` | 179 | 16 | Plugin architecture, registration, shipped plugins |

Updated `README.md` to point to `docs/architecture.md` as the first documentation
entry point (both in "I Want To" table and Documentation section).

## Key Decisions

**DESIGN.md kept as-is.** The 44KB design document has too much valuable content
(performance analysis, wire format details, protocol scope tables, data flow
diagrams) to delete or archive. `docs/architecture.md` is the new short entry
point and links to DESIGN.md for full detail.

**No link checker script created.** `make ze-doc-test` already exists and covers
documentation drift via `scripts/docvalid/doc_drift.go` and
`scripts/docvalid/commands.go`. A dedicated link checker would duplicate effort.

## What Worked

- Cherry-picking from existing sources (DESIGN.md, core-design.md, guide/configuration.md,
  guide/plugins.md, architecture/behavior/fsm.md) rather than writing from scratch.
  Every factual claim has a `<!-- source: ... -->` anchor pointing to the deeper doc
  or source file.
- Keeping each file under 220 lines. Depth lives in the existing docs; these files
  are navigation aids, not duplicates.

## What Did Not Work / Open Items

- **AC-8 (external wiki):** Wiki was not editable in this session. Someone needs to
  add `Source: main/docs/<file>.md` pointers to wiki pages that correspond to the
  canonical set.
- **Pre-existing doc drift:** `make ze-doc-test` has three pre-existing failures:
  1. `docs/DESIGN.md`: plugin "connected" registered but missing from Shipped Plugins table
  2. `docs/functional-tests.md`: release-gate suite list mismatch with Makefile
  3. `Makefile`: ze-functional-test help suite list mismatch with target
  These are not caused by this spec's changes but should be fixed separately.

## Patterns for Future Documentation Specs

- Documentation-only specs have awkward fits with code-oriented spec validators
  (Data Flow subsections, `.go` file requirements in Current Behavior, TDD
  checklist items). The minimum required structure to satisfy the validator was:
  a `.go` reference in Current Behavior, table-format Boundaries Crossed, and
  TDD checklist items even though there are no Go tests to write.

## Files

None recorded.
