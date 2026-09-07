# Spec: publish-the-regenerated-command-catalog

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | docs |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** Two published artifacts live in SIBLING checkouts and are
regenerated from this repository's live command registries. Neither can ride in
a commit made here, so a change to the command surface leaves both stale and
leaves `./le doc check verify` red until somebody runs the two generators in
those checkouts.

| Artifact | Generator | What is stale on 2026-09-08 |
|----------|-----------|------------------------------|
| `../gh-pages/data/cli-commands.json` and the `reference/cli` and `reference/command-equivalents` pages under it | `./le site build` | 411 commands published, 463 live. Every plugin-declared command is absent: `clear bgp healthcheck`, the seven `request bgp adj-rib-in *`, `request bgp rib fastpath`, `mark-stale`, `purge-stale`, and the rest of the 52. The `column-orders` key is carried by no published entry |
| `../wiki/command-catalog.md` | `./le wiki-catalog update file <catalog.md>` | the same 52 commands, and `show bgp rpki` is missing the `summary` pipe alias it now declares |

`compareWebsiteCommandCatalog` and `compareWikiCommandCatalog`
(`internal/le/docvalid/command_surfaces.go`) are the two checks that report it.

**Where it came from.** `spec-daemon-backed-command-catalog` (closed 2026-09-08,
`plan/learned/007-declaration-on-the-registration.md`) made every catalog reader
derive from `registry.Registration`, so 52 plugin-declared commands and one new
key became publishable. It recorded the staleness under Known Limitations and
did not regenerate, because both writes land outside this repository's commit.

**What the work is.** Regenerate both artifacts and commit them in their own
checkouts, then confirm `./le doc check verify` no longer names either file.

**The question this spec must answer, and the reason it is a spec rather than a
chore.** The condition RECURS on every command-surface change, and nothing here
schedules the regeneration. Either the publish becomes part of a command-surface
spec's closure, or the two checks state plainly that they measure a sibling
checkout rather than this tree. Deciding which is the work; running the two
generators is the smaller half.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/documentation-testing.md` - what `./le doc check verify` runs, and which of its findings judge this tree rather than a sibling

**Key insights:** (minimal context to resume after compaction)
- Both sibling checkouts were CLEAN on 2026-09-08, so the regeneration is the only thing that dirties them.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/docvalid/command_surfaces.go` - `compareWebsiteCommandCatalog` and `compareWikiCommandCatalog` compare a published file in a sibling checkout against the live registries

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every per-command contract field the wiki carries survives a regeneration.

**Behavior to change:** (only what the user asked for)
- The two published artifacts name what the live registries hold.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- <not designed yet>

### Transformation Path
1. <not designed yet>

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| this checkout <-> `../gh-pages` and `../wiki` | a generator writes a file in another git checkout | No |

### Integration Points
- <not designed yet>

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| <not designed yet> | → | <not designed yet> | <not designed yet> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le doc check verify` runs over a checkout whose command surface just changed | it names neither `../gh-pages/data/cli-commands.json` nor `../wiki/command-catalog.md` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| <not designed yet> | <not designed yet> | AC-1 | planned |

## Files to Modify
- <not designed yet>

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI commands/flags | <not designed yet> | <not designed yet> |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | <not designed yet> | <not designed yet> | <not designed yet> |

## Implementation Steps

1. **Phase: design** -- decide whether the publish belongs in a command-surface spec's closure or whether the two checks state which tree they judge

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Current Behavior and Data Flow sections completed

### Goal Gates (MUST pass)
- [ ] AC-1 demonstrated
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
