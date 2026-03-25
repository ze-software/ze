# Spec: command-history

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-03-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/cli/model.go` - history fields and handleHistoryUp/Down
4. `internal/component/cli/model_mode.go` - per-mode state with history
5. `cmd/ze/config/cmd_edit.go` - editor entry point (has Storage)
6. `cmd/ze/cli/main.go` - CLI entry point (needs Storage for history)

## Task

Persist command history to the zefs blob store so it survives application restarts.
Currently, the CLI editor (`ze config edit`) and operational CLI (`ze cli`) maintain
per-mode command history (edit mode and command mode) in memory only. On restart,
all history is lost.

History entries are stored as newline-delimited text under `meta/history/` keys in the
zefs database. A configurable maximum (default 100) is stored as `meta/history/max`.
When the history exceeds the max, oldest entries are trimmed (rolling window).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/zefs-format.md` - zefs blob store format and API
  → Constraint: Keys are flat strings with "/" separator; `meta/` namespace for instance metadata
  → Constraint: WriteFile/ReadFile are the primary API; WriteLock for batched writes
- [ ] `docs/architecture/config/yang-config-design.md` - editor mode switching
  → Decision: Per-mode state (history, viewport) saved/restored on mode switch

### RFC Summaries (MUST for protocol work)
N/A - not protocol work.

**Key insights:**
- zefs has no special "meta key" construct; `meta/` is an application-level namespace convention
- Both `ze config edit` and `ze cli` need history; the former has `Storage`, the latter needs access
- History is already per-mode (edit vs command) via `modeStates` in model_mode.go

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cli/model.go` (lines 101-104) - `history []string`, `historyIdx int`, `historyTmp string`
  → Constraint: history is `[]string`, oldest first; consecutive dedup on append (line 758)
- [ ] `internal/component/cli/model.go` (lines 861-900) - `handleHistoryUp()` / `handleHistoryDown()`
  → Constraint: Up starts from end, Down moves forward; restores `historyTmp` at boundary
- [ ] `internal/component/cli/model_mode.go` (lines 42-49, 64-85) - `modeState` saves/restores history per mode
  → Constraint: Each mode (edit, command) has independent history
- [ ] `internal/component/cli/model.go` (lines 270-284, 302-310) - `NewModel` / `NewCommandModel` constructors
  → Constraint: No store access currently; `historyIdx` initialized to -1
- [ ] `cmd/ze/config/cmd_edit.go` (lines 290-526) - receives `storage.Storage`, creates model
  → Constraint: `Storage` interface has ReadFile/WriteFile, sufficient for history
- [ ] `cmd/ze/cli/main.go` (lines 84-136) - `runBGP` creates `NewCommandModel`, no store access
  → Constraint: Currently only has `sshclient.Credentials`, not a store

**Behavior to preserve:**
- Per-mode history (edit and command mode have separate histories)
- Consecutive dedup (same command twice in a row only stored once)
- History navigation with Up/Down arrows (historyIdx, historyTmp)
- `modeStates` save/restore on mode switch
- Graceful degradation when no store is available (filesystem storage, no zefs)

**Behavior to change:**
- History persisted to zefs on every command entry (currently ephemeral)
- History loaded from zefs on model creation (currently starts empty)
- Rolling window: trim to max entries (currently unbounded growth within session)

## Data Flow (MANDATORY)

### Entry Point
- **Load:** On model creation, history is read from zefs: `meta/history/edit` and `meta/history/command`
- **Save:** On every command Enter (lines 758-759 in model.go), after appending to in-memory history
- **Max config:** Read once from `meta/history/max` at load time; default 100 if absent

### Transformation Path
1. **Load:** `store.ReadFile("meta/history/edit")` returns `[]byte` (newline-delimited)
2. **Split:** `strings.Split(string(data), "\n")` into `[]string`, filter empty lines
3. **Assign:** Set `m.history` for the appropriate mode
4. **Append (on Enter):** existing dedup logic appends to `m.history`
5. **Trim:** if `len(m.history) > max`, trim: `m.history = m.history[len(m.history)-max:]`
6. **Save:** `store.WriteFile("meta/history/edit", []byte(strings.Join(m.history, "\n")), 0)`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Model ↔ Store | HistoryStore interface (Load/Save) | [ ] |

### Integration Points
- `internal/component/cli/model.go` - history append sites (2 locations: line 709 and line 758-759)
- `cmd/ze/config/cmd_edit.go` - wire HistoryStore from existing Storage
- `cmd/ze/cli/main.go` - wire HistoryStore from zefs (open via ResolveDBPath)

### Architectural Verification
- [ ] No bypassed layers (history flows through HistoryStore interface)
- [ ] No unintended coupling (model depends on interface, not concrete store)
- [ ] No duplicated functionality (extends existing history, no new history mechanism)
- [ ] Zero-copy preserved where applicable (history is small text, copy is fine)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config edit` -> type commands -> restart -> Up arrow | -> | `History.Save()` + `History.Load()` | `test/editor/commands/cmd-history-persist.et` |
| `ze cli` -> type commands -> restart -> Up arrow | -> | `History.Save()` + `History.Load()` | `test/editor/commands/cmd-history-persist-command.et` |
| Rolling window: 3 commands -> restart -> all recalled | -> | `History.Save()` trim + `History.Load()` | `test/editor/commands/cmd-history-max.et` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Execute commands in `ze config edit`, exit, restart, press Up | Previous commands appear in history |
| AC-2 | Execute commands in `ze cli`, exit, restart, press Up | Previous commands appear in history |
| AC-3 | Edit mode and command mode have separate stored histories | `meta/history/edit` and `meta/history/command` are distinct keys |
| AC-4 | Execute 150 commands with max=100 | Only last 100 stored in zefs |
| AC-5 | `meta/history/max` absent | Default to 100 |
| AC-6 | `meta/history/max` set to 50 via `ze data import` | History trimmed to 50 |
| AC-7 | No zefs available (filesystem storage) | History works in-memory only (no error, graceful degradation) |
| AC-8 | Same command entered consecutively | Stored only once (existing dedup preserved) |
| AC-9 | Concurrent sessions do not corrupt history | Last-write-wins is acceptable; no crash or data loss |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHistoryStoreLoadSave` | `internal/component/cli/history_test.go` | Round-trip: save history, load returns same entries | |
| `TestHistoryStoreRolling` | `internal/component/cli/history_test.go` | Save 150 entries with max=100, load returns last 100 | |
| `TestHistoryStoreDefaultMax` | `internal/component/cli/history_test.go` | No meta/history/max key, defaults to 100 | |
| `TestHistoryStoreCustomMax` | `internal/component/cli/history_test.go` | meta/history/max=50, trims to 50 | |
| `TestHistoryStoreEmpty` | `internal/component/cli/history_test.go` | Load from empty store returns nil/empty slice | |
| `TestHistoryStorePerMode` | `internal/component/cli/history_test.go` | Edit and command histories stored independently | |
| `TestHistoryStoreNilGraceful` | `internal/component/cli/history_test.go` | Nil store (no zefs): load returns empty, save is no-op | |
| `TestModelHistoryPersistOnEnter` | `internal/component/cli/model_test.go` | Enter saves history via store | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| max | 1-10000 | 10000 | 0 (clamp to 1) | N/A (no upper limit enforced, 10000 is practical) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cmd-history-persist` | `test/editor/commands/cmd-history-persist.et` | Config edit session saves/restores history | |
| `cmd-history-persist-command` | `test/editor/commands/cmd-history-persist-command.et` | CLI session saves/restores history | |
| `cmd-history-max` | `test/editor/commands/cmd-history-max.et` | Rolling window: multiple commands survive restart | |

