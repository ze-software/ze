# Spec: layout-4 -- Protocol subpackage skeleton (advisory)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/3 (1 probe+rule, 2 advisory report, 3 verify+close) |
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
| A-1 | A skeleton can be defined that fits BGP, BFD, IKE, IS-IS, OSPF, LDP without forcing renames of stable packages | holo precedent (uniform skeleton across 8 crates); Ze's existing `yang//cmd//cli` uniformity | the skeleton stays advisory forever or fragments into per-protocol exceptions | design probe: a table mapping every existing protocol module to the proposed skeleton | **confirmed** (2026-07-08 probe, table below): every module of 7 protocols maps to a skeleton term, an RFC-named equivalent, a domain module, or a documented legacy exception (BGP `message`/`wireu`/`reactor`, IKE `wire`); zero renames needed. LDP/RSVP-TE are single-package protocols (root + `yang/` only; 11/19 root files) -- the skeleton applies once a protocol grows subpackages |
| A-2 | The advisory report can reuse an existing tree walker rather than a new parser | `dep_audit.py` already walks package trees | a second walker is needed; more code | read `dep_audit.py` structure at design; reuse or justify a new script | **broken** (2026-07-08, benign): `dep_audit.py` walks the IMPORT graph (`collect_edges`, dep_audit.py:101); the skeleton report needs only a directory listing + name classification -- no import parsing at all. Reusing dep_audit would couple an advisory lister to a 1300-line gate for zero shared logic. Decision: small standalone `scripts/dev/protocol_skeleton_report.py` (no second import parser is introduced, satisfying the no-duplication intent) |

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

## Design Probe (A-1): protocol module -> skeleton mapping

Subpackage listings verified 2026-07-08 (`ls` per protocol root); engine entry
points verified by `sdk.NewWithConn` grep (root `register.go` for
isis/ospf/ldp/rsvpte, `bfd.go` for BFD, `engine/register.go` for IKE; BGP's
root-level engine predates the SDK and starts via reactor/server -- the
documented archetype exception).

| Protocol | packet (codec) | engine (runtime) | transport (I/O) | session (per-peer state) | types | yang | cli/cmd | domain modules (RFC concepts, freely named) | legacy exceptions |
|----------|----------------|------------------|-----------------|--------------------------|-------|------|---------|---------------------------------------------|-------------------|
| BFD | `packet` | `engine` | `transport` | `session` | - | `yang` | `cmd`, `api` | `auth` | none -- the reference layout |
| IS-IS | `packet` | root pkg | `transport` | `adjacency` (RFC term) | `types` | `yang` | `cli` | `circuit`, `lsdb`, `spf`, `redistribute` | none |
| OSPF | `packet` | root pkg | `transport` | `neighbor` (RFC term) | `types` | `yang` | `cli` | `iface`, `lsdb`, `spf`, `sr`, `redistribute`, `v3` (wire-version dir), `wire` (raw handoff type, glossary sense) | none |
| IKE | `wire` (full codec) | `engine` | `transport` | (in engine) | - | `yang` | `cmd` | `crypto`, `eap`, `ipsec`, `dataplane` | `wire` should be `packet` by the glossary; kept |
| BGP | `message` + `wireu` | `reactor` + `server` | (in reactor/server) | `fsm` (RFC 4271 term) | `types` | `yang` | `cli` | `rib`, `store`, `route`, `filter`, `attrpool`, `config`, `plugins`, ... | platform archetype; `message`/`wireu`/`reactor` documented as historical in the glossary |
| LDP | single-package (11 root files) + `yang` | root pkg | root pkg | root pkg | - | `yang` | - | - | below the subpackage threshold; skeleton N/A |
| RSVP-TE | single-package (19 root files) + `yang` | root pkg | root pkg | root pkg | - | `yang` | - | - | below the subpackage threshold; skeleton N/A |

Probe conclusion: the skeleton fits. Required-when-multi-package: `packet`,
`transport`, `yang`, an engine home (root package or `engine/`), and a per-peer
state module named by the protocol's own RFC term (`session`/`adjacency`/
`neighbor`/`fsm`). Everything else is optional or a domain module. Exceptions
are exactly the two the glossary already documents (BGP vocabulary, IKE `wire`).

## Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | Every probe-table row matches the actual `ls` of the protocol root; the report's classifications agree with the table |
| Advisory-only (R-1) | The report exits 0 on the real tree in report mode regardless of divergence count; only `--selftest` may exit non-zero |
| No fragmentation (R-2) | Skeleton has one small required set + RFC-term aliasing; exceptions are enumerated (BGP, IKE `wire`), not per-term carve-outs |
| Consistency | Skeleton vocabulary is the child-3 glossary's, cross-referenced not duplicated |
| No moves/renames | Diff contains no package moves or renames |
| CLI grammar / Doctor / YANG / Prometheus | N/A - rule doc + advisory reporting script only |

## Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `ai/rules/protocol-skeleton.md` | `ls` the file; grep for the required-modules table and the probe mapping |
| Advisory report | `python3 scripts/dev/protocol_skeleton_report.py` exit 0 with one summary line; `--verbose` lists per-protocol detail; `--selftest` green |
| CI wiring, non-failing | report invoked from the `ze-tier-check` recipe (existing verify stage; no new stage plumbing); recipe still exits 0 |
| INDEX row | `make ze-rules-index` regenerates a row for protocol-skeleton.md; freshness check green |

## Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| No runtime surface | Rule doc + a read-only directory lister; no repo mutation, no external input |
| Advisory cannot mask real gates | The report is appended to `ze-tier-check` AFTER the enforcing commands; a report bug cannot change the gate's exit code path (verify recipe ordering) |

## Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-11, 13-15 | user-facing categories | no | contributor-facing layout convention |
| 12 | Internal architecture changed? | yes | `ai/rules/protocol-skeleton.md` (new rule); `ai/rules/naming.md` untouched (glossary already cross-referenced); `ai/rules/INDEX.md` regenerated |
| 16 | Source anchors on changed files? | verify | grep docs for anchors naming the new script (none expected pre-existing) |
| 17 | Examples elsewhere? | verify | grep docs for `protocol-skeleton` mentions |

## Review Gate

<!-- Filled by /ze-implement step 15. -->

### Run 1 (initial) -- 2026-07-08, full uncommitted diff. 1 ISSUE (fixed in-pass), 0 BLOCKER, 2 NOTE.
Pre-checks: `make ze-validate` exit 0; `audit-test-relaxation.py` clean. Wiring:
report reachable via `ze-tier-check` (Makefile:291, last line AFTER the enforcing
commands, so the advisory cannot mask the gate); rule discoverable via
`ai/rules/INDEX.md` row 78. Advisory contract: report mode hard-returns 0.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | A stale PROTOCOLS manifest row (protocol dir moved) rendered silently as "single-package" -- `protocol_modules` returns an empty list for a missing root | scripts/dev/protocol_skeleton_report.py (build_report) | FIXED in-pass: missing roots flagged in every output mode ("MISSING roots: ..." in the summary, per-protocol line in --verbose); three regression selftest cases added (missing flagged, not single-package, present in summary) |
| 2 | NOTE | Summary counts include the single-package protocols' `yang` modules; cosmetic double-signal with their "skeleton N/A" verbose line | same file (render) | acknowledged |
| 3 | NOTE | 8 pre-existing `check_doc_links.py` breaks remain, none touched by this diff | ai/LEARNED-INDEX.md:250-252 et al. | out of diff; recorded in layout-2's Review Gate |

### Fixes applied
- Finding 1: root cause is the hand-maintained manifest with no existence check
  (`build_report`); [source] fix at the owning layer (report marks missing roots
  visibly, still exit 0) over [workaround] (validating the manifest in selftest
  only, which would not help a runtime-stale tree). Regression cases in
  `selftest()`.

### Run 2+ (re-runs until clean) -- after the fix
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | Re-run over the fixed diff: selftest OK (incl. the three new cases), report exit 0, `ze-validate` 0, `ze-tier-check` 0, test-relaxation clean; no new findings | - | - |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")
(Evidence: Run 2 above is the final clean run -- 0 BLOCKER, 0 ISSUE.)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `ai/rules/protocol-skeleton.md` | yes | created this session; INDEX row 78 regenerated from it |
| `scripts/dev/protocol_skeleton_report.py` | yes | `--selftest` OK; report exit 0; wired at Makefile:291 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Rule doc + advisory report, never failing the build | rule doc exists with required/optional table; report summary "7 protocols; canonical 27, rfc-state 4, version 1, domain 29, legacy 4", exit 0; `make ze-tier-check` exit 0 with the advisory as its last line |
| AC-2 | Probe maps every module; exceptions documented, no forced renames | Design Probe table in this spec (verified per-protocol `ls` + engine grep); report --verbose output matches it 1:1; exceptions = BGP vocabulary + `ike/wire`, both already in the child-3 glossary |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | probe table complete for 7 protocols; zero renames required; LDP/RSVP-TE below threshold |
| A-2 | broken (benign, Mistake Log) | report needs a directory lister, not the import graph; standalone script chosen with written rationale; no second import parser exists |

TDD evidence (2026-07-08): selftest teeth proven by mutation red (removing the
`bgp/wireu` LEGACY_EXCEPTIONS row made `--selftest` FAIL with "bgp/wireu not
legacy" on a mutated copy under tmp/); rules-index fail-first captured
(`rules_index.py --check` exit 1 "stale" after creating the rule doc, green
after `make ze-rules-index`); review finding 1 fixed with three new selftest
cases that fail without the fix.

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

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The advisory report would reuse `dep_audit.py`'s tree walker (A-2) | dep_audit walks the IMPORT graph (`collect_edges`, dep_audit.py:101); the report needs only `os.listdir` + name classification, sharing zero logic | reading dep_audit at design time | standalone ~230-line script instead; the no-duplication intent (no second import parser) still holds |

## Notes
- ~~Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.~~ (picked up + designed 2026-07-08; probe table above)
- Umbrella / siblings: `plan/spec-layout-0-umbrella.md`, child 1 closed (`plan/learned/1088-layout-1-hygiene.md`), `plan/spec-layout-2-core-import-gate.md`, `plan/spec-layout-3-naming-glossary.md`.
