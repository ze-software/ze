# Spec: storage-2-blob-artifact

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Depends | storage-1-backend-parity |
| Phase | - |
| Handoff | verify |
| Updated | 2026-09-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The ZeFS blob is the artifact format of the store: one CRC-protected file
that a backup writes, a restore reads in one of two modes, `ze init --from
<source>` fetches and imports, and the offline editor opens. Every writer of
a blob takes a spare-capacity policy, so an artifact carries no padding and a
blob meant for in-place editing carries what the operator asked for (owner
decisions, 2026-09-16, reviewed the same day).**

After `spec-storage-1-backend-parity` the live store is a tree in the
configuration's folder; a blob found there is refused by name and imported
only by an explicit tool. The blob encoding remains in
`internal/component/config/storage` (import reads through it; the conformance
table proves it equal to the tree). What is missing is every operation that
treats a blob as a portable object:

| Need (owner, 2026-09-16) | Today |
|---|---|
| Back up a device's whole store to one file, consistently, while it runs | nothing: `zefs.Export` writes the raw bytes of an OPEN store and has no CLI; `ze data` is registered offline-only (`Mode: modeOffline`) and no daemon command writes a file |
| Restore in two modes: the backup's last config becomes the live config, or the whole store including history is replaced (owner, review answer 4) | storage-1's `ImportBlob` does the whole-store walk into an empty folder only |
| Start a device from a blob published on a web server or given as a path (owner, review answer 6) | the installer downloads `/install/database.zefs` (`downloadToFile`, `internal/install/disk`) for the appliance only; no `ze init` path does it |
| Edit a backup offline, no daemon | `ze config edit` opens the store in the config's folder; the blob backend cannot be named on the command line |
| Choose the spare capacity a blob is written with | `growCapacity` (`pkg/zefs/netcapstring.go`) hard-codes data length plus 10%; `writeFileNoFlush` adds 20 bytes to `file/active/` key slots; `encode` writes the container exact-fit |

Not in this spec: transporting a blob over the managed fleet transport
(`spec-fleet-*` own that path; they can call `restore`), and content
addressing of history (`spec-storage-3-content-addressed-history`).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/zefs-format.md` - format, capacity growth, in-place writes, `Check`/`Repair`
  → Constraint: a netcapstring header carries its own capacity, so a spare policy is a property of the WRITER, not of the file; a blob written exact-fit opens and edits like any other, the first growth of a key just costs a full rewrite
  → Constraint: three capacities exist per entry, key slot, data slot and the container; `writeFileNoFlush` pads `file/active/` key slots by 20 bytes and `growCapacity` pads data by 10%; the container is exact-fit on every rewrite. The policy governs key and data slots; the container stays exact-fit (review answer 3: an add already rewrites today and nothing measured says it hurts)
  → Constraint: `Repair` reads the source's raw bytes and never opens it, so it cannot learn a policy from the source; it takes the same `Spare` option every writer takes
- [ ] `docs/architecture/storage-backends.md` (written by storage-1) - the contract, detection, `Create`, `ImportBlob`
  → Constraint: `ImportBlob(blob, dir)` is the one blob-to-tree walk; `restore full` and `ze init --from` call it; `restore config` does not walk, it commits one config
  → Constraint: `Open` never converts; the blob encoding is opened offline only, through `storage.OpenBlob(file)`
- [ ] `docs/architecture/fleet-config.md` - hub-side client configs, `config-changed` push, the pushed-config inbox
  → Decision: `restore config` on a running daemon takes the path a SIGHUP candidate takes today (`stageSIGHUPCandidate` then `PromoteCandidate` in `cmd/ze/hub/main_reload.go`), so the write observer fires and a hub pushes `config-changed` as it does for an editor commit
- [ ] `docs/architecture/hub-architecture.md`, `docs/architecture/api/commands.md` - the daemon command channel
  → Constraint: the editor commits through its OWN local store handle and sends only `request reload` over SSH (`internal/component/cli/editor_commit.go`); the exec channel answers structured data through `ApplyPipes`, it does not stream a file. A live backup is therefore a daemon-side RPC that writes the file on the daemon's host: `request data backup path <file> [spare <n>]`, and a live config restore is `request data restore path <file> config`. Both are YANG-declared RPCs under `ze:command` like every other `request` verb
- [ ] `docs/guide/command-reference.md`, `docs/guide/operations.md` - `ze data`, `ze init`, `ze config edit`, backup guidance
  → Decision: offline verbs `ze data backup|restore`, daemon verbs `request data backup|restore`, `ze init --from <source>`, and `ze config edit --backup <file>`; the pages gain a "Backup and restore" section
- [ ] `ai/rules/cli.md`, `ai/patterns/cli-command.md` - grammar, offline command structure, map dispatch, owner-registered RPCs
  → Constraint: `ze data` dispatches through `subcommandHandlers` (`internal/component/config/storage/cli/main.go`); a new offline verb is one map entry and one handler file, and `register.go`'s `Subs` string names it. A daemon verb is a `ze:command` RPC in the owner's YANG with a registered handler
  → Constraint: values follow their keyword: `ze data backup <file> spare 0`, `ze data restore <file> config|full [name <source-name>]`, `ze init --from <source> [--sha256 <hex>]`
- [ ] `docs/architecture/appliance/on-device-installer.md`, `internal/install/disk/download_test.go` - `downloadToFile`, `downloadToDiskWithSHA256`
  → Decision: `ze init --from` reuses that helper for `http` and `https`; a local path is read directly; the scheme table is one place, so a later scheme (`tftp`, `ssh`) is one entry. The helper moves to a core leaf package both the installer and `init` import
- [ ] `ai/rules/principles.md` - no silent zero
  → Constraint: `restore` and `ze init --from` verify the whole source blob (`zefs.Check`) before writing one key; a bad source changes nothing and the error names the key
- [ ] `plan/journal/store-serializes-in-process-only.md`
  → Constraint: a blob opened offline is single-process by convention only; `ze config edit --backup` holds the stable sidecar lock `lockStoreFile` already takes (`<artifact>.lock`, `internal/component/config/storage/open.go`) for the session and refuses a second editor. It MUST NOT be a lock on the blob's own fd: `atomicWrite` renames a temp file over the blob and installs a new inode, so that lock dies on the first commit. On a non-unix build the flag is refused naming the platform

**Key insights:** (minimal context to resume after compaction)
- Backup = every key to one exact-fit blob under the store lock; live through `request data backup`, offline through `ze data backup`.
- Restore `config` = the source's active config committed on the device as a new version (live: candidate plus reload; offline: version, pointer, mirror); credentials, identity, state and history untouched. Restore `full` = the tree replaced by `ImportBlob`, offline only.
- `ze init --from <source>` = fetch (`http`, `https`, path), `zefs.Check`, `ImportBlob` into an empty folder. It is `ze init` with the keys coming from the blob instead of prompts.
- Spare is one percentage the writer is opened with; 0 for artifacts, 10 (today's constant) otherwise; it governs key and data slots.
- Offline edit opens the blob through `storage.OpenBlob`, skips the ephemeral daemon and the reload notifier, and locks the file.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `pkg/zefs/netcapstring.go` - `growCapacity` returns `len + len/10`; `EncodeNetcapstring(data, capacity)` exported
- [ ] `pkg/zefs/store.go` - `writeFileNoFlush` picks `growCapacity` on first write and on overflow, `file/active/` keys get +20 bytes of key capacity; `encode` writes the container exact-fit; `flush` takes the in-place path only when nothing was added and the layout is unchanged; `decode` discards the on-disk container capacity; `Export(w)`, `Import(r)`
- [ ] `pkg/zefs/check.go` - `Check`, `Repair(src, dst)` (`os.ReadFile(src)` then `Create(dst)`), `MoveAside`
- [ ] `internal/component/config/storage/cli/main.go` - `subcommandHandlers` map, `extractPathFlag`, `openStore`, `openOrCreateStore`; `register.go` `Subs`, `Mode: modeOffline`; `cmd_integrity.go` `cmdCheck`, `cmdRepair`, `cmdEncode`
- [ ] `internal/component/config/cli/cmd_edit.go` - `cmdEditWithStorage`: `-f` override, session mode, draft auto-load, history wiring; `runEditor`: SSH probe, `startEphemeralDaemon(configPath, ...)` when no daemon answers, `ed.SetReloadNotifier`
- [ ] `internal/component/cli/editor_commit.go` - commit through `e.store.AcquireLock`; SSH used for `request reload` and `run` only
- [ ] `cmd/ze/hub/main_reload.go` - `stageSIGHUPCandidate`, `PromoteCandidate`, `reloadAfterCommit`
- [ ] `internal/component/cli/sshclient/client.go` `ExecCommand`; `answer.go` `ExecCommandStream` (no non-test caller)
- [ ] `internal/component/config/storage/import.go` (storage-1) - `ImportBlob` with equality check and move-aside
- [ ] `internal/install/disk/system.go` `mountInjectDB`, `downloadToFile`, `downloadToDiskWithSHA256`
- [ ] `internal/plugins/init/main.go` - `Run`, `runInit` (the key set), `daemonRunning` (moved to the storage package by storage-1)
- [ ] `internal/plugins/imageserver/handler.go` `buildZefsDB`, `internal/appliance/cmd_assemble.go` `runAssemble` - seed builders; both write credentials, identity and web material, so a seed is a whole store
- [ ] `pkg/zefs/pwrite_other.go` (`//go:build !unix`), `pkg/zefs/store_test.go` `TestBlobStoreNoFlock` (package `zefs`, over a RAW blob: it says `pkg/zefs` writes no sidecar, and says nothing about the storage layer, which takes one)
- [ ] `internal/component/config/storage/open.go` - `lockStoreFile` (openat with `O_NOFOLLOW`, `secureNode`, `unix.Flock` with `LOCK_NB`, `ErrBusy`), `detect` (refuses every live open while `database.zefs` exists), `OpenBlob` and `CreateBlobPopulated` taking the `<artifact>.lock` sidecar
- [ ] `internal/component/config/storage/import.go` - `importOwned`, `finishImport`, `retireSource` (renames the source to `<source>.replaced-<stamp>` in BOTH `ImportBlob` and `ReplaceImportBlob`), `equalImport`, the durable `database.import-intent`, `resumeImport` and `verifyImportIdentity`
- [ ] `cmd/ze/hub/main_reload.go` `handleSIGHUPReload`, `runReloadContext`, and `cmd/ze/hub/config_source.go` `promoteConfigCandidate` - promotion is the LAST step, after the whole acceptance chain, with `clearUncommittedCandidate` and `rollbackReload` on every earlier failure

