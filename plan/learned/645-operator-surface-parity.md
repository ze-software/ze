# Learned: operator-surface-parity

Spec: `plan/spec-op-2-surface-parity.md`

## What was built

Two operator-facing surface mismatches resolved:

1. **Web config list rename** -- end-to-end rename through the per-user session editing model. The rename button in list tables was already rendered but had no backend. Now `POST /config/rename/<entry-path>/` renames a keyed list entry through `EditorManager.RenameListEntry`, preserving the subtree and session semantics.

2. **Hub-mode `--plugin` rejection** -- hub/orchestrator configs now reject `--plugin` with a clear error pointing to `plugin { external ... }` instead of silently dropping the flag.

## Key design decision: dedicated change-file structural ops

Rename is represented as a first-class structural operation in per-user change files, not decomposed into leaf delete/create churn.

- `internal/component/config/change_file.go` introduces `StructuralOp` and dedicated parse/serialize for per-user change files
- Rename lines use the same metadata prefix as leaf edits: `#user @source %time rename <parent> <list> <old> to <new>`
- Normal draft and committed config files never contain rename directives -- structural ops are change-file-only
- `SaveDraft()` applies structural ops first, then replays leaf edits, then writes a materialized draft
- Same-session rename chains are coalesced: `old -> mid -> new` becomes `old -> new`

This was the right call because:
- It avoids a mass of synthetic set/delete entries that obscure the operator's intent
- It makes rename count as exactly one pending change in the diff/count UI
- It keeps the existing materialized draft/commit format unchanged
- It integrates cleanly with conflict detection (rename vs leaf-edit overlap checks)

## What to watch

- The `PendingChange` type exists in both `config` and `contract` packages with identical field names but separate types. The adapter layer casts between them. If either type diverges, the cast silently loses data.
- `MetaTree.RenameListEntry` was added to support subtree rebase during rename. It should not be used for general-purpose meta manipulation outside the rename path.
