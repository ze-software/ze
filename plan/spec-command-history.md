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
| `ze config edit` -> type commands -> exit -> restart -> Up arrow | -> | `model.loadHistory()` + `model.saveHistory()` | `test/ui/history-persist-edit.ci` |
| `ze cli` -> type commands -> exit -> restart -> Up arrow | -> | `model.loadHistory()` + `model.saveHistory()` | `test/ui/history-persist-cli.ci` |
| `ze db cat meta/history/max` (after manual set) | -> | `historyStore.loadMax()` | `test/ui/history-max-config.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Execute commands in `ze config edit`, exit, restart, press Up | Previous commands appear in history |
| AC-2 | Execute commands in `ze cli`, exit, restart, press Up | Previous commands appear in history |
| AC-3 | Edit mode and command mode have separate stored histories | `meta/history/edit` and `meta/history/command` are distinct keys |
| AC-4 | Execute 150 commands with max=100 | Only last 100 stored in zefs |
| AC-5 | `meta/history/max` absent | Default to 100 |
| AC-6 | `meta/history/max` set to 50 via `ze db import` | History trimmed to 50 |
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
| `history-persist-edit` | `test/ui/history-persist-edit.ci` | Config edit session saves/restores history | |
| `history-persist-cli` | `test/ui/history-persist-cli.ci` | CLI session saves/restores history | |
| `history-max-config` | `test/ui/history-max-config.ci` | Custom max via meta key limits stored entries | |

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
- `internal/component/cli/history.go` - HistoryStore interface and zefs implementation
- `internal/component/cli/history_test.go` - unit tests for HistoryStore
- `test/ui/history-persist-edit.ci` - functional test: config edit history persistence
- `test/ui/history-persist-cli.ci` - functional test: CLI history persistence
- `test/ui/history-max-config.ci` - functional test: custom max configuration

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

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
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

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
