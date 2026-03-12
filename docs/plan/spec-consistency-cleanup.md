# Spec: Consistency Cleanup

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | 0/6 |
| Updated | 2026-03-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/go-standards.md` - error wrapping, naming
4. `.claude/rules/testing.md` - test conventions
5. `.claude/rules/design-principles.md` - file size limits

## Task

Fix all remaining codebase consistency issues identified in the comprehensive review.
Prior commit `78481a08` addressed the first wave. This spec covers everything remaining,
organized into 6 phases by risk level.

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/go-standards.md` - error wrapping rules
  → Constraint: Error wrapping uses `%w`, not `%v`/`%s`
- [ ] `.claude/rules/testing.md` - test conventions
  → Constraint: `make ze-verify` before commit
- [ ] `.claude/rules/design-principles.md` - file size, YAGNI
  → Constraint: Files >1000L should be split
- [ ] `.claude/rules/buffer-first.md` - encoding patterns
  → Constraint: No `append()` in wire encoding

### RFC Summaries (MUST for protocol work)
- N/A — no protocol changes in this spec

**Key insights:**
- Consistency work is cosmetic/structural, no behavioral changes
- Phase 4 (directory rename) requires user decision on naming convention
- Phase 6 (ChunkNLRI) needs its own spec due to hot-path risk

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] 17 test files with `tc` variable — should use `tt`
- [ ] ~27 packages missing `doc.go` — no package documentation
- [ ] ~8 inline `errors.New()` in function bodies — should be sentinels
- [ ] `route-refresh.go` — hyphenated filename, should be snake_case
- [ ] ~385 `fmt.Errorf` with `%v`/`%s` wrapping errors — should use `%w`
- [ ] 4 test files with `time.Sleep` + assert — flaky pattern
- [ ] 37 plugin directories with hyphens — non-standard Go
- [ ] ~415 test files without `t.Parallel()` — sequential execution
- [ ] `chunk_mp_nlri.go` — uses `append()` in wire encoding

**Behavior to preserve:**
- All existing test assertions and coverage
- All existing error messages (sentinel refactor preserves text)
- All import paths (until Phase 4)
- All wire encoding correctness (Phase 6)

**Behavior to change:**
- Error wrapping: `%v`/`%s` → `%w` where argument is an `error` type
- Test variables: `tc` → `tt` in table-driven loops
- File names: hyphens → underscores where Go convention requires

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- N/A — no new data paths. All changes are cosmetic/structural.

### Transformation Path
1. N/A — no data transformations changed

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | No boundary changes | [ ] |

### Integration Points
- No new integration points — all changes are internal consistency fixes

