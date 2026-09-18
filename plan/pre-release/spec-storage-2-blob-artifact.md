# Spec: storage-2-blob-artifact

| Field | Value |
|-------|-------|
| Status | design |
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

The initial-release runtime target for this spec is Linux (owner, 2026-09-18: "we only target Linux", clarified as "at least for the initial release"). Publication uses the existing Linux `renameat2(RENAME_NOREPLACE)` implementation. Other runtime platforms are neither promised nor removed by this work, and this spec defines no portability roadmap. Development can use another host OS, but publication proof must run on Linux.

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
  → Constraint: `importOwned` is the shared blob-to-tree walk behind `ImportBlob`; this spec extends its intent and replay protocol for source-preserving full restore while init keeps source retirement. `restore config` commits one config without that walk
  → Constraint: `Open` never converts; the blob encoding is opened offline only, through `storage.OpenBlob(file)`
- [ ] `docs/architecture/fleet-config.md` - hub-side client configs, `config-changed` push, the pushed-config inbox
  → Decision: `restore config` on a running daemon takes the path a SIGHUP candidate takes today, which is `stageSIGHUPCandidate`, then the whole `runReloadContext` acceptance chain, then `promoteConfigCandidate` LAST (`cmd/ze/hub/main_reload.go`, `config_source.go`). The write observer fires and a hub pushes `config-changed` as it does for an editor commit. Promotion never precedes acceptance, here or anywhere this spec describes
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
  → Constraint: a blob opened offline is single-process by convention only; `ze config edit --backup` holds the stable sidecar lock `lockStoreFile` already takes (`<artifact>.lock`, `internal/component/config/storage/open.go`) for the session and refuses a second editor. It MUST NOT be a lock on the blob's own fd: `atomicWrite` renames a temp file over the blob and installs a new inode, so that lock dies on the first commit

**Key insights:** (minimal context to resume after compaction)
- Backup = every key to one exact-fit blob under the store lock; live through `request data backup`, offline through `ze data backup`.
- Restore `config` = the source's active config committed on the device as a new version (live: candidate plus reload; offline: version, pointer, mirror); credentials, identity, state and history untouched. Restore `full` = offline tree replacement through the shared import protocol with keep-source policy and recorded old/new identities.
- `ze init --from <source>` = fetch (`http`, `https`, path), verify, import and retire the source. Pending imports replay from their durable intent, including after source retirement.
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
- `WriteGuard` gains `ReadKey(key)` and `ListKeys(prefix)` on both encodings, so a guarded walk has a raw route; the eight existing methods are unchanged
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
1. Backup: take the store's write guard, then `guard.ListKeys("")` over the whole key space → for each raw key `guard.ReadKey` → `BlobStore.WriteFile` into a new blob opened with the policy (default 0 for this verb) → `Close` → `Release`. The walk uses the guard throughout: `store.acquire` holds `s.mu` until `Release`, so calling `Storage.ListKeys` or `ReadKey` inside it deadlocks. This spec adds the guard's raw `ReadKey` and `ListKeys` on both encodings, each reaching the encoding without locking again. Existing `guard.List` resolves its prefix through `resolveDirKey`, which trims a trailing slash, then filters with `immediateChildren`; it returns immediate file children, never directories. It therefore cannot enumerate nested history or arbitrary namespaces. The guarded raw method follows `store.ListKeys`: recursive enumeration, literal-prefix filtering, and sorted results, including partial prefixes and the empty prefix. Under the blob guard, enumeration uses the held `blobLock.List("")` before filtering and sorting; calling the public `BlobStore.List` would acquire the blob mutex again. Storage-3 adds `WriteKey` and `RemoveKey` when its object sweep needs them. A later namespace such as `object/` travels with no change to the backup walk. The held write lock keeps a commit's version and pointer in the same snapshot. Live backup uses this walk inside the daemon and answers with the path, key count and size.
2. Restore `config`: `zefs.Check(source)` → select the source config (R-4) and read its active version (or its `file/active/<name>` mirror when the source has no pointer) → live: the SIGHUP order exactly as `cmd/ze/hub/main_reload.go` runs it, which is candidate FIRST and promotion LAST: refuse when a candidate exists (`stageSIGHUPCandidate` returns `storage.ErrCandidateExists`) → `WriteCandidateVersion` with the source bytes → the whole `runReloadContext` acceptance chain (`load`, `kernelcap.Refuse`, PKI, `ReloadConfig`, provider, AAA, TLS, host tuning) → `promoteConfigCandidate` only after acceptance → `clearUncommittedCandidate` or `rollbackReload` on every earlier failure. Offline: `WriteVersion`, per-name pointers, `file/active/<name>` mirror, under the guard. History gains one version; nothing else changes.
3. Restore `full`: resolve the source to an absolute path and refuse `<configdir>/database.zefs` before the lock and before any write → `zefs.Check(source)` → `daemonRunning` refusal → `importOwned` with the durable `keep-source` policy and replacement permission. The source and its sidecar are never renamed. Both current public import entry points retire the source, so restore needs the policy in the shared import protocol. The restartable protocol below extends the existing intent with previous-destination identities and fixed retirement names. It reuses `database.lock`, stage copying, full key-and-value equality and no-replace publication.
   Every recovery command comes from the validated intent policy: `ze data restore <source> full` for `keep-source`, `ze init --from <source>` for `retire-source`. Source, policy or destination conflicts refuse before mutation and name the recorded command and conflicting path. An unreadable intent or invalid policy cannot supply a safe command; the error names the intent and field and asks for the original operation, without guessing init. This covers `detect`, `missingTree`, source/digest errors, `verifyImportIdentity` and `resumeImport`.
   Keeping the source at `<configdir>/database.zefs` would leave a canonical blob beside the tree and prevent normal startup, so AC-22 remains an unconditional restore refusal. An unrelated canonical seed already at the destination is a separate previous destination and follows the recorded move-aside protocol below.
