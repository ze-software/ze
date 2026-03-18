# Spec: config-versioning

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-blob-namespaces (provides `file/<qualifier>/` key structure) |
| Phase | - |
| Updated | 2026-03-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/zefs-format.md` - ZeFS blob format
4. `internal/component/cli/editor.go` - backup/rollback logic
5. `internal/component/config/storage/storage.go` - Storage interface
6. `cmd/ze/config/cmd_rollback.go` - `ze config rollback` command
7. `cmd/ze/config/cmd_history.go` - `ze config history` command

## Task

Replace the current numbered-rollback backup system with date-based config versioning. Rollback numbers are computed from date ordering, not stored. Multiple versions of the same config key coexist, distinguished by date. The active version is the one with the most recent date. A special `draft` date marks a live edit in progress.

This applies to keys under the `file/` namespace (from `spec-blob-namespaces`).

Deliverables:
1. Date-based versioning: each config stored with a date, not a sequential number
2. Rollback number derived: `rollback 0` = most recent committed, `rollback 1` = second most recent, etc.
3. Multiple versions per key: same key, different dates, all stored simultaneously
4. Active version: the version with the latest non-draft date
5. Draft marker: a version with date `draft` represents a live edit in progress
6. `ze config history` shows versions with dates and computed rollback numbers
7. `ze config rollback <N>` restores from the Nth most recent version (by date)
8. `ze commit compare rollback <N>` compares current (or draft) against rollback N

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/zefs-format.md` - ZeFS blob format
- [ ] `docs/architecture/config/syntax.md` - config system

### RFC Summaries (MUST for protocol work)
No external RFCs apply.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cli/editor.go` - `createBackup()` writes to `rollback/<name>-<YYYYMMDD-HHMMSS.mmm>.conf`; `ListBackups()` returns sorted by timestamp descending; `RestoreBackup()` copies backup content over active file
- [ ] `internal/component/cli/editor_commit.go` - `CommitSession()` calls `createBackup()` before overwriting
- [ ] `internal/component/config/storage/storage.go` - Storage interface: `ReadFile`, `WriteFile`, `List`, `Remove`
- [ ] `cmd/ze/config/cmd_rollback.go` - takes `<N> <file>`, calls `ed.RestoreBackup(n)`
- [ ] `cmd/ze/config/cmd_history.go` - lists backups with revision numbers and timestamps
- [ ] `cmd/ze/config/cmd_diff.go` - `ze config diff <N> <file>` compares revision N against current

**Behavior to preserve:**
- `ze config rollback <N>` CLI syntax unchanged
- `ze config history` output format (revision number, date, file)
- `ze config diff <N>` comparison semantics
- Automatic backup on commit

**Behavior to change:**
- Storage: replace `rollback/<N>_<timestamp>` directory with date-keyed versions of same key
- Rollback numbers computed from date ordering, not stored as filenames
- Multiple versions of a key coexist with different dates
- Active version is latest non-draft date
- Draft version for live edits (replaces temp file or buffer approach)
- New command: `ze commit compare rollback <N>` to diff current/draft against a rollback

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Editor commit: saves current as new dated version, draft removed
- Editor open: creates draft version from active
- `ze config rollback <N>`: looks up Nth version by date order, copies to active
- `ze commit compare rollback <N>`: reads current/draft and Nth version, diffs them

### Transformation Path
1. Key + date -> versioned storage (e.g., key `etc/ze/router.conf` at date `2026-03-18T10:00:00`)
2. List versions of a key -> sorted by date descending
3. Compute rollback number -> index in sorted list (0 = most recent committed, 1 = next, ...)
4. Draft -> special date value, excluded from rollback numbering, always most recent if present

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Editor -> Storage | Writes versioned key with date | [ ] |
| CLI rollback -> Storage | Reads Nth version by computed index | [ ] |
| CLI history -> Storage | Lists all versions sorted by date | [ ] |
| CLI compare -> Storage | Reads two versions, diffs | [ ] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config history <file>` | -> | Lists date-based versions with computed rollback numbers | `test/parse/cli-config-history-versioned.ci` |
| `ze config rollback <N> <file>` | -> | Restores Nth version by date order | `test/parse/cli-config-rollback-versioned.ci` |
| `ze commit compare rollback <N>` | -> | Diffs current against Nth version | `test/parse/cli-commit-compare-rollback.ci` |
| Editor commit | -> | Creates new dated version, removes draft | Unit test |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Editor commits config | New version stored with current date, draft removed |
| AC-2 | Editor opens config for editing | Draft version created from active |
| AC-3 | `ze config history <file>` | Shows all versions with dates and computed rollback numbers (0 = most recent) |
| AC-4 | `ze config rollback 2 <file>` | Restores content from 3rd most recent version (by date) |
| AC-5 | Multiple commits of same file | All versions retained, each with its own date |
| AC-6 | `ze commit compare rollback 1` | Shows diff between current (or draft) and rollback 1 |
| AC-7 | Draft exists during live edit | Draft is visible in history as "draft", not numbered as a rollback |
| AC-8 | No old `rollback/` directory needed | Versions stored as date-keyed entries, not separate directory |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|

## Files to Modify
- `internal/component/cli/editor.go` - replace `createBackup()`/`ListBackups()`/`RestoreBackup()` with date-based versioning
- `internal/component/cli/editor_commit.go` - commit writes dated version instead of backup copy
- `internal/component/config/storage/storage.go` - versioned key API (write with date, list versions, read version)
- `cmd/ze/config/cmd_rollback.go` - use date-computed index instead of directory listing
- `cmd/ze/config/cmd_history.go` - show dates and computed rollback numbers
- `cmd/ze/config/cmd_diff.go` - support `compare rollback <N>` form

## Files to Create
- To be determined during design phase

## Implementation Steps

To be filled during design phase.

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
<!-- LIVE -- write IMMEDIATELY when you learn something -->

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Audit filled
- [ ] Write learned summary to `docs/learned/NNN-config-versioning.md`
- [ ] **Summary included in commit**
