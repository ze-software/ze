# Spec: show bgp rib best and show bgp rib describe one route differently

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Depends | `plan/immediate/spec-cli-show-bgp-answer-shapes.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two commands describe one route, and they put the same facts in different
places. `show bgp rib` writes every path attribute FLAT into the row.
`show bgp rib best` writes four fields at the top level and buries the
attributes one level down under `attributes`.

`serializeRouteItem` (`internal/component/bgp/plugins/rib/rib_pipeline.go`)
seeds the row with `family` and `prefix`, then calls
`enrichRouteMapFromEntry` (`rib_attr_format.go`) against that same row map, so
`next-hop`, `origin`, `as-path`, `med` and the rest land beside `prefix`.
`bestResultFor` (`rib_pipeline_best.go`) builds a `bestResult` whose JSON tags
are `family`, `prefix`, `best-peer` and `multipath-peers`, and hands
`enrichRouteMapFromEntry` a SEPARATE map that it stores under the `attributes`
tag.

The operator pays for the difference. `selectRecord`
(`internal/component/command/pipe_columns.go`) cuts a record to the fields the
operator named, keeping a key only when `keep` holds it. `attributes` is not a
name an operator types, so `display prefix next-hop` over `show bgp rib best`
drops the whole sub-map and answers rows carrying a prefix and nothing else.
The same operator chain over `show bgp rib` answers the next hop, because there
the next hop IS a top-level field.

The work is a decision about the PAYLOAD, not about what a command declares.
That is why `spec-cli-show-bgp-answer-shapes` did not take it: the row was
raised on 2026-08-24 while resolving that spec's AC-14 in Phase 4, and that
spec's "Behavior to preserve" undertakes not to change a payload. Moving a key
changes the answer for every reader of `show bgp rib best`, the web UI and the
looking glass included, and not only for the operator who typed an operator
chain.

The spec must settle which fields a best-path row carries at its top level,
apply one answer to both commands, and say what happens to the `attributes`
envelope. It matters because one route described two ways is synonym rotation
in a payload: the operator cannot carry a working operator chain from one
command to the other.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/pipes.md` - how a displayed-field selection cuts a record
  → Decision: <to be filled>
  → Constraint: <to be filled>
- [ ] `plan/immediate/spec-cli-show-bgp-answer-shapes.md` - the declaration channel this payload is read through
  → Constraint: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - `bestResultFor` builds a `bestResult` with JSON tags `family`, `prefix`, `best-peer`, `multipath-peers` and `attributes`, and fills the last from a map `enrichRouteMapFromEntry` writes into
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` - `serializeRouteItem` seeds a row with `family` and `prefix` and calls `enrichRouteMapFromEntry` against that same map, so the attributes are top-level fields
- [ ] `internal/component/bgp/plugins/rib/rib_attr_format.go` - `enrichRouteMapFromEntry` writes `next-hop`, `origin`, `as-path`, `med`, `stale` and the rest into whatever map it is given
- [ ] `internal/component/command/pipe_columns.go` - `selectRecord` keeps a key only when the displayed-field set holds it, so an unnamed `attributes` key is dropped whole

**Behavior to preserve:** (unless the user explicitly said to change it)
- <to be filled>

**Behavior to change:** (only what the user asked for)
- <to be filled>

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `show bgp rib best` and `show bgp rib`, typed by the operator, optionally with a `display` operator chain
- <to be filled>

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RIB plugin ↔ command pipe layer | record rows over the answer protocol | No |

### Integration Points
- <to be filled>

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | <to be filled> | <to be filled> | <to be filled> | <to be filled> | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Moving a key breaks the web UI and the looking glass, which read the same payload | a rendering test goes red | <to be filled> |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | every consumer of the `show bgp rib best` payload reads a key that moved |
| How is it reverted? | <to be filled> |
| Who else touches this path? | `spec-cli-show-bgp-answer-shapes`, `spec-cli-pipe-operator-coverage` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp rib best display prefix next-hop` | → | <to be filled> | <to be filled> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show bgp rib best display prefix next-hop` | the answer carries the next hop of each best path |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | asks the best paths for their next hop | <to be filled> | <to be filled> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| <to be filled> | <to be filled> | <to be filled> | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| <to be filled> | `test/` | <to be filled> | |

## Files to Modify
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - <to be filled>
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - <to be filled>

## Files to Create
- <to be filled>

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI grammar (keyword before value) | | <to be filled> |
| Pipe completeness | | <to be filled> |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- <to be filled>
2. **Phase: <to be filled>**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Naming | one name per concept across `show bgp rib` and `show bgp rib best` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| <to be filled> | <to be filled> |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |

## Known Limitations
- <to be filled>

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes
