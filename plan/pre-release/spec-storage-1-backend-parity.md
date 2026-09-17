# Spec: storage-1-backend-parity

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - (owner ordering of 2026-09-16 named `path-mtu-diagnostic`; it closed on main at `6cfeef34ad` the same day) |
| Phase | 7/7: publication and verification handoff |
| Handoff | verify |
| Updated | 2026-09-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**One live storage backend, a directory tree in the configuration's folder,
behind the contract the ZeFS blob fills today, with full feature parity. The
blob becomes an artifact: a seed, a backup, an import source. Nothing converts
on open; `ze init` and the explicit import tools are the only paths from a blob
to a live store (owner decisions, 2026-09-16, reviewed the same day).**

Before implementation, `storage.Storage` (`internal/component/config/storage/storage.go`) had
two implementations that were not equivalent. `blobStorage` (`blob.go`) had a
key space: `meta/...` for credentials, identity, CA root, pointers and runtime
state, `file/active/...` for configs, `file/<stamp>/...` for versions.
`filesystemStorage` (`storage.go`) has none: it hands every name to `os` as a
path. The product papered over that with 23 `IsBlobStorage` gates and 15 direct
`zefs.Open(<dir>/database.zefs)` calls that bypass `Storage` altogether.

Consequences, verified at the producers on 2026-09-16:

| On the filesystem backend today | Producer |
|---|---|
| bare `ze start` refuses; `ze start <file>` runs on the filesystem backend through an `os.Stat` fallback; web UI, looking-glass TLS, managed fleet client and server, `ze config import`, editor sessions, drafts and command history are refused or silently disabled | the `IsBlobStorage` sites in `cmd/ze/hub/main.go`, `service_web.go`, `service_lg.go`, `managed_server.go`, `ze_core_start.go`, `config/cli/cmd_edit.go`, `cmd_import.go` |
| Runtime state (BFD auth sequence, DDoS baseline, NTP last time, tc snapshot, update history, RIR delegation) is a no-op unless `ze.config.dir` is pinned, and then a SECOND handle on the same file is opened | `statestore.Put`, `openStateOnlyStore` (`cmd/ze/hub/statestore.go`) |
| The CA root, the BGP GR marker and the machine identity are written to `./meta/...` relative to the daemon's cwd | `filesystemStorage.WriteFile` via `atomicWriteFile` (`MkdirAll`), reached from `pki.LoadOrGenerateRoot`, `grmarker.Write`, `identity.Resolve` |
| SSH login, the SSH client credentials, `ze connect`, `ze systemd install`, the OSPF auth keystore and GR state, IRR and firewall domain caches, the hub user list and the doctor store check each open `database.zefs` by name and find nothing | the 15 `zefs.Open` sites listed under Current Behavior |
| One `meta/config/active` pointer serves every config name while versions are per name | `pointerKey` (`pointer.go`), journal `pointer-shared-across-the-names-it-indexes` |
| Two configs with one basename in different directories are one key | `resolvePathToKey` (`blob.go`), journal `suite-shares-one-persistent-store` |

Goal: after this spec, no code outside `internal/component/config/storage`
knows how the store is encoded. Every feature above works on the tree. The
store lives in the folder of the configuration it serves, with the folder's
permissions checked on open. `ze start <file>` creates the tree when the folder
has none. A blob where a live store belongs is refused by name. This spec supplies
`ImportBlob` and appliance seed import. → Decision (review round 1, 2026-09-17): the
local-path form `ze init --from <path>` is this spec (AC-2's error names a verb
that exists); the URL form `ze init --from <url>` and `--sha256` remain storage-2.