4. `ze init --from`: scheme table (`http`, `https` through the download helper with optional SHA-256; anything else a local path) → `ImportBlob` with `retire-source`, which runs `zefs.Check` on the selected original or archived source. A matching pending intent resumes before the ordinary existing-store refusal; a new import still refuses an existing store unless the existing force-replacement path was selected. The command must not reject a missing original source before the importer can select its recorded archive. Init keeps its source-retirement and replay behavior, including completion after the source was retired and the published tree later changed. The fetched copy and its retired name are both removed by the command. Seeds, backups and a `ze data write`-built blob all import the same way.
5. Offline edit: `storage.OpenBlob(file, Spare(n))` returns the blob-backed `Storage`; the `<artifact>.lock` sidecar held for the session, so the exclusion outlives the inode swap a commit's rewrite performs; the editor, sessions, drafts and history behave as on the tree (conformance table); no ephemeral daemon, no reload notifier; on `Release` the blob flushes with its policy.
6. Spare policy: `writeFileNoFlush` asks the policy for the key and data capacities; the container stays exact-fit.

### Restartable Full Import

The intent is one immutable record written before any destination moves. Progress is derived from the recorded identities at their original and retirement names; there is no persisted phase counter. `importOwned` currently records only the new stage's identity, and `moveAside` chooses a name when it moves a node. This protocol records the previous destinations and their chosen names first, so replay can distinguish the old tree from the new tree without treating an unrelated replacement as either.

| Durable field | Type | Contract |
|---|---|---|
| Source policy | Closed enum | Required `keep-source` or `retire-source`; missing and unknown values refuse. The requested command's policy must match. |
| Source and digest | Absolute path and SHA-256 | Retain the existing source binding and digest verification. Replay uses the recorded source. |
| Source archive | Absolute path or absent | Required sibling `<source>.replaced-<stamp>` for `retire-source`; forbidden for `keep-source`. Gate both current archive-path checks on the policy. |
| Stage and new-tree identity | Stage basename, device and inode | Retain the validated `database.import-tmp-*` name and record its identity after copying, equality verification and sync. This identity follows the stage to `database`. |
| Previous tree | Required absent/present descriptor | Present records the old `database` device/inode, directory type, and exact `database.replaced-<stamp>` basename. Absent explicitly records that no tree existed. |
| Canonical seed | Required absent/source/previous descriptor | `source` is legal only for a retiring import of `<configdir>/database.zefs`; it is retired through the source archive. `previous` records an unrelated `database.zefs` regular-file device/inode and exact `database.zefs.replaced-<stamp>` basename. `absent` records no seed. Restore can use `absent` or `previous` only. |

All retirement names are chosen once and checked absent before intent publication. Parsing validates each descriptor, identity and basename, including prefix, type and separation from the stage, source and other reserved paths. Missing descriptors refuse rather than imply absence. Previous destinations are recorded only after the existing replacement permission checks; replay of that validated intent authorizes only those recorded moves, so a forced init can finish with the recovery command without granting permission to replace a different tree.

1. Under the existing destination owner lock and source sidecar lock, read and validate any existing intent, select the original or archived source according to policy and actual retirement state, and verify that source before the usual existing-tree or unrelated-seed refusal. A matching intent enters replay directly; it never enters stale-stage cleanup or builds another stage.
2. For a new operation, build and compare the complete stage. Sync all frame files and stage directories, then the containing directory that holds the stage name. Capture the new-tree identity and the previous tree/seed descriptors. Atomically write and fsync the intent, including its containing directory, before any destination rename. Once intent publication has been attempted, an error must retain the stage if the intent can be present. The current unconditional unpublished-stage cleanup must not delete a recorded stage.
3. Classify every recorded location before any replay mutation. For each `present` previous destination, exactly one location must hold its recorded identity: its original name with its retirement name absent, or its retirement name with its original name absent. At `database`, the new-tree identity is also allowed after publication. An absent descriptor requires the original name absent until its specified new occupant is published. Validate existing nodes through the no-follow, type and ownership checks; byte equality alone never establishes identity.
   The complete set of locations must match a permitted table state before any move. In particular, a published new tree with an unretired previous seed refuses before that seed can move; an already-retired source with an unpublished tree also refuses. Completed-init detection uses the same classification, including the absent stage name and explicit absent descriptors.
4. Verify the source digest and compare every key and value of the stage or published new tree before proceeding, except for the already-retired init case below. Move each previous destination still at its original name, first the tree, then any unrelated canonical seed, with `RenameNoReplace` to its recorded retirement name; skip already-retired destinations. Sync the containing directory after each move and before starting the next effect. Recheck the identities at the held-directory entries before movement. A failed move or sync returns an error and keeps the intent and stage.
5. Publish the recorded stage as `database` with `RenameNoReplace`, then sync the containing directory. Replay that finds the new identity at `database` resumes completion without republishing, provided the stage name is absent and every previous destination is already at its recorded retirement name. Any other combination refuses. An existing tree with another identity is never overwritten or moved on the basis of an intent.
6. In `keep-source`, require the original source and the equality check from step 4, and leave the source and its sidecar at their original names; a missing source is a hard error with no archive fallback. In `retire-source`, use actual source location to derive the separate `retired` argument: original source present and archive absent means false; original absent and the verified archive present means true. Policy alone never sets `retired`. Both names present, neither present, or a digest mismatch refuses. A false value requires equality and then source retirement; a true value requires published new-tree identity but preserves later changes to that tree, as `resumeImport` does today. Derive and validate this state before any replay mutation.
7. For retiring imports, finish the existing source rename and sidecar move, including the already-renamed-sidecar replay arm, and sync the source directory before removing the intent. For both policies, remove the intent only after all preceding effects are durable, then sync the destination directory. A crash before that final sync can leave the intent visible again; repeating completion remains safe.

`detect` checks for an intent even beside an existing tree or canonical seed, before its ordinary blob/tree decision. `Open`, `OpenReadOnly`, `Create` and `CreatePopulated` report `ErrImportPending` for every valid unfinished state below and for unreadable or invalid intents. They do not move anything. The existing completed-init exception remains: when the new tree has the recorded identity, all previous destinations are at their recorded retirement locations, and the original source is absent with its digest-matching archive present, the retiring import has completed publication and source retirement. Ordinary open can use that tree while the remaining sidecar/intent cleanup waits for explicit replay; this preserves `retired-then-changed`. Keep-source has no such exception while its intent exists. Writer opens recheck the classification under `database.lock` before returning a handle.

| Observed state with a valid intent | Recovery effect |
|---|---|
| Old tree still at `database`; verified new stage present; each previous retirement name absent | Report pending even though the old tree exists. Replay moves only the recorded old tree and seed, publishes the same stage, then completes according to policy. |
| `database` absent, either from the outset or after its recorded old-tree move; stage present; unrelated seed still at its original name | Verify the old tree at its recorded retirement name when one existed, move the recorded seed, then publish. This includes a destination that held only an unrelated canonical seed. No new timestamp or replacement stage is chosen. |
| `database` absent; stage present; all previous destinations retired, or explicitly absent from the outset | Replay publishes the verified stage and completes. This also covers an initially empty destination. |
| New-tree identity at `database`; stage name absent; all previous destinations retired or initially absent; source still at its original name | Verify equality, then keep the source or retire it according to policy. A keep-source open continues to report pending until intent removal. |
| New-tree identity at `database`; stage name absent; all previous destinations retired or initially absent; retiring source at its archive | Skip tree/source equality, finish any source-sidecar move, then remove the intent. Preserve published-tree writes made after source retirement. |
| Intent absent after completion | Normal detection resumes. The old tree and unrelated seed remain at their recorded retirement names; keep-source leaves the backup unchanged. |
| Wrong identity/type at tree, stage, seed or retirement name; both old locations present or absent; missing stage without the new tree; changed source; malformed intent; command policy/source mismatch | Refuse without moving or deleting any recorded node. Name the conflict and intent. An unrelated replacement stays untouched, even if its bytes equal the source; removing the intent is an explicit operator decision after selecting the wanted tree. |

The Linux implementation of `zefs.RenameNoReplace` (`pkg/zefs/check_tree_linux.go`) calls `renameat2` with `RENAME_NOREPLACE` for files and directories. Each rename either preserves the original name or installs the same inode at its destination, and an existing destination refuses atomically. The table also covers crashes between that rename and its directory sync: replay classifies the names that survived and syncs the recognized state before the next effect. Unsupported operations return errors with the intent and recorded nodes retained; no overwriting rename or multi-call publication fallback is added.

