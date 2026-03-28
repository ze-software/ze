# 456 -- ZeFS Integration

## Objective

Integrate the zefs blob store as a multi-config database behind a storage abstraction, replacing direct filesystem I/O throughout ze's configuration system while preserving all existing behavior.

## Decisions

- **Two-implementation interface:** `Storage` interface with `filesystemStorage` (wraps os calls) and `blobStorage` (wraps zefs). All config I/O goes through the interface.
- **WriteGuard pattern:** Locked write operations return a `WriteGuard` that provides `ReadFile`/`WriteFile`/`Remove`/`Release`. Blob guard wraps zefs `WriteLock` (in-process mutex + flush-on-release). Filesystem guard wraps `flock`.
- **Key format:** Full resolved filesystem paths with leading `/` stripped (e.g., `/etc/ze/router.conf` becomes `etc/ze/router.conf`). Meaningful when inspected, works regardless of blob location.
- **`IsBlobStorage()` type check:** Minimal escape hatch (2-3 call sites) for behavior that genuinely differs by backend (PID files stay on filesystem, SSH host key served from memory).
- **`ListConfigs()` not a separate method:** `List()` + caller-side `.conf` filtering is simpler than a special-purpose method that duplicates List logic.
- **`editor_lock.go` deleted:** Lock logic moved into `storage.WriteGuard`. No layering -- replaced, not wrapped.
- **Interactive selection injected I/O:** `doSelectConfig(store, configDir, defaultPath, in, errw, timeout)` follows established `doPromptCreateConfig` testability pattern.

## Patterns

- **WithStorage/Impl pattern:** Each storage-aware command gets `cmdXyzWithStorage(store, args)` wrapper and `cmdXyzImpl(store, args)` testable core. Non-storage commands keep direct `cmdXyz(args)`.
- **Handler map split:** `storageHandlers` (6 entries needing storage) vs `subcommandHandlers` (5 entries not needing storage). `RunWithStorage(store, args)` dispatches to the right map.
- **Migration on first create:** `NewBlob` detects missing blob file, creates it, and imports existing configs. Uses hardcoded glob patterns (`*.conf`, `*.conf.draft`, `rollback/*.conf`, `ssh_host_*`). Idempotent -- skips keys already in blob.
- **Atomic filesystem writes:** `atomicWriteFile` uses temp file + sync + rename. Blob atomicity handled by zefs flush-on-Release.
- **Blob in tests:** Write config to filesystem first, let `NewBlob` migration auto-import it, then delete filesystem copy to prove test reads from blob.

## Gotchas

- **Empty blob `List()` error:** `store.List(configDir)` returns an error when the directory prefix does not exist in an empty blob (unlike filesystem which returns empty slice). Must treat List errors as "no configs found" in `doSelectConfig`.
- **Auto-linter import removal:** Adding an import without its usage in the same edit causes `goimports` hook to remove it. Fix: add the function using the import first, let goimports add the import automatically.
- **Test location vs spec:** Tests belong near the code they exercise, not necessarily where the spec predicted. `TestDefaultConfigName` near the constant in `cmd_edit_test.go`, not `storage_test.go`. `TestReloadWithBlobStorage` exercises `LoadReactorFile` API so lives in `loader_test.go`.
- **Test runner binary location:** `buildZe()` and `runner.NewRunner` were building to temp dirs, making `DefaultConfigDir()` return "" (temp path not under `bin/`). Fixed to build to `bin/ze` so config dir resolves via GNU prefix conventions.
- **Blob relative path mismatch:** Migration stores keys using `filepath.Abs()` (absolute paths), but CLI commands pass relative paths. Fixed with `resolveKey()` in blob storage that resolves relative paths against configDir before converting to blob keys.
- **`ZE_CONFIG_DIR` env var:** Added to `resolveStorage()` so `.ci` tests can override config directory to tmpfs working dir where blob migration imports configs.
- **Orchestrated runner env var bug:** `option=env:var=` was only propagated to the "client" path, not to `cmd=foreground` processes. Fixed by appending `rec.EnvVars` in the orchestrated path too.

## Files

- `internal/component/config/storage/storage.go` -- Storage interface, WriteGuard, filesystemStorage, atomicWriteFile
- `internal/component/config/storage/blob.go` -- blobStorage wrapping zefs, migration logic
- `internal/component/config/storage/storage_test.go` -- 21 tests for both backends
- `cmd/ze/main.go` -- resolveStorage(), -f flag, storage dispatch
- `cmd/ze/config/main.go` -- storageHandlers/subcommandHandlers split, RunWithStorage
- `cmd/ze/config/cmd_edit.go` -- doSelectConfig (AC-6/AC-7), defaultConfigName, blob-aware flow
- `cmd/ze/config/cmd_edit_test.go` -- 6 selection tests + default name test
- `internal/component/cli/editor.go` -- NewEditorWithStorage, storage field
- `internal/component/cli/editor_draft.go` -- draft ops through storage
- `internal/component/bgp/config/loader.go` -- LoadReactorFile(store, path)
- `internal/component/ssh/ssh.go` -- host key through storage
- `internal/core/pidfile/pidfile.go` -- storage-aware PID
- `internal/test/runner/runner.go` -- build to bin/ instead of temp dir
- `internal/test/runner/runner_exec.go` -- fixed EnvVars propagation to orchestrated commands
- `cmd/ze-test/bgp.go` -- buildZe() to bin/ze, removed temp cleanup
- `test/ui/cli-zefs-blob-storage.ci` -- functional test: blob storage wiring
- `test/ui/cli-zefs-filesystem-override.ci` -- functional test: filesystem override wiring
