# Backup and restore

## Meta

| Field | Value |
|-------|-------|
| Name | Backup and restore |
| Page | docs/guide/command-reference.md#ze-data |
| Kind | daemon |
| Scope | complete |
| Level | supported |
| Components | internal/component/config/storage/cli, internal/component/config/storage/backup.go, internal/plugins/init, internal/component/web/backup_workspace.go, internal/component/config/storage/artifact_workspace.go |
| Real-path tests | test/plugin/data-backup.ci, test/plugin/data-backup-live.ci, test/plugin/data-backup-refused-live.ci, test/plugin/data-restore-config.ci, test/plugin/data-restore-config-live.ci, test/plugin/data-restore-full.ci, test/plugin/data-restore-full-refused-live.ci, test/plugin/data-restore-full-resume.ci, test/managed/data-restore-client-live.ci, test/plugin/data-rm-live.ci, test/plugin/data-rm-refused-live.ci, test/plugin/data-write-live.ci, test/plugin/data-import-existing.ci, test/plugin/data-import-live.ci, test/plugin/config-import-live.ci, test/plugin/data-repair-live.ci, test/plugin/data-check-live.ci, test/plugin/data-authz-read-only.ci, test/plugin/ddos-flowspec-removed-section-orphan-cleared.ci |
| Docs | docs/guide/command-reference.md, docs/guide/web-interface.md, docs/guide/config-editor.md |
| Doc review | 2026-11-17: reviewed browser route registration, complete artifact transfer and artifact-only publication; preserved the existing CLI/storage maturity evidence. Earlier, 2026-11-09: re-read after the docs/guide/command-reference.md edits since 2026-10-07; the data backup, data restore and ze init --from rows are unchanged, every source anchor resolves, and the description still matches. Earlier, 2026-10-07: re-read at promotion; cmdBackup parses `<file> [spare <n>]`, backup.go creates the artifact 0600, cmdRestore takes config [name <n>] or full, open.go names the .replaced- infix, data_rpc.go takes path, init takes --sha256, config cli takes --backup; matches the row |
| Defect review | 2026-10-07: no open spec or journal row found naming backup or restore |

## Description

`ze data backup <file> [spare <n>]` copies every key of the live store to one new 0600 blob artifact under the write lock, so a commit is in the backup whole or not at all. `ze data restore <file> config [name <source-name>]` commits one config from a verified artifact as a new version of this device's config and leaves credentials, identity and history unchanged. `ze data restore <file> full` replaces the whole tree, moves the previous tree to `database.replaced-<stamp>`, and records a durable intent, so an interrupted restore is refused by every opener until the same command finishes it. Both verbs are offline; while a daemon owns the store, `request data backup path <file>` and `request data restore path <file> config` run over SSH, and a restored config becomes active only when the reload accepts it. `ze init --from <path-or-url> [--sha256 <hex>]` creates a store from a blob, with the digest checked before a key is written. `ze config edit --backup <artifact>` edits a config inside a backup with no daemon. A damaged store is checked and salvaged while ze runs: `ze data check` and `ze data repair --output <path>` on the host, and `show data check` and `request data repair path <file>` through the daemon, which writes the salvage on its host with its writes held for the walk; installing the salvage needs ze stopped. The store is maintained while ze runs, beside backup and restore: `ze data rm`, `ze data write` and `ze data import` on the host send their work to the daemon (`delete data key`, `update data key`, `update data config`), which removes or writes runtime state and imports a config as a new version, through the reload for its own config. Offline, `ze data import` of a config the store holds commits a new active version and keeps the previous one as rollback. <!-- source: internal/component/config/storage/cli/data_rpc_config.go -- handleDataConfigUpdate --> <!-- source: internal/component/config/storage/cli/main.go -- importConfigs, importConfigsThroughDaemon --> <!-- source: internal/component/config/storage/cli/data_rpc_repair.go -- handleDataRepair --> <!-- source: internal/component/config/storage/cli/storage_data.go -- dataCheck --> <!-- source: internal/component/config/storage/cli/cmd_backup.go -- cmdBackup --> <!-- source: internal/component/config/storage/cli/cmd_restore.go -- cmdRestore --> <!-- source: internal/component/config/storage/cli/data_rpc.go -- handleDataBackup, handleDataRestore --> <!-- source: internal/plugins/init/main.go -- runImport -->

The authenticated [offline backup workspace](../docs/guide/web-interface.md#offline-backup-workspace)
accepts a `.zefs` upload and lists its configurations. Open a committed
configuration, edit its native text, validate and review the changes, then save
and download the complete artifact. The workspace preserves other
configurations, credentials and saved pending records. Its changes remain
inside the uploaded copy, independent of the live configuration.
<!-- source: internal/component/web/backup_workspace.go -- HandleBackupWorkspace -->
<!-- source: internal/component/web/backup_transfer.go -- upload, download -->
<!-- source: internal/component/config/storage/artifact_workspace.go -- PublishArtifactConfig -->