### Future (if deferring any tests)
- Concurrent session history merge (currently last-write-wins, acceptable for v1)

## Files to Modify
- `internal/component/cli/model.go` - add HistoryStore field, call load on init, save on Enter
- `cmd/ze/config/cmd_edit.go` - wire HistoryStore from existing Storage
- `cmd/ze/cli/main.go` - open zefs for history, wire HistoryStore to model

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| RPC count in architecture docs | No | - |
| CLI commands/flags | No | - |
| CLI usage/help text | No | - |
| API commands doc | No | - |
| Plugin SDK docs | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |

## Files to Create
- `internal/component/cli/history.go` - History type and zefs implementation
- `internal/component/cli/history_test.go` - unit tests for History
- `test/editor/commands/cmd-history-persist.et` - functional test: edit-mode history persistence
- `test/editor/commands/cmd-history-persist-command.et` - functional test: command-mode history persistence
- `test/editor/commands/cmd-history-max.et` - functional test: rolling window persistence

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- added history persistence description |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- added .et format reference |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: HistoryStore interface and zefs implementation** -- Define interface, implement load/save/rolling
   - Tests: `TestHistoryStoreLoadSave`, `TestHistoryStoreRolling`, `TestHistoryStoreDefaultMax`, `TestHistoryStoreCustomMax`, `TestHistoryStoreEmpty`, `TestHistoryStorePerMode`, `TestHistoryStoreNilGraceful`
   - Files: `internal/component/cli/history.go`, `internal/component/cli/history_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Wire into Model** -- Add HistoryStore to Model, call on init and Enter
   - Tests: `TestModelHistoryPersistOnEnter`
   - Files: `internal/component/cli/model.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Wire entry points** -- Connect HistoryStore in config edit and CLI entry points
   - Tests: functional tests
   - Files: `cmd/ze/config/cmd_edit.go`, `cmd/ze/cli/main.go`
   - Verify: tests fail -> implement -> tests pass

