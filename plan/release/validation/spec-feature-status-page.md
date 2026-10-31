# Spec: public status page built from features

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | Truthful release support claims: with the RFC ledger reporting proof only, nothing public says which capabilities an operator can turn on and how far each is proven. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision (2026-10-30): availability is a feature fact, held by the Scope
of `features/<id>.md`, because an operator turns on L2TP, not PPP. The RFC
ledger now reports derived proof only. spec-rfc-support-derived-proof retired
the hand-written `Support status`, `Support coverage` and `Support remaining`
cells and made `docs/features/rfc-status.md` publish Proof, Evidence and Unmet
from `rfc.StemFiguresOf` (`internal/le/rfc/profilecoverage.go`). The same
decision gave this second spec the public status page: one page that lists the
features with their Scope and Level, and under each feature the derived RFC
proof of every RFC its `RFCs` field names.

This is a skeleton. Research and design run under `/ze-spec`.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/feature-maturity.md` - Scope (availability) and Level (proof) per feature; criteria S1 to S3
  → Constraint: S3 reads the derived RFC figure (`internal/le/feature/check.go::rfcProofUnmet`); the page must read the same figure, never count proof its own way.
- [ ] `docs/contributing/rfc-conformance-gates.md` - "Coverage figures", the derived ledger row
  → Constraint: an unwalked RFC publishes no number (`not walked`).

**Key insights:**
- Two producers already exist: `./le feature report` (Scope, Level) and `rfc.StemFiguresOf` (per-RFC proof). The page joins them through each feature's `RFCs` row.

## Current Behavior (MANDATORY)

**Source files read:** to be read in research.
- [ ] `internal/le/feature/check.go` - S3 on the derived figure (`criterionRFC`, `rfcProofUnmet`, `stemFigures`)
- [ ] `internal/le/rfc/render_ledger.go` - `renderStatusPage`, the per-RFC page this one sits beside

**Behavior to preserve:**
- `docs/features/rfc-status.md` and its derived cells.

**Behavior to change:**
- A new generated page; to be designed.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `features/<id>.md` declarations; `rfc/short/<stem>.md`, `rfc/discrimination/`, `rfc/extraction/`.

### Transformation Path
1. Feature declarations and levels (`internal/le/feature`).
2. Per-RFC figures (`rfc.StemFiguresOf`).
3. The page renderer: to be designed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| feature package → rfc package | `rfc.StemFiguresOf` | No |

### Integration Points
- `rfc.StemFiguresOf` - the one derived proof source.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | design phase |
| No unintended coupling (components stay isolated) | No | design phase |
| No duplicated functionality (extends existing, does not recreate) | No | design phase |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A tooling |
| Registration over hardcoding, outbound | No | design phase |
| Registration over hardcoding, inbound | No | design phase |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every feature's `RFCs` row names the RFCs its capability implements | spec-rfc-support-derived-proof AC-12 added the class E stems | a feature reads proven while an RFC it implements is unlisted | grep of `features/*.md` against the summaries' feature mapping | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The page re-derives proof and disagrees with `./le rfc tree` | a figure differs between the two pages | read `StemFiguresOf` only |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in the shipped binary; a public page over- or under-claims |
| How is it reverted? | Single commit revert |
| Who else touches this path? | The rfc-website work on `internal/le/site` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| the update action that regenerates the page | → | the page renderer (to be designed) | `TestFeatureStatusPageReadsDerivedProof` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The declared features | A generated page lists every feature with its Scope and Level, read from `features/<id>.md` and the feature levels, with no authored copy |
| AC-2 | A feature listing RFCs | Under it, each RFC its `RFCs` row names shows the derived Proof, Evidence and Unmet cells from `StemFiguresOf`; an unwalked RFC reads `not walked` |
| AC-3 | A stale page | A check refuses it, naming the regenerate command |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFeatureStatusPageReadsDerivedProof` | to be placed in design | AC-1, AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| to be named in design | `test/ui/` | a maintainer regenerates the page and reads one feature with its RFC proof | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- To be listed in design: the feature and rfc tooling packages under `internal/le/`.

## Files to Create
- To be listed in design.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling |
| YANG validation constraints | N-A | tooling |
| YANG custom validators | N-A | tooling |
| CLI commands/flags | No | design decides whether an existing update action renders it |
| CLI grammar (keyword before value) | No | as above |
| Editor autocomplete | N-A | no YANG |
| Functional test for new RPC/API | Yes | `test/ui/` |
| Pipe completeness | N-A | no product command |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | none |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | the page renders existing declarations |
| 2 | Config syntax changed? | N-A | none |
| 3 | CLI command added/changed? | No | to be confirmed in design |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | N-A | tooling |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | reads proof only |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/feature-maturity.md` |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | No | to be confirmed in design |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | none |
| 16 | Any changed source file referenced by existing doc source anchors? | No | run `./le spec citation anchors spec plan/release/validation/spec-feature-status-page.md` in design |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none yet |

## Implementation Steps

1. **Phase: Wiring** -- to be planned in design.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No proof figure on the page is computed other than through `StemFiguresOf` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The generated page | the regenerate command, then one feature's rows |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Feature and summary files are authored input: a malformed one refuses, never renders a default |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Availability is a feature fact, proof an RFC fact | per-RFC availability axis | owner decision 2026-10-30 |

## Known Limitations
- None yet: a skeleton.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass: the scoped package tests of `internal/le/feature` and `internal/le/rfc` through `./le job run`, the `.ci` through `./le test ui`, then `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove` the spec only