Companions: `spec-storage-2-blob-artifact` (backup, restore with two modes,
`ze init from <url>`, offline editing of a blob, spare capacity) and
`spec-storage-3-content-addressed-history` depend on this spec.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/zefs-format.md` - the blob format, key namespaces, locking, statestore contract
  → Decision: single-process ownership of the live store is the model. The daemon owns it; terminal commands become SSH clients for live operations. The tree keeps that model and adds nothing cross-process
  → Constraint: keys are `fs.ValidPath` names, `<namespace>/<qualifier>/<path>`; the tree maps a key to `{root}/<key>` unchanged, so no key needs escaping
  → Constraint: `statestore` writes through the ONE handle the config system opened. A second handle on the same store is the lost-update defect the page names; this spec deletes the two places that open one (`openStateOnlyStore`, `ResolveSSHStorage`) and moves the two plugin-process writers (OSPF) behind the daemon
  → Constraint: the page's "Entry added within container capacity: pwrite" row and "keys are exact fit" sentence disagree with `flush` and `writeFileNoFlush` (`pkg/zefs/store.go`); the page edit that lands with this spec corrects both
- [ ] `docs/architecture/fleet-config.md` - hub-side config storage, the write observer
  → Constraint: the managed server reads `file/active/client-<name>.conf` through `Storage` and pushes `config-changed` from the write observer; parity makes the observer a property of the shared core, not of the blob encoding
- [ ] `docs/guide/config-editor.md` - editing modes
  → Constraint: the page says file mode is entered "when no zefs database exists"; `cmdEditWithStorage` refuses instead. The page is corrected to the new rule: session mode on the store in the config's folder, `-f` for a loose file
- [ ] `docs/guide/operations.md`, `docs/guide/command-reference.md` - `ze init`, `ze data`, env table
  → Decision: `ze.storage.blob` leaves the env table and nothing replaces it. Both pages carry the full `ze init` flag set and the `ze data` subcommands they omit today (`write`, `check`, `repair`, `encode`)
- [ ] `docs/guide/appliance.md`, `docs/guide/ze-install.md`, `docs/architecture/appliance/on-device-installer.md` - `/perm/ze/database.zefs` layout, PXE bootstrap
  → Decision: the seed the image and PXE carry stays a blob at `/perm/ze/database.zefs`, injected by `debugfs` as today. The appliance's first `ze start` runs `gokrazyAutoInit`, which IMPORTS the seed into `/perm/ze/database/` as an explicit, logged step and moves the blob aside. That is the import tool, not a conversion inside `Open`
- [ ] `docs/architecture/resolve.md` - IRR stored delegation opens `database.zefs` read-only
  → Constraint: that read-only open becomes a `storage.Open` on the config folder; the page sentence changes with it
- [ ] `docs/architecture/testing/ci-format.md`, `docs/functional-tests.md` - runner semantics
  → Constraint: the runner rewrites a `.ci` daemon line `ze -` into `ze start <workdir>/ze-bgp.conf` (`runner_exec.go`, the stdin route), so every functional daemon has a per-test folder; the auto-create rule is what lets them start without `ze init`
- [ ] `ai/rules/config.md` - YANG versus env
  → Decision: there is no backend selection to configure. No YANG leaf (config parses after the store opens) and no env var (the env var is what silently switched backends today)
- [ ] `ai/rules/repo-maintenance.md` - discovery, `./le fs-persistence`, generated site and wiki
  → Constraint: `fspersistence.check` returns `ErrDeadAllowlistEntry` when an entry suppresses nothing; moving init's `os.Rename` into the storage package reddens the gate until `internal/plugins/init/main.go` leaves the allowlist in the same change
  → Constraint: the wiki `command-catalog.md` and the website reference pages are generated by `./le site build`; a command change owes that run and a commit in the wiki checkout (owner, 2026-09-16: "do not forget to update all docs, wiki, website")
- [ ] `ai/rules/principles.md` - no silent zero
  → Constraint: `storage.Open` on a folder with a blob and no tree returns an error naming `ze init from`; it never returns an empty store, because an empty store is how every feature vanished on the filesystem backend. Auto-create applies to a folder with NO store of either shape, from `ze start <file>` only

**Key insights:** (minimal context to resume after compaction)
- The whole divergence is one missing thing: a key space on the filesystem side. Give it one and the gates become dead code.
- Two classes of bypass exist, and both must go: 23 `IsBlobStorage` gates, and 15 direct `zefs.Open(database.zefs)` sites.
- The store root is the config file's directory. A client process resolves the same folder from the config path it was given, then `ze.config.dir`, then `paths.DefaultConfigDir()`; a daemon started on a non-default folder is reached by clients through `ze.config.dir`.
- `Open` never converts. Blob found: refuse by name. Nothing found: `ze start <file>` creates the tree; every other opener errors naming `ze init`.
- The folder and every file in it are permission-checked on open: root 0700, files 0600, owned by the process user; a looser mode is refused naming the path and the mode.
- Per-name pointers and basename-collision are fixed in the shared core.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/config/storage/storage.go` - `Storage`, `WriteGuard`, `filesystemStorage` (raw `os` calls, in-process mutex, `writeVersionFS` to `<dir>/rollback/<stem>-<stamp>.conf`), `IsBlobStorage`, `BlobStoreFrom`
- [ ] `internal/component/config/storage/blob.go` - `blobStorage` over `*zefs.BlobStore`; `resolveKey` (basename to `file/active/<base>`, namespaced keys pass through), `resolveDirKey`, `resolvePathToKey` (`filepath.Base`), in-memory `metas` (ModTime, ModifiedBy), `writeObserver`, `migrateExistingFiles`, `NewBlob` self-heal via `zefs.MoveAside` then recreate
- [ ] `internal/component/config/storage/pointer.go` - `pointerPath` (`meta/config/<ptr>` on blob, `<dir>/meta/config/<ptr>` on filesystem, no name in either), `versionPath`, `ReadActiveConfig` (pointer, else the mirror), `ReadCandidateConfig`, `WriteCandidateVersion`, `EnsureActiveVersion` (versions the mirror when no active pointer), `PromoteCandidate`, `ClearCandidate`, `removeVersionLocked` (every removal runs under a guard)
- [ ] `internal/core/resolve/resolve.go` - `Storage()`: `ze.storage.blob=false`, empty config dir, or a blob open error each return `NewFilesystem()`; `NewBlob` CREATES a store where none exists; `DefaultConfig` reads `meta/instance/name` through `Storage`
- [ ] `internal/core/statestore/statestore.go` - `SetStore(*zefs.BlobStore)`, `Put/Get/Remove` no-op on nil; six keys, all `meta/...`: `KeyBFDAuthSeq`, `KeyConfigUpdateHistory`, `KeyRIRDelegation`, `KeyDDoSDetectBaseline`, `KeyNTPLastTime`, `KeyTrafficTCSnapshot`; the two `Store()` reads are nil checks
- [ ] `cmd/ze/hub/main.go`, `cmd/ze/hub/statestore.go` - `BlobStoreFrom` then `openStateOnlyStore` (second handle when `ze.config.dir` is pinned); the `IsBlobStorage` switch on config read with an `os.ReadFile` fallback for a config outside the store (gokrazy read-only root); managed client and `wireManagedCommit` gated
- [ ] `cmd/ze/hub/managed_server.go`, `service_web.go`, `service_lg.go`, `main_reload.go`, `main_servers.go` (`loadZefsUsers` opens `database.zefs`), `cert_store.go` (`meta/web/*` through `Storage`)
- [ ] `cmd/ze/ze_core_start.go` (`ze start <file>`: blob lacks the key and the file exists on disk → `NewFilesystem`), `ze_core_autoinit.go` (`gokrazyAutoInit`: mkdir then re-resolve, expects blob), `ze_core_dispatch.go` (`ze <file>` no longer exists: "unknown command", removed by `spec-fixit-config-file-positional-grammar`), `setup_dispatch.go` (`ze_setup` build runs `hub.Run(NewFilesystem(), "-")`), `login.go` (serial login opens `database.zefs`; on any open error it grants access without authentication)
- [ ] `internal/component/config/cli/cmd_edit.go` (store resolved by `registry.RuntimeStorage()` BEFORE `-f`/`<file>` are parsed; ephemeral daemon start when no daemon answers), `cmd_import.go`, `cmd_list.go`, `cmd_set.go`, `cmd_deactivate.go`, `config_data.go`, `main.go` (`RunWithStorage(NewFilesystem())`), `cmd_show.go`, `cmd_diff.go`, `cmd_history.go`, `cmd_rollback.go` - the gates and the offline `NewFilesystem()` family, which reads a loose file with NO store (574 `.ci` uses of `ze config validate|show` on loose files)
- [ ] `internal/component/config/infra/ssh.go` - `ResolveSSHStorage` opens or CREATES a second blob when the main store is not blob
- [ ] `internal/component/bgp/config/loader_create.go`, `register.go` - `os.ReadFile` fallback when the blob lacks the config path
- [ ] `internal/component/pki/ca.go` `LoadOrGenerateRoot` (`meta/ca/*` via `Storage.Exists`), `internal/component/bgp/grmarker/grmarker.go`, `internal/core/identity/identity.go` - ungated `meta/` writers
- [ ] `internal/plugins/init/main.go` - flags `--managed --force --yes --web-cert --web-cert-name --seed`; config dir from `sshclient.ResolveDBPath`; `zefs.Create(<db>.init-tmp)` then `os.Rename`; keys `meta/ssh/{host}/{port}/username|password`, `meta/auth/local/username|password`, `meta/ssh/default`, `meta/instance/managed|name`, `meta/web/cert|key`, `file/active/ze.conf`; `--force` = `daemonRunning` check, prompt, `zefs.MoveAside`
- [ ] `internal/component/config/storage/cli/main.go`, `register.go`, `cmd_integrity.go` - `ze data write|import|rm|list|cat|registered|check|repair|encode`, `--path` or `DefaultConfigDir()/database.zefs`, every subcommand opens with `zefs.Open`/`zefs.Create`; `Mode: modeOffline`
- [ ] `internal/component/doctor/checks_storage.go` `checkStoreIntegrity` (`zefs.Check` on the file, absent = healthy), `doctor.go` `resolveStorageWithDiag` (branches on the error; later checks receive the store)
- [ ] `internal/core/ssh/client/client.go` `ResolveDBPath` (10 callers: init, connect x4, client x3, cli/client x2; all `DefaultConfigDir`) and the `zefs.Open` at its use
- [ ] `internal/plugins/connect/main.go` (WRITES credentials from a client process), `internal/plugins/systemd/cmd_install.go`, `internal/plugins/local/cmd_install.go`, `internal/plugins/ospf/gr_nvs.go` and `ospf/auth_keystore.go` (a PLUGIN process opening and writing the daemon's store), `internal/component/resolve/irr/stored.go`, `irr/store/store.go`, `internal/component/firewall/plugins/domain/cache.go`, `firewall/plugins/irr/cache.go`, `firewall/plugins/irr/doctor.go`, `internal/component/bgp/plugins/filter_irr/cache.go`, `internal/component/cli/client/main.go` - direct `zefs.Open` of `database.zefs`
- [ ] `internal/component/cli/editor.go` `NewEditor` (filesystem editor used in-daemon by `bgp/plugins/cmd/peer/save.go`, `prefix_update.go`, `resolve/cmd/rir.go`)
- [ ] `pkg/zefs/store.go` - `writeFileNoFlush` (`growCapacity` = len + 10%, `file/active/` keys +20 bytes), `encode` (`containerCap := entriesSize`), `flush` (in-place only when nothing added and layout unchanged), `Export`, `Import`
- [ ] `pkg/zefs/netcapstring.go` - `EncodeNetcapstring` exported; `decodeNetcapstring` and `decodeNetcapstringRef` (CRC verified) unexported
- [ ] `pkg/zefs/check.go` - `Check(path)`, `Repair(src, dst)` (reads raw bytes, never opens `src`), `MoveAside(path)` (`.replaced-<date>T<time>`)
- [ ] `pkg/zefs/keys.go`, `registry.go` - `KeyConfigActive|Candidate|Rollback|Recovery` fixed patterns; `KeyFileVersion` = `file/{date}/{basename}`; `KeyFileCandidate` registered with zero callers; `KeyEntry.Key` rejects empty and `..` params
- [ ] `internal/appliance/cmd_build.go` `injectZeFS` (debugfs `mkdir ze` + `write`), `cmd_assemble.go`, `diskverify.go`, `cmd_export.go`; `internal/install/disk/system.go` `mountInjectDB` (`downloadToFile`), `bakedSeedPresent` (regular file, size > 0); `internal/plugins/imageserver/handler.go` `/install/database.zefs`
- [ ] `internal/test/runner/runner_exec.go` (`clientEnv` pin at ~317, daemon pin at ~873, `zeDaemonShouldForceFileStorage`; the stdin route writes `ze-bgp.conf` into the per-test WorkDir and runs `ze start <file>`), `internal/component/cli/testing/runner.go` (`option=storage`, `expect=file`)
- [ ] `internal/le/fspersistence/fspersistence.go` - `ScanFile` matches `os.WriteFile|Create|Rename|Symlink|Link|OpenFile` and `ioutil.WriteFile` on the `os` import name only; it cannot see `zefs.Open`; `dirAllowlist`, `fileAllowlist`, dead-entry refusal
- [ ] `plan/spec-fixit-functional-suite-pins-the-unshipped-backend.md` disposition - the 2026-08-28 unpin measurement: 31 of 43 reload tests regressed; with a per-test `ze.config.dir` 17 still failed, 15 of them timeouts, unexplained

**Behavior to preserve:** (unless the user explicitly said to change it)
- The `Storage` and `WriteGuard` method set: every caller compiles unchanged except the ones this spec deletes
- The blob file format: byte-for-byte unchanged; a blob written today is importable tomorrow
- The key namespaces and every registered key in `pkg/zefs/keys.go`
- `ze init` refuses when a store exists; `--force` still requires no running daemon and a confirmation (or `--yes`); the moved-aside name keeps `.replaced-<stamp>`
- `ze start <file>`, `ze -`, `ze config edit -f`, and the offline `ze config validate|show|diff|set|rollback|history` read a loose config file from the path given; `validate` and `show` need no store
- `statestore.Put` stays best-effort: a no-op when no store is registered (before wiring, in tests)
- The managed server's `config-changed` push on a client config write
- `ze data check` exit codes: 0 clean, 1 corrupt, 2 unreadable
- Serial `ze login` keeps its "grant access without authentication" fallback when the store cannot be read; the error it prints names the store folder

**Behavior to change:** (only what the user asked for)
- `filesystemStorage` is deleted and replaced by a tree backend with the blob's key space (no layering: no path-mode arm survives)
- One opener, `storage.Open(dir)`, replaces `resolve.Storage`'s env-and-fallback logic, `NewBlob`, `NewFilesystem`, `ResolveSSHStorage`, `openStateOnlyStore` and every direct `zefs.Open(<dir>/database.zefs)` outside the storage package
- `Open` never creates and never converts. `storage.Create(dir)` creates; `ze init` and `ze start <file>` (auto-create, owner decision 1) are its callers, plus `gokrazyAutoInit`
- A blob found where a live store belongs is refused by `Open` with an error naming the file and `ze init from <file>`
- The store root is the directory of the config file; `ze.config.dir`, then `paths.DefaultConfigDir()`, when there is no config path
- The folder and its files are permission-checked on open (owner decision 2)
- `ze.storage.blob` is deleted
- Pointers are per config name; the nameless legacy keys are neither read nor written (pre-release: no store to migrate; `ReadActiveConfig` already falls back to the mirror and `EnsureActiveVersion` re-versions it)
- `resolvePathToKey` no longer collapses directories: the store root is the config's directory, so the basename is unique within a store by construction
- `statestore.SetStore` takes `storage.Storage`; the OSPF keystore and GR state persist through `statestore` over the plugin boundary instead of a second-process open
- Every `IsBlobStorage` gate is deleted; the feature behind it runs on the tree
- `ze data --path` accepts the directory; `ze data` stays offline
- The functional runner stops pinning a backend; `.et` gains `expect=key`
- A blob that will not decode is moved aside for post-mortem as today, and `Open` then errors naming the moved file and `ze init` instead of recreating an empty store (an empty store silently drops every credential, the zero-value answer `ai/rules/principles.md` bans)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `storage.Open(dir)` (`internal/component/config/storage/open.go`, new): every daemon, CLI process and in-process plugin obtains its `Storage` here. Input: the store folder. Output: a `Storage` over the tree, or an error naming what was found and the command that fixes it.
- `storage.Create(dir)` (`open.go`): builds `database.init-tmp/`, renames it to `database/`, returns the `Storage`. Callers: `ze init`, `ze start <file>` on a folder with no store, `gokrazyAutoInit`.
- `resolve.StorageFor(configPath)` (`internal/core/resolve/resolve.go`, replaces `Storage()`): the folder is `filepath.Dir(configPath)` when a path is given, else `ze.config.dir`, else `paths.DefaultConfigDir()`; then `storage.Open`.
- `ze init ...` (`internal/plugins/init/main.go`): creates the tree and seeds its `meta/` keys. `ze init --from <path>` imports a local blob here (→ Decision, review round 1); the URL form is storage-2.
- Every `Storage` method call, with a name that is a filesystem path, a bare config name, or a `meta/`, `file/` key.

### Transformation Path
1. Name to key, in the shared core (`store.go`, hoisted from `blob.go`): a namespaced key passes through; a path or bare name becomes `file/active/<basename>`; a version is `file/<stamp>/<basename>`; a pointer is `meta/config/<basename>/<pointer>`.
2. Key to bytes, per encoding: tree reads `{root}/<key>`, decodes the netcapstring frame and verifies its CRC32c, and returns the data; blob calls `BlobStore.ReadFile`. Writes are the inverse: tree encodes one frame and installs it by temp, fsync, rename; blob writes through `WriteLock` and flushes on release.
3. Metadata: `metas` (ModTime, ModifiedBy) and the write observer live in the core and fire after either encoding's write.
4. Open-time detection: `Open(dir)` stats `database/` and `database.zefs`. Tree alone: permission check, then open. Blob alone, or both: error naming the blob and `ze init from`. Neither: error naming `ze init`. A `database.init-tmp/` or `database.import-tmp/` is ignored as a leftover.
5. Permission check: the folder must be a directory owned by the process's user with no group or other bits; every frame file must be 0600 and owned likewise; a symlink anywhere under the root is refused. The error names the path and the mode found.
6. Auto-create: `ze start <file>` calls `resolve.StorageFor(file)`; on the "neither" error it calls `storage.Create` in that folder, logs it, and continues. No other command auto-creates.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Process ↔ store folder | one file per key, rename-installed, permission-checked | No |
| Daemon ↔ terminal command | live operations (edit commit, reload) keep their existing SSH command path; offline commands open the folder directly, read-only where they only read | No |
| Daemon ↔ plugin process | `statestore` through the daemon's handle over the plugin protocol; a plugin process never opens the store | No |
| Installer ↔ appliance first boot | the seed blob at `/perm/ze/database.zefs` is imported by `gokrazyAutoInit` through `storage.ImportBlob`, an explicit step | No |

### Integration Points
- `statestore.SetStore(store)` in `cmd/ze/hub/main.go` - receives the one `Storage`; the second-handle branch is deleted; the plugin protocol gains a state put/get so OSPF's two writers move behind it
- `pki.LoadOrGenerateRoot`, `grmarker`, `identity.Resolve` - unchanged code; their `meta/` keys now land in the store root
- `managed_server.startManagedServer` - runs whenever a server block has clients; `SetWriteObserver` becomes a `Storage` method
- `web.startWebServer`, `service_lg.buildLGService` - run unconditionally; `blobCertStore` reads `meta/web/*` from the tree
- `doctor.checkStoreIntegrity` - calls `zefs.CheckPath` (file or directory) on the store root; `resolveStorageWithDiag` keeps passing a nil store to nothing
- `ze data` - `--path` accepts the directory or a blob file; every subcommand opens through `storage.Open` (tree) or the blob encoding (file), offline
- `gokrazyAutoInit` - seed blob present: `storage.ImportBlob(seed, dir)` then `MoveAside`; absent: `storage.Create`
- `internal/test/runner` - no backend pin; each test's WorkDir holds its own `database/` after the daemon's auto-create

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the `fspersistence` detector extended by this spec refuses `zefs.Open`/`zefs.Create` of the live store outside `internal/component/config/storage`; the 15 sites are migrated in Phase 5 |
| No unintended coupling (components stay isolated) | Yes | `statestore` depends on `storage.Storage`, an interface, not on `pkg/zefs`; no component imports the tree or blob type; OSPF reaches state over the plugin protocol |
| No duplicated functionality (extends existing, does not recreate) | Yes | the tree reuses `EncodeNetcapstring`, the exported decode, `MoveAside`; versions and pointers exist once in the core; `defaultConfigName` and `service_web.resolveConfigPath` are deleted in favor of `resolve.DefaultConfig`; `ImportBlob` is the one blob-to-tree walk, used by `gokrazyAutoInit`, `ze init from` and storage-2's restore |
| Zero-copy preserved where applicable (refs, not copies) | Yes | blob reads keep the lock-scoped slices; tree reads are one `os.ReadFile` per key with the frame decoded in place (`decodeNetcapstringRef`) |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | no new command; the plugin-protocol state message registers like every other message type; the two encodings are the two answers of one detection rule, not a registry |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | lists searched: `ze data` `subcommandHandlers` (no new verb), `register.go` `Subs` string (corrected to name the existing verbs it omits), doctor `codes.go` (one new code for a permission refusal, one for a corrupt tree key, registered where `doctor-store-integrity` is), `fspersistence` allowlists (entries removed; the side-store allowlist derives from A-4's list), `pkg/zefs/keys.go` (new keys registered through `MustRegister`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The store root is the directory of the config file served; a client process resolves the same folder from the config path it was given, then `ze.config.dir`, then `paths.DefaultConfigDir()` | owner, 2026-09-16: "data should always be in the folder of the configuration", confirmed at review: "it lives in the config directory" | a client cannot find a daemon started on a non-default folder unless told through `ze.config.dir`; documented, not hidden | `TestStorageForResolution` (path, env, default, in that order); `docs/guide/operations.md` states the rule | confirmed by owner |
| A-2 | The appliance seed stays a blob injected by the unchanged image tooling; `gokrazyAutoInit` imports it explicitly on first boot | owner: "appliance convert to FS" and "conversion is only a tool for import/export not for live operation" | image build and PXE tooling would need to write a directory | `internal/le/qemu` install scenario asserting `/perm/ze/database/` after first boot and `database.zefs.replaced-*` beside it | confirmed by owner |
| A-3 | `Open` never converts; a blob where a live store belongs is refused by name; only `ze init from`, `gokrazyAutoInit` and storage-2's restore import one | owner, review answer 3 | a dev host with a `database.zefs` must run `ze init from database.zefs` once; documented in operations and quickstart | `TestOpenRefusesBlob` | confirmed by owner |
| A-4 | Side stores that are not the live config store (`debug.zefs` in `internal/plugins/debug/profile.go`, the imageserver's generated seed, `cmd_assemble`'s seed) stay blob files | they are artifacts written once and read by another consumer | a tree device still carries `.zefs` side files, acceptable under the roles agreed on 2026-09-16 | grep at closure lists every `zefs.Open`/`zefs.Create` outside the storage package and names each as a side store or a seed builder | unvalidated |
| A-5 | No caller reads the raw `*zefs.BlobStore` out of `statestore.Store()` | verified 2026-09-16: the two `Store()` reads are nil checks (`resolve/irr/stored.go`, `traffic/netlink/backend_linux.go`) | the `Storage` interface would need a blob accessor, which is the coupling this spec removes | compile after the signature change | unvalidated |
| A-6 | The 28 `.ci` files that set `ze.storage.blob` and the `expect=file` assertions on `meta/config/*` and `rollback/*.conf` (10 `.ci`, 13 `.et`) can be re-cut to assert through the store without losing what they prove | agent report 2026-09-16 | some test would need the raw path, which means a feature reads a loose path the store should own; that is a defect to fix, not a reason to keep the pin | each re-cut test is run RED against the pre-change tree once | unvalidated |
| A-7 | Removing the `ze.storage.blob` registration is safe once no `.ci` sets it: an `option=env` with an unregistered key would otherwise end the process at `env.Get` | journal `env-key-read-but-never-registered` | the 28 files must land in the same commit as the deletion | the functional suite after the change | unvalidated |
| A-8 | A read-only open of a tree by a second process while the daemon runs (serial login, `ssh/client` credentials, doctor) is safe, because a tree read is one file installed by rename | the per-key rename in `atomicWriteFile`; journal `store-serializes-in-process-only` names the blob's whole-file hazard | the tree would need a lock file | `TestTreeReadDuringWrite` | unvalidated |
| A-9 | Client processes that WRITE today from outside the daemon (`ze connect add` credentials, `ze systemd install` chown, OSPF keystore and GR) either run when no daemon does (`connect`, `systemd install`, refused otherwise by `daemonRunning`) or move behind the daemon (OSPF, through `statestore` over the plugin protocol) | the files read above | a second-process writer on a live tree is last-writer-wins per key; the permission check does not prevent it | `TestConnectRefusesWithDaemon`, `TestOSPFStateThroughDaemon` | unvalidated |
| A-10 | The 2026-08-28 unpin regression (31 of 43 reload tests; 17 residual with a per-test `ze.config.dir`, 15 timeouts) is not explained by store sharing alone | the fixit spec's disposition table | Phase 7 meets an unexplained residual | reproduce the residual on the pre-change tree before Phase 7 and record its mechanism in the spec | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `ImportBlob` loses a key or a byte | `TestImportEqualsSource` compares key sets and bytes; the import refuses to move the blob aside on any mismatch | the blob stays; the partial `database.import-tmp/` is removed |
| R-2 | Two processes race to create or import in one folder (serial login and the daemon at appliance first boot) | `TestCreateRaceLoser` | `Create` and `ImportBlob` build under a temp name and `os.Rename` onto `database/`; rename onto an existing directory fails, the loser re-opens what the winner installed. Only `ze start`, `ze init` and `gokrazyAutoInit` create; login, doctor and clients only open |
| R-3 | A folder with loose group or other bits is refused and the daemon does not start after an operator `chmod` | `doctor-store-permissions` names the path and the mode | the error text carries the `chmod`/`chown` to run; `ze init` and `Create` write 0700 |
| R-4 | A migrated `zefs.Open` site that ran in another process now opens the daemon's tree concurrently | the `ospf/auth_keystore.go` comment names the SIGBUS class for mmap; a tree has no mmap | read-only sites open through `storage.Open` and close; OSPF's writers move behind the daemon (A-9) |
| R-5 | The `./le fs-persistence` gate goes red on a dead allowlist entry after init's rename moves into the storage package | the gate's `ErrDeadAllowlistEntry` | the entries for `init/main.go`, `ze_core_autoinit.go`, `local/cmd_install.go` are removed in the same change |
| R-6 | The functional suite regresses when the pin goes | the suite; A-10 | per-test folders isolate the store (the runner's stdin route); the residual 17 is reproduced first |
| R-7 | `ze init` on the tree has no atomic install equivalent of `.init-tmp` then rename | a killed `ze init` leaves a half tree | init builds `database.init-tmp/` and renames the directory; `Open` treats a leftover temp as absent |
| R-8 | Per-key rename on the tree makes a multi-key commit (version plus pointer) non-atomic across a crash | a pointer naming a version that is absent | order is version first, pointer second, so a crash between them leaves an orphan version, never a dangling pointer; `ReadActiveConfig` treats a missing version as an error |
| R-9 | `bakedSeedPresent` reads a directory as absent on a re-install over a converted `/perm` and re-downloads the seed | the installer log | `bakedSeedPresent` accepts a `database/` directory |
| R-10 | `ze data` on a running daemon's tree edits files under the daemon | unchanged from today's blob rule: offline verbs | `ze data` write verbs keep the `daemonRunning` refusal that `ze init --force` uses |
| R-11 | The three in-daemon `cli.NewEditor(configPath)` users (peer save, prefix update, rir) open a loose file where the store holds the config | `TestPeerSaveWritesStore` | they take the daemon's `Storage` through `NewEditorWithStorage`; `NewEditor` is deleted with `NewFilesystem` |
| R-12 | The offline `ze config set|rollback|history <file>` on a folder with no store has nowhere to write or read versions | `TestOfflineSetWithoutStore` | `set` writes the file and prints that no version was recorded because the folder has no store; `rollback` and `history` refuse naming `ze init`; `validate` and `show` never open a store |
| R-13 | A stdin daemon (`ze -`, the `ze_setup` build) has no config folder | `TestStdinDaemonStore` | `resolve.StorageFor("-")` uses `ze.config.dir` then `DefaultConfigDir()`; with neither writable and no store there, the daemon runs with no store and every store-backed feature is refused by name at startup, in one warning per feature |
| R-14 | `registry.RuntimeStorage()` resolves the store before `ze config edit` parses `-f`/`<file>` | `TestEditResolvesAfterArgs` | the open moves after argument parsing; `withRuntimeStore`'s "config storage unavailable" text carries the `Open` error, which names the fix |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every process that opens the store: a daemon that cannot start, a login that cannot read credentials, a fleet client that cannot connect. Nothing on the wire changes |
| How is it reverted? | One commit revert restores the code. An imported install keeps `database.zefs.replaced-<stamp>` beside its tree; renaming it back is the data revert. The blob format is unchanged |
| Who else touches this path? | `spec-storage-2-blob-artifact`, `spec-storage-3-content-addressed-history` (depend on this), `spec-config-at-rest-encryption` (skeleton; the tree needs the same answer per file, out of scope here), `spec-fixit-ca-root-location` (its filesystem branch is resolved by this spec), `spec-fixit-functional-suite-pins-the-unshipped-backend` (its pin is removed by this spec), `spec-managed-server-hardening`, `spec-fleet-*` (read client configs through `Storage`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze init` in an empty directory | → | `init.Run` → `storage.Create(dir)` builds `database.init-tmp/` and renames it | `test/plugin/init-creates-tree.ci` |
| `ze start <file>` in a directory holding `database/` | → | `resolve.StorageFor` → `storage.Open` → permission check → tree | `test/plugin/start-on-tree.ci` |
| `ze start <file>` in a directory holding neither | → | `Open` "neither" error → `storage.Create` → tree; logged | `test/plugin/start-auto-creates.ci` |
| `ze start <file>` in a directory holding `database.zefs` | → | `Open` error naming the blob and `ze init from` | `test/plugin/start-refuses-blob.ci` |
| `ze doctor` in a directory holding neither | → | `Open` error → `doctor-storage-unavailable` warning naming `ze init`; no directory created | `test/plugin/doctor-without-store.ci` |
| `ze start <file>` on a tree whose root is 0755 | → | permission check → error naming the path and mode | `test/plugin/start-refuses-loose-mode.ci` |
| SSH login against a tree daemon | → | `hub.loadZefsUsers` through `Storage` | `test/plugin/login-on-tree.ci` |
| web UI on a tree daemon | → | `startWebServer` unconditional, `blobCertStore` reads `meta/web/*` | `test/web/web-on-tree.wb` |
| BFD auth sequence persisted across a restart on a tree | → | `statestore.Put` through `Storage` | `test/plugin/statestore-on-tree.ci` |
| OSPF boot count on a tree daemon | → | plugin → state message → daemon `statestore` | `test/plugin/ospf-state-through-daemon.ci` |
| `ze doctor` on a tree with one corrupt key | → | `checkStoreIntegrity` → `zefs.CheckPath` | `test/plugin/doctor-tree-corrupt-key.ci` |
| `ze data check --path <dir>/database` | → | `cmdCheck` → `zefs.CheckPath` | `test/plugin/data-check-tree.ci` |
| appliance first boot with a seed blob | → | `gokrazyAutoInit` → `storage.ImportBlob` → `MoveAside` | QEMU install scenario (`internal/le/qemu`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `storage.Open(dir)` with `dir/database/` present, root 0700 and files 0600 owned by the caller | returns a tree-backed `Storage`; a key `k` is the file `dir/database/k` holding one netcapstring whose data is the value |
| AC-2 | `storage.Open(dir)` with `dir/database.zefs` present, with or without a tree | returns an error naming the blob path and `ze init --from <path>` (spelling fixed in review round 1; the earlier `ze init from` named no command); nothing is created, renamed or read beyond the stat |
| AC-3 | `storage.Open(dir)` with neither | returns an error naming `ze init`; no directory is created |
| AC-4 | `storage.Open(dir)` with a tree whose root has any group or other bit, or a frame file not 0600, or an owner other than the caller, or a symlink under the root | returns an error naming the path, the mode or owner found, and the command that repairs it; no key is read |
| AC-5 | `storage.Create(dir)` | builds a private `dir/database.init-tmp-*/` at 0700 and publishes `dir/database/` with atomic no-replace rename. Under O-2, a concurrent creator returns `ErrBusy` while the winner retains writer ownership; after release, auto-create opens the winner without replacing it. |
| AC-6 | `storage.ImportBlob(blob, dir)` | every key and value of the blob is present in `dir/database/`, byte-equal, verified before the blob is renamed to `database.zefs.replaced-<stamp>`; a blob failing `zefs.Check` imports nothing and the error names the key; a partial `database.import-tmp/` never carries the live name |
| AC-7 | a tree file whose CRC32c does not match its data is read through `ReadFile` | the read fails with an error naming the key; it never returns the bytes |
| AC-8 | the conformance table (every `Storage` and `WriteGuard` method, `Stat` ModTime and ModifiedBy, `SetWriteObserver`, `WriteVersion`, `ListVersions`, every pointer function) run over the tree and over the blob encoding | identical observable results for identical inputs, including error cases |
| AC-9 | `List(dir)` on either encoding | returns namespaced keys that `ReadFile` accepts unchanged |
| AC-10 | `PromoteCandidate` for config name `x.conf` | writes `meta/config/x.conf/active` and `meta/config/x.conf/rollback`; `meta/config/y.conf/*` is untouched; the nameless `meta/config/active` is never written or read |
| AC-11 | `resolve.StorageFor("/a/b/x.conf")` | opens `/a/b`; `StorageFor("")` and `StorageFor("-")` open `ze.config.dir` when set, else `paths.DefaultConfigDir()` |
| AC-12 | `ze init` in a directory with no store | creates `database/` holding the same key set init writes today, root 0700, files 0600; `ze init` beside a `database/` or a `database.zefs` refuses with "database already exists" naming the path found |
| AC-13 | `ze init` interrupted after some keys are written | no `database/` exists afterwards; a `database.init-tmp/` may, and `Open` ignores it |
| AC-14 | `ze init --force` on a tree with no daemon running, confirmed | the tree is renamed to `database.replaced-<stamp>` and a new one created |
| AC-15 | `ze start <file>` in a folder with no store of either shape | the tree is created in that folder, the creation is logged with the path, and the daemon starts; a second `ze start` finds it |
| AC-16 | `ze start <file>` in a folder holding `database.zefs` | refused with the AC-2 error; no tree created |
| AC-17 | a daemon on a tree: SSH login, web UI, looking-glass self-signed TLS, managed client, managed server with clients, `ze config import`, editor session mode, drafts, command history, CA root, GR marker, machine identity, `ze support export`, the web editor's diff | each works, and its keys are under the store root, never under the process cwd |
| AC-18 | a daemon on a tree restarts | `meta/bfd/auth-seq/*`, `KeyConfigUpdateHistory`, `KeyRIRDelegation`, `KeyDDoSDetectBaseline`, `KeyNTPLastTime`, `KeyTrafficTCSnapshot` are read back from the store without any `ze.config.dir` pin |
| AC-19 | the OSPF plugin's boot count and GR state | persisted through a state message to the daemon's `statestore`; the plugin process opens no store file |
| AC-20 | serial login, `ssh/client` credential read, `ze connect`, `ze systemd install`, IRR and firewall caches, the hub user list, the doctor store check | each reaches its keys on a tree through `storage.Open`; no `zefs.Open` of the live store remains outside the storage package; `ze connect add` and `ze systemd install` refuse while a daemon runs, naming it |
| AC-21 | `./le fs-persistence check` on the finished tree | green; a fixture that adds `zefs.Open(<dir>/database.zefs)` in a component is refused by the extended detector with the rule's name; the A-4 side stores are its allowlist |
| AC-22 | `ze data list|cat|rm|write|import` with `--path <dir>/database` or `--path <file>.zefs` | operate on the named store offline; `ze data check` and `repair --output` on a directory walk every file, report per-key CRC results with the same exit codes, and `repair` writes a new tree holding only the good keys |
| AC-23 | `ze doctor` on a tree with one corrupt file; on a tree with a loose mode; with no store | `doctor-store-integrity` names the key; `doctor-store-permissions` names the path and mode; `doctor-storage-unavailable` names `ze init` and creates nothing |
| AC-24 | `ze -`, the `ze_setup` build, `ze config edit -f <file>`, offline `ze config validate|show <file>` | read the loose config; `validate` and `show` open no store; `ze -` uses the AC-11 folder and, when it holds no store, runs with every store-backed feature refused by name in one warning each |
| AC-25 | offline `ze config set|rollback|history <file>` | `set` writes the file, and records a version only when the file's folder holds a store, saying so otherwise; `rollback` and `history` refuse without a store naming `ze init` |
| AC-26 | an appliance image built by the unchanged `injectZeFS`, or a PXE install through `mountInjectDB`, booted once | `/perm/ze/database/` exists, `database.zefs.replaced-*` beside it, the import is logged, and the daemon serves the seeded config; `bakedSeedPresent` reports true for the directory |
| AC-27 | the functional suite with no `ze.storage.blob` in any `.ci`, no runner pin, `.et` default tree | green; the 23 re-cut tests assert through `expect=key` or `ze data`, each shown RED once against the pre-change tree |
| AC-28 | `env.Get("ze.storage.blob")` | the key is unregistered; no file in `cmd/`, `internal/`, `test/`, `docs/`, `website/` names it |
| AC-29 | `./le site build` after the change | the wiki `command-catalog.md` and the website reference pages describe `ze init`, `ze data` and the store folder as this spec leaves them; the regenerated catalog is committed in the wiki checkout |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | `ze init` then `ze start` on a fresh host | init creates `database/` → start opens it → CA root, host keys, identity written under it → SSH login | `init-creates-tree.ci`, `start-on-tree.ci`, `login-on-tree.ci` |
| 2 | runs `ze start x.conf` in a scratch folder with nothing else | auto-create → daemon up | `start-auto-creates.ci` |
| 3 | upgrades a dev host that has `database.zefs` | `ze start` refuses naming `ze init --from <blob>` → operator runs it (this spec, local path; `init-from-blob.ci`) → tree | `start-refuses-blob.ci` |
| 4 | boots a freshly installed appliance | seed blob in `/perm/ze` → `gokrazyAutoInit` imports → web UI up | QEMU install scenario, AC-26 |
| 5 | edits config over SSH, commits, restarts | draft under `file/draft/`, version under `file/<stamp>/`, per-name pointer → restart reads active | `test/editor/*.et` on the tree, `start-on-tree.ci` |
| 6 | runs `ze data check` after a disk error | tree walk names the bad key; `repair --output` writes a good tree | `data-check-tree.ci`, `doctor-tree-corrupt-key.ci` |
| 7 | loosens the folder mode by mistake | `ze start` and `ze doctor` name the path and the fix | `start-refuses-loose-mode.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStorageConformance` (table over both encodings) | `internal/component/config/storage/conformance_test.go` | AC-8, AC-9 | |
| `TestTreeKeyIsFile`, `TestTreeFrameCRC`, `TestTreeReadDuringWrite` | `storage/tree_test.go` | AC-1, AC-7, A-8 | |
| `TestOpenDetection` (tree alone, blob alone, both, neither, leftover temp dirs) | `storage/open_test.go` | AC-2, AC-3 | |
| `TestOpenPermissionCheck` (root mode, file mode, owner, symlink) | `storage/open_test.go` | AC-4 | |
| `TestCreateAtomic`, `TestCreateRaceLoser` | `storage/open_test.go` | AC-5, R-2 | |
| `TestImportEqualsSource`, `TestImportRefusesOnMismatch`, `TestImportRefusesCorrupt` | `storage/import_test.go` | AC-6, R-1 | |
| `TestPointerPerName` | `storage/pointer_test.go` | AC-10 | |
| `TestStorageForResolution` | `internal/core/resolve/resolve_test.go` | AC-11 | |
| `TestStatestoreThroughStorage` | `internal/core/statestore/statestore_test.go` | AC-18 | |
| `TestInitCreatesTree`, `TestInitRefusesExistingShape`, `TestInitAtomicTree` | `internal/plugins/init/main_test.go` | AC-12, AC-13 | |
| `TestStartAutoCreates`, `TestStartRefusesBlob` | `cmd/ze/ze_core_start_test.go` | AC-15, AC-16 | |
| `TestOSPFStateThroughDaemon` | `internal/plugins/ospf/auth_keystore_test.go` | AC-19 | |
| `TestConnectRefusesWithDaemon` | `internal/plugins/connect/main_test.go` | AC-20 | |
| `TestCheckPathDirectory`, `TestRepairDirectory` | `pkg/zefs/check_test.go` | AC-22 | |
| `TestDecodeNetcapstringExported` | `pkg/zefs/netcapstring_test.go` | the exported decode verifies CRC and rejects a bad frame | |
| `TestFSPersistenceRefusesLiveStoreOpen` | `internal/le/fspersistence/fspersistence_test.go` | AC-21 | |
| `TestPeerSaveWritesStore` | `internal/component/bgp/plugins/cmd/peer/save_test.go` | R-11 | |
| `TestOfflineSetWithoutStore`, `TestEditResolvesAfterArgs` | `internal/component/config/cli/*_test.go` | AC-25, R-14 | |
| `TestStdinDaemonStore` | `cmd/ze/hub/main_test.go` | AC-24, R-13 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| tree frame data length | 0 to available memory | an empty value round-trips | N/A | N/A |
| key path depth | 1 to `PATH_MAX` segments | `meta/a/b/c/d` | `..` and empty segment refused by `KeyEntry.Key` | a key longer than `PATH_MAX` fails the write with the OS error named |
| folder mode | exactly 0700 | 0700 | 0600 (not searchable, refused) | 0710 and up (refused) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `init-creates-tree` | `test/plugin/*.ci` | AC-12 | |
| `start-on-tree`, `start-auto-creates`, `start-refuses-blob`, `start-refuses-loose-mode`, `start-refuses-unfinished-import` | `test/plugin/*.ci` | AC-1, AC-4, AC-15, AC-16 through `ze start` | |
| `doctor-without-store`, `doctor-tree-corrupt-key` | `test/plugin/*.ci` | AC-23 | |
| `login-on-tree` | `test/plugin/*.ci` | serial and SSH login credentials from the tree | |
| `statestore-on-tree`, `ospf-state-through-daemon` | `test/plugin/*.ci` | AC-18, AC-19 | |
| `data-check-tree` | `test/plugin/*.ci` | AC-22 | |
| `web-on-tree` | `test/web/*.wb` | AC-17 web arm | |
| appliance first-boot import | `internal/le/qemu` install scenario | AC-26 | |
| the 23 re-cut tests | existing `.ci`/`.et` | AC-27 | |

## Files to Modify
- `internal/component/config/storage/storage.go` - interface keeps its methods; `SetWriteObserver` becomes a method; `IsBlobStorage`, `BlobStoreFrom`, `NewFilesystem`, `filesystemStorage`, `writeVersionFS`, `listVersionsFS` deleted
- `internal/component/config/storage/blob.go` - becomes the blob encoding only: get/put/delete/list by key over `*zefs.BlobStore`; key resolution, `metas`, observer, versions move to `store.go`; `NewBlob` and `migrateExistingFiles` deleted
- `internal/component/config/storage/pointer.go` - per-name pointer keys; `pointerPath`/`versionPath` lose their branches
- `internal/component/config/storage/cli/main.go`, `register.go`, `cmd_integrity.go` - `--path` accepts a directory; open through `storage.Open` or the blob encoding; `check`/`repair` call `zefs.CheckPath`/`RepairPath`; `Subs` names every verb. Design doc: `docs/architecture/zefs-format.md`
- `internal/core/resolve/resolve.go` - `Storage()` becomes `StorageFor(configPath)`; `EnvKeyStorageBlob` deleted
- `internal/core/statestore/statestore.go` - `SetStore(storage.Storage)`, `Store() storage.Storage`
- `internal/component/plugin/` protocol and `pkg/plugin/` SDK - a state put/get message so an external plugin persists through the daemon's `statestore`. Design docs: `docs/architecture/api/process-protocol.md`, `ai/rules/plugins.md`
- `cmd/ze/hub/main.go`, `main_reload.go`, `main_servers.go`, `managed_server.go`, `service_web.go`, `service_lg.go` - gates deleted, `loadZefsUsers` through `Storage`, the stdin-daemon store warning (R-13)
- `cmd/ze/hub/statestore.go` - deleted
- `cmd/ze/ze_core_start.go`, `ze_core_autoinit.go`, `ze_core_dispatch.go`, `setup_dispatch.go`, `login.go`, `help_ai.go` - gates deleted, `resolve.StorageFor`, auto-create on `ze start <file>`, `gokrazyAutoInit` imports the seed or creates, login through `Storage`, the stale `ZE_STORAGE_BLOB` and "creates database.zefs with TLS certs" texts corrected
- `internal/component/config/cli/cmd_edit.go`, `cmd_import.go`, `cmd_list.go`, `cmd_set.go`, `cmd_deactivate.go`, `config_data.go`, `main.go`, `cmd_show.go`, `cmd_diff.go`, `cmd_history.go`, `cmd_rollback.go` - gates deleted; store resolved after argument parsing from the file's folder; `validate`/`show` open no store; `set`/`rollback`/`history` per AC-25; `defaultConfigName` deleted for `resolve.DefaultConfig`
- `internal/component/config/infra/ssh.go` - `ResolveSSHStorage` deleted
- `internal/component/bgp/config/loader_create.go`, `register.go` - one read path through `ReadActiveConfig`, loose-file read for a config path the store does not hold
- `internal/component/cli/editor.go` - `NewEditor` deleted; `bgp/plugins/cmd/peer/save.go`, `prefix_update.go`, `resolve/cmd/rir.go` take the daemon's store
- `internal/core/ssh/client/client.go` - `ResolveDBPath` becomes `ResolveStoreDir` (config path, `ze.config.dir`, default); the open goes through `storage.Open`
- `internal/plugins/init/main.go` - `storage.Create`, exists-check on both shapes, `--force` on a directory, `daemonRunning` through `Storage`
- `internal/plugins/connect/main.go`, `internal/plugins/systemd/cmd_install.go`, `internal/plugins/local/cmd_install.go`, `internal/component/resolve/irr/stored.go`, `irr/store/store.go`, `internal/component/firewall/plugins/domain/cache.go`, `firewall/plugins/irr/cache.go`, `firewall/plugins/irr/doctor.go`, `internal/component/bgp/plugins/filter_irr/cache.go`, `internal/component/cli/client/main.go` - `storage.Open` replaces `zefs.Open`; the writers among them refuse while a daemon runs
- `internal/plugins/ospf/gr_nvs.go`, `ospf/auth_keystore.go` - state through the plugin protocol to the daemon's `statestore`
- `internal/component/doctor/checks_storage.go`, `doctor.go`, `internal/core/diagnostic/codes.go` - store check walks the tree; `doctor-store-permissions` added; description text for `doctor-disk-space` no longer names "the zefs database"
- `internal/install/disk/system.go` - `bakedSeedPresent` accepts a directory
- `pkg/zefs/netcapstring.go` - export the decode; `pkg/zefs/check.go` - `CheckPath`, `RepairPath` over a directory, `MoveAside` documented for either
- `pkg/zefs/keys.go` - register `meta/config/{basename}/active|candidate|rollback|recovery`; delete the four nameless pointer keys
- `internal/le/fspersistence/fspersistence.go` - `ScanFile` gains a `zefs.Open|Create` selector case with its own allowlist (the A-4 side stores); dead allowlist entries removed
- `internal/test/runner/runner_exec.go`, `runner_exec_util.go`, `internal/component/cli/testing/runner.go` - pins removed, `expect=key` added, `option=storage` values `tree|blob`
- `test/**/*.ci` (28 env pins, 10 path assertions), `test/**/*.et` (13 path assertions) - re-cut
- `docs/architecture/zefs-format.md`, `docs/architecture/fleet-config.md`, `docs/architecture/resolve.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/core-design.md`, `docs/architecture/appliance/on-device-installer.md`, `docs/architecture/testing/ci-format.md`, `docs/architecture/api/process-protocol.md`, `docs/guide/config-editor.md`, `docs/guide/operations.md`, `docs/guide/command-reference.md`, `docs/guide/appliance.md`, `docs/guide/ze-install.md`, `docs/guide/ubuntu-build-install.md`, `docs/guide/quickstart.md`, `docs/functional-tests.md`, `docs/features.md`, `docs/guide/status.md` - the page edits named in Required Reading, in the phase that changes each behavior
- `website/guides/index.md`, `website/docs/docs.md`, `website/data/nav.json` and every website page `grep -rl zefs website` lists - "create zefs" wording; the reference pages regenerate through `./le site build`
- `../wiki/command-catalog.md`, `../wiki/appliance.md` - regenerated and committed in the wiki checkout (AC-29)
- `ai/INDEX.md`, `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md` - the new files and the pages they declare

## Files to Create
- `internal/component/config/storage/store.go` - the shared core: key resolution, `metas`, observer, versions, pointers over an `encoding` with get/put/delete/list by key
- `internal/component/config/storage/tree.go`, `tree_test.go` - the tree encoding
- `internal/component/config/storage/open.go`, `open_test.go` - `Open(dir)`, `Create(dir)`, detection, permission check
- `internal/component/config/storage/import.go`, `import_test.go` - `ImportBlob(blob, dir)` with equality check and move-aside
- `internal/component/config/storage/conformance_test.go` - the table over both encodings
- `test/plugin/init-creates-tree.ci`, `start-on-tree.ci`, `start-auto-creates.ci`, `start-refuses-blob.ci`, `start-refuses-loose-mode.ci`, `start-refuses-unfinished-import.ci`, `doctor-without-store.ci`, `login-on-tree.ci`, `statestore-on-tree.ci`, `ospf-state-through-daemon.ci`, `data-check-tree.ci`, `doctor-tree-corrupt-key.ci`
- `test/web/web-on-tree.wb`
- `docs/architecture/storage-backends.md` - the contract, the tree encoding, detection, permissions, auto-create and import rules; `zefs-format.md` keeps the blob format only

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config leaf; the plugin-protocol state message is a process-protocol message, not a YANG RPC |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | Yes | `internal/component/config/storage/cli/main.go` (`--path` accepts a directory); `ze init` flag set unchanged; `ze start <file>` gains auto-create |
| CLI grammar (keyword before value) | N-A | no new keyword; `--path <value>` keeps its shape |
| Editor autocomplete | N-A | no YANG leaf; `ze init` is an offline command |
| Functional test for new RPC/API | Yes | the `.ci` files under Files to Create; `ospf-state-through-daemon.ci` for the protocol message |
| Pipe completeness | N-A | no new output command; `ze data` rows keep their shape |
| Env var registration | Yes | `ze.storage.blob` deregistered in `internal/core/resolve/resolve.go`; no new key |
| Doctor check for runtime dependencies | Yes | `internal/component/doctor/checks_storage.go` walks the tree and checks permissions; codes `doctor-store-permissions` and a corrupt-tree-key code in `internal/core/diagnostic/codes.go`; `doctor-tree-corrupt-key.ci`, `start-refuses-loose-mode.ci` |
| Prometheus counters/metrics | N-A | no observable state added; auto-create and import are startup events logged at info |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: storage row names the tree, auto-create, the blob as artifact, permissions |
| 2 | Config syntax changed? | No | no YANG change; verified by the Integration Checklist |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: `ze init` creates a directory, every `ze init` flag, every `ze data` verb, `--path` directory, `ze start <file>` auto-create |
| 4 | API/RPC added/changed? | No | no YANG RPC; `show data *` payloads unchanged |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` if it lists `ze init` flags or OSPF persistence; `docs/architecture/api/process-protocol.md` for the state message |
| 6 | Has a user guide page? | Yes | `docs/guide/operations.md` (env table, `--force`, backup name, store folder and client resolution rule, permissions), `docs/guide/config-editor.md` (file mode sentence), `docs/guide/appliance.md`, `docs/guide/ze-install.md`, `docs/guide/ubuntu-build-install.md`, `docs/guide/quickstart.md` (`.replaced-` name, `ze init from` for an existing blob) |
| 7 | Wire format changed? | No | nothing on the wire |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md`: the state put/get message |
| 9 | RFC behavior implemented, changed, or newly proven? | No | not protocol |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md`: pins removed, `expect=key`, `option=storage` values |
| 11 | Affects daemon comparison? | No | storage encoding is not a comparison row; verified by grep of `docs/comparison.md` for `zefs` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/storage-backends.md` (new), `docs/architecture/zefs-format.md` (statestore section, in-place rows, exact-fit sentence), `docs/architecture/fleet-config.md`, `docs/architecture/resolve.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | No | not route metadata |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` (doctor codes), `docs/plugin-overview.md` (the state message type) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation: `./le spec citation anchors spec plan/pre-release/spec-storage-1-backend-parity.md`; `storage.go`, `blob.go`, `pointer.go`, `statestore.go`, `init/main.go`, `ca.go` each declare or are anchored by the pages in Required Reading |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ze init` and `ze data` examples in `command-reference.md`, `operations.md`, `quickstart.md`, `ubuntu-build-install.md` and the website guides verified against the new behavior; `./le site build` run and the wiki catalog committed (AC-29) |

Design documents declared by the `// Design:` headers of files in scope, and
what this spec does to each:

| Page | Affected? | Reason |
|------|-----------|--------|
| `docs/architecture/appliance/on-device-installer.md` | Yes | `bakedSeedPresent` accepts a `database/` directory; the seed is imported by `gokrazyAutoInit`, not read live (Phase 7) |
| `docs/architecture/config/syntax.md` | No | config file syntax is untouched; the offline CLI files declare it for the parser they call, which this spec does not change |
| `docs/architecture/config/yang-config-design.md` | No | no YANG module or leaf changes |
| `docs/architecture/core-design.md` | Yes | its storage paragraph names the blob as the store; it gains one sentence pointing at `storage-backends.md` (Phase 2) |
| `docs/architecture/hub-architecture.md` | Yes | the daemon startup sequence it describes opens the store through `resolve.Storage` and registers a second state-only store; both sentences change (Phase 5) |
| `docs/architecture/system-architecture.md` | No | the component map is unchanged; `internal/component/config/storage` keeps its place and role |
| `docs/architecture/testing/ci-format.md` | Yes | `option=env` with `ze.storage.blob` leaves its examples; `expect=key` is documented (Phase 7) |
| `docs/architecture/api/process-protocol.md` | Yes | the state put/get message (Phase 5) |
| `docs/architecture/ospf/ospf-ext-9-graceful-restart.md` | Yes | declared by `internal/plugins/ospf/gr_nvs.go`; its "non-volatile storage" sentence names `database.zefs` opened by the plugin, which becomes a state message to the daemon (Phase 5) |
| `docs/features/ai-first.md` | No | declared by `cmd/ze/help_ai.go`, whose only change is the stale "creates database.zefs with TLS certs" line; the page's claims about AI-facing help do not name the store |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the opener and the conformance harness exist and fail
   - Tests: `TestStorageConformance` (table with both encodings, tree rows failing), `TestOpenDetection`, `init-creates-tree.ci`, `start-on-tree.ci`
   - Files: `storage/open.go` (`Open`, `Create` stubs returning "not implemented"), `storage/conformance_test.go`, `internal/core/resolve/resolve.go` (`StorageFor` calling `Open`), `internal/plugins/init/main.go` (calls `Create`)
   - Verify: the daemon compiles against `StorageFor`; the `.ci` fails because `Create` is a stub
2. **Phase: Shared core** -- hoist key resolution, `metas`, observer, versions, pointers out of `blob.go` into `store.go` over an encoding interface; blob becomes an encoding
   - Tests: `TestStorageConformance` blob rows green; `TestPointerPerName`
   - Files: `store.go`, `blob.go`, `pointer.go`, `pkg/zefs/keys.go` (new keys, nameless keys deleted), `docs/architecture/storage-backends.md`, `docs/architecture/zefs-format.md`, `docs/architecture/core-design.md`
   - Verify: no behavior change on the blob; the per-name pointer AC passes
3. **Phase: Tree encoding** -- `tree.go` with netcapstring frames, exported decode, `CheckPath`/`RepairPath`, permission check
   - Tests: `TestTreeKeyIsFile`, `TestTreeFrameCRC`, `TestTreeReadDuringWrite`, `TestOpenPermissionCheck`, `TestCheckPathDirectory`, `TestRepairDirectory`, `TestDecodeNetcapstringExported`; conformance tree rows green
   - Files: `tree.go`, `open.go`, `pkg/zefs/netcapstring.go`, `pkg/zefs/check.go`, `storage/cli/cmd_integrity.go`, `doctor/checks_storage.go`, `diagnostic/codes.go`, `docs/guide/command-reference.md`, `docs/guide/status.md`
   - Verify: AC-1, AC-4, AC-7, AC-8, AC-22, AC-23
4. **Phase: Detection, creation, import** -- `Open` refusals, `Create` atomic and race-safe, `ImportBlob`, `ze init`, `ze start` auto-create, `gokrazyAutoInit`
   - Tests: `TestOpenDetection` full table, `TestCreateAtomic`, `TestCreateRaceLoser`, `TestImportEqualsSource`, `TestImportRefusesOnMismatch`, `TestImportRefusesCorrupt`, `TestInitCreatesTree`, `TestInitRefusesExistingShape`, `TestInitAtomicTree`, `TestStartAutoCreates`, `TestStartRefusesBlob`, `start-auto-creates.ci`, `start-refuses-blob.ci`, `doctor-without-store.ci`
   - Files: `open.go`, `import.go`, `init/main.go`, `ze_core_start.go`, `ze_core_autoinit.go`, `fspersistence.go` (dead entries removed), `docs/guide/operations.md`, `docs/guide/quickstart.md`, `docs/guide/ubuntu-build-install.md`
   - Verify: AC-2, AC-3, AC-5, AC-6, AC-12 to AC-16
5. **Phase: Gate deletion and bypass migration** -- every `IsBlobStorage`, `BlobStoreFrom`, `NewFilesystem`, `ResolveSSHStorage`, `openStateOnlyStore`, `ze.storage.blob` site; `statestore` over `Storage`; the in-daemon editors take the store; the 15 direct openers through `storage.Open`; OSPF through the plugin protocol; the `fspersistence` detector
   - Tests: `TestStatestoreThroughStorage`, `TestStorageForResolution`, `TestPeerSaveWritesStore`, `TestOSPFStateThroughDaemon`, `TestConnectRefusesWithDaemon`, `TestFSPersistenceRefusesLiveStoreOpen`, `TestOfflineSetWithoutStore`, `TestEditResolvesAfterArgs`, `TestStdinDaemonStore`, `login-on-tree.ci`, `statestore-on-tree.ci`, `ospf-state-through-daemon.ci`, `web-on-tree.wb`
   - Files: the `cmd/ze/**`, `internal/component/config/cli/**`, plugin and component rows under Files to Modify, `statestore.go`, `infra/ssh.go`, `cli/editor.go`, `bgp/config/*.go`, `ssh/client/client.go`, the plugin protocol and SDK, `docs/guide/config-editor.md`, `docs/architecture/fleet-config.md`, `docs/architecture/resolve.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/api/process-protocol.md`, `ai/rules/plugins.md`
   - Verify: AC-11, AC-17 to AC-21, AC-24, AC-25, AC-28; grep for each deleted symbol returns nothing
6. **Phase: Appliance and suite** -- `bakedSeedPresent`, the QEMU first-boot scenario, runner pins, the 23 re-cut tests, A-10's residual reproduced first
   - Tests: the install scenario, the full functional suite
   - Files: `install/disk/system.go`, `runner_exec.go`, `runner_exec_util.go`, `cli/testing/runner.go`, the `.ci`/`.et` files, `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md`, `docs/guide/appliance.md`, `docs/guide/ze-install.md`, `docs/architecture/appliance/on-device-installer.md`, `docs/features.md`, `ai/INDEX.md`, `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md`
   - Verify: AC-26, AC-27; each re-cut test shown RED once against the pre-change tree
7. **Phase: Published surfaces** -- website pages, `./le site build`, wiki catalog
   - Tests: AC-29's grep and build
   - Files: `website/**` rows under Files to Modify, `../wiki/command-catalog.md`, `../wiki/appliance.md`
   - Verify: AC-29

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | `Open` never creates, never converts; `Create` and `ImportBlob` install by rename; the permission check runs before any key is read; a CRC mismatch on a tree read fails closed; import equality is checked before the blob moves |
| Naming | keys registered in `pkg/zefs/keys.go` through `MustRegister`; the tree directory is `database/` and the blob file `database.zefs`; temp names `database.init-tmp/`, `database.import-tmp/` |
| Data flow | `statestore`, every client and every plugin reach the store only through `storage.Open`, the daemon's handle, or the plugin protocol; `pkg/zefs` is imported outside the storage package only by side-store owners named in A-4 and by `Key*` users |
| Rule: `ai/rules/no-layering.md` | `filesystemStorage` is deleted before the tree lands; no arm reads "if tree ... else blob" outside the encoding boundary; no on-open conversion arm exists |
| Rule: `ai/rules/principles.md` | no method returns an empty value for "no store": `Open` errors, `ReadFile` on a bad frame errors, a stdin daemon names each refused feature |
| Rule: `ai/rules/documentation.md` | each page, website and wiki edit lands in the phase that changed the behavior, not at closure |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| No backend gate outside the storage package | `grep -rn "IsBlobStorage\|BlobStoreFrom\|NewFilesystem\|ResolveSSHStorage\|openStateOnlyStore" cmd internal` returns nothing |
| No live-store open outside the storage package | `./le fs-persistence check` green, and `grep -rn "zefs\.Open\|zefs\.Create" cmd internal --include=*.go \| grep -v _test \| grep -v config/storage` lists only the A-4 side stores and seed builders |
| `ze.storage.blob` gone | `grep -rn "ze.storage.blob" cmd internal test docs website` returns nothing |
| Conformance table covers both encodings | `go test ./internal/component/config/storage -run TestStorageConformance -v` shows every row for `tree` and `blob` |
| Import round trip | `TestImportEqualsSource` |
| Appliance imports on first boot | the `internal/le/qemu` install scenario log shows the import line and `database.zefs.replaced-` |
| Pages, site and wiki updated | `./le doc wiring` green; `./le spec citation anchors spec plan/pre-release/spec-storage-1-backend-parity.md` lists no unaddressed anchor; `./le site build` run; wiki commit SHA recorded |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | a key from `ze data --path` or a `List` result is validated as `fs.ValidPath` before it becomes a tree path: no `..`, no absolute, no empty segment |
| File modes | the tree root is created 0700 and every frame 0600; `Open` refuses anything looser or a foreign owner; `meta/auth/*`, `meta/ca/key`, `meta/web/key` are never world-readable |
| Symlinks in the tree | a symlink under `database/` is refused on open (`Lstat`), so a planted link cannot redirect a credential read |
| Import trust | `ImportBlob` reads only a blob that passed `zefs.Check`; a blob that fails imports nothing and is reported |
| Auto-create scope | only `ze start <file>` creates; `ze doctor`, clients and plugins never create a store in a folder they were pointed at |
| Error text | a CRC, permission or open error names the key or path, never a value |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The filesystem backend was never a backend: it was the absence of one. Every gate was a feature declining to run because a key space was missing, and every direct `zefs.Open` was a feature routing around the abstraction to reach the key space anyway.
- Rooting the store at the config's directory removes a whole journal class (`suite-shares-one-persistent-store`) as a side effect: the runner already gives every daemon its own folder.
- Conversion inside `Open` looked like the cheapest migration and was the most dangerous line in the first draft: every process that opens the folder would have been a converter, racing the daemon. An explicit import tool has one caller at a time.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One netcapstring per key in the tree | plain files with a sidecar checksum manifest | the manifest is a second declaration of every key and drifts; one frame per file reuses the blob's encoder, decoder and CRC and gives `check`/`repair` one code path over both encodings |
| Shared core over an encoding interface | add a filesystem arm to each of the 23 gates | the arms are the state this spec leaves; the core is where versions, pointers and the observer already had to exist once |
| `Open` detects and refuses; import is an explicit tool (owner, review answer 3) | convert on open | every opener would be a converter; the daemon, serial login and doctor race at appliance first boot; a conversion by a non-daemon process has no lock to take |
| `ze start <file>` auto-creates (owner, review answer 1) | refuse without `ze init` | the runner turns every `.ci` daemon into `ze start <workdir>/ze-bgp.conf`; refusing breaks ~1000 tests and the dev loop; `ze init` stays the only creator of credentials |
| Store root = config's directory, with a permission check (owner, review answers 2) | always `paths.DefaultConfigDir()` | owner directive; clients resolve the same folder from the path they are given, then `ze.config.dir`, then the default |
| Every blob is an artifact; no live blob backend (owner, 2026-09-16) | `ze init backend blob` | one live encoding means one set of operational facts; the blob keeps its roles as seed, backup and offline-edit artifact in storage-2 |
| OSPF state through the plugin protocol | a second-process `storage.Open` with a lock | the page's one-handle rule; a plugin process holding the daemon's store is the lost-update class |
| Per-name pointers with no legacy fallback | keep the nameless pointer, or read it as a fallback | pre-release: no store to migrate; `ReadActiveConfig` already falls back to the mirror and `EnsureActiveVersion` re-versions it |
| Stamp-addressed versions kept; content addressing deferred | fold `spec-storage-3` in | owner: "own spec for it"; storage-3 lands with no migration because no pre-release store needs one |

## Known Limitations

- Side stores (`debug.zefs`, the seed builders in `internal/appliance` and `internal/plugins/imageserver`) stay blob files (A-4). Making them use the live store, where that is right, is a separate decision.
- Encryption at rest is `spec-config-at-rest-encryption`; the tree needs a per-file answer that spec will have to give.
- Backup, restore with modes, `ze init from <url>`, offline editing of a blob, and the spare capacity option are `spec-storage-2-blob-artifact`.
- Content-addressed history (versions named by SHA-256, objects stored once, a targeted sweep) is `spec-storage-3-content-addressed-history`.
- A client process reaching a daemon started on a non-default folder needs `ze.config.dir`; no runtime discovery of the daemon's folder exists.
- A stdin daemon with no writable folder runs without a store; the features it refuses are named at startup, not made to work.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Pre-implementation Design Audit (2026-09-17)

**Verdict: design changes required before implementation.** The audit covered
AC-1 through AC-29 in three independent contexts. The supervising context read
the producing functions for the findings below. No product code changed.
Thomas authorized the review-model exception by selecting "Use this harness"
on 2026-09-17. This audit is not the implementation Review Gate.

The findings below identify contradictions and missing contracts. Their
recommended corrections are proposals, not owner-approved scope changes.
The existing acceptance criteria remain in scope.

### Findings

| ID | Priority / AC | Finding and required design correction | Evidence |
|----|---------------|----------------------------------------|----------|
| D-1 | High / AC-5, AC-12, AC-13 | `Create` publishes before init seeds its keys. Use one staged population operation for init and import; publish only after successful population. A callback avoids exposing an independently managed staging handle. | `internal/plugins/init/main.go` `runInit` currently seeds the temporary blob before its final rename. Data Flow returns the published tree before init writes. |
| D-2 | High / AC-5 | Rename can replace an existing empty directory. The shared temporary name also fails to isolate concurrent creators. Specify per-attempt staging and atomic no-replace publication. Only auto-create can reopen a winner; init must retain its existing-store refusal. | Executed filesystem probe: rename onto an empty directory succeeded and changed its inode; rename onto a populated directory returned `ENOTEMPTY`. This disproves R-2's premise. |
| D-3 | High / AC-8, AC-10 | A crash after active becomes candidate but before candidate deletion leaves both references. Boot cleanup then deletes the active version. Make cleanup reference-aware and define recovery at every persisted promotion transition. | `storage/pointer.go` `PromoteCandidate`, `ClearCandidate`; `cmd/ze/hub/main.go` `clearStaleCandidateOnBoot`. Here and below, `storage/` means `internal/component/config/storage/`. This is a design consequence of independently persisted tree writes. |
| D-4 | High / AC-5, AC-6, AC-8 | Version-first ordering needs successful directory durability barriers before pointer publication. Specify durable ancestor creation, rename, and final store publication, including sync errors. | `storage/storage.go` `atomicWriteFile` syncs the file but ignores parent-directory sync failures and does not sync each newly created ancestor. |
| D-5 | High / AC-6, AC-26 | Publishing the tree before moving the seed leaves a both-present state that `Open` refuses. Specify restartable explicit-import recovery that cannot overwrite an unrelated tree. Give `ImportBlob` sole responsibility for seed retirement. | AC-2 and AC-6 conflict at the interruption boundary. `pkg/zefs/check.go` `MoveAside` is a separate rename; Integration Points assigns it to the appliance caller as well. |
| D-6 | High / AC-2 | The corrupt-blob move-aside requirement contradicts AC-2's stat-only, non-mutating refusal. Keep `Open` non-mutating; put diagnosis and any approved quarantine operation in explicit import or repair. | Behavior to change requires a rename; AC-2 forbids it. `storage/blob.go` `NewBlob` is the old self-healing behavior the cutover removes. |
| D-7 | High / AC-9, AC-22 | Data commands need raw-key access and recursive enumeration. Their current contract cannot be implemented by substituting `Storage.List` and its config-name resolver. Also distinguish the config folder accepted by `Open` from the `database/` path accepted by `ze data`. | `storage/cli/main.go` `cmdList`, `cmdWrite`, `cmdCat`; `pkg/zefs/store.go` `BlobStore.list`; `storage/blob.go` `List`, `resolveDirKey`, `resolveKey`. Empty-prefix raw listing visits all keys; storage listing visits immediate active-config children. |
| D-8 | High / AC-6, AC-7, AC-22 | Harden the reused decoder before exporting it. Require overflow-safe bounds and exactly one complete frame in each tree file. Test oversized capacities, truncation, appended bytes, and concatenated frames. | Executed `zefs.Check` probe panicked with index `-9223372036854775757` for capacity `9223372036854775807`. `pkg/zefs/netcapstring.go` `decodeNetcapstringRef` adds one before comparing and returns a next offset without requiring EOF. |
| D-9 | Medium / AC-1, AC-7, AC-22 | Temporary write files must stay outside the logical key tree on the same filesystem. Otherwise list/check/repair can observe partial frames or unintended credential copies. Define ignored leftovers without reserving previously valid key names. | `storage/storage.go` `atomicWriteFile` creates `.ze-storage-*` beside the target. The proposed tree maps every file to a key. |
| D-10 | Medium / AC-6 | Define corruption diagnostics when no key can be recovered. A container checksum error currently prevents entry inspection. Require bounded diagnostic scanning where framing permits it; report path and offset where the key is unknowable. | `pkg/zefs/check.go` `Check` returns on a container CRC failure before reading entries. A value corruption can therefore fail before its key is reported. |
| D-11 | Medium / AC-4, AC-23 | Define ownership, exact modes, and allowed node types for the root and every intermediate directory. Reject root symlinks and non-regular leaves before reading. Apply the same secure traversal to check/repair. | Data Flow accepts a root with no group/other bits, while Boundary Tests require exactly 0700. Root symlinks, intermediate-directory permissions, and FIFO leaves have no specified verdict. |
| D-12 | High / AC-17, AC-20 | Local editing and command history are live writers outside the daemon. Move these operations behind the owning daemon; refusing all client writes would disable the promised features. Cover concurrent local and daemon editor sessions. | `config/cli/cmd_edit.go` `cmdEditWithStorage`, `runEditor` create a local editor and use SSH for reload. `internal/component/cli/editor_commit.go` `CommitSession` writes locally. `internal/component/cli/client/main.go` `runInteractiveSession` opens history locally, and `history.go` `History.Save` writes it. Here `config/` means `internal/component/config/`. |
| D-13 | High / AC-14, AC-20, AC-22 | An SSH target probe does not establish store ownership. Define exclusion tied to the store and held for the write lifetime, covering startup, web-only owners, and concurrent offline commands. | `init/main.go` `daemonRunning` reads the mutable remote target and treats open/dial errors as no daemon; `internal/plugins/connect/main.go` `SetDefault` changes that target. `cmd/ze/hub/service_web.go` `runWebOnly` uses storage without an SSH listener. Here `init/` means `internal/plugins/init/`. |
| D-14 | High / AC-4, AC-20, AC-23 | The service runs as `ze`, but installation and maintenance run as root. Specify recursive ownership transfer and a privileged maintenance policy. Strict caller ownership requires maintenance to operate as the store owner. | `internal/plugins/systemd/cmd_install.go` `cmdInstall`, `chownConfig`; `unit.go` `buildUnitFile`. Current transfer changes the folder and blob only. AC-4 otherwise rejects root access after transfer and service access before transfer. |
| D-15 | High / AC-24 | Storeless startup needs an explicit CA and plugin dependency contract. Warnings alone do not permit startup while leaving CA construction unchanged. An ephemeral authority is a possible full-contract design, but its lifetime and refused persistent features need an owner decision. | `cmd/ze/hub/main.go` `runYANGConfig` requires CA creation. `internal/component/pki/ca.go` `LoadOrGenerateRootFor` rejects nil storage and persists certificate and key before returning. |
| D-16 | High / AC-15, AC-24, AC-25 | Decide which config is authoritative after an explicit-file start. The proposed first start creates an active version; a later loose-file edit can then be hidden by that version at restart. Specify synchronization and conflict behavior across file edits, SSH commits, reload, and restart. | `cmd/ze/hub/main.go` `runYANGConfig` calls `EnsureActiveVersion`; `storage/pointer.go` `ReadActiveConfig` prefers the stored pointer. This follows from combining the proposed startup path with the preserved loose-file contract. |
| D-17 | Medium / AC-11, AC-24, AC-25 | Multiple input files do not share a config-folder identity. Read explicit diff operands independently. Specify import's destination separately from source paths and define duplicate-basename handling. | `config/cli/cmd_diff.go` `resolveDiff`, `loadAndResolve` use one store for both operands; `cmd_import.go` `cmdImportWithStorage` reads multiple files and writes their basenames to one destination. |
| D-18 | High / AC-19 | Move OSPF state access out of construction/configure and into a runtime-safe stage before packet processing. Define durable acknowledgement and separate absent, unavailable, and corrupt reads. Cover internal, external, and reload-created engines. | `internal/plugins/ospf/instance.go` `newEngine` reads boot state; `register.go` `runEngine` constructs engines before `Plugin.Run` and during configure. `server/startup_driver.go` `runStartupHandshake` accepts only stage methods then. `pkg/plugin/sdk/sdk.go` `Plugin.Run` permits runtime calls in `OnStarted`. `statestore.Get` hides read errors and `Put` can return false without an error. |
| D-19 | High / AC-27 | Use a stable directory per concurrent daemon, reused by its restarts. A directory per test is insufficient. Account for wrapped stdin launches and update rewrite/assertion paths with the directory change. | `internal/test/runner/runner_exec.go` writes different daemon config names in one `WorkDir`; `test/ipsec/ipsec-child-rekey.ci` starts such a pair. `runner_exec_util.go` `routeStdinBlock` leaves the wrapped launch in `test/l2tp/subscriber-reader-failing-socket.ci` on stdin. |
| D-20 | Medium / AC-19 | Model state operations through the existing YANG RPC transport. Name the schema, request/result types, SDK methods, and common dispatch registration. Correct the checklist's no-schema/no-RPC answers. | `internal/core/ipc/yang/ze-plugin-engine.yang`; `internal/component/plugin/server/dispatch_registry.go` `engineOps`; `pkg/plugin/sdk/sdk.go` `callEngineRaw`. |
| D-21 | Medium / AC-21 | Give the live-store bypass detector complete production coverage and rule-specific exemptions. Test core callers, components, aliased imports, allowed artifact owners, and stale exemptions. | `internal/le/fspersistence/fspersistence.go` `scanRoots`, `check` omit `internal/core`; `internal/core/ssh/client/client.go` `openStoreIfReadable` is a live-store opener there. |
| D-22 | Medium / AC-26 | Inspect the sibling tree path from the installer. Accepting a directory only at the existing helper argument does not preserve converted installs. Test the production caller with a tree and no blob. | `internal/install/disk/system.go` `mountInjectDB` passes `database.zefs` to `bakedSeedPresent`; the new live tree is `database/`. |
| D-23 | Medium / AC-27, AC-28 | Keep the compatible test harness fixed while reverting or mutating product behavior. Unknown `expect=key` or `option=storage:tree` is a setup failure, not discrimination. Remove pins and deregister their key in the same cutover. | `internal/component/cli/testing/expect.go` `checkExpectation` rejects unknown expectations; `runner.go` rejects unknown storage modes. Phase 5 removes the key before Phase 6 removes its remaining uses. |
| D-24 | Medium / AC-29 | Run `./le wiki-catalog update file ../wiki/command-catalog.md` explicitly. Site build and the authored wiki appliance page remain separate obligations. | `internal/le/site/build.go` `publishCommandCatalog` writes the site catalog. `internal/le/wikicatalog/report.go` `Update` and `actions.go` `runUpdate` produce the wiki catalog. |

### Coverage and File-Plan Corrections

| Area | Required correction |
|------|---------------------|
| AC-8 conformance | Include file/directory conflicts and empty-parent pruning, raw versus config key semantics, guarded reads, observer timing, and error outcomes. `pkg/zefs/tree.go` `node.set` and `removeRecursive` define existing collision/pruning behavior. |
| AC-14, AC-17, AC-18, AC-20 | Add functional coverage for forced replacement, all named parity features, all six state consumers across restart, owner transfer, and live-client writes. A web test and a BFD test cannot establish every arm. |
| AC-22, AC-23 | Cover directory list/cat/write/rm/import and repair-output reopen, plus permission and non-regular-node refusals. The named data-check test alone does not prove those commands. |
| AC-24 | Include `internal/component/config/cli/register.go`: its root handler requires storage before dispatch. Include `internal/component/support/support.go` `collectConfig`, another resolver caller. |
| AC-19 | Include `internal/plugins/ospf/doctor.go` `grStoreOpenable`, which calls the opener scheduled for deletion. Specify offline and runtime diagnostics. |
| AC-20 | Use the registered verb `ze install systemd`, declared by `internal/plugins/systemd/register.go` `init`. |
| AC-27 | Include `internal/component/cli/testing/expect.go` and its `State` interface, where `expect=key` must be implemented. Preserve rejection of unknown expectation types. |
| Import recovery | Interrupt after each durable publication step, then resume through the explicit importer. Assert that an unrelated or modified tree is never replaced. |

### Evidence and Remaining Decisions

Executed probes established D-2 and the decoder panic in D-8. Other findings
come from specification contradictions and producer inspection; their future
tree failure scenarios are design inferences, not executed tree tests.
No functional suite or appliance scenario ran because this was a design audit.

Before returning to `ready`, resolve the ownership/exclusion policy (D-13 and
D-14), storeless authority (D-15), and config authority (D-16). Revise the
affected contracts and test rows for every finding above. Keep all 29
acceptance criteria in scope.

### Owner Decision Log

| ID | Date | Owner answer | Design constraint |
|----|------|--------------|-------------------|
| O-1 | 2026-09-17 | "Require the store owner" | Keep AC-4's strict caller-ownership check, including for root. Storage maintenance runs as the store owner. The installer retains root for system changes and transfers ownership of the complete tree before service startup. Do not add a root bypass or automatically change store ownership on open. |
| O-2 | 2026-09-17 | "One owning process" | Hold an exclusive store-ownership lock for the daemon lifetime, including startup and web-only operation. Route live editor/history writes through that daemon. Offline writers acquire the same lock and refuse while it is held; read-only clients remain permitted. Lock identity must survive store replacement, and a kernel-released lock must not depend on the selected SSH target. |
| O-3 | 2026-09-17 | "Use an ephemeral CA" | A genuinely storeless stdin daemon uses an explicitly selected, process-lifetime CA in memory, reusing the existing root-generation code. It writes no CA material to disk and warns that its authority changes on restart. Never use this path to recover from an unreadable, corrupt, or permission-refused persistent store or CA. Name unavailable persistence-dependent features at startup. |
| O-4 | 2026-09-17 | "The explicit file" | Every explicit-file start reads the supplied file, even when the store holds an active version. Daemon commits in that mode update the same file and stored history, with conflict handling when the file changed externally. Bare `ze start` uses the stored active config. A stored version must never silently hide an edit to the explicit file. |

D-14's policy question is resolved by O-1. Its implementation and verification
remain required: recursive ownership transfer, service startup as `ze`, refusal
of root access to a `ze`-owned store, and successful maintenance as the owner.

O-2 supersedes the Required Reading claim that the tree adds no cross-process
mechanism. D-13's policy question is resolved; D-12's daemon-side write paths
and ownership tests remain required.

D-15's authority policy is resolved by O-3. Specify initialization of the
process's active authority, trust delivery to its plugins, and renewal from
that same authority. Test stdin/setup startup, no CA files written, a new
authority after restart, and fatal persistent-store errors. Update
`docs/architecture/pki/pki-store.md` with the implementation.

D-16's source-authority question is resolved by O-4. Carry the selected source
mode through startup, reload, editor commit, and restart. Specify publication
ordering and recovery for the file and history together; a failed write must
not be reported as a successful commit. Test start, offline edit, restart;
SSH commit followed by restart; and an external edit racing a daemon commit.

All owner policy questions identified by this audit are answered. The
technical contract and coverage corrections in D-1 through D-24 still require
incorporation before this spec returns to `ready`. No product implementation
is authorized by the decision log alone.

### Implementation Authorization (2026-09-17)

Thomas instructed "implement" after O-1 through O-4. Implement all 29
acceptance criteria with D-1 through D-24 corrected; the audit's stale
assumptions do not override those corrections. The final handoff remains
`verify`.

The cutover uses explicit writable and read-only opens. `Open` owns the
writer lock; `OpenReadOnly` permits inspection while that owner runs and
refuses mutations. `Create` handles empty auto-creation; `CreatePopulated`
seeds an unpublished store and refuses an existing store. Both use private
staging and no-replace publication. `ImportBlob` owns resumable import and
seed retirement. Lock files are outside the replaceable store directory.

Raw data commands use `ReadKey`, `WriteKey`, `RemoveKey`, and recursive
`ListKeys`; ordinary config callers keep the existing immediate-child
`List` and config-name mapping. Blob artifact access stays inside storage.
Standalone frames must consume the complete file; integrity traversal
rejects symlinks and non-regular nodes. Per-key staging stays outside the
logical key tree.

Candidate cleanup preserves every version referenced by active, rollback,
or recovery. Durable version publication precedes pointer changes. Import
records enough durable identity to resume its own interrupted cutover while
refusing an unrelated or changed destination. Explicit-file mode remains
distinct from stored-config mode through restart, reload, and editor commit.

Implementation reconciliation: O-2's lifetime writer ownership takes precedence
over AC-5's original promise of two concurrent writable handles. Temporary
creation and import directories are private per attempt, not shared fixed
names. Atomic no-replace publication cannot overwrite an empty winner.

## Implementation Evidence (2026-09-17)

The implementation is integrated; this spec remains open with handoff `verify`.
The design-audit model exception does not authorize implementation review or closure.
The following results are implementation evidence, not an independent review verdict.

Evidence root: `tmp/session/2026-09-17-e4a8cc45-6dfc-4065-aba0-e6dc9afe3c42/scratch/`.
`implementation-evidence.json` indexes the logs, private-cache results, and limitations.
Preserved binaries and counterfactual inputs remain available for the next phase.

### Goal Validation

| Task goal | Evidence | Result |
|---|---|---|
| One live tree behind the shared storage contract | `job-storage-runtime-fixed-da53cbf1.log`: tree/blob conformance, CRC refusal, durable publication, import recovery, and per-name pointer tests | Pass |
| No live encoding knowledge outside storage | `fspersistence` check and negative detector tests; all legacy live constructors and backend-selection pins removed | Pass for the detector's production population |
| Every named feature works on the tree | Browser draft/commit/restart, authenticated SSH history, looking-glass TLS, managed 14/14, support export, identity, GR marker, six state consumers, and IRR/domain cache restart proofs; see evidence index | Exercised paths pass; this is not a claim of complete protocol interop |
| Store folder ownership and permissions are enforced | Offline smoke's 12 owner refusals; actual installer guest preserves frames and lock inode, transfers ownership, refuses root inspection, and starts as the service user | Pass; actual systemd PID 1 startup was not exercised |
| Explicit-file startup creates the tree without losing file authority | Canonical startup/explicit-file tests, actual loose editor commit, CLI transaction, reload, and restart proofs | Pass |
| Live blobs are refused and explicit import preserves seed data | Import crash/recovery tests and actual installed appliance: 11 seed values byte-equal, seed retired, SSH/TLS and serial password login succeed | Pass; standalone import verb is reserved for storage-2 |
| Migrated tests have isolated stores and detect the defect | Editor 170/170; 13 editor and nine CI recuts fail on consumer key assertions under a compatible fixed harness | Partial: 22/23 discriminated; AC-27 remains open |
| Updated website and wiki describe the cutover | Site build published 985 pages; regenerated wiki catalog and authored workflows committed in `../wiki` at `92aa8cb` | Pass; no push authorized or performed |

### Verification Still Owed

- `audit-config-commit` exceeds its 5-second HTTP deadline. A separate ordinary
  REST commit returned HTTP 200 in 4.47 seconds, but did not exercise the audit observer.
- A-10's historical 43-case/17-failure cohort was not reproduced. The modern
  63-case measurement recorded 34 passes, six failures, eight timeouts, and 15 skips.
- Full functional-suite green is not established. Scoped repaired paths pass;
  initial suite results and residual failures remain in the evidence index.
- Required lint ran and remains red. Constructor/typecheck failures, FreeBSD
  compilation, and formatting were corrected and specifically verified.
  Other lint diagnostics remain open under the pre-release rule.
- FreeBSD publication cross-compiles. No FreeBSD runtime proof was performed.
- Documentation verification retains two unrelated undeclared-symbol anchors.
- Independent verification, critical review, RFC approval/discrimination obligations,
  and closure remain owed. No scope reduction or completion claim is made here.