An unrelated canonical seed is never mistaken for the source, and an init source at the canonical seed name is never recorded or moved as a previous seed. Existing source-sidecar replay remains: after its rename, a resumed import can hold a fresh original-name sidecar and remove that fresh lock once the archive already has its sidecar.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Terminal command ↔ running daemon | `request data backup|restore` are daemon RPCs over the existing command channel; the file path is on the daemon's host; offline verbs refuse while a daemon runs | No |
| Live tree ↔ blob file | `ImportBlob` in one direction, the backup walk in the other, equality-checked | No |
| Network ↔ `ze init --from` | the download helper with retry and optional SHA-256, shared with the installer | No |
| Publisher ↔ device | out of scope: a fleet spec carries the blob; this spec gives it `restore config` | No |

### Integration Points
- `storage.ImportBlob` (storage-1) - `ze init --from` retains source retirement while sharing the extended restartable protocol; `restore full` uses its keep-source policy and recorded previous-destination moves; `backup` is its inverse
- `daemonRunning` (moved to the storage package by storage-1) - `backup`, `restore full`, `ze init --from` share it
- `handleSIGHUPReload`'s sequence (`cmd/ze/hub/main_reload.go`): `stageSIGHUPCandidate`, `runReloadContext`, then `promoteConfigCandidate` (`config_source.go`), with `clearUncommittedCandidate` and `rollbackReload` on the failure arms - `request data restore ... config` reuses the whole sequence in that order, so the write observer fires and the managed server pushes `config-changed`
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
| R-12 | A restore crashes after intent publication, before or after moving either previous destination, or after publishing the new tree | `TestRestoreFullResumeKeepsSource`, `TestRestoreFullUnrelatedReplacementRefused` on Linux | the immutable intent binds policy, stage and previous destinations to identities and fixed retirement names; Linux atomic no-replace renames leave classifiable states. Pending detection includes an existing tree, and replay of `ze data restore <source> full` completes recognized states with the backup unchanged; an unrelated replacement refuses |
| R-13 | The operator names `<configdir>/database.zefs` as the restore source | `TestRestoreFullRefusesLiveBlobName` | refused before the lock and before any write, naming the collision and the two exits: restore from a copy held elsewhere, or use `ze init --from`, which consumes the source |
| R-11 | A lock taken on the blob's own fd is lost when the blob is rewritten | `TestOpenBlobLockSurvivesRewrite` | `atomicWrite` in `pkg/zefs/store.go` installs a new inode, so the editor retains the existing `<artifact>.lock` sidecar through `lockStoreFile`. No new lock pair is written. `TestBlobStoreNoFlock` constrains raw `pkg/zefs` artifacts and says nothing about the storage layer's sidecar |

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
| `ze data restore out.zefs full` | → | `cmdRestore` → `zefs.Check` → keep-source import protocol → recorded moves and publication | `test/plugin/data-restore-full.ci` |
| `ze data restore out.zefs config` offline | → | version, pointers, mirror under guard | `test/plugin/data-restore-config.ci` |
| `request data restore path out.zefs config` on a daemon | → | RPC → candidate → reload accepted → promote → observer | `test/plugin/data-restore-config-live.ci` |
| `ze init --from https://host/seed.zefs --sha256 <hex>` | → | fetch → digest → `zefs.Check` → `ImportBlob` | `test/plugin/init-from-url.ci` |
| `ze init --from ./backup.zefs` | → | `zefs.Check` → `ImportBlob` | `test/plugin/init-from-path.ci` |
| `ze config edit --backup out.zefs` | → | `storage.OpenBlob` → editor session, no daemon | `test/editor/edit-backup.et` |
| `zefs.Create(path, Spare(0))` then write | → | policy in `writeFileNoFlush` | `TestSpareZeroExactFit` |
| a guarded backup walk on both encodings | → | `guard.ListKeys` → `guard.ReadKey` | `TestGuardListKeysRecursive` |
| pending full restore before old-tree movement, during absence, or after publication | → | pending detection → printed restore command → identity-checked replay | `test/plugin/data-restore-full-resume.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze data backup <file>` on a store, no daemon | `<file>` is a valid blob (`ze data check` exit 0), holds every key of the store byte-equal, and every key slot and data slot is exact-fit (capacity equals used); the command prints the path, key count and size, and that the file holds secrets; the file is 0600 |
| AC-2 | `ze data backup <file> spare 25` | every key and data slot's capacity is used plus 25% of used, truncated as today's `growCapacity` truncates (`len + len*25/100`); `spare 101`, a negative or a non-integer is refused naming the range 0 to 100 |
| AC-3 | `ze data backup` while a daemon runs | refused, naming the daemon's SSH address and `request data backup` |
| AC-4 | `request data backup path <abs> [spare N]` on a running daemon | the daemon writes the file under its lock; a commit landing concurrently is in the backup both version and pointer, or neither; the answer carries path, key count and size; a relative path, `..`, a symlink, an existing file without `force`, and every path in R-10's exclusion list (with or without `force`) is refused naming which rule refused it |
| AC-5 | `ze data restore <file> full` with no daemon | the tree holds exactly the source's keys, any previous tree is at its recorded `database.replaced-<stamp>` name, any unrelated canonical seed is at its recorded `database.zefs.replaced-<stamp>` name, the source is unchanged, exit 0 |
| AC-6 | `ze data restore <file> full` while a daemon runs; or `<file>` fails `zefs.Check` | refused naming the daemon or the failing key; nothing written |
| AC-7 | `ze data restore <file> config` with no daemon, the source holding one config name | the selected source config becomes a new version under the DEVICE's name, the per-name active pointer moves to it, the previous active becomes rollback, `file/active/<name>` is the new bytes; `meta/auth/*`, `meta/ca/*`, identity, runtime state and every other version are byte-unchanged |
| AC-8 | `request data restore path <file> config [name <n>]` on a running daemon | the same result as AC-7 through the candidate and reload path; the daemon serves the new config; a hub with a client named by the key pushes `config-changed`; a source holding no config at all, or `name <n>` naming a config the source lacks, is refused naming what the source does hold |
| AC-9 | `ze init --from <path>` (storage-1's local form) in an empty folder, after the scheme table is added | unchanged, and that includes the retire: the tree holds the source's keys, `ze start` serves the seeded config and credentials, and the source is renamed `<source>.replaced-<stamp>` as `runImport` does today (`internal/plugins/init/main.go`) |
| AC-10 | `ze init --from https://host/x.zefs --sha256 <hex>` | the file is fetched with the shared helper, the digest checked, `zefs.Check` run, then imported; the fetched copy and the retired name the import leaves beside it are both removed; a digest mismatch or a failed check imports nothing and names the reason; without `sha256` the digest step is skipped and the check still runs |
| AC-11 | a new `ze init --from` beside an existing store without force, or with an unsupported scheme | refused naming the store found, or the scheme and the supported list; a matching pending intent follows the recovery protocol before the existing-store refusal |
| AC-12 | `ze config edit --backup <file>` | the editor opens on the blob with session mode, draft and history; no daemon is probed or started; a commit writes a version, pointers and `file/active/*` into the blob; a second `--backup` editor on the same file is refused, and is still refused AFTER the first editor's commit has rewritten the blob (the sidecar outlives the inode swap) |
| AC-13 | `ze config show|diff|set|list --backup <file>` | operate on the blob's `file/active/*` as they do on the tree |
| AC-14 | `--backup` with `-f` | refused naming both flags |
| AC-15 | `zefs.Create(p, Spare(0))`, write a key, close, `zefs.Open(p, Spare(0))`, write the same length | exact-fit, and the equal-length write is in place; growing it by one byte is a full rewrite that leaves the file exact-fit again. The reopen states `Spare(0)` because the policy is NOT persisted (A-3): `zefs.Open(p)` with no option pads at the 10% default, whatever the file was written with, and a test that omits it would measure the default |
| AC-16 | `zefs.Open(p)` or `zefs.Create(p)` with no option | 10% spare on both slot kinds (`len + len/10`, today's `growCapacity` for data) and an exact-fit container. This is today's DATA behavior; the `file/active/` key slot changes from `len + 20` to `len + len/10`, and `TestKeyCapacityFileActive` and `TestKeyCapacityNonFileActive` (`pkg/zefs/store_test.go`) are rewritten to the one rule. Only padding moves: every blob written before or after opens with every reader, and Ze is pre-release so no stored file is owed the old byte count. The cost is that renaming an active config to a longer name rewrites the file, which is what every non-`file/active/` key already does |
| AC-17 | `zefs.Repair(src, dst, Spare(n))` | `dst` is written with `n`; without the option, with 10 |
| AC-18 | `imageserver.buildZefsDB`, `appliance.runAssemble` outputs | exact-fit |
| AC-22 | `ze data restore <configdir>/database.zefs full` | refused before the lock and before any write, naming the live-store name that `detect` refuses and the two exits; the tree, the source and the lock are untouched |
| AC-23 | a full restore interrupted after durable intent publication: before moving the old tree, between old-tree and unrelated-seed moves, while `database` is absent, after new-tree publication, or during completion | `Open`, `OpenReadOnly`, `Create` and `CreatePopulated` report pending even beside an existing tree, with `ze data restore <source> full`; that command alone completes each recognized state, leaves the source and its sidecar unrenamed, preserves each previous destination at its recorded retirement name, and removes the intent. Replay never invents another stage or retirement name. Missing/unknown policy, malformed descriptors, changed source, missing recorded nodes or an unrelated replacement refuse before mutation. Init retains its prepared, published, retired, retired-locked and retired-then-changed replay boundaries, with retirement progress independent of policy. |
| AC-21 | `WriteGuard.ReadKey` and `ListKeys` on the tree and on the blob, inside a held guard | each answers what the same-named `Storage` method answers outside a guard, and neither deadlocks; the test fails by TIMING OUT, which is what the `Storage` call it replaces would do. `ListKeys` is LITERAL-PREFIX, RECURSIVE and SORTED, so `ListKeys("file/")` and `ListKeys("fil")` both answer every key that string-starts with the argument, in the same order on both encodings, and `ListKeys("")` answers the whole store |
| AC-20 | `ze data restore <file> config` on a source holding two config names, no `name` keyword, and no config matching the device's name | refused before any write, listing both source names and the device's name; with `name <one-of-them>` that config is committed under the device's name and the rename is printed |
| AC-19 | `ze data backup`, `restore`, `request data backup|restore`, `ze init --from` documented | `command-reference.md`, `operations.md` carry them; `./le site build` regenerates the wiki catalog and website pages; the catalog is committed in the wiki checkout |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | backs up a running router before an upgrade | `request data backup` → daemon walk under lock → exact-fit blob on the host | `data-backup-live.ci` |
| 2 | rebuilds a failed box from that backup | `ze init --from backup.zefs` → tree → `ze start` serves the old config and credentials | `init-from-path.ci` |
| 3 | provisions a new box from a blob on the provisioning web server | `ze init --from https://... --sha256 ...` → fetch → import → `ze start` | `init-from-url.ci` |
| 4 | applies yesterday's config from a backup to a live router, keeping everything else | `request data restore ... config` → stage candidate → whole reload acceptance chain → promote last → write observer | `data-restore-config-live.ci` |
| 5 | rolls a stopped router back to a full backup including history | `ze data restore ... full` → tree swap | `data-restore-full.ci` |
| 6 | edits a backup offline to fix a typo before restoring | `ze config edit --backup` → commit into the blob → `restore` | `edit-backup.et`, `data-restore-config.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSpareZeroExactFit`, `TestSparePolicyBoundaries`, `TestContainerExactFit`, `TestSpareZeroGrowthRewrites`, `TestSpareDefaultPolicy`, updated `TestKeyCapacityFileActive` and `TestKeyCapacityNonFileActive` | `pkg/zefs/store_test.go` | AC-2, AC-15, AC-16, A-2; the default retains 10% data padding and changes key padding to the same rule | |
| `TestRepairTakesSpare` | `pkg/zefs/check_test.go` | AC-17 | |
| `TestBackupCarriesEveryKey`, `TestBackupUnderLock`, `TestBackupExactFit` | `internal/component/config/storage/backup_test.go` | AC-1, AC-4 | |
| `TestGuardReadKeyNoDeadlock`, `TestGuardListKeysRecursive`, `TestGuardListKeysPartialPrefix`, `TestGuardListKeysSorted` (all on both encodings) | `internal/component/config/storage/conformance_test.go` | AC-21 | |
| `TestRestoreFullResumeKeepsSource` | `internal/component/config/storage/import_test.go` | AC-23: stop after durable intent before either move, after each previous-destination rename and sync, after publication and its sync, and before/after intent unlink; cover existing tree, empty destination, and unrelated canonical seed; replay preserves the source and each recorded archive | |
| `TestImportPendingBesideTree`, `TestImportPendingBesideSeed` | `internal/component/config/storage/open_test.go` | AC-23: all ordinary open/create paths refuse a pending restore with old tree, absent tree, or new tree; completed retired init remains openable; malformed intent refuses even beside a valid tree | |
| `TestRestoreFullUnrelatedReplacementRefused`, `TestImportRecoveryRefusesChangedIdentityOrBytes` | `internal/component/config/storage/import_test.go` | AC-23: independently replace tree, stage, previous seed or retirement target, including a byte-equal replacement; also remove recorded nodes, change source bytes and introduce dual-location conflicts; no recorded node is moved or deleted | |
| `TestImportResumesCrashBoundaries` (retain and extend) | `internal/component/config/storage/import_test.go` | AC-9, AC-23: retain prepared, published, retired, retired-locked and retired-then-changed; prove published init still retires and already-retired init skips equality without discarding later tree changes; add pre-move replay for forced init with old tree and unrelated seed | |
| `TestImportAtomicPublicationLinux` | `internal/component/config/storage/import_test.go` | AC-23: exercise actual Linux `renameat2(RENAME_NOREPLACE)` for files and directories; interrupt each recorded rename before and after its directory sync, resume every recognized state, and refuse an unrelated empty or populated replacement without changing it. Run on Linux; mocks and cross-compilation alone are insufficient | |
| `TestRestoreFullRefusesLiveBlobName`, `TestImportIntentPolicyValidation`, `TestImportIntentDestinationValidation`, `TestImportRetainsStageAfterIntentError` | `internal/component/config/storage/import_test.go` | AC-22, AC-23: no-lock canonical-source refusal; missing/unknown/mismatched policy and invalid descriptor refusal; keep-source rejects archive fallback; sync/publication errors retain every stage an intent can reference | |
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
| `data-restore-full-resume` | `test/plugin/data-restore-full-resume.ci` | AC-23: interrupted fixtures for pre-move, absent-tree and published-tree states, including an unrelated canonical seed; invoke normal open, then its printed restore command; confirm source unchanged, old destinations preserved and normal open succeeds; unrelated replacement refuses without changing it | |
| `init-from-path`, `init-from-url`, `init-from-refused` | `test/plugin/*.ci` | AC-9 to AC-11 | |
| `edit-backup` | `test/editor/*.et` | AC-12, AC-13 | |

## Files to Modify
- `pkg/zefs/store.go`, `netcapstring.go`, `check.go` - `Spare` option on `Create`, `Open`, `Repair`; `growCapacity` and the key-slot spare as policy methods. Design doc: `docs/architecture/zefs-format.md`
- `internal/component/config/storage/import.go`, `open.go` - immutable intent with source policy, explicit previous tree/seed descriptors and fixed retirement names; identity-based replay of each state in Restartable Full Import; retain the recorded stage after intent publication; keep actual `retired` state separate from policy; detect pending operations beside existing trees and seeds while preserving completed-init replay; mode-derived recovery commands throughout. Reuse the existing `database.lock` and artifact sidecar locks
- `internal/component/config/storage/cli/main.go`, `register.go` - two map entries, `Subs` names every verb
- `internal/component/config/storage/yang/` and `register.go` - the `request data backup|restore` RPCs and handlers
- `internal/component/config/cli/cmd_edit.go`, `cmd_show.go`, `cmd_diff.go`, `cmd_set.go`, `cmd_list.go` - `--backup <file>`
- `internal/component/config/storage/blob.go` - pass spare options through `OpenBlob(file, opts...)` while retaining its existing artifact sidecar lock
- `internal/core/statestore/storage.go`, `internal/component/config/storage/storage.go`, `store.go`, `blob.go`, `tree.go` - `ReadKey` and `ListKeys` on `WriteGuard`, declared in the leaf tier, aliased in the component, implemented on both encodings without re-locking
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
- `test/plugin/data-restore-full-resume.ci`
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
| 6 | Has a user guide page? | Yes | `docs/guide/operations.md` (backup and restore modes, initial-release Linux runtime target, pending-restore command and refusal of unrelated replacements), `docs/guide/config-editor.md` (`--backup`), `docs/guide/ze-install.md` (`ze init --from` and preserved retirement/replay behavior) |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | not protocol |
| 10 | Test infrastructure changed? | No | existing `.ci`/`.et` forms |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it carries a backup/restore row; verified by grep at implementation |
| 12 | Internal architecture changed? | Yes | `docs/architecture/zefs-format.md` (spare policy), `docs/architecture/storage-backends.md` (artifact roles, raw guarded reads versus immediate-child `List`, self-deadlock rule, durable source policy separate from retirement progress, recorded previous destinations, state/effect table and fsync order, pending detection beside a tree, completed-init exception, and Linux native publication for the initial-release target), `docs/architecture/provisioning/image-server.md` (exact-fit seed import) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` if it lists `ze data` verbs or `request` RPCs |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation: `./le spec citation anchors spec plan/pre-release/spec-storage-2-blob-artifact.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ze data` and `ze init` examples in `command-reference.md`, `operations.md`, `ze-install.md` and the website guides; `./le site build` run and the wiki catalog committed (AC-19) |

Design documents declared by the `// Design:` headers of files in scope:

| Page | Affected? | Reason |
|------|-----------|--------|
| `docs/architecture/zefs-format.md` | Yes | spare policy (Phase 2) |
| `docs/architecture/storage-backends.md` | Yes | artifact roles, restore modes, `ze init --from`, guarded raw reads, and the complete Restartable Full Import contract including prior-destination identities, source retirement progress, detection and fsync boundaries on the initial-release Linux runtime target (Phase 3) |
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
   - Verify: AC-2, AC-15 to AC-18; existing content/format tests remain valid, and the key-capacity tests change to the explicit AC-16 policy
3. **Phase: Backup and restore** -- the walks, the two modes, the daemon RPCs and restartable import
   - Tests: `backup_test.go`, `restore_test.go`, `import_test.go`, `open_test.go`, `conformance_test.go`, `data_rpc_test.go`, `data-backup*.ci`, `data-restore*.ci`, including the crash-state and refusal rows
   - Files: `backup.go`, `restore.go`, `import.go`, `open.go`, `data_rpc.go`, `main_reload.go`, `internal/core/statestore/storage.go` and the guard implementations, the YANG, `docs/architecture/storage-backends.md`, `docs/architecture/api/commands.md`, `docs/architecture/hub-architecture.md`, `docs/guide/operations.md`
   - Verify: AC-1, AC-3 to AC-8, AC-20 to AC-23 on Linux; retain init retirement/replay boundaries in `TestImportResumesCrashBoundaries`; run the publication crash-state cases on an actual Linux kernel and filesystem, including in a Linux VM when the host is another OS
4. **Phase: `ze init --from`** -- fetch helper moved, scheme table, import
   - Tests: `TestFetchSHA256`, `TestFetchSchemeTable`, `TestInitFrom*`, `init-from-*.ci`
   - Files: `internal/core/fetch/`, `install/disk/system.go`, `init/main.go`, `docs/guide/ze-install.md`, `docs/architecture/appliance/on-device-installer.md`
   - Verify: AC-9 to AC-11
5. **Phase: Offline edit** -- `OpenBlob` with lock, `--backup` on the editor and the offline family
   - Tests: `TestOpenBlobConformance`, `TestOpenBlobLock`, `TestEditBackupFlagConflicts`, `TestEditBackupNoDaemon`, `edit-backup.et`
   - Files: `open.go` and existing artifact openers reusing `lockStoreFile` and its sidecar, `cmd_edit.go`, `cmd_show.go`, `cmd_diff.go`, `cmd_set.go`, `cmd_list.go`, `docs/guide/config-editor.md`, `docs/guide/command-reference.md`, `docs/features.md`; no new lock pair
   - Verify: AC-12 to AC-14
6. **Phase: Published surfaces** -- website pages, `./le site build`, wiki catalog
   - Verify: AC-19

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | `restore full` never touches the live tree before the new tree passes equality; `restore config` changes exactly one version, three pointer keys and one mirror; `zefs.Check` runs before any write; spare 0 produces capacity equal to used for every key and data slot; the live backup runs under the daemon's lock |
| Import recovery | pending detection runs with or without a tree; the intent precedes recorded moves, stage durability precedes the intent, and every rename is synced before the next effect; old/new/unrelated identities remain distinct; keep-source never substitutes for retirement progress; completed init preserves later tree changes |
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
| Full-restore recovery without source loss | AC-23 crash-state tests and `data-restore-full-resume.ci` on Linux, including previous seed retirement and unrelated replacement refusal; runtime evidence exercises Linux native publication rather than a mocked rename or cross-compile-only check |
| Pages, site and wiki updated | `./le doc wiring` green; `./le site build` run; wiki commit SHA recorded |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | every key in a restored or imported blob is validated as `fs.ValidPath` before it becomes a tree path; the RPC `path` is absolute, no `..`, no symlink, owned by the daemon's user |
| Secrets in artifacts | a backup holds credentials, the CA key and web key; the file is created 0600 and the command prints that it contains secrets, as `cmd_assemble` does today |
| Restore overreach | `restore config` touches config keys only; `restore full` is offline and moves only recorded previous identities to recorded names, never an unrelated replacement; source policy and archive validation prevent restore from consuming its backup |
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
| One immutable import intent records policy and old/new identities with fixed retirement names | caller-only policy, a mutable phase counter, or caller-side renames | identity and location distinguish every recoverable move without a second progress record; pending detection covers an existing tree, and replay refuses unrelated replacements. Actual source retirement remains separate from policy, so init keeps its existing completion semantics |
| Linux initial-release runtime target (owner, 2026-09-18) | new non-Linux publication representations or fallback protocols in this release | reuse the existing native `renameat2(RENAME_NOREPLACE)` path for both file and directory moves; retain unrelated-replacement refusal without adding another publication mechanism. Other platforms are neither promised nor removed |
| The offline editor's lock is the existing `<artifact>.lock` sidecar | an `flock` on the blob's own fd | `atomicWrite` installs a new inode on every rewrite, so an fd lock is silently lost on the first commit and a second editor gets in |
| One spare percentage on both slot kinds, the `file/active/` +20 deleted | keep the absolute key allowance beside the percentage | the allowance is one caller's namespace spelled inside a generic store, no percentage reproduces it, and the spec's own no-layering row already requires the literal deleted rather than defaulted around |
| The guard's raw-key read pair lands in THIS spec | leave it to storage-3, which also needs raw keys | the backup walk needs it first, and a spec does not depend on a later one for the API its own acceptance criteria exercise; storage-3 adds the write pair it uses, so each spec adds what it calls |
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
