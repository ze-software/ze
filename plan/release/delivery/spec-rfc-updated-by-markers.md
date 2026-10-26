# Spec: rfc-updated-by-markers

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | The priority tree orders a rewritten row with its updater only where the row carries `{updated-by}`; the release profiles reach updater pairs that carry none, so `./le rfc next` can name a root row as owed that the updating RFC has rewritten |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`spec-rfc-priority-tree` added the `{updated-by: <ID>; why}` row marker
(`parseUpdatedBy`, `internal/le/rfc/summary.go`; `checkUpdatedBy`,
`internal/le/rfc/check_core.go`) and applied it to one pair only: the 13
rfc4271 rows RFC 7606 rewrites (`fb3027cc78`). The agreed design says a row of
an updated RFC that the updating RFC rewrites carries the marker, mandatory and
optional rows alike. The summaries declare about 127 `Updates` links, and every
other pair is still unmarked.

Goal: every updater pair a release profile reaches (`./le rfc tree profile
<name>` lists the `updated-by` edges of its closure) has each rewritten row of
the updated RFC marked, each mapping read row by row against both texts in
`rfc/full/`, as the rfc4271/rfc7606 set was. The marker mechanism and its gate
are done and are not reopened; this is data work over the existing check.

How a row is read and what is left unmarked follow the first set's rules,
recorded in `plan/learned/036-rfc-priority-tree.md`: one updater id per row,
naming the governing row where several rewrite one; a row the updater states
"remains unchanged", or reads the same way, is not marked.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - "The updated-by marker" and "The priority tree"
- [ ] `ai/skills/ze-rfc.md` - how a summary row is authored

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/summary.go` (`parseUpdatedBy`) - one `{updated-by: <ID>; why}` per row, reason mandatory
- [ ] `internal/le/rfc/check_core.go` (`checkUpdatedBy`) - holds the marker against the graph's header Updates edge and the updater id
- [ ] `rfc/short/rfc4271.md` - the 13 rows marked for RFC 7606, the model for every other pair

To be filled at research: the list of updater pairs per release profile, from
`./le rfc tree profile <name> | json` (`updated-by` field per row).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `rfc/short/<stem>.md` requirement rows

### Transformation Path
1. The author reads the updating RFC's text against the updated RFC's rows
2. A rewritten row gains `{updated-by: <updater id>; <reason>}`
3. `./le rfc check` (`checkUpdatedBy`) holds each marker against the graph's header Updates edge and the updater id

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `rfc/short` → `internal/le/rfc` | summary parse | No |

### Integration Points
- None new: the marker parser, the check, `tree` and `next` exist

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | data only, read by the existing parser |
| No unintended coupling (components stay isolated) | Yes | no code |
| No duplicated functionality (extends existing, does not recreate) | Yes | uses the existing marker |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no code |
| Registration over hardcoding, outbound | N-A | no code |
| Registration over hardcoding, inbound | N-A | no code |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every updater pair a release profile reaches has both texts in `rfc/full/` | the graph is built from those texts | a pair cannot be read | `./le rfc tree` lists no `no summary` updater | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A marker on a row the updater does not rewrite | review against the RFC text | each marker's reason names the updater section |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | ordering in `./le rfc next` and the profile pages |
| How is it reverted? | one commit per pair |
| Who else touches this path? | every session editing `rfc/short/` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` | → | `checkUpdatedBy` | `TestCheckUpdatedByHoldsThePointerAgainstTheUpdater` (`internal/le/rfc/updatedby_test.go`, exists) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Each updater pair in a release profile's closure | every rewritten row carries `{updated-by}`, and the spec lists the pair's mapping table with the RFC sections read |
| AC-2 | `./le rfc check` | no `{updated-by}` refusal |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckUpdatedByHoldsThePointerAgainstTheUpdater` | `internal/le/rfc/updatedby_test.go` | the gate the data passes | exists |

### Boundary Tests (numeric inputs)
N-A: no numeric input.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le rfc check` | rfc gate | the marked corpus passes | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `rfc/short/*.md` - the rows of each updated RFC

## Files to Create
- none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI commands/flags | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/*.md` rows only |

## Implementation Steps

1. List the updater pairs per release profile from `./le rfc tree profile <name> | json`
2. Per pair, read both texts and write the mapping table here
3. Mark the rows; `./le rfc check`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | each marker quotes the updater section that rewrites the row |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Pairs marked | `./le rfc tree profile <name> \| json` shows every reached updater pair with its rows marked |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N-A: data only |

### Failure Routing

| Failure | Route To |
|---------|----------|
| `checkUpdatedBy` refusal | fix the marker against the RFC text |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Marker syntax and gate unchanged | - | decided and implemented in `spec-rfc-priority-tree` |

## Known Limitations
- none yet

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated

### Goal Gates (MUST pass)
- [ ] The targeted tests pass: `./le rfc check`, and `./le verify changed` over the closing commits (`/ze-close` step 6f)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