### Architectural Verification
- [ ] No bypassed layers (no data flow changes)
- [ ] No unintended coupling (cosmetic changes only)
- [ ] No duplicated functionality (renames and refactors only)
- [ ] Zero-copy preserved where applicable (Phase 6 deferred to own spec)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-verify` | → | All consistency changes | All existing tests pass unchanged |

No new features — wiring is proven by existing test suite passing after each phase.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `grep 'for _, tc := range' **/*_test.go` | Zero matches |
| AC-2 | Every package under `internal/component/` | Has `doc.go` |
| AC-3 | `grep 'errors.New(' *.go` in function bodies | Zero inline instances in handler/command code |
| AC-4 | `ls bgp-route-refresh/` | Files use snake_case |
| AC-5 | `fmt.Errorf` with error-typed `%v`/`%s` args | All use `%w` |
| AC-6 | `time.Sleep` + assert in tests | Zero remaining instances |
| AC-7 | Plugin directory names | No hyphens (user decision required) |
| AC-8 | Test files in pure-logic packages | Use `t.Parallel()` |
| AC-9 | `chunk_mp_nlri.go` | Uses `WriteTo(buf, off)` not `append()` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| All existing tests | `*_test.go` | No regressions from cosmetic changes | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-verify` | All | Full test suite passes after each phase | |

### Future (if deferring any tests)
- Phase 5 (`t.Parallel`) deferred — requires per-package state analysis, risk of flaky tests
- Phase 6 (`ChunkNLRI`) deferred — needs dedicated spec with benchmarks

## Files to Modify

### Phase 1 — Mechanical Low-Risk (single commit)

**Task 1.1: Test variable `tc` → `tt` (17 files)**

| File | Change |
|------|--------|
| `internal/core/ipc/message_test.go` | `tc` → `tt` in 4 table-driven loops |
| `internal/core/ipc/method_test.go` | `tc` → `tt` in 3 loops |
| `internal/component/config/schema_test.go` | `tc` → `tt` |
| `internal/component/plugin/server/benchmark_test.go` | `tc` → `tt` |
| `internal/component/bgp/attribute/len_test.go` | `tc` → `tt` |
| `internal/component/bgp/plugins/bgp-nlri-evpn/types_test.go` | `tc` → `tt` |
| `internal/component/bgp/plugins/bgp-rib/storage/attrparse_test.go` | `tc` → `tt` |
| `internal/component/bgp/plugins/bgp-rib/pool/perattr_test.go` | `tc` → `tt` |
| `internal/component/bgp/plugins/bgp-nlri-ls/types_test.go` | `tc` → `tt` |
| `internal/component/bgp/plugins/bgp-cmd-update/update_text_test.go` | `tc` → `tt` |
| `internal/component/bgp/plugins/bgp-nlri-flowspec/types_test.go` | `tc` → `tt` |
| `internal/component/bgp/nlri/len_test.go` | `tc` → `tt` |
| `internal/component/bgp/nlri/rd_test.go` | `tc` → `tt` |
| `internal/component/bgp/message/chunk_mp_nlri_test.go` | `tc` → `tt` |
| `internal/component/bgp/format/message_receiver_test.go` | `tc` → `tt` |
| `internal/component/bgp/format/json_test.go` | `tc` → `tt` |
| `internal/component/bgp/reactor/collision_test.go` | `tc` → `tt` |

**Task 1.2: Missing `doc.go` (~27 packages)**

| Package | Purpose |
|---------|---------|
| `internal/component/bgp` | Top-level BGP subsystem |
| `internal/component/bgp/attrpool` | Per-attribute-type dedup pools |
| `internal/component/bgp/config` | BGP config tree resolution |
| `internal/component/bgp/context` | Encoding context (PackContext, ContextID) |
| `internal/component/bgp/filter` | Route filtering |
| `internal/component/bgp/format` | JSON/text output formatting |
| `internal/component/bgp/nlri` | NLRI types and parsing |
| `internal/component/bgp/rib` | RIB abstraction layer |
| `internal/component/bgp/route` | Route representation |
| `internal/component/bgp/schema` | BGP YANG schema definitions |
| `internal/component/bgp/server` | BGP server (listener + reactor wiring) |
| `internal/component/bgp/store` | Attribute storage interfaces |
| `internal/component/bgp/textparse` | Text command parsing |
| `internal/component/bgp/types` | Shared BGP types |
| `internal/component/bgp/wire` | Wire format types and buffers |
| `internal/component/bgp/wireu` | Wire utilities (rewrite, transform) |
| `internal/component/plugin/all` | Blank imports triggering all plugin init() |
| `internal/component/plugin/cli` | Plugin CLI dispatch |
| `internal/component/plugin/ipc` | Plugin IPC framing |
| `internal/component/plugin/manager` | Plugin process manager |
| `internal/component/plugin/process` | Plugin process lifecycle |
| `internal/component/plugin/registry` | Central plugin registry |
| `internal/component/plugin/schema` | Plugin YANG schema loading |
| `internal/component/plugin/server` | Plugin server (RPC dispatch) |
| `internal/component/config/editor` | Interactive config editor (TUI) |
| `internal/component/bus` | Content-agnostic pub/sub bus |
| `internal/component/engine` | Engine supervisor |

**Task 1.3: Inline `errors.New()` → sentinels (~8 instances)**

| File | Error | Sentinel Name |
|------|-------|---------------|
| `cmd/ze-test/ui.go` | `"tests failed"` | `ErrTestsFailed` |
| `internal/test/peer/peer.go` | `"OPEN mismatch"` | `ErrOpenMismatch` |
| `internal/test/peer/peer.go` | `"connection closed before completion"` | `ErrConnectionClosed` |
| `internal/component/plugin/process/delivery.go` | `"connection closed"` | `ErrConnectionClosed` |

**Task 1.4: File rename**

| Old | New |
|-----|-----|
| `bgp-route-refresh/route-refresh.go` | `bgp-route-refresh/route_refresh.go` |
| `bgp-route-refresh/route-refresh_test.go` | `bgp-route-refresh/route_refresh_test.go` |

### Phase 2 — Error Wrapping Audit (single commit)

~385 `fmt.Errorf` calls to audit. Classification:

| Pattern | Action |
|---------|--------|
| `fmt.Errorf("...: %v", err)` where `err` is `error` | → `%w` |
| `fmt.Errorf("...: %s", err.Error())` | → `%w` with `err` |
| `fmt.Errorf("...: %s", stringVal)` | Keep `%s` |
| `fmt.Errorf("...: %v", nonErrorVal)` | Keep `%v` |
| `fmt.Errorf("unknown X: %s", val)` (terminal, not wrapping) | Keep `%s` |

Focus areas: `pkg/plugin/rpc/`, `internal/component/plugin/`, `internal/component/config/`

### Phase 3 — Remaining Flaky Tests (single commit)

| File | Current | Fix |
|------|---------|-----|
| `fsm/timer_test.go` | Sleep → check timer fired | Channel-based signal |
| `reactor/session_flow_test.go` | `Sleep(10ms)` → assert paused | `require.Eventually` |
| `bgp-rs/worker_test.go` | Sleep → idle timeout check | Extend `waitForCount` |
| `plugin/server/reload_test.go` | `Sleep(50ms)` × 3 | Channel signals |

Validation: each test must pass `-count=100 -race`.

### Phase 4 — Directory & Package Rename (single atomic commit)

**User decision required:** naming convention for 37 plugin directories.

| Option | Dir Example | Package | Pro | Con |
|--------|------------|---------|-----|-----|
| A: Concatenated | `bgprib/` | `bgprib` | Go standard | Less readable |
| B: Underscored | `bgp_rib/` | `bgp_rib` | Readable | Underscores discouraged |
| C: Keep hyphens | `bgp-rib/` | `bgp_rib` | No churn | Non-standard |

37 directories affected. All imports change. Must be done when no parallel branches.

### Phase 5 — `t.Parallel()` Adoption (iterative commits)

Safe packages (pure logic, no I/O): `selector/`, `nlri/`, `attribute/`, `capability/`,
`wire/`, `types/`. Needs-analysis packages: `reactor/`, `server/`, `config/editor/`,
`process/`, `test/integration/`.

### Phase 6 — `ChunkNLRI` Buffer-First (dedicated spec)

Replace `append()` with `WriteTo(buf, off) int`. Needs benchmarks.
Write `docs/plan/spec-chunknlri-buffer-first.md` before implementing.

## Files to Create

- ~27 `doc.go` files (Phase 1.2)
- `docs/plan/spec-chunknlri-buffer-first.md` (Phase 6 prerequisite)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

## Implementation Steps

Each phase is a self-contained commit:

1. **Phase 1** — `tc`→`tt`, `doc.go`, sentinels, file rename → `make ze-verify`
2. **Phase 2** — Error wrapping audit → `make ze-verify`
3. **Phase 3** — Flaky test fixes → `make ze-verify` + `-count=100 -race` per test
4. **Phase 4** — Directory rename (after user decision) → `make ze-verify`
5. **Phase 5** — `t.Parallel()` per package batch → `make ze-verify`
6. **Phase 6** — Write spec, implement with TDD + benchmarks → `make ze-verify`

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix rename (tc→tt body refs, import paths) |
| Test fails after `%w` change | Check if `errors.Is()` chain broke — revert that instance |
| Flaky test after `t.Parallel()` | Remove parallel from that test, analyze shared state |
| Import error after dir rename | Fix missed import path |

## Execution Order

```
Phase 1 (mechanical)     → immediate, one session
Phase 2 (error wrapping) → next session
Phase 3 (flaky tests)    → can parallel with Phase 2
Phase 4 (dir rename)     → user decision on convention first
Phase 5 (t.Parallel)     → iterative, low priority
Phase 6 (ChunkNLRI)      → dedicated spec, separate effort
```

## Not In Scope (validated as correct)

| Dimension | Status |
|-----------|--------|
| JSON field naming (kebab-case) | Consistent |
| Acronym case in exports | Clean after ASN4 fix |
| `// Design:` comments | 100% coverage |
| Cross-reference bidirectionality | Validated |
| Plugin registration pattern | Intentional divergence for cmd-* |
| YANG suffix convention | Consistent |
| Logger hierarchy | All dot-separated |
| Testify adoption | Good in modern code |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

N/A — no protocol changes.

## Implementation Summary

### What Was Implemented
- [To be filled per phase]

### Bugs Found/Fixed
- [To be filled per phase]

### Documentation Updates
- [To be filled per phase]

### Deviations from Plan
- [To be filled per phase]

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated (per phase)
- [ ] Wiring Test table complete — existing tests pass after each phase
- [ ] `make ze-test` passes after each phase
- [ ] No behavioral changes — cosmetic/structural only
- [ ] Critical Review passes

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Existing tests pass (no regressions)
- [ ] Phase 3: flaky test fixes validated with -count=100
- [ ] Phase 6: boundary tests for buffer encoding

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `docs/learned/NNN-consistency-cleanup.md`
- [ ] Summary included in commit
