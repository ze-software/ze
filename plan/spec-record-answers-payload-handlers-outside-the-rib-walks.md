# Spec: every payload handler outside the two RIB walks still buffers its answer

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The record answer protocol streams a walk one row at a time, so a consumer
reading `| first 10` over a large table pays for the rows it keeps. Two handlers
use it, and every other handler still builds the whole answer as one document
first.

`plugin.Records` (`pkg/plugin/records.go`) is what a command handler answers
with when its walk produces rows: the engine writes a head, one line per record
and a terminator carrying the counts. Outside test helpers, exactly two payload
producers use it, and both are RIB walks:
`internal/component/bgp/plugins/rib/rib_pipeline.go` and
`internal/component/bgp/plugins/rib/rib_pipeline_best.go`. Every other handler
answers with `plugin.Map` (`internal/component/plugin/types.go`,
`type Map map[string]any`), which is 302 non-test occurrences across the tree.

This was deferred by the `spec-record-answers-3-zero-alloc` design on
2026-08-20, with the reason recorded: the other 380-odd handlers walk
collections bounded by peer count, interface count or table size, so each
answers as one `type=json` document whatever this family does. The per-row cost
does not compound, and converting them would touch the `plugin.Map` call sites
for no measured gain.

The row's destination note is the condition on this spec: **its own spec, raised
with the owner only if a bounded walk is ever measured as a cost.** So the first
thing this spec owes is that measurement. Without it there is nothing here to
build, and converting 302 call sites on a hunch is the machinery
`ai/rules/simplicity.md` refuses.

If a bounded walk is measured as a cost, the work is to convert the payload
handlers that walk it, deciding per handler whether its collection can grow
without a bound the operator controls.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ipc_protocol.md` - the answer wire grammar and the record head, line and terminator
  → Constraint: <to be filled>
- [ ] `docs/architecture/api/process-protocol.md` - the plugin side of the same protocol
  → Decision: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `pkg/plugin/records.go` - `Records` is what a handler answers with when its walk produces rows; `Row` is an appender so a walk puts no allocation on a row, and the engine writes head, rows and terminator
- [ ] `internal/component/plugin/types.go` - `Map` is `map[string]any`, the buffered answer every other handler returns
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` - one of the two converted walks
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - the other

**Behavior to preserve:** (unless the user explicitly said to change it)
- the answer a buffered consumer sees, which `CollapseRecords` (`pkg/plugin/rpc/collapse.go`) rebuilds from the rows

**Behavior to change:** (only what the user asked for)
- None until a bounded walk is measured as a cost

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- any command whose handler walks a collection and returns `plugin.Map`
- <to be filled>

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| plugin handler ↔ engine | `plugin.Records` rows or a `plugin.Map` document | No |

### Integration Points
- `pkg/plugin/rpc/answer_write.go` - `WriteRecordAnswer`, the one writer both ends use
- `pkg/plugin/rpc/collapse.go` - `CollapseRecords`, the document a buffered reader gets

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
| A-1 | every walk outside the two RIB pipelines is bounded by peer count, interface count or table size | `spec-record-answers-3-zero-alloc` design, 2026-08-20 | a growing walk is already paying the cost | measure one | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | converting call sites with no measurement adds machinery for no gain | <to be filled> | the measurement gates the work |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the answer shape of every converted command |
| How is it reverted? | <to be filled> |
| Who else touches this path? | the record-answers family, `spec-cli-pipe-operator-coverage` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a converted command piped through `first` | → | <to be filled> | <to be filled> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a bounded walk is measured against the record path and the buffered one | the measurement is recorded, and it decides whether the conversion runs |
| AC-2 | a converted handler answers a buffered consumer | the document is identical to the one it produced before |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | pipes a large answer through `first 10` | <to be filled> | <to be filled> |

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
- the payload handlers the measurement names - <to be filled>

## Files to Create
- <to be filled>

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| Pipe completeness | | `ai/rules/cli.md` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 8 | Plugin SDK/protocol changed? | | `docs/architecture/api/process-protocol.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- measure a bounded walk on both paths and record the result
2. **Phase: <to be filled>**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | a converted handler's collapsed document equals the map it used to return |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the measurement that authorizes the conversion | a pasted benchmark result |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |

## Known Limitations
- Two rows from the closed `spec-streaming-answer-protocol` still govern this family: record-level streaming for the REST, gRPC, web, MCP and looking-glass surfaces, and `table` and `text` rendering buffering whatever the wire does, which is a permanent limit.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes
