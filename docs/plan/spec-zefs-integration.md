# Spec: ZeFS Integration

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/core/paths/paths.go` - config dir resolution
4. `internal/component/cli/editor.go` - config editor file I/O
5. `internal/component/cli/editor_draft.go` - write-through draft protocol
6. `internal/component/cli/editor_lock.go` - flock-based locking
7. `pkg/zefs/store.go` - BlobStore API
8. `docs/architecture/zefs-format.md` - netcapstring and ZeFS file format

## Task

Integrate the zefs blob store into ze's configuration storage as a multi-config database. After determining the `etc/ze` directory (using existing path resolution code), create and use `etc/ze/database.zefs` as the backing store for all configuration files, SSH host keys, and PID. The blob holds multiple named configs (e.g., `router-east.conf`, `router-west.conf`), each with its own draft and rollback history. The config name is passed via CLI; the editor defaults to `ze.conf` when no name is given. Blob storage is optional but enabled by default. The `-f` flag bypasses blob for all I/O (not just config).

### Goals

1. **Storage abstraction:** A single interface through which all config-related file I/O flows. Two implementations: filesystem (current behavior) and zefs blob store.
2. **Multi-config database:** The blob stores multiple configuration files. Each config has its own sidecar files (draft, rollback). Keys are full resolved filesystem paths (minus leading `/`).
3. **Config by name:** `ze router-east.conf` selects which config to load from the blob. `ze config edit router-east.conf` edits a specific config. The CLI resolves the name against the local directory and `{configDir}` (via `paths.DefaultConfigDir()`), and the full resolved path becomes the blob key. This enables teams to maintain a central `database.zefs` with configs for multiple servers, copy it everywhere, and have each server pick its own.
4. **Default config name:** When no config name is given, use `ze.conf` as the default. If `ze.conf` does not exist in the blob, the editor lists available configs and asks the user to choose interactively (within the session).
5. **Blob by default:** When ze starts, it opens (or creates) `etc/ze/database.zefs`. All subsequent file operations go through the blob (config, SSH host key, PID).
6. **Filesystem override (`-f`):** `ze -f /path/to/router.conf` bypasses the blob entirely -- all I/O (config, SSH host key, PID) uses the real filesystem. This is the escape hatch for debugging or configs not yet in the blob.
7. **Opt-out:** An environment variable disables blob storage globally, falling back to direct filesystem I/O (current behavior).
8. **Migration:** On first blob-enabled startup, existing flat files are imported into the blob. Originals are preserved (not deleted) until the user removes them.
9. **Blob management CLI:** `ze db` command for importing files into the blob, removing entries, listing contents, and inspecting entries (already implemented: `cmd/ze/db/main.go`).

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/config/syntax.md` - config file formats and parsing pipeline
  → Constraint: config files can be hierarchical or set+meta format; format detection is automatic
- [ ] `docs/architecture/config/yang-config-design.md` - YANG-driven config editing
  → Decision: write-through protocol uses lock, read, modify, write, unlock cycle
- [ ] `docs/architecture/zefs-format.md` - netcapstring and ZeFS file format
  → Decision: self-describing header `:<number>:<cap>:<used>:`, WriteTo API for single-allocation encoding

### Source Files

- [ ] `internal/core/paths/paths.go` - binary-relative config dir resolution
  → Constraint: returns empty string when binary layout is unknown; caller must provide explicit path
- [ ] `internal/component/config/environment.go` - socket path, config path resolution
  → Decision: socket uses XDG_RUNTIME_DIR cascade; config search uses XDG_CONFIG_HOME cascade
- [ ] `internal/component/cli/editor.go` - config editor (1056 lines)
  → Constraint: uses `atomicWriteFile`, `os.ReadFile`, `os.WriteFile` for config/draft/backup
- [ ] `internal/component/cli/editor_draft.go` - write-through protocol
  → Constraint: lock -> read -> modify -> serialize -> write -> unlock
- [ ] `internal/component/cli/editor_lock.go` - flock-based advisory locking
  → Decision: flock on `{config}.lock` file for filesystem mode; flock on blob file for blob mode (cross-process safety requires flock in both modes)
- [ ] `internal/component/cli/editor_session.go` - DraftPath(), LockPath()
  → Constraint: derives draft and lock paths from config path by appending suffix
- [ ] `internal/component/config/archive/archive.go` - config archival
  → Constraint: `ToFile()` writes backups via `os.WriteFile`; HTTP archives unaffected
