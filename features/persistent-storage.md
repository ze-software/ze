# Persistent storage

## Meta

| Field | Value |
|-------|-------|
| Name | Persistent storage |
| Page | docs/architecture/storage-backends.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/storage, cmd/ze/hub/config_source.go |
| Real-path tests | test/plugin/data-check-tree.ci, test/plugin/data-check-history.ci |
| Docs | docs/architecture/storage-backends.md |
| Doc review | 2026-10-07: history.go stores each version under its sha256 digest (entryDigestPrefix) and verifies on read; anchors recoverFileCommit and promoteConfigCandidate exist |
| Defect review | 2026-10-07: open: plan/journal/store-serializes-in-process-only.md row 12 (writes serialize in one process only) |
| Extra criteria | supported: a user guide page for the database/ store = none yet; supported: crash mid-publication then recovery driven by a functional test = none yet; supported: cross-process write safety or a published single-writer bound = none yet |

## Description

One live `database/` tree holds checksummed keys, credentials, runtime state, and per-config version pointers. Config history is content-addressed: each version's bytes are stored once under their SHA-256, and every read verifies the hash. <!-- source: internal/component/config/storage/history.go -- writeVersionObject, readVersionEntry --> One process owns writes; read-only inspection remains available. Explicit config files remain authoritative on every explicit-file start. Daemon commits update the file and history through a recoverable publication intent. ZeFS blobs remain explicit offline artifacts and appliance seeds. <!-- source: internal/component/config/storage/open.go -- Open, CreatePopulated --> <!-- source: internal/component/config/storage/import.go -- ImportBlob --> <!-- source: cmd/ze/hub/config_source.go -- recoverFileCommit, promoteConfigCandidate -->
