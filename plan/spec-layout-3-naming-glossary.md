# Spec: layout-3 -- Package-naming glossary

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-layout-0-umbrella.md` - set context (this is child 3 of 4)
4. `ai/rules/naming.md` - the existing naming rule the glossary extends
5. `internal/component/bgp/wireu/doc.go` - the opaque `wireu` name (rename decision)

## Task

Umbrella gap 3 (first half): no package-naming glossary. Each protocol invents its
own module vocabulary and sibling packages collide in meaning. Add a package-naming
glossary section to `ai/rules/naming.md` (do NOT fork a second naming doc):

- Define the recurring package-name vocabulary: `packet`, `message`, `wire`,
  `session`, `engine`, `transport`, `reactor` (what each term means as a package name).
- Document the colliding trio `internal/component/{cli,cmd,command}` (what each is,
  when to use which).
- Document the four rib-named packages (`internal/core/rib`, `internal/core/routingtable`,
  `internal/component/bgp/rib`, `internal/plugins/routingtable`) so a reader can tell
  them apart.
- Decide the `wireu` question: rename via a deterministic tool, or keep the name with
  a glossary entry + `doc.go` clarification. Rules constrain NEW packages; no mass
  renames. Renames only via an explicit, user-approved shortlist.

This is a skeleton (captured intent). Moves to `design` when picked up.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/naming.md` - the existing naming rule
  → Constraint: the glossary extends this file; do not fork a second naming doc.
- [ ] `ai/rules/plugin-design.md` - registration patterns, Proximity Principle
  → Constraint: glossary terms must not contradict the registration/proximity rules.
- [ ] `ai/rules/canonical-sources.md` - `ai/rules/INDEX.md` is regenerated, not hand-edited
  → Constraint: the new glossary section gets an `ai/rules/INDEX.md` row via regeneration.
- [ ] `plan/spec-layout-0-umbrella.md` - the umbrella (R-2 rename-churn risk)
  → Decision: glossary-first; renames limited to an approved shortlist (at most `wireu`), executed with a deterministic migration tool.

**Key insights:**
- The value is a machine-checkable-adjacent convention new code follows, not a
  610-package rename. `wireu` is the one name opaque enough to consider renaming.

## Current Behavior (MANDATORY)

**Source files read:** (anchors verified this session, 2026-07-08)
- [ ] `internal/component/bgp/wireu/doc.go` - "Package wireu implements lazy-parsed BGP UPDATE messages with zero-copy iterators over wire bytes"
  → Constraint: "u" means UPDATE; the name is opaque to every reader; the glossary child decides rename vs glossary-entry-only.
- [ ] `ai/rules/naming.md` - current naming conventions (file/package casing, forbidden `utils`/`helpers`/`common`/`misc`)
  → Constraint: the glossary is a new section here, consistent with the existing rules.

**Behavior to preserve:**
- All existing package names and imports (no mass renames). Existing protocols keep
  their current module vocabulary; the glossary documents meaning, it does not force
  migration.

**Behavior to change:**
- `ai/rules/naming.md` gains a package-naming glossary section constraining FUTURE
  package creation. Optionally `wireu` is renamed if the user approves the shortlist.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A contributor looks up what a package name means, or creates a new package and
  needs the canonical term, or reads the glossary before naming a protocol module.

### Transformation Path
1. Author consults `ai/rules/naming.md` glossary before naming a new package.
2. The glossary defines each recurring term and disambiguates the `cli`/`cmd`/`command` trio and the four rib-named packages.
3. `ai/rules/INDEX.md` is regenerated with a row for the glossary so the rule is discoverable.
4. If `wireu` is on the approved rename shortlist, a deterministic migration tool renames it and updates all importers in one change.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Author intent <-> canonical name | glossary lookup in `ai/rules/naming.md` | [ ] glossary present |
| Rule <-> discovery index | `ai/rules/INDEX.md` row regenerated | [ ] `make ze-rules-index` clean |
| Old name <-> new name (if `wireu` renamed) | deterministic tool renames + rewrites importers | [ ] build green after rename |

### Integration Points
- `ai/rules/naming.md` - the glossary lives here.
- `ai/rules/INDEX.md` - regenerated row.
- deterministic module-migration tool (`migrate_module.py`-style) - only if a rename is approved.