- [ ] `internal/component/bgp/config/loader.go` - config loading
  → Constraint: `LoadReactorFileWithPlugins` reads config via `os.ReadFile`
- [ ] `internal/component/ssh/ssh.go` - SSH host key in config dir
  → Decision: SSH host key goes in the blob when blob mode is active; served to Wish library from memory. Only stays on filesystem in `-f` mode.
- [ ] `internal/core/pidfile/pidfile.go` - PID file with flock
  → Decision: PID goes in the blob when blob mode is active; `ze pid` command to read it. Daemon-lifetime mutual exclusion needs design (see Locking strategy).
- [ ] `pkg/zefs/store.go` - BlobStore API
  → Decision: provides ReadFile, WriteFile, Remove, fs.FS; has RLock/WriteLock guards; WriteTo API for single-allocation encoding

**Key insights:**
- Config dir resolution is binary-relative (GNU prefix conventions)
- All config sidecar files (draft, lock, rollback) derive their path from the config file path
- zefs `sync.RWMutex` is in-process only; cross-process flock via a persistent second fd on the blob file is needed for concurrent editor sessions (SSH, multiple terminals)
- API socket is the only item that cannot go into the blob (kernel Unix socket object)
- Rollback backups use filepath.Glob for listing; zefs provides fs.ReadDirFS which can replace this
- `detectConfigType` and `Orchestrator.Reload` both hardcode `os.ReadFile` and must be updated to use storage
- Zero-copy reads via lock-scoped API (ReadLock/WriteLock) -- callers hold the lock while processing raw bytes, then release; no copy needed
- `ze db` command already implemented for blob management (import, rm, ls, cat)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/paths/paths.go` - resolves `etc/ze` from binary location using GNU prefix conventions
- [ ] `internal/component/cli/editor.go` - reads/writes config, draft, backup files via os package
- [ ] `internal/component/cli/editor_draft.go` - write-through protocol: flock, os.ReadFile, atomicWriteFile
- [ ] `internal/component/cli/editor_lock.go` - advisory locking via syscall.Flock on `.lock` file
- [ ] `internal/component/cli/editor_session.go` - DraftPath appends ".draft", LockPath appends ".lock"
- [ ] `internal/component/config/archive/archive.go` - backup writes via os.WriteFile, listing via filepath.Glob
- [ ] `internal/component/bgp/config/loader.go` - config loading via os.ReadFile

**Current CLI behavior (critical for migration):**
- `ze` with no arguments: prints usage and exits with code 1 (`cmd/ze/main.go:50-53`)
- `ze <arg>`: if `looksLikeConfig(arg)` is true, dispatches to `hub.Run()`. `looksLikeConfig` checks `.conf` extension, path separators, `os.Stat()`, and stdin (`-`)
- `ze config edit <file>`: requires config file path as positional argument. If missing, prints "error: missing config file" and exits (`cmd/ze/config/cmd_edit.go:269-273`). If file does not exist, prompts to create it
- `detectConfigType(path)`: reads file from disk via `os.ReadFile(path)`, calls `config.ProbeConfigType(string(data))` to determine BGP vs hub config type
- SIGHUP reload: `Orchestrator.Reload(configPath)` reads config from disk via `os.ReadFile(configPath)` (`internal/component/hub/reload.go:64-69`)

**Behavior to preserve:**
- Config dir resolution logic in `paths.go` (binary-relative, GNU prefix conventions)
- Config file format auto-detection (hierarchical vs set+meta)
- Write-through protocol semantics (lock, read, modify, write, unlock)
- Concurrent editing conflict detection via metadata
- Rollback backup creation on commit
- Archive file upload via HTTP (unaffected by blob storage)
- XDG-based config search path for bare filenames
- `ze` with no args: continues to print usage and exit (NOT silently changed to load default config -- see Design Decisions for rationale)

**Behavior to change:**
- File I/O for config, draft, rollback, SSH host key, and PID goes through a storage abstraction instead of direct os calls
- Locking in blob mode: flock on blob file for write batching, plus separate mechanism for daemon-lifetime mutual exclusion (see Locking strategy)
- DraftPath retained (drafts stored as `{path}.draft` keys in blob); LockPath changes in blob mode
- atomicWriteFile replaced by storage WriteFile (blob handles atomicity internally via flush-on-Release)
- `detectConfigType` reads content via storage, not `os.ReadFile(path)` (already calls `ProbeConfigType(string(data))` internally)
- `Orchestrator.Reload` reads via storage instead of `os.ReadFile(configPath)`
- `ze config edit` without args: defaults to `ze.conf` in blob (currently requires positional arg)
- SSH host key: served to Wish library from blob memory instead of filesystem path
- PID: stored in blob; `ze pid` command to read it

## Data Flow (MANDATORY)

### Entry Point

- Config name enters as positional CLI argument (`ze router-east.conf`) or defaults to `ze.conf`
- CLI resolves the name: searches local directory, then `{configDir}` (via `paths.DefaultConfigDir()`)
- The full resolved absolute path (minus leading `/`) becomes the blob key
- `-f /path/to/file.conf` bypasses blob entirely -- all I/O on real filesystem
- Blob store opened or created at `{configDir}/database.zefs`

### Transformation Path

1. **Startup:** resolve configDir, open/create blob store, construct storage provider
2. **Config selection:** CLI provides config name (positional arg); stdin (`-`) and `-f` bypass blob. `ze` with no args remains unchanged (usage + exit)
3. **Config read:** lock-scoped zero-copy read from blob (ReadLock -> ReadFile -> parse -> Release). Callers process raw bytes within the lock scope; parsed data (Go strings/structs) is owned and survives lock release. Filesystem mode uses os.ReadFile (current behavior).
4. **Config type probing:** `detectConfigType` currently calls `os.ReadFile(path)` then `ProbeConfigType(string(data))`. With storage: read bytes via storage first, then call `ProbeConfigType`. The existing `hub.Run()` already has the content-based path (`ProbeConfigType(string(data))` at line 46), so the fix is upstream: `cmd/ze/main.go` reads via storage, passes bytes to `hub.Run()` instead of a path
5. **Editor set/delete:** storage AcquireLock replaces flock; read/modify/write cycle uses storage API
6. **Commit:** read config and draft from storage, detect conflicts, write config, update/remove draft via storage
7. **Backup:** write timestamped backup into storage (key = full resolved path of rollback file)
8. **SIGHUP reload:** `Orchestrator.Reload()` currently takes a path and calls `os.ReadFile(configPath)`. With storage: Reload takes a storage provider (or the Orchestrator holds one), reads config via lock-scoped read
9. **Migration:** on first blob startup, scan configDir for existing files, import into blob (or use `ze db import`)

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> Storage | Storage provider injected at startup | [ ] |
| Editor -> Storage | Editor uses storage interface instead of os package | [ ] |
| Loader -> Storage | Loader reads config via storage interface | [ ] |
| Archive -> Storage | File-based archival routes through storage | [ ] |
| SSH -> Storage | Host key served from blob memory to Wish library | [ ] |
| PID -> Storage | PID stored in blob; read via `ze pid` | [ ] |

### Integration Points

- `cmd/ze/main.go` - `detectConfigType` reads via storage instead of `os.ReadFile`; constructs storage provider before dispatching
- `cmd/ze/hub/main.go` - `Run()` receives storage provider; passes to loader and orchestrator
- `cmd/ze/db/main.go` - blob management CLI (already implemented)
- `internal/component/hub/reload.go` - `Reload()` reads via storage instead of `os.ReadFile`
- `internal/component/bgp/config/loader.go` - `LoadReactorFileWithPlugins` accepts storage parameter
- `internal/component/cli/editor.go` - Editor struct needs storage field
- `internal/component/cli/editor_draft.go` - write-through uses storage instead of os/flock
- `internal/component/cli/editor_lock.go` - replaced by storage locking when blob enabled
- `internal/component/config/archive/archive.go` - `ToFile` uses storage for file:// archives
- `internal/component/ssh/ssh.go` - reads host key from blob, serves to Wish from memory

### Zero-copy reads

zefs provides two ReadFile paths. The lock-scoped API returns zero-copy sub-slices of the mmap'd backing:

| API | Copy? | Scope |
|-----|-------|-------|
| `ReadLock.ReadFile()` | No -- sub-slice of mmap'd backing | Valid while ReadLock held |
| `WriteLock.ReadFile()` | No -- sub-slice of mmap'd backing | Valid while WriteLock held |
| `BlobStore.ReadFile()` | Yes -- copies bytes (convenience method) | Caller-owned, any lifetime |

For the storage integration, callers use lock-scoped reads: acquire ReadLock, read raw bytes, parse into owned Go structs (strings, maps, etc.), release ReadLock. The parsed data survives lock release because parsing creates owned copies. No explicit byte copy needed.

Config files are small (KB), so the lock scope is brief. The lock prevents `flush()` (which remaps the backing) from running while slices are in use.

### Architectural Verification

- [ ] No bypassed layers (all config file I/O goes through storage interface)
- [ ] No unintended coupling (storage is injected, not imported globally)
- [ ] No duplicated functionality (single storage interface, two implementations)
- [ ] Lock-scoped reads used correctly (parse within lock scope, release after)

## Design Decisions

### Multi-config model

The blob is a **multi-config database**, not a backend for a single config. Multiple configs coexist in the same blob:

| Scenario | Usage |
|----------|-------|
| Single server | `ze ze.conf` loads the default config from blob |
| Named config | `ze edge-router.conf` loads that specific config from blob |
| Filesystem override | `ze -f /path/to/router.conf` all I/O on real filesystem, blob not used |
| Stdin | `ze -` reads config from stdin (blob not used) |
| Team central repo | One `database.zefs` contains `site-a.conf`, `site-b.conf`, etc. Copied to all servers. Each server runs `ze site-a.conf` |
| Editor default | `ze config edit` opens `ze.conf` if it exists in blob |
| Editor no default | `ze config edit` with no `ze.conf` lists available configs in the blob and asks the user to pick (interactive prompt within the session) |
| Editor filesystem | `ze config edit -f /path/to/file.conf` edits a file directly on disk, not in blob |

### Config name resolution

The CLI resolves config names by searching the local directory and `{configDir}`. The full resolved absolute path (minus leading `/`) becomes the blob key.

| CLI input | Source | Resolved key | Notes |
|-----------|--------|-------------|-------|
| `ze router.conf` | blob | full resolved path (e.g., `etc/ze/router.conf`) | `looksLikeConfig()` matches `.conf` extension; path resolved via local dir + configDir search |
| `ze` (no arg) | -- | -- | Prints usage, exits code 1 (unchanged from current behavior) |
| `ze -f /etc/ze/router.conf` | filesystem | reads file directly from disk | Bypasses blob entirely -- all I/O on filesystem |
| `ze -` | stdin | -- | Stdin bypasses blob (current behavior preserved) |
| `ze config edit router.conf` | blob | full resolved path | Config name resolved to path, path becomes key |
| `ze config edit` (no arg) | blob | resolved path of `ze.conf` if exists; otherwise interactive selection | Editor-only default |
| `ze config edit -f /path/to/file.conf` | filesystem | reads file directly from disk | Same `-f` semantics as daemon |

**No-arg rationale:** The current behavior (`ze` with no args prints usage) is preserved deliberately. Silent defaulting to `ze.conf` from blob would be a breaking change and confusing to users who expect usage text. The default config name only applies to `ze config edit` (where the user is explicitly in editing mode and a reasonable default saves typing).

### Key format

Blob keys are full resolved filesystem paths with the leading `/` stripped. For example, `/etc/ze/router.conf` becomes key `etc/ze/router.conf`. This means:

- Keys are meaningful when inspected directly (e.g., with `ze db ls`)
- The same key works regardless of which directory the blob is physically in
- The CLI resolves the config name to a full path; the storage layer uses it as-is (no prefix prepended)
- `ze db import /etc/ze/router.conf` stores under key `etc/ze/router.conf` (implemented)

### Interactive config selection (editor only)

When `ze config edit` is invoked without a config name and `ze.conf` does not exist in the blob:
1. List all `.conf` files in the blob (excluding `.draft`, rollback)
2. Present numbered list to user
3. User selects by number or name
4. If no configs exist at all, create a new `ze.conf` and open the editor

This is an in-session prompt (not a CLI flag). It only applies to `ze config edit`, not to daemon startup (which defaults to `ze.conf` or fails if not found).

### What goes into the blob

| File | Key in blob | Rationale |
|------|-------------|-----------|
| Config files | full resolved path (e.g., `etc/ze/router.conf`) | Multiple configs per blob |
| Draft files | `{config-key}.draft` | Per-config concurrent editing draft |
| Rollback backups | full resolved path of rollback file | Per-config history |
| File-based archives | full resolved path of archive file | When archive location is `file://` and inside configDir |
| SSH host key | full resolved path (e.g., `etc/ze/ssh_host_ed25519_key`) | Served to Wish library from memory |
| PID | full resolved path (e.g., `run/ze/ze.pid`) | Read via `ze pid` command |

