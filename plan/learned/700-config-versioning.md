# 700 -- Config Versioning

## Context

Config backups were handled entirely in the editor: `createBackup` built filesystem-style `rollback/<stem>-<stamp>.conf` paths, `ListBackups` parsed the rollback directory with a regex. For blob storage, these paths resolved incorrectly through `resolveKey` (stripped to basename, landed in `file/active/`), so listing versions from blob storage never worked. The `resolveRollbackBaseline` method read backups via `os.ReadFile`, bypassing the Storage interface entirely.

## Decisions

- **Version API on Storage**, not on editor: `WriteVersion` and `ListVersions` added to the `Storage` interface and `WriteVersion` to `WriteGuard`, chosen over keeping path logic in the editor because the two backends need different key layouts
- **Filesystem keeps rollback/ directory**: `writeVersionFS` writes `rollback/<stem>-<stamp>.conf`, same layout as before, chosen over changing the filesystem layout because it preserves backward compatibility with existing deployments
- **Blob uses `file/<stamp>/<basename>` keys**: via `KeyFileVersion` (`file/{date}/{basename}`), matching the namespace convention from spec-blob-namespaces
- **No `ReadVersion` on the interface**: callers use `ListVersions` to get `VersionInfo.Path`, then `ReadFile(path)`. A dedicated `ReadVersion(name, stamp)` method was added then removed because it had zero production callers
- **Draft shown in history via `HasDraft`**, not by including drafts in `ListVersions` return, to avoid breaking rollback numbering in all callers

## Consequences

- Blob storage properly tracks historical config versions for the first time
- Editor's `createBackup` reduced from 15 to 6 lines, `ListBackups` from 50 to 10
- `resolveRollbackBaseline` now works for blob storage (reads through `editor.ReadBackupContent`)
- `ParseVersionStamp` validates stamp format (rejects traversal attempts, ms > 999)
- The `reserved` map in `blobStorage.ListVersions` must be updated if new `file/<qualifier>/` namespaces are added

## Gotchas

- `DraftPath` returns `configPath + ".draft"` which resolves to `file/active/<name>.draft` in blob storage, not `file/draft/<name>`. The draft system and the version namespace are separate concerns
- The editor's own `atomicWriteFile` (no MkdirAll) differs from the storage package's `atomicWriteFile` (with MkdirAll). Version writes go through the storage version, which creates the rollback directory automatically

## Files

- `internal/component/config/storage/storage.go` -- `VersionInfo`, `WriteVersion`/`ListVersions` on Storage, `WriteVersion` on WriteGuard, filesystem implementations, stamp format/parse helpers
- `internal/component/config/storage/blob.go` -- blob implementations of `WriteVersion`/`ListVersions`, guard `WriteVersion`
- `internal/component/cli/editor.go` -- simplified `createBackup`/`ListBackups`, added `ReadBackupContent`/`HasDraft`
- `internal/component/cli/model_commands_show.go` -- `resolveRollbackBaseline` reads through Storage
- `internal/component/config/cli/cmd_history.go` -- draft line in history output
- `pkg/zefs/keys.go` -- `KeyFileVersion`
- `docs/architecture/zefs-format.md` -- removed "(future)" from version key docs