**Behavior to preserve:** (unless the user explicitly said to change it)
- The blob format byte-for-byte; a blob written with any spare value opens with every existing reader
- Default spare of 10% for every writer this spec does not name, and a variadic option changes no existing call site. The default changes one thing and only one: a `file/active/` key slot is padded by 10% of the key length instead of a flat 20 bytes (AC-16), which moves padding and no content
- `ze data check`, `repair`, `encode`, `import`, `write`, `rm`, `list`, `cat`, `registered` and their exit codes
- `Export`/`Import` on `BlobStore`
- `ze init` interactive, `--seed` and local-path `--from` paths (storage-1); the URL schemes and `--sha256` are an addition
- The seeds `buildZefsDB` and `runAssemble` write; only their padding changes

**Behavior to change:** (only what the user asked for)
- `zefs.Create`, `zefs.Open` and `zefs.Repair` take a `Spare(percent)` option (default 10); `growCapacity` and the `+20` key slot become the policy's
- `ze data backup <file> [spare N]`, `ze data restore <file> config|full` added (offline); `request data backup path <file> [spare N]` and `request data restore path <file> config` added (daemon RPCs)
- `ze init --from <source> [--sha256 <hex>]` added
- `ze config edit --backup <file>` and the offline `ze config show|diff|set|list --backup <file>` open a blob through the storage package
- No artifact kind key: the two restore modes make the source's origin irrelevant (review answer 6, "no opinion", resolved by the modes)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze data backup <file> [spare N]` (`internal/component/config/storage/cli/cmd_backup.go`): offline; refuses while a daemon runs (`daemonRunning`), walks the tree under `AcquireLock`, writes one blob.
- `request data backup path <file> [spare N]` (daemon RPC, owner `internal/component/config/storage`): the daemon walks its own handle under its lock and writes the file on its host.
- `ze data restore <file> config|full` (`cmd_restore.go`): offline; `full` refuses while a daemon runs.
- `request data restore path <file> config` (daemon RPC): the daemon stages the source's active config as a candidate and reloads.
- `ze init --from <source> [--sha256 <hex>]` (`internal/plugins/init/main.go`): fetch, verify, `ImportBlob` into the folder.
- `ze config edit --backup <file>` (`internal/component/config/cli/cmd_edit.go`): opens the blob offline as the editor's `Storage`.
- `zefs.Create(path, zefs.Spare(n))`, `zefs.Open(path, zefs.Spare(n))`, `zefs.Repair(src, dst, zefs.Spare(n))` (`pkg/zefs`): every blob writer.