### What stays on filesystem

| File | Reason |
|------|--------|
| `database.zefs` | The blob file itself |
| API socket | Kernel Unix socket object |
| `{config}.lock` | Retained for filesystem mode only (not used when blob enabled) |

### Locking strategy

**Single-process ownership.** Only one process ever has the blob open. `ze router.conf` (the daemon) opens the blob and owns it exclusively. The daemon runs Wish (SSH server), so SSH editor sessions are goroutines within the same process. All concurrency is in-process, handled by zefs's existing `sync.RWMutex`. No cross-process flock needed.

**How each access path reaches the blob:**

| Access path | Process | Blob access |
|-------------|---------|-------------|
| `ze router.conf` (daemon startup) | daemon | Direct -- opens blob, holds in-process mutex |
| SSH `ze config edit` (via Wish) | daemon (goroutine) | In-process -- same mutex, same process |
| Terminal `ze config edit` (daemon running) | separate process | API client -- sends edit commands via Unix socket to daemon |
| Terminal `ze config edit` (no daemon) | editor process | Direct -- opens blob (sole process, no contention) |
| Terminal `ze db ls` (daemon running) | separate process | API client -- sends command via Unix socket to daemon |
| Terminal `ze db ls` (no daemon) | db process | Direct -- opens blob (sole process, no contention) |

