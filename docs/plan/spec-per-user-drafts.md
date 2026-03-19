# Spec: Per-User Change Files

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-03-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/yang-config-design.md` - config editor design
4. `internal/component/cli/editor_draft.go` - current write-through protocol
5. `internal/component/cli/editor_commit.go` - commit/discard/disconnect

## Task

Replace the single shared draft file (`config.conf.draft`) with per-user change files (`config.conf.change.<user>`). Each user's edits are written immediately to their own change file. A shared draft file (`config.conf.draft`) serves as a saved checkpoint between edits and commit.

### Commit backup via key rename

On commit, the current `config.conf` is backed up by renaming the blob key to `config.conf-YYYYMMDD-HHMMSS.mmm`. This is an in-place key rewrite (no data copy) if the key's netcapstring capacity has room for the suffix.

To ensure this, `BlobStore.writeFileNoFlush` must allocate key capacity with 16 extra bytes (length of `-YYYYMMDD-HHMMSS.mmm`) when creating keys under `file/active/`. This allows the backup rename to update only the key's used-length and data bytes without reallocating the entire entry.

The Storage interface needs a `Rename(oldKey, newKey string) error` method. For filesystem storage: `os.Rename`. For blob storage: in-place key rewrite if capacity allows, otherwise realloc.

### Three-layer model

| Layer | File | Purpose |
|-------|------|---------|
| Committed | `config.conf` | Applied to daemon, source of truth |
| Saved draft | `config.conf.draft` | Checkpoint created by `save` |
| User changes | `config.conf.change.<user>` | Per-user edit journal, write-through |

### Operations

| Command | Behavior |
|---------|----------|
| `set`/`delete` | Write-through to own `config.conf.change.<user>` immediately. After write, scan all `config.change.*` files and report conflicts (same leaf, different value) |
| `save` | Apply changes from own change file to `config.conf.draft` (creates new draft). Check for conflicts with committed version before saving |
| `commit` | Save first, then apply `config.conf.draft` to `config.conf` |
| `discard` | Delete own `config.conf.change.<user>`, reload from base state (draft if exists, else committed) |

### Change file format

Each change file contains only the changed entries (not a full tree). One entry per leaf, last write wins (replace on same leaf). Uses existing set+meta format:

```
set bgp router-id 5.6.7.8 #thomas @local %2026-03-19T12:00:00Z ^1.2.3.4
set bgp local-as 65001 #thomas @local %2026-03-19T12:01:00Z ^65000
```

### Conflict detection (on load and on each edit)

On session start (loading the change file) and after each set/delete, scan all `config.conf.change.*` files. For each other user's change file:
- Parse their entries
- If any leaf path matches one in our change file with a different value, report conflict
- Check if the other user's SSH session is still alive
  - Active session: warn immediately ("conflict with alice on bgp router-id")
  - Dead session: ignore (report orphaned change file on reconnect)

### Base state

The "base" for editing is: `config.conf.draft` if it exists, else `config.conf`. When a user starts editing, their tree is loaded from the base state. Changes are applied on top.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - config editor design
  → Constraint: write-through protocol keeps disk and memory in sync
  → Decision: per-user change files replace shared draft for edit tracking

### RFC Summaries (MUST for protocol work)
N/A (not protocol work)

**Key insights:**
- Every `ze config edit` connects via SSH, making live session detection reliable
- Storage interface has List() for scanning files by prefix
- Existing set+meta serialization format supports metadata-only entries (for deletes)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cli/editor_draft.go` - write-through to shared `config.conf.draft` on every set/delete
  → Constraint: `writeThroughSet` acquires lock, reads draft, applies change, writes draft, releases lock
  → Constraint: `writeThroughDelete` follows same protocol
  → Constraint: `readDraftOrConfig` falls back to cloning in-memory tree when no draft exists
  → Constraint: `readCommittedTree` re-reads config.conf under lock for Previous field
