# Spec: layout-4 -- Protocol subpackage skeleton (advisory)

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
3. `plan/spec-layout-0-umbrella.md` - set context (this is child 4 of 4)
4. `ai/rules/naming.md` - the package-naming glossary (child 3) the skeleton builds on
5. `ai/rules/plugin-design.md` - registration + Proximity Principle the skeleton must respect

## Task

Umbrella gap 3 (second half): no shared protocol skeleton. Each protocol invents its
own module layout (BGP: `fsm`/`reactor`/`message`/`wireu`; BFD:
`engine`/`packet`/`session`/`transport`; IKE: `engine`/`wire`/`crypto`; IS-IS:
`adjacency`/`circuit`/`lsdb`/`packet`). Holo's strongest structural property is the
inverse: one fixed module skeleton repeated across every protocol so learning one
teaches all.

- Create `ai/rules/protocol-skeleton.md` defining the standard protocol subpackage
  skeleton and which modules are required vs optional.
- Produce an advisory conformance report that lists per-protocol divergence for
  existing protocols WITHOUT failing the build (an enforced gate today would need a
  huge allowlist, repeating the tiers Path B lesson).
- The skeleton applies to NEW protocols and opportunistically to touched code. No
  moves, no renames in this child.

Design-heavy: the core deliverable at design time is the A-1 probe table mapping
every existing protocol module to the proposed skeleton.

This is a skeleton (captured intent). Moves to `design` when picked up.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/naming.md` - the package-naming glossary (child 3)
  → Constraint: the skeleton reuses the glossary vocabulary; it does not invent a parallel set of terms.
- [ ] `ai/rules/plugin-design.md` - registration patterns, Proximity Principle
  → Constraint: the skeleton must not contradict the registration/proximity rules.
- [ ] `ai/rules/module-tiers.md` - the tier taxonomy
  → Constraint: the skeleton describes subpackage layout WITHIN a protocol; it moves no package between tiers (tiers umbrella owns that).
- [ ] `plan/spec-layout-0-umbrella.md` - the umbrella (A-1 design-probe requirement, R-1 advisory-first)
  → Decision: advisory report before any enforcement; existing protocols are not force-migrated.

**Key insights:**
- The skeleton must fit BGP, BFD, IKE, IS-IS, OSPF, LDP without forcing renames of
  stable packages. If it cannot, it stays advisory and documents exceptions rather
  than fragmenting into per-protocol carve-outs.

## Current Behavior (MANDATORY)

**Source files read:** (anchors verified this session, 2026-07-08)
- [ ] `internal/component/bgp/wireu/doc.go` - BGP's module vocabulary example (`wireu` = lazy-parsed UPDATE); representative of per-protocol divergence
  → Constraint: the skeleton maps this existing module to a standard term; it does not require renaming it in this child.
- [ ] `ai/rules/plugin-design.md` - existing structural rules (registration, proximity)
  → Constraint: the advisory report reads the actual protocol package trees; it must classify divergence, not mandate migration.

**Behavior to preserve:**
- All existing protocol package layouts. The report is advisory: it never fails the
  build and never triggers a move or rename.

**Behavior to change:**
- `ai/rules/protocol-skeleton.md` exists and is discoverable; an advisory report
  lists per-protocol divergence. New protocol package sets follow the skeleton.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A contributor creates a new protocol package set, or runs the advisory conformance
  report, or reads `ai/rules/protocol-skeleton.md` before laying out a protocol.

### Transformation Path
1. The design probe maps every existing protocol module (BGP, BFD, IKE, IS-IS, OSPF, LDP) to the proposed skeleton term.
2. `ai/rules/protocol-skeleton.md` records the skeleton, which modules are required vs optional, and how existing protocols map to it.
3. The advisory report walks protocol package trees and lists divergence per protocol; it exits 0 always (no build failure).
4. `ai/rules/INDEX.md` is regenerated with a row for the skeleton rule.
5. A new protocol package set follows the skeleton; touched code adopts it opportunistically.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Protocol layout <-> standard skeleton | mapping table in `ai/rules/protocol-skeleton.md` | [ ] probe table complete |
| Report <-> CI | advisory report runs, always exits 0 | [ ] report runs green |
| Rule <-> discovery index | `ai/rules/INDEX.md` row regenerated | [ ] `make ze-rules-index` clean |

### Integration Points
- `ai/rules/protocol-skeleton.md` - the skeleton rule.
- advisory conformance report (script or a `dep_audit.py` advisory section) - lists divergence.
- `ai/rules/INDEX.md` - regenerated row.

### Architectural Verification
- [ ] No bypassed layers (documentation + advisory report only)
- [ ] No unintended coupling (report reads package trees; no runtime path touched)
- [ ] No duplicated functionality (one skeleton rule; the report reuses the existing import-graph walker where possible, no second parser)
- [ ] Registration over hardcoding - N/A (layout convention; no runtime registration surface)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A skeleton can be defined that fits BGP, BFD, IKE, IS-IS, OSPF, LDP without forcing renames of stable packages | holo precedent (uniform skeleton across 8 crates); Ze's existing `yang//cmd//cli` uniformity | the skeleton stays advisory forever or fragments into per-protocol exceptions | design probe: a table mapping every existing protocol module to the proposed skeleton | unvalidated |
| A-2 | The advisory report can reuse an existing tree walker rather than a new parser | `dep_audit.py` already walks package trees | a second walker is needed; more code | read `dep_audit.py` structure at design; reuse or justify a new script | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An enforced skeleton gate today needs a huge allowlist (tiers Path B lesson) | the probe shows wide divergence | advisory-only; the report lists divergence, it does not fail the build |
| R-2 | The skeleton fragments into per-protocol carve-outs | each protocol needs its own exception row | keep the skeleton small and required-vs-optional; document exceptions rather than special-casing terms |

