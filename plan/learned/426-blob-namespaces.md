# 426 — Blob Namespaces

## Context

ZeFS blob store used flat, unstructured keys (`ssh/username`, `etc/ze/router.conf`). There was no way to distinguish instance metadata from config files, and no slot for future config versioning (drafts, historical snapshots). The goal was to introduce structured key namespaces as groundwork for config-versioning and fleet-config.

## Decisions

- **Two namespaces:** `meta/` for instance metadata, `file/active/` for current config files, chosen over a single flat namespace
- **Qualifier slot** (`active`) in file keys enables future `draft` and date-stamped qualifiers without format changes, chosen over versioning in a separate store
- **`resolveKey()` is idempotent** — already-namespaced keys pass through unchanged, so `List()` results feed back to `ReadFile()` without double-prefixing
- **Strip to basename** — `resolvePathToKey()` uses `filepath.Base()`, so `/etc/ze/router.conf` becomes `file/active/router.conf` (no directory path in blob), chosen over preserving full filesystem paths
- **`ze init` writes `meta/instance/name` and `meta/instance/managed`** — foundation for fleet management

## Consequences

- `spec-config-versioning` can add `file/draft/` and `file/<date>/` qualifiers without touching the storage layer
- `spec-fleet-config` can use `meta/instance/managed` to toggle hub connectivity
- `ze data ls meta/` and `ze data ls file/` cleanly separate concerns
- Migration happens automatically on first blob open via `migrateExistingFiles()`

## Gotchas

- `sshclient.go` and `init/main.go` must stay in sync — if init writes `meta/ssh/*` but client reads `ssh/*`, all CLI commands break
- The doc in `zefs-format.md` had stale examples showing full paths (`file/active/etc/ze/router.conf`) when the code actually strips to basename — fixed

## Files

- `internal/component/config/storage/blob.go` — `resolveKey()`, `migrateExistingFiles()`
- `cmd/ze/init/main.go` — `meta/ssh/*` constants, `meta/instance/name`, `meta/instance/managed`
- `cmd/ze/internal/ssh/client/client.go` — reads `meta/ssh/*` keys
- `docs/architecture/zefs-format.md` — namespace convention documentation
- `test/managed/init-meta-keys.ci`, `cli-reads-meta-keys.ci` — functional tests