- [ ] `internal/component/cli/editor_commit.go` - commit reads shared draft, applies to config.conf
  → Constraint: `CommitSession` reads both config.conf and draft under lock
  → Constraint: conflict detection: stale (committed changed) and live (other session disagrees)
  → Constraint: `DiscardSessionPath` reads shared draft, removes session entries, restores values
  → Constraint: `DisconnectSession` removes another session from shared draft
- [ ] `internal/component/cli/editor.go` - Editor struct, SetValue/DeleteValue dispatch
  → Constraint: `SetValue` calls `writeThroughSet` when session active, else mutates tree directly
  → Constraint: `DeleteValue` calls `writeThroughDelete` when session active
- [ ] `internal/component/cli/editor_session.go` - EditSession, DraftPath helper
  → Constraint: `DraftPath` = configPath + ".draft"
  → Decision: add `ChangePath(configPath, user)` = configPath + ".change." + user
- [ ] `internal/component/cli/model_commands.go` - cmdSet, cmdDelete, cmdSave, cmdCommit
  → Constraint: cmdSave in session mode is currently a no-op ("write-through keeps draft on disk")
- [ ] `internal/component/cli/model.go` - autoSaveOnQuit, CheckDraftChanged polling
  → Constraint: autoSaveOnQuit is no-op in session mode (write-through already persisted)

**Behavior to preserve:**
- In-memory tree always in sync with on-disk state (write-through on each edit)
- Metadata format: `#user @source %time ^previous` prefixes
- Lock-based concurrency for file writes
- `walkOrCreateIn`, `walkOrCreateMeta`, `walkPath` navigation helpers unchanged
- `parseConfigWithFormat` auto-detection unchanged
- Editor TUI (model, viewport, completions) unchanged
- Non-session mode (no session set) unchanged

**Behavior to change:**
- Write-through target: `config.conf.draft` (shared) to `config.conf.change.<user>` (per-user)
- Change file format: full tree+meta to changes-only entries
- Save: no-op to applying changes from change file to draft
- Commit: read shared draft to read own change file + scan others
- Discard: remove from shared draft to delete own change file + reload base
- After each edit: scan all change files and report conflicts to user

## Data Flow (MANDATORY)

### Entry Point
- User types `set bgp router-id 5.6.7.8` in editor TUI
- Dispatched via `model_commands.go:cmdSet` to `editor.go:SetValue` to `editor_draft.go:writeThroughSet`

### Transformation Path
1. `cmdSet` validates path and value against YANG schema
2. `SetValue` dispatches to `writeThroughSet` (session mode)
3. `writeThroughSet`: lock, read change file (or base), apply set, record meta, write change file, unlock
4. After write: `detectConflicts` scans all `config.conf.change.*` files, returns conflicts
5. Conflicts returned to model via commandResult for display in status bar

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Model → Editor | `SetValue(path, key, value)` returns error | [ ] |
| Editor → Storage | `WriteGuard.WriteFile` (within lock) | [ ] |
| Editor → Storage | `store.List(prefix)` to scan change files | [ ] |
| Editor → Model | Conflict info via new `Editor.DetectConflicts()` method | [ ] |

### Integration Points
- `storage.Storage.List(prefix)` - scan for `config.conf.change.*` files
- `config.SerializeSetWithMeta` - serialize change entries
- `config.NewSetParser().ParseWithMeta` - parse change entries
- `walkOrCreateMeta`, `getValueAtPath` - metadata and value navigation

### Architectural Verification
- [ ] No bypassed layers (write-through still goes through storage with locks)
- [ ] No unintended coupling (change files are independent per user)
- [ ] No duplicated functionality (reuses existing set+meta format)
- [ ] Zero-copy preserved where applicable (N/A for config editing)

## Wiring Test (MANDATORY)