## Wiring Test (MANDATORY)

Wiring is the discoverable rule doc plus an always-green advisory report
(contributor-facing; N/A for a `.ci`).

| Entry Point | → | Feature Code | Test |
|-------------|----|--------------|------|
| advisory report in CI | → | per-protocol divergence listing, exit 0 | report runs without failing the build (CI step) |
| skeleton rule discoverable | → | `ai/rules/INDEX.md` row for the skeleton | `make ze-rules-index` clean |
| new protocol package set created | → | layout follows `ai/rules/protocol-skeleton.md` | probe table maps its modules to the skeleton |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A new protocol package set is created | `ai/rules/protocol-skeleton.md` exists defining the standard subpackage skeleton and when each module is required; an advisory report lists per-protocol divergence for existing protocols without failing the build |
| AC-2 | The design probe (A-1) | A table maps every existing protocol module (BGP, BFD, IKE, IS-IS, OSPF, LDP) to the proposed skeleton term; unmappable modules are documented as exceptions, not forced renames |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| rules-index freshness | existing `make ze-rules-index` check | INDEX row for the skeleton rule regenerates cleanly | |
| advisory report exit code | the report script (or its selftest) | report runs and always exits 0 (advisory, never fails the build) | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A - contributor-facing layout convention, no user-facing behaviour; the advisory report is proven by its exit-0 selftest, not a `.ci` | CI advisory step | report lists divergence; build stays green | |

## Files to Modify

- `ai/rules/INDEX.md` - regenerated row for the skeleton rule
- `ai/rules/module-tiers.md` - one cross-reference line to the skeleton rule (only if a pointer is warranted)
- Makefile / `mk/*.mk` - wire the advisory report into CI as a non-failing step (only if a new target is needed)

## Files to Create
- `ai/rules/protocol-skeleton.md` - the standard protocol subpackage skeleton
- advisory protocol-skeleton conformance report (a script, or an advisory section in `dep_audit.py`; decided at design per A-2)

## Implementation Steps

1. **Phase: design probe** - build the A-1 mapping table for every existing protocol; confirm the skeleton fits without forced renames.
2. **Phase: rule doc** - write `ai/rules/protocol-skeleton.md` (required vs optional modules, the mapping); regenerate `ai/rules/INDEX.md`.
3. **Phase: advisory report** - implement the divergence report reusing the existing tree walker if possible (A-2); wire it as a non-failing CI step.
4. **Full verification** - `make ze-rules-index` clean; report runs and exits 0.
5. **Complete spec** - learned summary, two-commit closure per `ai/rules/planning.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-2 demonstrated (rule doc exists; probe table complete)
- [ ] Advisory report runs in CI and always exits 0 (no build-time enforcement)
- [ ] `ai/rules/INDEX.md` row regenerated for the skeleton rule
- [ ] Wiring Test table complete (concrete test names)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected (N/A here; recorded for completeness)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `plan/spec-layout-0-umbrella.md`, `plan/spec-layout-1-hygiene.md`, `plan/spec-layout-2-core-import-gate.md`, `plan/spec-layout-3-naming-glossary.md`.
