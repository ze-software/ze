# Backup and restore

## Meta

| Field | Value |
|-------|-------|
| Name | Backup and restore |
| Page | docs/guide/command-reference.md#ze-data |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/storage/cli, internal/component/config/storage/backup.go, internal/plugins/init |
| Real-path tests | test/plugin/data-backup.ci, test/plugin/data-backup-live.ci, test/plugin/data-backup-refused-live.ci, test/plugin/data-restore-config.ci, test/plugin/data-restore-config-live.ci, test/plugin/data-restore-full.ci, test/plugin/data-restore-full-refused-live.ci, test/plugin/data-restore-full-resume.ci, test/managed/data-restore-client-live.ci |
| Docs | docs/guide/command-reference.md |
| Doc review | 2026-10-07: cmdBackup parses `<file> [spare <n>]`, backup.go creates the artifact 0600, open.go names the .replaced- infix; matches the row |
| Defect review | 2026-10-07: no open spec or journal row found naming backup or restore |

## Description

`ze data backup <file> [spare <n>]` copies every key of the live store to one new 0600 blob artifact under the write lock, so a commit is in the backup whole or not at all. `ze data restore <file> config [name <source-name>]` commits one config from a verified artifact as a new version of this device's config and leaves credentials, identity and history unchanged. `ze data restore <file> full` replaces the whole tree, moves the previous tree to `database.replaced-<stamp>`, and records a durable intent, so an interrupted restore is refused by every opener until the same command finishes it. Both verbs are offline; while a daemon owns the store, `request data backup path <file>` and `request data restore path <file> config` run over SSH, and a restored config becomes active only when the reload accepts it. `ze init --from <path-or-url> [--sha256 <hex>]` creates a store from a blob, with the digest checked before a key is written. `ze config edit --backup <artifact>` edits a config inside a backup with no daemon. <!-- source: internal/component/config/storage/cli/cmd_backup.go -- cmdBackup --> <!-- source: internal/component/config/storage/cli/cmd_restore.go -- cmdRestore --> <!-- source: internal/component/config/storage/cli/data_rpc.go -- handleDataBackup, handleDataRestore --> <!-- source: internal/plugins/init/main.go -- runImport -->