### Transformation Path
1. Backup: `ListKeys("")` over the whole key space → for each raw key `ReadKey` → `BlobStore.WriteFile` into a new blob opened with the policy (default 0 for this verb) → `Close`. The walk is RAW and RECURSIVE: `Storage.List(prefix)` returns immediate children only and directories are never entries, so `List("file")` over `file/<stamp>/<name>` returns the empty slice (`immediateChildren`, `internal/component/config/storage/store.go`); and the name-based API resolves an unrecognized prefix into `file/active/` (`resolveKey`, `resolveDirKey`), so a key outside `meta/` and `file/` would be silently renamed. A key namespace added later (storage-3's `object/`) travels with no edit to this walk. The walk holds the store's write lock so a commit's version and pointer never straddle the snapshot. Live: the same walk inside the daemon; the RPC answers the path, the key count and the size.
2. Restore `config`: `zefs.Check(source)` → select the source config (R-4) and read its active version (or its `file/active/<name>` mirror when the source has no pointer) → live: the SIGHUP order exactly as `cmd/ze/hub/main_reload.go` runs it, which is candidate FIRST and promotion LAST: refuse when a candidate exists (`stageSIGHUPCandidate` returns `storage.ErrCandidateExists`) → `WriteCandidateVersion` with the source bytes → the whole `runReloadContext` acceptance chain (`load`, `kernelcap.Refuse`, PKI, `ReloadConfig`, provider, AAA, TLS, host tuning) → `promoteConfigCandidate` only after acceptance → `clearUncommittedCandidate` or `rollbackReload` on every earlier failure. Offline: `WriteVersion`, per-name pointers, `file/active/<name>` mirror, under the guard. History gains one version; nothing else changes.
3. Restore `full`: `zefs.Check(source)` → `daemonRunning` refusal → the import path in `internal/component/config/storage/import.go` with the source PRESERVED. `ImportBlob` and `ReplaceImportBlob` both retire their source unconditionally (`importOwned` → `finishImport` → `retireSource` renames it to `<source>.replaced-<stamp>`), so neither serves a restore: an operator's backup must still be there afterwards. `importOwned` takes a keep-source mode, `retireSource` is skipped under it, the intent's `Archive` field is empty in that mode, and `resumeImport` follows. Everything else of the protocol is reused unchanged: the `database.lock` owner lock, the fully copied stage, `equalImport` over the key set and every value, the durable `database.import-intent` written before anything moves, `RenameNoReplace` publication, and `ErrImportPending` recovery. A caller-side two-rename swap is banned: it leaves a crash window `detect` and `missingTree` cannot classify.
4. `ze init --from`: scheme table (`http`, `https` through the download helper with optional SHA-256; anything else a local path) → `zefs.Check` → refuse when the folder already holds a store → `ImportBlob`, which RETIRES its source to `<source>.replaced-<stamp>` as it does today and as `ze init --from`'s own help already states. Init consumes a source once, which is what retiring says; `restore full` repeats over an operator's backup, which is why it takes the keep-source mode instead. The fetched copy and its retired name are both removed by the command. Seeds, backups and a `ze data write`-built blob all import the same way.
5. Offline edit: `storage.OpenBlob(file, Spare(n))` returns the blob-backed `Storage`; the `<artifact>.lock` sidecar held for the session, so the exclusion outlives the inode swap a commit's rewrite performs; the editor, sessions, drafts and history behave as on the tree (conformance table); no ephemeral daemon, no reload notifier; on `Release` the blob flushes with its policy.
6. Spare policy: `writeFileNoFlush` asks the policy for the key and data capacities; the container stays exact-fit.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Terminal command ↔ running daemon | `request data backup|restore` are daemon RPCs over the existing command channel; the file path is on the daemon's host; offline verbs refuse while a daemon runs | No |
| Live tree ↔ blob file | `ImportBlob` in one direction, the backup walk in the other, equality-checked | No |
| Network ↔ `ze init --from` | the download helper with retry and optional SHA-256, shared with the installer | No |
| Publisher ↔ device | out of scope: a fleet spec carries the blob; this spec gives it `restore config` | No |

### Integration Points
- `storage.ImportBlob` (storage-1) - `ze init --from` calls it as it stands; `restore full` calls the same `importOwned` protocol with the source KEPT, because `ImportBlob` and `ReplaceImportBlob` both retire their source; `backup` is its inverse
- `daemonRunning` (moved to the storage package by storage-1) - `backup`, `restore full`, `ze init --from` share it
- `stageSIGHUPCandidate`, `PromoteCandidate`, `reloadAfterCommit` (`cmd/ze/hub/main_reload.go`) - `request data restore ... config` reuses them, so the write observer fires and the managed server pushes `config-changed`
- `downloadToFile` (`internal/install/disk`) - moved to `internal/core/fetch` so `init` and the installer share one helper
- `imageserver.buildZefsDB`, `appliance.runAssemble` - spare 0
- `cmd_edit.cmdEditWithStorage`, `runEditor` - `--backup` replaces the store before the mode decision, skips the daemon probe, the ephemeral daemon and the reload notifier; `-f` and `--backup` together are refused

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every verb reaches the live store through `Storage` or the daemon's RPC; the blob is opened through the storage package's encoding, never through `zefs.Open` in the CLI (the storage-1 detector enforces it); live restore uses the daemon's own candidate path |
| No unintended coupling (components stay isolated) | Yes | the spare policy lives in `pkg/zefs`; the storage package passes it; the fetch helper is a core leaf both `init` and the installer import; no other package names either |
| No duplicated functionality (extends existing, does not recreate) | Yes | `restore full` and `ze init --from` are `ImportBlob`; `restore config` is the SIGHUP candidate path; `check` before every read is `zefs.Check`; the download helper is moved, not copied |
| Zero-copy preserved where applicable (refs, not copies) | Yes | the backup walk reads each key once under the lock; the blob writer takes the slice as today |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | two entries in the owner's own `subcommandHandlers` map; two `ze:command` RPCs in the storage owner's YANG with registered handlers; `from` is a keyword on the init owner's command; the scheme table is in the fetch helper |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | lists searched: `subcommandHandlers` and `Subs` (owner-side, edited by design), the `request` verb tree (RPC registration derives it), `ze init` flag set (owner-side), `ze config edit` flag set (owner-side), `docs/guide/command-reference.md` (page) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Restore has exactly two modes: `config` (the source's last config becomes the live config, history gains one version, nothing else changes) and `full` (the whole store including history is replaced) | owner, review answer 4 | a config-only transfer that also carries PKI material would need a third mode; deferred until asked | `TestRestoreConfigTouchesOnlyConfig`, `TestRestoreFullReplaces` | confirmed by owner |
| A-2 | One percentage governs key and data slots; the container stays exact-fit; no absolute floor. Today's fixed `+20` bytes on `file/active/` key slots is DELETED, not preserved: `writeFileNoFlush` (`pkg/zefs/store.go`) reads `strings.HasPrefix(name, KeyFileActive.Prefix())`, which is one caller's namespace spelled inside a generic store (`ai/rules/principles.md`), and no percentage can reproduce an absolute allowance (23-byte key +20 is 87%, 13-byte key +20 is 154%, every other key 0%) | owner: "an option to set the spare size"; review answer 3 accepted; `ai/rules/no-layering.md` row below, which already requires the literal deleted rather than defaulted around | a floor or a container reserve is a measurement-driven later change | `TestSparePolicyBoundaries`, `TestContainerExactFit`, the two rewritten capacity tests | confirmed by the producer |
| A-3 | The policy is not persisted; every writer, `Repair` included, is told or takes the default | the header is self-describing (`docs/architecture/zefs-format.md`); `Repair` never opens its source | none: an operator restating `spare` on the next write is the whole cost | design gate | unvalidated |
| A-4 | A daemon never serves from a blob after storage-1, so offline edit is the only live use of the blob encoding | storage-1 A-3 | if a daemon could still open a blob, `--backup` editing would need daemon detection | storage-1 closure | unvalidated |
| A-5 | `request data restore ... config` can reuse the SIGHUP candidate path unchanged, with the config bytes coming from the source blob instead of the on-disk file | verified at the producer: `handleSIGHUPReload` stages the candidate, `runReloadContext` runs the whole acceptance chain, and `promoteConfigCandidate` is the LAST step, with `clearUncommittedCandidate` and `rollbackReload` on every earlier failure (`cmd/ze/hub/main_reload.go`, `config_source.go`) | the RPC would write the source config to the on-disk mirror first and send SIGHUP, which is the same path with one extra write | `TestRestoreConfigLive`, `TestRestoreConfigRejectedLeavesPointers` | confirmed by the producer |
| A-6 | `ze init --from` accepts `http`, `https` and a local path in this spec; other schemes are one table entry each | owner: "on a webserver (and other protocols supported)" | a scheme the owner expects now is a one-entry follow-up in the same table | owner confirmation at the gate | unvalidated |
| A-7 | The seed the appliance carries and a `ze data backup` output are the same kind of thing: a whole store; the source's origin never matters to `restore` or `ze init --from` | `buildZefsDB` and `runAssemble` write credentials, identity and web material | none | `TestInitFromSeed`, `TestInitFromBackup` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A backup taken without the lock straddles a commit | `TestBackupUnderLock` writes a version and pointer concurrently and checks the backup holds both or neither | the walk runs under `AcquireLock`; offline the daemon is refused, live it is the daemon's own lock |
| R-2 | `restore full` on a running daemon replaces the tree under it | `daemonRunning` | refused with the daemon's address in the error; `restore config` is the live verb |
| R-3 | `restore config` on a source with no active pointer (a seed, a hand-built blob) | `TestRestoreConfigFromMirror` | the source's `file/active/<name>` mirror is used; a source with neither is refused naming the name |
| R-4 | `restore config` of a source whose config name differs from the device's, or which holds several named configs | `TestRestoreConfigName`, `TestRestoreConfigAmbiguousSource` | source selection and destination naming are separate decisions. SELECTION takes `name <source-name>` when given, else the source's only config name, else the device's name when the source holds it, else refuses listing the source's names. NAMING is always the device's name (`resolve.DefaultConfig` over `meta/instance/name`, defaulting `ze.conf`), and a differing source name is printed |
| R-5 | Spare 0 on a blob later opened for editing makes every GROWING write a full rewrite | `TestSpareZeroGrowthRewrites` | expected and documented; equal-size writes stay in place; `backup <file> spare 10` re-pads on the next backup |
| R-6 | The equality check after `restore full` fails on a half-written tree | the check | the live tree is untouched until the new tree passes; the temp tree is removed |
| R-7 | Two offline editors on one backup file | second `lockStoreFile` returns `ErrBusy` | the second editor is refused naming the file, before and after the first has rewritten it |
| R-8 | `ze init --from https://...` fetches a file that is not a blob, or a truncated one | `zefs.Check` | refused before any write; the fetched copy is removed; with `--sha256 <hex>` the digest is checked before `Check` |
| R-9 | `--backup` starts an ephemeral daemon on the tree in the config's folder | `TestEditBackupNoDaemon` | `--backup` skips the SSH probe, the ephemeral daemon and the reload notifier |
| R-10 | The RPC writes a file path the operator chose on the daemon's host, and an owned absolute path can be inside the store's own folder | `TestBackupRPCPath`, `TestBackupRefusesStorePaths` | ownership is not enough, so the destination is refused by an explicit list as well: `<configdir>/database` and anything beneath it (`treeEncoding.ReadFile` decodes exactly one frame per key file and a blob is two, so a backup written there is a permanently corrupt key), `<configdir>/database.zefs` (`detect` refuses every live open while that name exists, `open.go`), `database.lock` and any `*.lock` sidecar (`lockStoreFile` flocks an INODE, and a staged rename installs a new one, so the daemon's exclusion is silently lost), `database.import-intent`, `database.replaced-*`, and the private stages `database.init-tmp-*` and `database.import-tmp-*`. `force` lifts only "destination file already exists" for an ordinary path, never a row of this list |
| R-11 | A lock taken on the blob's own fd is lost the moment the blob is rewritten | `TestOpenBlobLockSurvivesRewrite` | `atomicWrite` (`pkg/zefs/store.go`) writes a temp file and renames, which installs a NEW inode, so a second editor opens and flocks the replacement while the first holds an unlinked one. The lock is therefore the STABLE SIDECAR the storage package already takes: `lockStoreFile(folder, <artifact>+".lock", mode)` in `internal/component/config/storage/open.go`, as `OpenBlob` and `CreateBlobPopulated` take it today. No new lock pair is written. `TestBlobStoreNoFlock` constrains package `zefs` over a raw blob and says nothing about the storage layer's sidecar; the non-unix arm refuses `--backup` naming the platform |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A bad `restore full` replaces a device's store; the previous tree is beside it as `database.replaced-<stamp>`, one rename from undone. A bad `restore config` is one more version; rollback undoes it. A bad spare policy changes file size, never content |
| How is it reverted? | One commit revert; blobs written with any spare value stay readable |
| Who else touches this path? | storage-1 (this depends on it), storage-3 (a backup carries its objects as keys), `spec-fleet-2-config-templates` and `spec-fleet-5-staged-rollout` (publishers of client configs), `spec-config-at-rest-encryption` (a backup of an encrypted store is a later question), `spec-support-export` (may bundle a backup) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze data backup out.zefs` on a tree, no daemon | → | `cmdBackup` → walk under lock → blob spare 0 | `test/plugin/data-backup.ci` |
| `request data backup path /var/lib/ze/out.zefs` over SSH | → | RPC handler → daemon walk under its lock → file on the host | `test/plugin/data-backup-live.ci` |
| `ze data restore out.zefs full` | → | `cmdRestore` → `zefs.Check` → `ImportBlob` → swap | `test/plugin/data-restore-full.ci` |
| `ze data restore out.zefs config` offline | → | version, pointers, mirror under guard | `test/plugin/data-restore-config.ci` |
| `request data restore path out.zefs config` on a daemon | → | RPC → candidate → promote → reload → observer | `test/plugin/data-restore-config-live.ci` |
| `ze init --from https://host/seed.zefs --sha256 <hex>` | → | fetch → digest → `zefs.Check` → `ImportBlob` | `test/plugin/init-from-url.ci` |
| `ze init --from ./backup.zefs` | → | `zefs.Check` → `ImportBlob` | `test/plugin/init-from-path.ci` |
| `ze config edit --backup out.zefs` | → | `storage.OpenBlob` → editor session, no daemon | `test/editor/edit-backup.et` |
| `zefs.Create(path, Spare(0))` then write | → | policy in `writeFileNoFlush` | `TestSpareZeroExactFit` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze data backup <file>` on a store, no daemon | `<file>` is a valid blob (`ze data check` exit 0), holds every key of the store byte-equal, and every key slot and data slot is exact-fit (capacity equals used); the command prints the path, key count and size, and that the file holds secrets; the file is 0600 |
| AC-2 | `ze data backup <file> spare 25` | every key and data slot's capacity is used plus 25% of used, truncated as today's `growCapacity` truncates (`len + len*25/100`); `spare 101`, a negative or a non-integer is refused naming the range 0 to 100 |
| AC-3 | `ze data backup` while a daemon runs | refused, naming the daemon's SSH address and `request data backup` |
| AC-4 | `request data backup path <abs> [spare N]` on a running daemon | the daemon writes the file under its lock; a commit landing concurrently is in the backup both version and pointer, or neither; the answer carries path, key count and size; a relative path, `..`, a symlink, an existing file without `force`, and every path in R-10's exclusion list (with or without `force`) is refused naming which rule refused it |
| AC-5 | `ze data restore <file> full` with no daemon | the tree holds exactly the source's keys, the previous tree is `database.replaced-<stamp>`, the source is unchanged, exit 0 |
| AC-6 | `ze data restore <file> full` while a daemon runs; or `<file>` fails `zefs.Check` | refused naming the daemon or the failing key; nothing written |
| AC-7 | `ze data restore <file> config` with no daemon, the source holding one config name | the selected source config becomes a new version under the DEVICE's name, the per-name active pointer moves to it, the previous active becomes rollback, `file/active/<name>` is the new bytes; `meta/auth/*`, `meta/ca/*`, identity, runtime state and every other version are byte-unchanged |
| AC-8 | `request data restore path <file> config [name <n>]` on a running daemon | the same result as AC-7 through the candidate and reload path; the daemon serves the new config; a hub with a client named by the key pushes `config-changed`; a source holding no config at all, or `name <n>` naming a config the source lacks, is refused naming what the source does hold |
| AC-9 | `ze init --from <path>` (storage-1's local form) in an empty folder, after the scheme table is added | unchanged, and that includes the retire: the tree holds the source's keys, `ze start` serves the seeded config and credentials, and the source is renamed `<source>.replaced-<stamp>` as `runImport` does today (`internal/plugins/init/main.go`) |
| AC-10 | `ze init --from https://host/x.zefs --sha256 <hex>` | the file is fetched with the shared helper, the digest checked, `zefs.Check` run, then imported; the fetched copy and the retired name the import leaves beside it are both removed; a digest mismatch or a failed check imports nothing and names the reason; without `sha256` the digest step is skipped and the check still runs |
| AC-11 | `ze init --from` beside an existing store, or with an unsupported scheme | refused naming the store found, or the scheme and the supported list |
| AC-12 | `ze config edit --backup <file>` | the editor opens on the blob with session mode, draft and history; no daemon is probed or started; a commit writes a version, pointers and `file/active/*` into the blob; a second `--backup` editor on the same file is refused, and is still refused AFTER the first editor's commit has rewritten the blob (the sidecar outlives the inode swap) |
| AC-13 | `ze config show|diff|set|list --backup <file>` | operate on the blob's `file/active/*` as they do on the tree |
| AC-14 | `--backup` with `-f`; `--backup` on a non-unix build | refused naming both flags; refused naming the platform |
| AC-15 | `zefs.Create(p, Spare(0))`, write a key, close, `zefs.Open(p, Spare(0))`, write the same length | exact-fit, and the equal-length write is in place; growing it by one byte is a full rewrite that leaves the file exact-fit again. The reopen states `Spare(0)` because the policy is NOT persisted (A-3): `zefs.Open(p)` with no option pads at the 10% default, whatever the file was written with, and a test that omits it would measure the default |
| AC-16 | `zefs.Open(p)` or `zefs.Create(p)` with no option | 10% spare on both slot kinds (`len + len/10`, today's `growCapacity` for data) and an exact-fit container. This is today's DATA behavior; the `file/active/` key slot changes from `len + 20` to `len + len/10`, and `TestKeyCapacityFileActive` and `TestKeyCapacityNonFileActive` (`pkg/zefs/store_test.go`) are rewritten to the one rule. Only padding moves: every blob written before or after opens with every reader, and Ze is pre-release so no stored file is owed the old byte count. The cost is that renaming an active config to a longer name rewrites the file, which is what every non-`file/active/` key already does |
| AC-17 | `zefs.Repair(src, dst, Spare(n))` | `dst` is written with `n`; without the option, with 10 |
| AC-18 | `imageserver.buildZefsDB`, `appliance.runAssemble` outputs | exact-fit |
| AC-20 | `ze data restore <file> config` on a source holding two config names, no `name` keyword, and no config matching the device's name | refused before any write, listing both source names and the device's name; with `name <one-of-them>` that config is committed under the device's name and the rename is printed |
| AC-19 | `ze data backup`, `restore`, `request data backup|restore`, `ze init --from` documented | `command-reference.md`, `operations.md` carry them; `./le site build` regenerates the wiki catalog and website pages; the catalog is committed in the wiki checkout |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | backs up a running router before an upgrade | `request data backup` → daemon walk under lock → exact-fit blob on the host | `data-backup-live.ci` |
| 2 | rebuilds a failed box from that backup | `ze init --from backup.zefs` → tree → `ze start` serves the old config and credentials | `init-from-path.ci` |
| 3 | provisions a new box from a blob on the provisioning web server | `ze init --from https://... --sha256 ...` → fetch → import → `ze start` | `init-from-url.ci` |
| 4 | applies yesterday's config from a backup to a live router, keeping everything else | `request data restore ... config` → candidate → promote → reload | `data-restore-config-live.ci` |
| 5 | rolls a stopped router back to a full backup including history | `ze data restore ... full` → tree swap | `data-restore-full.ci` |
| 6 | edits a backup offline to fix a typo before restoring | `ze config edit --backup` → commit into the blob → `restore` | `edit-backup.et`, `data-restore-config.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSpareZeroExactFit`, `TestSparePolicyBoundaries`, `TestContainerExactFit`, `TestSpareZeroGrowthRewrites`, `TestSpareDefaultUnchanged` | `pkg/zefs/store_test.go` | AC-2, AC-15, AC-16, A-2 | |
| `TestRepairTakesSpare` | `pkg/zefs/check_test.go` | AC-17 | |
| `TestBackupCarriesEveryKey`, `TestBackupUnderLock`, `TestBackupExactFit` | `internal/component/config/storage/backup_test.go` | AC-1, AC-4 | |
| `TestRestoreFullReplaces`, `TestRestoreFullRefusesCorrupt`, `TestRestoreConfigTouchesOnlyConfig`, `TestRestoreConfigFromMirror`, `TestRestoreConfigName`, `TestRestoreConfigAmbiguousSource` | `storage/restore_test.go` | AC-5 to AC-8, AC-20, R-3, R-4 | |
| `TestRestoreConfigLive`, `TestBackupRPCPath` | `cmd/ze/hub/data_rpc_test.go` | AC-4, AC-8, R-10 | |
| `TestInitFromSeed`, `TestInitFromBackup`, `TestInitFromRefusesExisting`, `TestInitFromScheme` | `internal/plugins/init/main_test.go` | AC-9, AC-11, A-7 | |
| `TestFetchSHA256`, `TestFetchSchemeTable` | `internal/core/fetch/fetch_test.go` | AC-10 | |
| `TestOpenBlobConformance` (the storage-1 table over an offline blob) | `storage/conformance_test.go` | AC-12 | |
| `TestEditBackupFlagConflicts`, `TestEditBackupNoDaemon`, `TestOpenBlobLock` | `internal/component/config/cli/cmd_edit_test.go`, `storage/open_test.go` | AC-12, AC-14, R-7, R-9 | |
| `TestAssembleExactFit`, `TestImageServerSeedExactFit` | `internal/appliance/cmd_assemble_test.go`, `internal/plugins/imageserver/handler_test.go` | AC-18 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `spare` percent | 0 to 100 | 100 | -1 | 101 |
| entry capacity at spare 0 | equals data length | an empty value has capacity 0 | N/A | N/A |
| `sha256` hex length | exactly 64 | 64 | 63 refused | 65 refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `data-backup`, `data-backup-live`, `data-backup-refused-live` | `test/plugin/*.ci` | AC-1, AC-3, AC-4 | |
| `data-restore-full`, `data-restore-full-refused-live`, `data-restore-config`, `data-restore-config-live` | `test/plugin/*.ci` | AC-5 to AC-8 | |
| `init-from-path`, `init-from-url`, `init-from-refused` | `test/plugin/*.ci` | AC-9 to AC-11 | |
| `edit-backup` | `test/editor/*.et` | AC-12, AC-13 | |

## Files to Modify
- `pkg/zefs/store.go`, `netcapstring.go`, `check.go` - `Spare` option on `Create`, `Open`, `Repair`; `growCapacity` and the key-slot spare as policy methods. Design doc: `docs/architecture/zefs-format.md`
- `internal/component/config/storage/import.go` - `importOwned` takes a keep-source mode for `restore full`; `retireSource` is skipped under it, the intent's `Archive` is empty, and `resumeImport` follows. No new lock file pair is written: the offline editor holds the `<artifact>.lock` sidecar `lockStoreFile` already takes (`open.go`)
- `internal/component/config/storage/cli/main.go`, `register.go` - two map entries, `Subs` names every verb
- `internal/component/config/storage/yang/` and `register.go` - the `request data backup|restore` RPCs and handlers
- `internal/component/config/cli/cmd_edit.go`, `cmd_show.go`, `cmd_diff.go`, `cmd_set.go`, `cmd_list.go` - `--backup <file>`
- `internal/component/config/storage/open.go` - `OpenBlob(file, opts...)` with the lock
- `cmd/ze/hub/main_reload.go` - the candidate path takes bytes from a source, so the RPC and SIGHUP share it
- `internal/plugins/init/main.go` - `--from <source>` gains the `http`/`https` schemes and `--sha256 <hex>` on the existing flag set (storage-1 ships the local-path `--from`); fetch, check, `ImportBlob`
- `internal/install/disk/system.go` - imports the moved fetch helper
- `internal/plugins/imageserver/handler.go`, `internal/appliance/cmd_assemble.go` - spare 0
- `docs/architecture/zefs-format.md`, `docs/architecture/storage-backends.md`, `docs/architecture/api/commands.md`, `docs/architecture/appliance/on-device-installer.md`, `docs/architecture/provisioning/image-server.md`, `docs/guide/command-reference.md`, `docs/guide/operations.md`, `docs/guide/config-editor.md`, `docs/guide/ze-install.md`, `docs/features.md`, `website/**` pages naming `ze init`, `../wiki/command-catalog.md`, `ai/INDEX.md`, `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md`

## Files to Create
- `internal/core/fetch/fetch.go`, `fetch_test.go` - the download helper moved from `internal/install/disk`, with the scheme table and SHA-256 option
- `internal/component/config/storage/backup.go`, `restore.go` and their tests - the walks and the config-mode commit
- `internal/component/config/storage/cli/cmd_backup.go`, `cmd_restore.go` - the offline verbs
- `cmd/ze/hub/data_rpc.go`, `data_rpc_test.go` - the daemon-side handlers
- `test/plugin/data-backup.ci`, `data-backup-live.ci`, `data-backup-refused-live.ci`, `data-restore-full.ci`, `data-restore-full-refused-live.ci`, `data-restore-config.ci`, `data-restore-config-live.ci`, `init-from-path.ci`, `init-from-url.ci`, `init-from-refused.ci`
- `test/editor/edit-backup.et`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/config/storage/yang/`: `request data backup`, `request data restore` as `ze:command` RPCs with `path`, `spare`, `mode`, `force` inputs |
| YANG validation constraints | Yes | `spare` `range "0..100"`, `mode` `enumeration { config }` on the live RPC, `path` `pattern` absolute |
| YANG custom validators | N-A | native constraints suffice |
| CLI commands/flags | Yes | `internal/component/config/storage/cli/` (two verbs, `spare`, `config|full`), `internal/plugins/init/main.go` (`--from` schemes, `--sha256`), `internal/component/config/cli/cmd_edit.go` (`--backup`) |
| CLI grammar (keyword before value) | Yes | `backup <file> spare <n>`; `restore <file> config|full [name <source-name>]`; `request data backup path <p> spare <n>`; `request data restore path <p> config [name <source-name>]`; `ze init` stays in the offline `--flag` register (`--from`, `--sha256`) beside `--force` and `--seed` (`ai/rules/cli.md`) |
| Editor autocomplete | Yes | the RPCs complete like every `request` verb from their YANG |
| Functional test for new RPC/API | Yes | `data-backup-live.ci`, `data-restore-config-live.ci` |
| Pipe completeness | Yes | the RPC answers are structured (path, keys, bytes) so `\| json`, `\| yaml`, `\| table` render them |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no new runtime dependency; a backup file is an operator artifact; network fetch is one-shot at init |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: backup, restore modes, `ze init --from`, offline edit |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: two `ze data` verbs, two `request data` RPCs, `spare`, `ze init --from`, `--backup` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: `request data backup|restore` |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/operations.md` (backup and restore section, the two modes), `docs/guide/config-editor.md` (`--backup`), `docs/guide/ze-install.md` (`ze init --from`) |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | not protocol |
| 10 | Test infrastructure changed? | No | existing `.ci`/`.et` forms |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it carries a backup/restore row; verified by grep at implementation |
| 12 | Internal architecture changed? | Yes | `docs/architecture/zefs-format.md` (spare policy), `docs/architecture/storage-backends.md` (artifact roles, the two restore modes, `ze init --from`), `docs/architecture/provisioning/image-server.md` (the seed is exact-fit and importable by `ze init --from`) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` if it lists `ze data` verbs or `request` RPCs |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation: `./le spec citation anchors spec plan/pre-release/spec-storage-2-blob-artifact.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ze data` and `ze init` examples in `command-reference.md`, `operations.md`, `ze-install.md` and the website guides; `./le site build` run and the wiki catalog committed (AC-19) |

Design documents declared by the `// Design:` headers of files in scope:

| Page | Affected? | Reason |
|------|-----------|--------|
| `docs/architecture/zefs-format.md` | Yes | spare policy (Phase 2) |
| `docs/architecture/storage-backends.md` | Yes | artifact roles, restore modes, `ze init --from` (Phase 3) |
| `docs/architecture/config/syntax.md` | No | the offline CLI files declare it for the parser; the parser is untouched |
| `docs/architecture/appliance/on-device-installer.md` | Yes | the download helper moves to `internal/core/fetch`; the page's source anchor follows it (Phase 4) |
| `docs/architecture/provisioning/image-server.md` | Yes | the `/install/database.zefs` it describes is exact-fit and is what `ze init --from` imports (Phase 4) |
| `docs/architecture/system-architecture.md` | No | declared by `internal/plugins/init/main.go`; `from` changes no component boundary the page draws |
| `docs/architecture/hub-architecture.md` | Yes | the daemon gains two `request data` RPCs beside `request reload` (Phase 3) |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- verbs, RPCs, keyword and flag exist and fail
   - Tests: `data-backup.ci`, `data-backup-live.ci`, `init-from-path.ci`, `edit-backup.et`, `TestSpareZeroExactFit`
   - Files: `cli/main.go` (two entries), `cmd_backup.go`, `cmd_restore.go` (stubs), the RPC YANG and handler stubs, `init/main.go` (`from` parsed), `cmd_edit.go` (`--backup` parsed), `pkg/zefs/store.go` (`Spare` option accepted, ignored)
   - Verify: the commands dispatch; the tests fail on behavior
2. **Phase: Spare policy** -- `pkg/zefs` policy on key and data slots, `Repair` takes it, seeds exact-fit
   - Tests: the `pkg/zefs` rows of the plan, `TestAssembleExactFit`, `TestImageServerSeedExactFit`
   - Files: `store.go`, `netcapstring.go`, `check.go`, `imageserver/handler.go`, `cmd_assemble.go`, `docs/architecture/zefs-format.md`, `docs/architecture/provisioning/image-server.md`
   - Verify: AC-2, AC-15 to AC-18; every existing blob test unchanged
3. **Phase: Backup and restore** -- the walks, the two modes, the daemon RPCs
   - Tests: `backup_test.go`, `restore_test.go`, `data_rpc_test.go`, `data-backup*.ci`, `data-restore*.ci`
   - Files: `backup.go`, `restore.go`, `data_rpc.go`, `main_reload.go`, the YANG, `docs/architecture/storage-backends.md`, `docs/architecture/api/commands.md`, `docs/architecture/hub-architecture.md`, `docs/guide/operations.md`
   - Verify: AC-1, AC-3 to AC-8
4. **Phase: `ze init --from`** -- fetch helper moved, scheme table, import
   - Tests: `TestFetchSHA256`, `TestFetchSchemeTable`, `TestInitFrom*`, `init-from-*.ci`
   - Files: `internal/core/fetch/`, `install/disk/system.go`, `init/main.go`, `docs/guide/ze-install.md`, `docs/architecture/appliance/on-device-installer.md`
   - Verify: AC-9 to AC-11
5. **Phase: Offline edit** -- `OpenBlob` with lock, `--backup` on the editor and the offline family
   - Tests: `TestOpenBlobConformance`, `TestOpenBlobLock`, `TestEditBackupFlagConflicts`, `TestEditBackupNoDaemon`, `edit-backup.et`
   - Files: `open.go`, the lock pair, `cmd_edit.go`, `cmd_show.go`, `cmd_diff.go`, `cmd_set.go`, `cmd_list.go`, `docs/guide/config-editor.md`, `docs/guide/command-reference.md`, `docs/features.md`
   - Verify: AC-12 to AC-14
6. **Phase: Published surfaces** -- website pages, `./le site build`, wiki catalog
   - Verify: AC-19

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | `restore full` never touches the live tree before the new tree passes equality; `restore config` changes exactly one version, three pointer keys and one mirror; `zefs.Check` runs before any write; spare 0 produces capacity equal to used for every key and data slot; the live backup runs under the daemon's lock |
| Naming | verbs `backup`, `restore`; modes `config`, `full`; keywords `spare`, `path`, `force`; `ze init` flags `--from`, `--sha256` |
| Data flow | the CLI never calls `zefs.Open` on a live store; the offline blob is reached through `storage.OpenBlob`; the live verbs are daemon RPCs, not local handles; the backup and restore walks use raw-key methods, never the resolving name API |
| Rule: `ai/rules/no-layering.md` | `growCapacity`'s constant and the `+20` literal are deleted, not defaulted around; one policy type |
| Rule: `ai/rules/principles.md` | a source with no config for the device's name is named in the refusal; a fetch that is not a blob imports nothing |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Two offline verbs and two RPCs registered | `ze data` usage lists `backup restore`; `ze help request data` lists both RPCs |
| Exact-fit artifacts | `TestBackupExactFit`, `TestAssembleExactFit` |
| Live backup consistent | `TestBackupUnderLock` through the RPC |
| Offline blob equals tree behavior | `TestOpenBlobConformance` passes every row |
| Pages, site and wiki updated | `./le doc wiring` green; `./le site build` run; wiki commit SHA recorded |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | every key in a restored or imported blob is validated as `fs.ValidPath` before it becomes a tree path; the RPC `path` is absolute, no `..`, no symlink, owned by the daemon's user |
| Secrets in artifacts | a backup holds credentials, the CA key and web key; the file is created 0600 and the command prints that it contains secrets, as `cmd_assemble` does today |
| Restore overreach | `restore config` touches config keys only, by test; `restore full` is offline and moves the old tree aside rather than deleting it |
| Untrusted blob | `zefs.Check` and the size and entry-count limits `Import` enforces (`maxImportSize`, `maxEntryCount`) apply to every restore and `ze init --from` input; the optional SHA-256 pins the fetched bytes; the fetch helper follows no redirect to another host |
| Authorization | `request data backup|restore` are `request` verbs and take the same authorizer as `request reload` |

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

- A blob's spare is a writer's choice the file records in every header, so "exact-fit for artifacts, padded for editing" needs no format change and no flag day.
- Two restore modes remove the need for an artifact kind: the operator says what to take from the source, so where the source came from stops mattering.
- The first draft's "backup through the daemon over SSH like a commit" described a path that does not exist; the editor commits locally. Live operations on the store are daemon RPCs, and saying so early is what kept the offline verbs offline.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Two restore modes on one verb (owner, review answer 4) | `apply` and `restore` as separate verbs with an artifact kind key | the mode names what the operator takes; the kind guessed it from the file and would have refused a seed |
| Live backup and config restore as daemon RPCs | offline only, or a local walk over the daemon's tree | a local walk cannot take the daemon's lock; the RPC is the only consistent snapshot and the only path that fires the observer |
| `restore full` offline only | restore through the daemon with a restart | a whole-store swap under a running daemon has no consistent point; the daemon holds handles into the old tree |
| `ze init --from` reuses the installer's fetch helper (owner, review answer 6) | a new HTTP client in `init` | one helper, one retry policy, one SHA-256 option, one scheme table |
| One percentage for key and data slots; container exact-fit; no floor | a container reserve with an in-place append path | review answer 3: an add already rewrites today; a reserve needs a stored container capacity and a new append path nothing measured asks for |
| Policy not persisted | `meta/store/spare` | the header carries capacity per entry; the policy only matters at the next write, and the writer states it |
| `--backup <file>` on `ze config *` | reuse `--path` | `--path` on the config family reads as the config file's path; `--backup` names what the flag opens |
| `restore full` extends `importOwned` with a keep-source mode, `ze init --from` keeps retiring | one mode for both callers; or a caller-side stage, two renames and a swap | the import already owns the ownership lock, the stage, the `equalImport` comparison, the durable intent and the `ErrImportPending` recovery; a caller-side swap leaves a crash window `detect` and `missingTree` cannot classify |
| The offline editor's lock is the existing `<artifact>.lock` sidecar | an `flock` on the blob's own fd | `atomicWrite` installs a new inode on every rewrite, so an fd lock is silently lost on the first commit and a second editor gets in |
| One spare percentage on both slot kinds, the `file/active/` +20 deleted | keep the absolute key allowance beside the percentage | the allowance is one caller's namespace spelled inside a generic store, no percentage reproduces it, and the spec's own no-layering row already requires the literal deleted rather than defaulted around |
| The backup walk is `ListKeys` over raw keys | `List` over `meta` and `file` | `List` returns immediate children and never a directory, so it enumerates nothing under `file/<stamp>/`; and a name outside `meta/` and `file/` is rewritten into `file/active/` by the resolver |

## Known Limitations

- Transport of a blob to a device over the managed fleet is a fleet spec's work; this spec provides `restore config` and `ze init --from`.
- Schemes beyond `http`, `https` and a local path are one table entry each, added when asked.
- No absolute spare floor and no container reserve (A-2).
- A backup of an encrypted store is decided by `spec-config-at-rest-encryption`.
- No signature: authenticity of a fetched blob comes from TLS and the optional SHA-256 the operator supplies; the CRC detects corruption only.

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