**Terminal commands detect whether the daemon is running** by checking the API socket. If the daemon is running, they become API clients and send commands through the bus. The daemon's config component executes the operations with mutex protection and sends results back. This is the same pattern as plugins communicating with the engine.

**Daemon mutual exclusion:** Prevents two `ze router.conf` instances from running simultaneously. PID stored in the blob. On startup: acquire WriteLock (brief), read PID entry, check `kill(pid, 0)`. If alive, refuse to start. If dead (or no PID entry), write own PID, release WriteLock. The WriteLock is held only for the check-and-write (seconds), not for the daemon's lifetime.

**Write batching (in-process):** zefs WriteLock batches all writes and flushes atomically on `Release()`. This replaces the per-file atomicWriteFile pattern. Since all blob access is in-process, the `sync.RWMutex` serializes readers and writers without flock.

| Mode | Daemon mutual exclusion | Write coordination | Read path |
|------|------------------------|--------------------|-----------|
| Blob (daemon running) | PID in blob + `kill` check | In-process `sync.RWMutex` | Lock-scoped zero-copy read |
| Blob (no daemon) | PID in blob + `kill` check | In-process `sync.RWMutex` (single process) | Lock-scoped zero-copy read |
| Filesystem (`-f`) | flock on PID file (current) | flock on `{config}.lock` (current) | os.ReadFile (current) |