4. **Functional tests** -> Create after feature works. Cover user-visible behavior.
5. **Full verification** -> `make ze-verify` (lint + all ze tests except fuzz)
6. **Complete spec** -> Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Rolling trim preserves newest, not oldest; empty-line filtering on load |
| Naming | Keys use `meta/history/edit`, `meta/history/command`, `meta/history/max` |
| Data flow | Save happens in model.go at both history append sites (line ~709 and ~758) |
| Rule: no-layering | No duplicate history mechanism; extends existing in-memory history |
| Graceful degradation | Nil store does not panic, does not print errors |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `internal/component/cli/history.go` exists | `ls internal/component/cli/history.go` |
| `internal/component/cli/history_test.go` exists | `ls internal/component/cli/history_test.go` |
| History loads on model creation | grep `LoadHistory\|loadHistory` in model.go |
| History saves on Enter | grep `SaveHistory\|saveHistory` in model.go |
| Config edit wires HistoryStore | grep `HistoryStore\|SetHistoryStore\|historyStore` in cmd_edit.go |
| CLI wires HistoryStore | grep `HistoryStore\|SetHistoryStore\|historyStore` in cmd/ze/cli/main.go |
| Rolling trim implemented | grep `max` in history.go |
| Functional tests exist | `ls test/ui/history-persist-*.ci test/ui/history-max-config.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | meta/history/max: parse as int, clamp to [1, 10000], reject non-numeric |
| Resource exhaustion | History entries capped at max; no unbounded growth on disk |
| Newline injection | History entries must not contain newlines (commands are single-line by design) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

N/A - not protocol work.

## Implementation Summary

### What Was Implemented
- `internal/component/cli/history.go` - History type with browsing, persistence, rolling window, consecutive dedup
- `internal/component/cli/history_test.go` - 18 unit tests covering all ACs + boundary cases
- `internal/component/cli/model.go` - SetHistory method, history save on Enter, Up/Down wiring
- `cmd/ze/config/cmd_edit.go` - Wire History from blob storage for ze config edit
- `cmd/ze/cli/main.go` - Wire History from zefs for ze cli
- `internal/component/cli/testing/runner.go` - option=history:store, option=mode:value=command, restart= action
- `internal/component/cli/testing/parser.go` - StepRestart type, restart action parsing
- `internal/component/cli/testing/headless.go` - NewHeadlessCommandModel for command-only mode
- 3 `.et` functional tests for persistence across restart (edit-mode, command-mode, rolling window)

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/features.md` - Added command history persistence feature description
- `docs/functional-tests.md` - Added .et test format reference section

