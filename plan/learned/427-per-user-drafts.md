# 427 -- Per-User Change Files

## Context

When multiple users edit config via `ze config edit` over SSH, they all wrote to a single shared `config.conf.draft`. This created race conditions where concurrent edits could overwrite each other silently. The goal was to replace the shared draft with per-user change files so each user's edits are isolated, write-through is immediate, and conflicts are detected explicitly.

## Decisions

- Chose per-user change files (`config.conf.change.<user>`) over shared draft for edit tracking, because per-user files eliminate read-merge-write complexity entirely.
- Change files contain only changed entries (sparse format), not full tree dumps. Simpler, avoids redundancy with draft/committed layers.
- Three-layer model: committed (`config.conf`) as source of truth, saved draft (`config.conf.draft`) as checkpoint, per-user change files as edit journals.
- `save` applies changes from change file to draft (creates a checkpoint), over keeping the draft continuously updated, because the draft is a deliberate save point, not a live mirror.
- Conflict detection scans all `config.conf.change.*` files after each edit, over only checking at commit time, because immediate feedback prevents wasted work.
- SSH session liveness distinguishes active conflicts from orphaned change files, over treating all change files equally.
- Key capacity pre-allocated +20 bytes for `file/active/` keys in blob store, enabling in-place rename for date-stamped backups without realloc.
- User identity sanitized via `filepath.Base` + whitelist validation to prevent path traversal in change filenames.

## Consequences

- Multiple users can edit config concurrently without silent overwrites.
- Each user sees conflicts immediately after each `set`/`delete`, not only at commit time.
- `save` is now a meaningful operation (applies changes to shared draft) instead of a no-op.
- `discard` cleanly deletes the user's change file and reloads from base state.
- `commit` does save-then-commit with both live (other users) and stale (committed changed) conflict detection.
- Backup rename on commit uses date-stamped paths in `rollback/` directory.
- `who` and `disconnect` commands work with per-user change files (disconnect deletes another user's change file).

## Gotchas

- Initial design assumed deferred-write with shared draft. User corrected to immediate write-through per-user. Three design iterations before landing on the right model.
- Change file format must be sparse (only changes), not full tree. Full tree was rejected because it duplicates the draft/committed content.
- `readDraftOrConfig` fallback clones `e.tree` which still has user changes. `DiscardSessionPath` must read committed config directly instead.
- Partial discard must re-apply remaining change file entries after reloading base state.
- Blob store key capacity is +20 (not +16 as originally specced) to fit the full `-YYYYMMDD-HHMMSS.mmm` suffix (20 chars).

## Files

- `internal/component/cli/editor_draft.go` -- write-through to per-user change files, SaveDraft, DetectConflicts
- `internal/component/cli/editor_commit.go` -- CommitSession (save+commit), DiscardSessionPath, DisconnectSession
- `internal/component/cli/editor_session.go` -- ChangePath, ChangePrefix, ValidateUser, sanitizeUser
- `internal/component/cli/model_commands.go` -- cmdSave calls SaveDraft, conflict display in cmdSet/cmdDelete
- `internal/component/config/storage/storage.go` -- Rename method on Storage interface
- `pkg/zefs/store.go` -- key capacity pre-allocation for backup renames
- `test/editor/session/` -- 5 functional tests (write-through, save-draft, commit, conflict-live, discard-path)