### Storage interface

A storage interface with these operations:

| Method | Purpose |
|--------|---------|
| ReadFile(name) | Lock-scoped zero-copy read (blob) or os.ReadFile (filesystem) |
| WriteFile(name, data, perm) | Write file content (atomic for blob, atomicWriteFile for filesystem) |
| Remove(name) | Remove a file |
| List(prefix) | List files matching a prefix (replaces filepath.Glob for backups) |
| ListConfigs() | List available config names (`.conf` files, excluding `.draft` and rollback) |
| AcquireLock() | Acquire exclusive write access, returns a WriteGuard (WriteLock for blob, flock for filesystem) |

WriteGuard is an interface with a single method `Release() error`. For blob mode, `AcquireLock()` calls `BlobStore.Lock()` to get a WriteLock (in-process mutex). `Release()` calls `WriteLock.Release()` (flushes writes). For filesystem mode, `AcquireLock()` acquires flock on `{config}.lock`. `Release()` releases flock.

Two implementations:
1. **filesystemStorage** -- wraps os.ReadFile/os.WriteFile/flock (current behavior, zero change). ListConfigs scans configDir for `.conf` files.
2. **blobStorage** -- wraps zefs BlobStore methods. ReadFile uses lock-scoped zero-copy reads. ListConfigs uses fs.ReadDirFS to enumerate `.conf` entries, filtering out drafts and rollback. When daemon is running, terminal commands use API client mode instead of direct blob access.

### Opt-out mechanism

| Mechanism | Scope | Effect |
|-----------|-------|--------|
| `-f /path/to/file.conf` | Per-invocation | Bypass blob entirely; all I/O (config, SSH key, PID) uses filesystem |
| `-` (stdin) | Per-invocation | Read config from stdin, blob not used |
| `ZE_STORAGE_BLOB=false` | Global | Disable blob entirely; all operations use filesystem |
| Default | - | Blob enabled |
| Fallback | - | If `database.zefs` cannot be created, fall back to filesystem with warning |