### Deviations from Plan
- Functional tests use `.et` format (editor tests) instead of `.ci` format. The `.ci` framework supports BGP protocol testing, not interactive TUI sessions. The `.et` framework was extended with `option=history:store`, `option=mode:value=command`, and `restart=` to support persistence testing.
- Test names changed from `history-persist-edit.ci` to `cmd-history-persist.et` etc.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Persist history to zefs | ✅ Done | `history.go:219-233` | Save() writes newline-delimited to blob store |
| Load history on model creation | ✅ Done | `model.go:1262-1278` | SetHistory() loads for both modes |
| Rolling window (trim to max) | ✅ Done | `history.go:226-229` | Trim on save, also on load (line 209) |
| Configurable max (meta/history/max) | ✅ Done | `history.go:58-71` | Read on NewHistory, clamped to [1, 10000] |
| Per-mode history | ✅ Done | `history.go:191, 232` | Key includes mode name |
| Graceful degradation | ✅ Done | `history.go:54-56, 187-189, 220-222` | Nil rw: no-op |
| Consecutive dedup | ✅ Done | `history.go:95-97` | Check last entry before append |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `cmd-history-persist.et`, `TestModelHistoryPersistOnEnter` | Type commands, restart, Up recalls |
| AC-2 | ✅ Done | `cmd-history-persist-command.et`, `TestCommandModelHistoryPersistOnEnter` | Command-mode persistence |
| AC-3 | ✅ Done | `TestHistoryPerMode` | Separate keys for edit/command |
| AC-4 | ✅ Done | `TestHistoryRolling` | 150 entries with max=100, loads 100 |
| AC-5 | ✅ Done | `TestHistoryDefaultMax` | No key returns 100 |
| AC-6 | ✅ Done | `TestHistoryCustomMax` | max=50 trims to 50 |
| AC-7 | ✅ Done | `TestHistoryNilGraceful` | Nil store: no error, in-memory only |
| AC-8 | ✅ Done | `TestHistoryAppendDedup`, `cmd-history-dedup.et` | Consecutive dedup |
| AC-9 | ⚠️ Partial | Design decision: last-write-wins | Spec says "acceptable for v1" |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestHistoryLoadSave | ✅ Done | history_test.go:26 | Named differently (no "Store" prefix) |
| TestHistoryRolling | ✅ Done | history_test.go:42 | |
| TestHistoryDefaultMax | ✅ Done | history_test.go:59 | |
| TestHistoryCustomMax | ✅ Done | history_test.go:66 | |
| TestHistoryEmpty | ✅ Done | history_test.go:89 | |
| TestHistoryPerMode | ✅ Done | history_test.go:97 | |
| TestHistoryNilGraceful | ✅ Done | history_test.go:117 | |
| TestModelHistoryPersistOnEnter | ✅ Done | model_test.go:637 | |
| TestCommandModelHistoryPersistOnEnter | ✅ Done | model_test.go:669 | Added (not in original plan) |
| cmd-history-persist.et | ✅ Done | test/editor/commands/ | .et instead of .ci |
| cmd-history-persist-command.et | ✅ Done | test/editor/commands/ | .et instead of .ci |
| cmd-history-max.et | ✅ Done | test/editor/commands/ | .et instead of .ci |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/cli/history.go` | ✅ Created | 233 lines |
| `internal/component/cli/history_test.go` | ✅ Created | 363 lines, 18 tests |
| `test/editor/commands/cmd-history-persist.et` | ✅ Created | .et format |
| `test/editor/commands/cmd-history-persist-command.et` | ✅ Created | .et format |
| `test/editor/commands/cmd-history-max.et` | ✅ Created | .et format |

### Audit Summary
- **Total items:** 27
- **Done:** 26
- **Partial:** 1 (AC-9: concurrent sessions -- last-write-wins by design, spec says acceptable)
- **Skipped:** 0
- **Changed:** 3 (functional tests: .et format instead of .ci)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/cli/history.go` | Yes | 233 lines |
| `internal/component/cli/history_test.go` | Yes | 363 lines |
| `test/editor/commands/cmd-history-persist.et` | Yes | Created |
| `test/editor/commands/cmd-history-persist-command.et` | Yes | Created |
| `test/editor/commands/cmd-history-max.et` | Yes | Created |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Edit history persists across restart | `cmd-history-persist.et` passes: Up recalls "compare" and "show" after restart |
| AC-2 | CLI history persists across restart | `cmd-history-persist-command.et` passes: Up recalls "daemon status" and "peer list" after restart |
| AC-3 | Per-mode storage | `TestHistoryPerMode`: edit and command keys have distinct entries |
| AC-4 | Rolling window | `TestHistoryRolling`: 150 entries, load returns 100 newest |
| AC-5 | Default max=100 | `TestHistoryDefaultMax`: h.Max() == 100 |
| AC-6 | Custom max | `TestHistoryCustomMax`: max=50, load returns 50 entries |
| AC-7 | Graceful degradation | `TestHistoryNilGraceful`: nil store, no panic, in-memory works |
| AC-8 | Consecutive dedup | `TestHistoryAppendDedup`: second "commit" rejected, entries = ["show", "commit"] |
| AC-9 | Concurrent safety | Design: last-write-wins (spec TDD plan: "future") |

### Wiring Verified (end-to-end)
| Entry Point | Test File | Verified |
|-------------|-----------|----------|
| `ze config edit` -> type -> restart -> Up | `test/editor/commands/cmd-history-persist.et` | Yes: types "show" + "compare", restart, Up recalls both |
| `ze cli` -> type -> restart -> Up | `test/editor/commands/cmd-history-persist-command.et` | Yes: types "peer list" + "daemon status", restart, Up recalls both |
| Rolling window persistence | `test/editor/commands/cmd-history-max.et` | Yes: types 3 commands, restart, Up recalls all 3 |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