| Entry Point | to | Feature Code | Test |
|-------------|---|--------------|------|
| `set bgp router-id X` in TUI | to | `writeThroughSet` writes to change file | `test/editor/session/write-through.et` |
| `save` in TUI | to | `SaveDraft` applies changes to draft | `test/editor/session/save-draft.et` |
| `commit` in TUI | to | `CommitSession` saves then commits | `test/editor/session/commit.et` |
| `discard` in TUI | to | `DiscardSessionPath` deletes change file | `test/editor/session/discard-path.et` |
| Two users edit same leaf | to | `detectConflicts` reports conflict | `test/editor/session/conflict-live.et` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `set` with active session | Change written to `config.conf.change.<user>`, not `config.conf.draft` |
| AC-2 | `delete` with active session | Delete entry written to `config.conf.change.<user>` |
| AC-3 | `set` on same leaf twice | Second entry replaces first in change file (not appended) |
| AC-4 | Two users edit same leaf differently | Both users see conflict warning after their edit |
| AC-12 | Session starts with existing change files from other users | Conflicts reported immediately on load |
| AC-5 | Two users edit different leaves | No conflict reported |
| AC-6 | `save` command | Changes from `config.conf.change.<user>` applied to `config.conf.draft`, change file deleted |
| AC-7 | `commit` command | Saves first (AC-6), then applies draft to `config.conf`, draft deleted |
| AC-8 | `discard` command | Own change file deleted, in-memory state reloaded from base (draft or committed) |
| AC-9 | Change file format | Contains only changed entries, not full tree |
| AC-10 | Conflict with dead SSH session | Ignored silently, reported on reconnect |
| AC-11 | `save` when committed config changed since editing started | Conflict detected and reported |
| AC-13 | `commit` applies draft to config.conf | Current config.conf renamed to `config.conf-<date>` as backup before overwrite |
| AC-14 | Blob key for config.conf created | Key netcapstring capacity includes room for `-YYYYMMDD-HHMMSS.mmm` suffix (16 extra bytes) so rename is in-place, no realloc |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWriteThroughToChangeFile` | `editor_test.go` | AC-1: set writes to change file |  |
| `TestDeleteToChangeFile` | `editor_test.go` | AC-2: delete writes to change file |  |
| `TestChangeFileReplacesSameLeaf` | `editor_test.go` | AC-3: same leaf replaced |  |
| `TestDetectConflictSameLeaf` | `editor_test.go` | AC-4: conflict detected |  |
| `TestNoConflictDifferentLeaves` | `editor_test.go` | AC-5: no false conflict |  |
| `TestSaveDraftAppliesToDraft` | `editor_test.go` | AC-6: save creates draft |  |
| `TestCommitSavesThenApplies` | `editor_test.go` | AC-7: commit = save + apply |  |
| `TestDiscardDeletesChangeFile` | `editor_test.go` | AC-8: discard cleans up |  |
| `TestChangeFileFormatChangesOnly` | `editor_test.go` | AC-9: not full tree |  |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `write-through.et` | `test/editor/session/` | set writes to per-user change file |  |
| `save-draft.et` | `test/editor/session/` | save applies changes to draft |  |
| `commit.et` | `test/editor/session/` | commit saves and applies |  |
| `conflict-live.et` | `test/editor/session/` | two users editing same leaf |  |

## Files to Modify
- `internal/component/cli/editor_session.go` - add `ChangePath`, `ChangePrefix` helpers
- `internal/component/cli/editor_draft.go` - redirect write-through to change file, add `SaveDraft`, `DetectConflicts`
- `internal/component/cli/editor_commit.go` - update `CommitSession` (save+commit, rename backup), `DiscardSessionPath` (delete change file)
- `internal/component/cli/editor.go` - add fields for change file tracking
- `internal/component/cli/model_commands.go` - update `cmdSave` to call `SaveDraft`, propagate conflicts from `cmdSet`/`cmdDelete`
- `internal/component/cli/model.go` - update `autoSaveOnQuit`
- `internal/component/config/storage/storage.go` - add `Rename(old, new string) error` to Storage interface
- `internal/component/config/storage/blob.go` - implement Rename for blob storage (in-place key rewrite)
- `pkg/zefs/store.go` - key capacity pre-allocation for `file/active/` keys (+16 bytes for date suffix), add `Rename` method

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes | `test/editor/session/*.et` |

## Files to Create
- `test/editor/session/save-draft.et` - functional test for save command

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6-12 | Standard flow |

### Implementation Phases

1. **Phase: Path helpers and change file format** - Add `ChangePath`/`ChangePrefix` to `editor_session.go`. Define change-file-only serialization (entries without full tree).
   - Tests: `TestChangeFileFormatChangesOnly`
   - Files: `editor_session.go`, `editor_draft.go`

2. **Phase: Redirect write-through** - Change `writeThroughSet`/`writeThroughDelete` to write to per-user change file instead of shared draft. Change file contains only changed entries.
   - Tests: `TestWriteThroughToChangeFile`, `TestDeleteToChangeFile`, `TestChangeFileReplacesSameLeaf`
   - Files: `editor_draft.go`

3. **Phase: Conflict detection** - After each write-through, scan all `config.conf.change.*` files and detect overlapping paths with different values.
   - Tests: `TestDetectConflictSameLeaf`, `TestNoConflictDifferentLeaves`
   - Files: `editor_draft.go`, `model_commands.go`

4. **Phase: Save and commit** - Implement `SaveDraft` (apply changes to draft) and update `CommitSession` (save first, then commit draft).
   - Tests: `TestSaveDraftAppliesToDraft`, `TestCommitSavesThenApplies`
   - Files: `editor_draft.go`, `editor_commit.go`, `model_commands.go`

5. **Phase: Discard** - Update `DiscardSessionPath` to delete own change file and reload from base state.
   - Tests: `TestDiscardDeletesChangeFile`
   - Files: `editor_commit.go`

6. **Phase: Update .et tests and remaining test fixes**
   - Update existing `.et` session tests for per-user change files
   - Fix unit tests that assumed shared draft

7. **Full verification** - `make ze-verify`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has implementation with file:line |
| Correctness | Change file contains only changes, not full tree |
| Correctness | Conflict detection scans all change files, not just own |
| Correctness | Save applies changes to draft correctly (set + delete) |
| Correctness | Commit deletes both change file and draft after success |
| Data flow | write-through path goes to change file, not shared draft |
| Rule: no-layering | Old shared-draft write-through code fully replaced |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `ChangePath` function | `grep "func ChangePath" editor_session.go` |
| `SaveDraft` method | `grep "func.*SaveDraft" editor_draft.go` |
| `DetectConflicts` method | `grep "func.*DetectConflicts" editor_draft.go` |
| Per-user change file created on set | `write-through.et` passes |
| Conflict detection works | `conflict-live.et` passes |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | User identity in filename: no path traversal (no `/`, `..`) |
| Resource exhaustion | Change file scanning bounded by number of users |
| File permissions | Change files created with 0o600 |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| In-memory only, save defers to disk | Every edit writes to disk immediately (change file) | User correction | Redesigned from deferred-write to per-user write-through |
| Single shared draft updated by save | Save creates a NEW draft from change file | User correction | Draft is a checkpoint, not a continuously updated file |
| Full tree in change file | Only changed entries | User correction | Simpler format, cleaner separation |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Deferred-write with shared draft | User wanted immediate writes + per-user isolation | Per-user change files with write-through |
| In-memory accumulation | User said "no in-memory" - every edit persists | Write-through to per-user change file |

## Design Insights
- Per-user change files eliminate read-merge-write complexity entirely
- SSH session liveness can be used to distinguish active vs orphaned change files
- Change-file-only format (not full tree) is cleaner and avoids redundancy with draft/committed