**Env var naming rationale:** Existing ze env vars use `ze.bgp.*` or `ze.log.*` with dots (see `environment.go`). Storage is not a BGP subsystem concern, so a new top-level namespace is needed. `ZE_STORAGE_BLOB` uses standard uppercase-underscore convention to avoid conflating storage control with BGP runtime config.

### Migration strategy

On first blob-enabled startup when `database.zefs` does not exist:

1. Create empty `database.zefs` in configDir
2. Scan configDir for files matching these exact patterns:

| Pattern | Blob key | Example |
|---------|----------|---------|
| `*.conf` | full resolved path | `/etc/ze/router.conf` becomes `etc/ze/router.conf` |
| `*.conf.draft` | full resolved path | `/etc/ze/router.conf.draft` becomes `etc/ze/router.conf.draft` |
| `rollback/*.conf` | full resolved path | `/etc/ze/rollback/router-20260313.conf` becomes `etc/ze/rollback/router-20260313.conf` |
| `ssh_host_*` | full resolved path | `/etc/ze/ssh_host_ed25519_key` becomes `etc/ze/ssh_host_ed25519_key` |

3. For each matched file: read via `os.ReadFile`, write into blob via `WriteLock`
4. Skip files that already exist in blob (idempotent -- safe to re-run if interrupted)
5. Log each imported file at INFO level (filename, size)
6. Log summary: "imported N files into database.zefs"
7. Do NOT delete originals (user can clean up manually after verifying blob works)

**Excluded from migration** (these stay on filesystem):
- `database.zefs` itself
- `*.lock` files
- API socket
- PID files (not in configDir anyway)
- Any non-config files

On subsequent startups, blob is opened normally (no re-scan). Migration is triggered only by the absence of `database.zefs`.