### Architectural Verification
- [ ] No bypassed layers (documentation + optional deterministic rename only)
- [ ] No unintended coupling (glossary constrains new code; touches no runtime path)
- [ ] No duplicated functionality (extend `naming.md`; do not fork a second naming doc)
- [ ] Registration over hardcoding - N/A (naming convention, no runtime registration surface)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The glossary can document existing terms without forcing any rename beyond an approved shortlist | 610-package tree; glossary-first decision in the umbrella | glossary implies churn; scope creep | keep the glossary descriptive; renames only from a user-approved list | unvalidated |
| A-2 | `wireu` is the only actively misleading name worth a rename | umbrella review; `wireu/doc.go` read this session | more names need renaming; larger shortlist | list candidate opaque names at design; user approves the final shortlist | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The glossary triggers mass-rename churn across a 610-package tree | the Files to Modify list balloons | glossary-first: rules constrain NEW packages; renames limited to an explicit user-approved shortlist, executed with a deterministic tool |
| R-2 | A glossary term contradicts an existing package's `doc.go` | reviewer spots a term used two ways | reconcile the term with actual `doc.go` comments; pick the dominant meaning, note exceptions |

## Wiring Test (MANDATORY)

Wiring is the discoverable rule doc plus an optional deterministic rename
(contributor-facing; N/A for a `.ci`).

| Entry Point | → | Feature Code | Test |
|-------------|----|--------------|------|
| new package created after the rules land | → | glossary in `ai/rules/naming.md` cited | `make ze-rules-index` clean (glossary row present) |
| rules-index regeneration | → | `ai/rules/INDEX.md` row for the glossary | `make ze-rules-index` no diff |
| `wireu` rename (only if approved) | → | deterministic tool renames + rewrites importers | `go build ./...` green after rename |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An author looks up what `packet`/`message`/`wire`/`session`/`engine`/`cli`/`cmd`/`command` mean as package names | `ai/rules/naming.md` carries a package-naming glossary defining each term, the `internal/component/{cli,cmd,command}` trio, and the four rib-named packages; `ai/rules/INDEX.md` row updated |
| AC-2 | The `wireu` rename question | Decided in this spec: renamed via a deterministic tool (build green) OR kept with a glossary entry + `doc.go` clarification; no other package renamed without user approval |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| rules-index freshness | existing `make ze-rules-index` check | INDEX row for the glossary regenerates cleanly | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A - contributor-facing naming convention, no user-facing behaviour; a rename (if approved) is proven by `go build ./...`, not a `.ci` | rules docs + optional rename | new package cites the glossary; approved rename builds green | |

## Files to Modify

- `ai/rules/naming.md` - package-naming glossary section (terms, the `cli`/`cmd`/`command` trio, the four rib-named packages)
- `ai/rules/INDEX.md` - regenerated row for the glossary
- `internal/component/bgp/wireu/doc.go` - clarification if `wireu` is kept (only touched if the rename is declined)
- deterministic module-migration tool invocation - only if the `wireu` rename is approved (renames the package + rewrites importers)

## Implementation Steps

1. **Phase: audit** - list candidate opaque names (A-2); confirm each glossary term against actual `doc.go` comments (R-2).
2. **Phase: glossary** - write the glossary section in `ai/rules/naming.md`; regenerate `ai/rules/INDEX.md`.
3. **Phase: rename decision** - present the shortlist (at most `wireu`) to the user; if approved, run the deterministic tool and rebuild.
4. **Full verification** - `make ze-rules-index` clean; `go build ./...` green if a rename ran.
5. **Complete spec** - learned summary, two-commit closure per `ai/rules/planning.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-2 demonstrated
- [ ] Glossary present in `ai/rules/naming.md`; INDEX row regenerated
- [ ] `wireu` decision recorded (rename via tool + build green, or keep + doc.go note)
- [ ] Wiring Test table complete (concrete test names)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected (N/A here; recorded for completeness)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `plan/spec-layout-0-umbrella.md`, `plan/spec-layout-1-hygiene.md`, `plan/spec-layout-2-core-import-gate.md`, `plan/spec-layout-4-protocol-skeleton.md`.