**Manual import:** `ze db import <file>...` can also import files into the blob at any time.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze router.conf` (named config, blob enabled) | -> | Storage reads named config from blob | `test/parse/cli-zefs-config-read.ci` |
| `ze -f /path/to/config.conf` (filesystem override) | -> | Storage reads from real filesystem | `test/parse/cli-zefs-filesystem-override.ci` |
| `ze config edit router.conf` (blob enabled) | -> | Editor write-through uses blob storage | `test/editor/lifecycle/commit-zefs-blob.et` |
| `ze config edit` (no arg, ze.conf in blob) | -> | Editor opens default config from blob | `test/editor/lifecycle/edit-zefs-default.et` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze router.conf` with blob enabled and config in blob | Config is read from blob; daemon starts normally |
| AC-2 | `ze` with no args | Prints usage and exits code 1 (unchanged behavior) |
| AC-3 | `ze router.conf` with blob enabled and no `database.zefs` | `database.zefs` is created; existing files imported |
| AC-4 | `ze config edit router.conf` set/commit with blob enabled | Draft and config written to blob under full resolved path keys |
| AC-5 | `ze config edit` with no arg and `ze.conf` exists in blob | Opens `ze.conf` for editing |
| AC-6 | `ze config edit` with no arg and no `ze.conf` in blob | Lists available configs, prompts user to select |
| AC-7 | `ze config edit` with no arg and empty blob | Creates new `ze.conf` and opens editor |
| AC-8 | `ze -f /path/to/config.conf` (filesystem override) | All I/O on real filesystem, blob not used |
| AC-9 | `ZE_STORAGE_BLOB=false` environment variable set | All file I/O uses filesystem directly (current behavior) |
| AC-10 | `ze config edit` commit with blob enabled | Rollback backup is written inside blob |
| AC-11 | `ze router.conf` after previous blob startup | Re-opening blob finds all previously stored files |
| AC-12 | `database.zefs` creation fails (permissions) | Falls back to filesystem with warning log |
| AC-13 | Blob contains `site-a.conf` and `site-b.conf` | `ze site-a.conf` loads only site-a; site-b is untouched |
| AC-14 | `ze -` (stdin) with blob enabled | Config read from stdin; blob not used |
| AC-15 | `ze config edit -f /path/to/file.conf` | All I/O on real filesystem; blob not used |
| AC-16 | SIGHUP sent to running daemon (blob enabled) | Config re-read from blob (not disk); reload succeeds |
| AC-17 | `ze db import /etc/ze/router.conf` | File imported into blob with key `etc/ze/router.conf` |
| AC-18 | `ze db ls` on populated blob | Lists all keys in blob |
| AC-19 | `ze db rm etc/ze/old.conf` | Entry removed from blob |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFilesystemStorageReadWrite` | `internal/component/config/storage/storage_test.go` | Filesystem implementation read/write/remove cycle | |
| `TestBlobStorageReadWrite` | `internal/component/config/storage/storage_test.go` | Blob implementation read/write/remove cycle | |
| `TestBlobStorageMultiConfig` | `internal/component/config/storage/storage_test.go` | Multiple configs coexist independently in blob | |
| `TestBlobStorageLocking` | `internal/component/config/storage/storage_test.go` | Lock/Unlock serializes concurrent access | |
| `TestBlobStorageMigration` | `internal/component/config/storage/storage_test.go` | Import existing files into new blob | |
| `TestBlobStorageListConfigs` | `internal/component/config/storage/storage_test.go` | List available config names (excluding drafts and rollback) | |
| `TestBlobStorageFallback` | `internal/component/config/storage/storage_test.go` | Uncreateable blob path falls back to filesystem | |
| `TestDefaultConfigName` | `internal/component/config/storage/storage_test.go` | Default config name is `ze.conf` when no name given | |
| `TestEditorWithBlobStorage` | `internal/component/cli/editor_test.go` | Editor set/commit cycle using blob storage | |
| `TestEditorConfigSelection` | `internal/component/cli/editor_test.go` | Editor lists configs and accepts selection when default missing | |
| `TestEditorFilesystemOverride` | `internal/component/cli/editor_test.go` | Editor with `-f` flag uses filesystem, not blob | |
| `TestLoaderWithBlobStorage` | `internal/component/bgp/config/loader_test.go` | Config loading through blob storage | |
| `TestReloadWithBlobStorage` | `internal/component/hub/reload_test.go` | SIGHUP reload reads config from blob storage | |
| `TestBlobStorageCrossProcessLock` | `internal/component/config/storage/storage_test.go` | AcquireLock acquires flock on persistent second fd of blob file | |

### Boundary Tests (MANDATORY for numeric inputs)

No new numeric inputs introduced by this spec.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-zefs-config-read` | `test/parse/cli-zefs-config-read.ci` | Named config loaded from blob store | |
| `cli-zefs-filesystem-override` | `test/parse/cli-zefs-filesystem-override.ci` | Config loaded from filesystem via `-f` override | |
| `commit-zefs-blob` | `test/editor/lifecycle/commit-zefs-blob.et` | Editor commit writes named config and backup to blob | |
| `edit-zefs-default` | `test/editor/lifecycle/edit-zefs-default.et` | Editor opens default ze.conf from blob when no arg given | |

### Future (if deferring any tests)

- Performance benchmarks comparing blob vs filesystem I/O (deferred -- optimization, not correctness)
- Stress test with many rollback backups in blob (deferred -- advanced behavior)

## Files to Modify

- `cmd/ze/main.go` - add `-f` flag to global parsing; construct storage provider (blob or filesystem based on `-f` and `ZE_STORAGE_BLOB`); `detectConfigType` reads via storage instead of `os.ReadFile`
- `cmd/ze/hub/main.go` - `Run()` receives storage provider; pass to loader and orchestrator
- `cmd/ze/config/main.go` - pass storage provider to editor; handle `-f` flag; handle config name arg with `ze.conf` default
- `cmd/ze/config/cmd_edit.go` - accept storage provider; add `-f` flag; implement interactive config selection when default missing
- `internal/component/hub/reload.go` - `Reload()` reads via storage instead of `os.ReadFile(configPath)`
- `internal/component/bgp/config/loader.go` - accept storage interface for config reads
- `internal/component/cli/editor.go` - use storage interface for config/draft/backup I/O
- `internal/component/cli/editor_draft.go` - use storage locking and I/O instead of flock + os
- `internal/component/cli/editor_lock.go` - storage-mode dispatch (flock on `.lock` file for filesystem, flock on blob file for blob)
- `internal/component/cli/editor_session.go` - DraftPath retained; LockPath conditional on storage mode
- `internal/component/config/archive/archive.go` - `ToFile` routes through storage when path is within configDir
- `internal/component/config/environment.go` - read `ZE_STORAGE_BLOB` env var (standard uppercase format)
- `internal/component/ssh/ssh.go` - read host key from blob, serve to Wish from memory
- `internal/core/pidfile/pidfile.go` - write PID to blob in blob mode

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| RPC count in architecture docs | No | - |
| CLI commands/flags | Yes | `cmd/ze/main.go` -- add `-f` flag to global flag parsing |
| CLI usage/help text | Yes | `cmd/ze/main.go:usage()` -- document `-f` flag; `cmd/ze/config/cmd_edit.go` -- document `-f` flag and default config name |
| API commands doc | No | - |
| Plugin SDK docs | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |

## Files to Create

- `internal/component/config/storage/storage.go` - storage interface + WriteGuard interface + filesystem implementation
- `internal/component/config/storage/blob.go` - blob storage implementation wrapping zefs (lock-scoped zero-copy reads, AcquireLock wraps flock + WriteLock)
- `internal/component/config/storage/storage_test.go` - unit tests for both implementations
- `test/parse/cli-zefs-config-read.ci` - functional test: named config read from blob
- `test/parse/cli-zefs-filesystem-override.ci` - functional test: config read from filesystem via `-f` override
- `test/editor/lifecycle/commit-zefs-blob.et` - editor test: commit cycle with blob storage
- `test/editor/lifecycle/edit-zefs-default.et` - editor test: default ze.conf opened when no arg given

## Already Implemented

| Item | Location | Status |
|------|----------|--------|
| Netcapstring format | `pkg/zefs/netcapstring.go` | Done -- self-describing header `:<number>:<cap>:<used>:` |
| ZeFS file format | `pkg/zefs/store.go` | Done -- magic `ZeFS` + container netcapstring |
| WriteTo API | `pkg/zefs/netcapstring.go` | Done -- `writeNetcapstring`, `writeNetcapstringHeader`, single-allocation `encode()` |
| Format documentation | `docs/architecture/zefs-format.md` | Done |
| `ze db` CLI | `cmd/ze/db/main.go` | Done -- import, rm, ls, cat subcommands |

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Define storage interface** in `internal/component/config/storage/storage.go`. Write unit tests for the interface contract. Implement filesystem storage first (wraps current os calls). Review: does it cover all I/O operations the editor and loader need?
2. **Run tests** -> Verify FAIL (test exercises interface, no blob impl yet for blob tests). Filesystem tests should PASS.
3. **Implement blob storage** in `internal/component/config/storage/blob.go`. Wraps zefs BlobStore. Includes migration logic (import existing files on first create).
4. **Run tests** -> Verify PASS for both implementations.
5. **Wire into loader** -- `LoadReactorFileWithPlugins` accepts storage, reads config through it.
6. **Wire into editor** -- Editor struct gains storage field, write-through uses storage locking and I/O. `ze config edit -f` bypasses blob.
7. **Wire startup** -- `cmd/ze/main.go` constructs storage provider (blob or filesystem based on `-f` flag and `ZE_STORAGE_BLOB` env var). `detectConfigType` reads via storage. Storage provider passed to `hub.Run()`.
8. **Wire reload** -- `Orchestrator` holds storage provider. `Reload()` reads via storage.
9. **Wire SSH host key** -- read from blob, serve to Wish library from memory.
10. **Wire PID** -- store in blob, implement `ze pid` command.
11. **Functional tests** -- Create `.ci` and `.et` files exercising blob-enabled, blob-disabled, `-f` override, and SIGHUP reload paths.
12. **Verify all** -> `make ze-verify`
13. **Critical Review** -> All 6 checks from `rules/quality.md`
14. **Complete spec** -> Fill audit tables, write learned summary

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 or 6 (fix types/signatures) |
| Test fails wrong reason | Step 1 (fix test expectations) |
| Editor write-through broken | Re-read editor_draft.go, trace locking path |
| Migration misses files | Step 3 (review scan patterns) |
| Blob fallback not triggered | Step 7 (review error handling in startup) |
| SIGHUP reload fails | Step 8 (verify Orchestrator holds storage, reads via storage) |
| Config type detection fails | Step 7 (verify detectConfigType reads via storage, not os.ReadFile) |
| Cross-process locking fails | Step 6 (verify flock on persistent second fd) |
| SSH host key not served | Step 9 (verify Wish receives key bytes from blob) |

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

## RFC Documentation

No RFC references for this spec (config storage is not a protocol feature).

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

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

- [ ] AC-1..AC-19 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)

- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design

- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)

- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/NNN-zefs-integration.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
