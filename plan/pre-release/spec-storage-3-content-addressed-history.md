# Spec: storage-3-content-addressed-history

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Depends | storage-1-backend-parity, storage-2-blob-artifact |
| Phase | - |
| Handoff | verify |
| Updated | 2026-09-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Config history is content-addressed: a version's bytes are stored once
under `object/<hex>`, and the dated history entry `file/<stamp>/<name>` holds
`sha256:<hex>` instead of a copy. Every key stays a netcapstring frame with
its CRC32c, so bit rot is caught on read and identity is proven by the hash
(owner decision, 2026-09-16: "use sha256 to be safe for the name and the crc
in the file as netstring"; reviewed the same day).**

After `spec-storage-1-backend-parity` both encodings share one version model
in the storage core: `WriteVersion` copies the config under
`file/<stamp>/<name>`, the pointers `meta/config/<name>/active|candidate|
rollback|recovery` name a stamp, `ListVersions` walks the stamps, and
`RemoveVersion` deletes a copy. Two commits of an unchanged config are two
copies, and nothing proves that a version's bytes are the ones committed.

Goal: the same operations, the same pointers and the same `ListVersions`
answer, over objects stored once and named by their content. Ze is
pre-release and no store on `main` holds history anyone keeps
(`ai/rules/pre-release.md`), so there is no migration: a store created before
this lands is re-initialised. That is why this sits in `plan/pre-release/`:
after a release the same change would owe a migration under operators.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/storage-backends.md` (storage-1) - the contract, the version and pointer model in the core
  → Constraint: versions and pointers exist once, in the shared core, over an encoding that stores bytes by key; this spec changes that core and neither encoding
  → Constraint: the conformance table runs over both encodings; every row this spec adds runs over both
  → Constraint: `WriteGuard` (`internal/core/statestore/storage.go`) already carries `List`; what it has no route to is a RAW key. Its eight methods are all name-based or `Has`, and `resolveKey` turns a name outside `meta/` and `file/` into `file/active/<base>`, so an object cannot be reached through any of them. The sweep needs `ReadKey`, `WriteKey`, `RemoveKey` and `ListKeys` on the guard
  → Constraint: calling `Storage` under a held guard HANGS the process, it does not error: `store.acquire` takes `s.mu.Lock()` and only `Release` unlocks it, so `ReadFile`, `ReadKey`, `Exists`, `Stat`, `List`, `ListKeys`, `ListVersions` and a second `AcquireLock` all self-deadlock. The guard's own methods bypass both mutexes, as `guard.Has` does, and the four raw-key methods this spec adds MUST do the same. Nothing tests this today
- [ ] `docs/architecture/zefs-format.md` - frames and CRC
  → Constraint: the frame CRC stays on every key, objects included; the hash names the object, the CRC guards the frame. They answer different questions and both are kept
- [ ] `plan/journal/pointer-shared-across-the-names-it-indexes.md`
  → Constraint: pointers are per name after storage-1; an object can be referenced by entries of different names, so a removal deletes an object only when no remaining entry of any name references it
- [ ] `cmd/ze/pushed_config.go` `writeConfigActiveHash`, `pkg/zefs/keys.go` `KeyConfigActiveHash`, `KeyConfigLastKnownGood`
  → Constraint: the repository already spells a digest `sha256:<hex>`; the entry value uses the same spelling
- [ ] `ai/rules/principles.md` - no silent zero
  → Constraint: an entry whose object is missing is an error naming the hash, never an empty config; a removal that cannot enumerate the remaining entries refuses to delete the object
- [ ] `ai/rules/simplicity.md`
  → Decision: the indirection lives in `ReadVersion(name, stamp)`, and `ReadFile` stays raw for every key, so `ze data cat`, backup, import and check see what is stored. `ReadVersion` does NOT exist today: `pointer.go` has the unexported `readVersion` and `readVersionLocked`, and there is no exported `ReadVersion` or `RemoveVersion` anywhere in the tree. Every byte reader of a version sits in another package, so `ReadVersion` is added to the `statestore.Storage` contract and aliased in `storage.go`, beside `WriteVersion` and `ListVersions`
  → Decision: `VersionInfo.Path` is KEPT. It holds the ENTRY key `file/<stamp>/<name>` (`store.ListVersions`), which still exists and is still what `ze data cat` takes; what changes is that its bytes are no longer the config. Deleting the field would change `ze config history` output and the `keyPath` field of the `data history` JSON for no gain
- [ ] `ai/rules/no-layering.md`
  → Constraint: the copying `WriteVersion` is deleted; no store holding copies is read through a fallback arm, because none needs to exist

**Key insights:** (minimal context to resume after compaction)
- Entry = `file/<stamp>/<name>` holding `sha256:<hex>`; object = `object/<hex>` holding the bytes. Both are ordinary keys to the RAW key API of both encodings; neither encoding learns what they mean. The name-based API is another matter, and objects never go through it.
- Write: hash, write the object if absent AND its stored bytes hash to its name, write the entry. Read: entry → object → verify hash on the bytes → return. Remove: delete the entry, and only if it was really deleted, delete the object when no remaining entry names it.
- Objects are reached by RAW key only (`ReadKey`, `WriteKey`, `RemoveKey`, `ListKeys`), from inside package `storage` only. The name-based API would silently move them: `resolveKey` maps a name outside `meta/` and `file/` to `file/active/<base>`, and `resolveDirKey` makes `List("object")` list `file/active/`.
- No mark-and-sweep in the write path. A full reachability walk belongs to `ze data check` and `repair`.
- No migration: pre-release, re-init.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/config/storage/pointer.go` - `WriteCandidateVersion`, `EnsureActiveVersion`, `PromoteCandidate` (writes a version from the mirror for a legacy config, moves candidate to active and active to rollback, clearing the candidate pointer LAST so no crash point loses a stamp), `ClearCandidate` (removes the candidate's version), `readVersion` and `readVersionLocked` (unexported; there is no exported `ReadVersion` and no `RemoveVersion` in the tree), `removeVersionLocked` (every removal, including the rollback arms in `WriteCandidateVersionWithGuard` and `EnsureActiveVersion`, runs under a guard), `ReadActiveConfig`, `ReadCandidateConfig`
  → Constraint: `removeVersionLocked` RETAINS a version any of the four pointers (`active`, `rollback`, `recovery`, `candidate`) names, and returns nil without deleting. A sweep that runs regardless would delete the object of a version that is still there, so the removal MUST report whether it deleted and the object step MUST NOT run when it did not
- [ ] `internal/component/config/storage/store.go` (storage-1) - `WriteVersion`, `ListVersions` (walk `file/` for stamp directories, skip `active|draft|template`, keep those holding `<name>`), `VersionInfo{Stamp, Date, Path}`
- [ ] `internal/component/config/storage/storage.go`, `internal/core/statestore/storage.go` - the contract is declared in the leaf tier and aliased here: `Storage` carries the raw-key four (`ReadKey`, `WriteKey`, `RemoveKey`, `ListKeys`) beside the name-based methods; `WriteGuard` carries `ReadFile`, `WriteFile`, `Remove`, `Has`, `List`, `Release`, `SetModifier`, `WriteVersion`, and no raw-key method
- [ ] `internal/component/config/storage/blob.go` `AcquireLock` (store-wide `store.Lock()`, name ignored); `ListVersions` takes the store's read lock
- [ ] `pkg/zefs/keys.go` - `KeyFileVersion` = `file/{date}/{basename}`; `KeyEntry.Key` rejects empty and `..`
- [ ] The readers of `VersionInfo.Path`, five, three of which read BYTES and must move to `ReadVersion`: `internal/component/config/stamp.go` (`store.ReadFile(v.Path)`); `internal/component/cli/editor.go` `ListBackups` → `Rollback` / `readBackupContent` (`e.store.ReadFile(path)`), reached by `internal/component/config/cli/cmd_rollback.go`; and `internal/component/config/cli/cmd_diff.go` `resolveRollbackPath` → `loadAndResolve` (`store.ReadFile(path)`). The other two only PUBLISH the key and stay as they are: `cmd_history.go` prints `b.Path`, `config_data.go` `dataHistory` emits `keyPath`
- [ ] `internal/component/config/storage/store.go` `ListVersions` - already the recursive raw walk this spec's sweep needs: `ListKeys("file/")`, a three-part split and `parseVersionStamp`. `Storage.List(prefix)` is NOT that walk: it returns immediate children and never a directory, so `List("file")` over `file/<stamp>/<name>` returns the empty slice
- [ ] `pkg/zefs/check.go`, `internal/component/config/storage/cli/main.go` `cmdWrite` - `Check` verifies magic, container framing, per-entry CRC32c and `fs.ValidPath`, and nothing about content; `cmdWrite` writes any bytes under any valid key. A CRC-valid `object/<hex>` holding the WRONG bytes is therefore reachable and today's `check` calls it ok
- [ ] `internal/component/config/storage/cli/main.go` `cmdRm` - raw key removal, offline
- [ ] `internal/component/config/storage/backup.go`, `restore.go` (storage-2) - the walks copy every key, so objects travel with entries; `restore config` reads the source's active version, which is now entry → object

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every exported function in `pointer.go` and the `Storage` version methods keep their observable results
- `ListVersions` order (date descending), `VersionInfo.Stamp` and `Date`
- `ze config history`, `ze config rollback`, editor commit and rollback, `ze start` reading the active version, SIGHUP candidate staging, storage-2 backup and restore
- `Storage.ReadFile` returns the stored bytes of any key, an entry's `sha256:<hex>` included
- The frame CRC on every key

**Behavior to change:** (only what the user asked for)
- `file/<stamp>/<name>` holds `sha256:<hex>`; bytes live at `object/<hex>`
- `ReadVersion(name, stamp)` is ADDED to the `statestore.Storage` contract and resolves entry → object with the hash verified; `readVersion` and `readVersionLocked` dereference too, so `ReadActiveConfig`, `ReadCandidateConfig` and `PromoteCandidate`'s guarded read all get bytes rather than a hash; the three byte readers of `VersionInfo.Path` call it by stamp; `VersionInfo.Path` keeps its value and its meaning, the entry key
- `RemoveVersion` and `ClearCandidate` delete the object when no remaining entry of any name references it
- `WriteGuard` gains the raw-key four, `ReadKey`, `WriteKey`, `RemoveKey` and `ListKeys`, on both encodings, each bypassing the store mutex and the blob mutex as `guard.Has` does
- `removeVersionLocked` reports whether it deleted, so the object sweep runs only when the entry really went
- storage-2 `restore config` reads the source's active version through the same entry → object path; `restore full` and `ImportBlob` copy objects like any key, writing `object/*` before `file/*`
- `ze data check` reports orphan objects (warning) and dangling entries (error); `repair` drops a dangling entry and reports it

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `Storage.WriteVersion(name, data, stamp)` and `WriteGuard.WriteVersion` (`internal/component/config/storage/store.go`): every commit, candidate and legacy promotion.
- `ReadVersion`, `ReadActiveConfig`, `ReadCandidateConfig`, `ListVersions`, `RemoveVersion`, `ClearCandidate` (`pointer.go`): every reader and remover.
- `zefs.CheckPath`, `RepairPath` (storage-1): the integrity walk.

### Transformation Path
1. Write: SHA-256 over `data` → `object/<hex>`; when the key is absent, write the object frame; when it is present, read it and hash it, and write the entry only when the stored bytes hash to the key. Existence alone never settles identity, because `ze data write` and a repaired frame can both put wrong bytes under a right name. Both steps under the guard's lock, through the guard's raw-key methods; object before entry, so a crash leaves an unreferenced object, never a dangling entry.
2. Read: entry → `sha256:<hex>` → object frame (CRC checked by the encoding) → SHA-256 over the bytes must equal the hex → bytes. A mismatch is an error naming the entry and both hashes; an entry whose value is not `sha256:<64 hex>` is an error naming the entry.
3. Remove: read the entry's hash → ask `removeVersionLocked` to delete the entry → when it reports the entry RETAINED (a pointer names that stamp), stop, the object stays → otherwise `guard.ListKeys("file/")`, which is recursive and raw, read each remaining three-part key of any name through `guard.ReadKey` → delete `object/<hex>` when none holds that hash. Under the guard the caller already holds, and never through `Storage`, which would deadlock.
4. Import and restore (storage-1, storage-2): the walks order `object/*` before `file/*` so an interrupted walk never leaves a dangling entry; the equality check is unchanged (keys and bytes).
5. Check: every `file/<stamp>/*` entry's hash must name an existing object (else error); every `object/<hex>` must hash to its own name (else error: a CRC-valid frame proves only that the bytes are the ones written, never that they are the content the name promises); every `object/*` must be named by an entry (else warning). `repair` drops a dangling entry and a wrong-hash object, reports each, and reports every pointer left naming a dropped entry, which is a store a daemon refuses to start on.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Core ↔ encoding | ordinary keys; no encoding learns about objects; `List` added to the guard on both | No |
| Store ↔ blob artifact (storage-2) | a backup carries `object/*` and entries as keys; import writes objects first | No |

### Integration Points
- storage-1 conformance table - new rows: write two equal versions, one object; remove one, object stays; remove the other, object goes; a version a pointer names is retained with its object; an entry with a missing object errors; an object whose bytes do not hash to its name errors on write and on check; the guard's raw-key four on both encodings
- storage-2 `restore config` - reads the source's active version through `ReadVersion` on the blob-backed `Storage`; a source whose object is absent is refused naming the hash
- storage-1 `ImportBlob`, storage-2 backup and restore walks - object-first ordering
- `ze data check` and `repair` - reachability report

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | objects are RAW keys, reached by `ReadKey`/`WriteKey`/`RemoveKey`/`ListKeys` inside package `storage` only; the name-based API is never used on them, because `resolveKey` would rewrite `object/<hex>` to `file/active/<hex>` and `resolveDirKey` would make `List("object")` list `file/active/`. `isNamespaced` is deliberately NOT extended: that is a contract change both encodings and `CheckName` would carry, for a namespace no caller outside the core names. `ReadFile` stays raw for the keys it does resolve |
| No unintended coupling (components stay isolated) | Yes | `crypto/sha256` is used in the storage core only; the three `Path` readers move from `ReadFile` to `ReadVersion`, an existing function |
| No duplicated functionality (extends existing, does not recreate) | Yes | the frame CRC is reused; the digest spelling is the repository's; the walk ordering is a sort in the existing walks |
| Zero-copy preserved where applicable (refs, not copies) | Yes | hashing reads the slice once; the object write takes the same slice |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | no command; one registered key pattern (`object/{hex}`), which buys DISCOVERY only. `MustRegister` (`pkg/zefs/registry.go`) feeds the `Key()`/`Prefix()` builders, the `ze data registered` listing and `statestore.RegisterPluginKeys`; `zefs.IsRegistered` has no non-test caller, so no read, write, list, check or repair path consults the registry and registration authorizes nothing |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | lists searched: `ListVersions`' reserved-name set (`active`, `draft`, `template`) gains nothing because objects live outside `file/`; `pkg/zefs/keys.go` (registry); `ze data registered` derives from it |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No store needs migrating: Ze is pre-release and stores on `main` are re-initialised | `ai/rules/pre-release.md`, owner directive 2026-08-30 | a migration spec is written then; nothing in this design forecloses one, because entries and objects are ordinary keys | owner confirmation at the gate | unvalidated |
| A-2 | Only history is content-addressed; `file/active/*`, `file/draft/*`, `file/template/*` and `meta/*` stay direct keys | design 2026-09-16: mutable keys addressed by content would make every edit an object plus a rename | nothing else in the model changes | owner confirmation | unvalidated |
| A-3 | Every reference to an object is a `file/<stamp>/*` entry, found by the recursive raw walk `ListKeys("file/")` and a three-part split, as `ListVersions` already walks; no other key holds `sha256:<hex>` that names an object | `pkg/zefs/keys.go`: `KeyConfigActiveHash` and `KeyConfigLastKnownGood` hold digests of a config, not object references, and never cause a removal to keep or drop an object | a reference outside `file/<stamp>/` would be missed by the removal check; the check reads `file/<stamp>/*` only, by design, and `ze data check` reports the orphan | `TestRemoveChecksEveryName` | unvalidated |
| A-4 | SHA-256 over configs of a few KB per commit is not a performance concern | commit is an operator action | none | `BenchmarkWriteVersion` | unvalidated |
| A-5 | Adding the raw-key four to `WriteGuard` is safe on both encodings: the guard already reaches the in-memory tree and the tree encoding without re-locking, as `Has`, `List` and `ReadFile` do (`guard.List` → `blobLock.List` → `node.walk`; the tree arm → `tree.list`) | `internal/component/config/storage/store.go` guard methods, `pkg/zefs/lock.go` | the sweep would need to run after `Release`, outside the guard, with its own lock, and the removal would stop being atomic | `TestGuardRawKeyNoDeadlock` | confirmed by the producer |
| A-6 | The two encodings' guarded list agree closely enough for a raw recursive walk: `guard.ListKeys("file/")` is a whole-directory prefix, which the blob's segment walk and the tree's string-prefix filter answer identically | `pkg/zefs/tree.go` `node.walk`, `tree.go` `list`; they diverge only on a prefix that is not a whole directory path | the sweep would take one implementation in the core over `ListKeys("")` and filter itself | conformance row for `guard.ListKeys` on both encodings | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A crash between object write and entry write leaves an orphan object | `ze data check` reports it | orphans are a warning; `repair` keeps them; a later write of the same content reuses them |
| R-2 | A crash between entry delete and object delete leaves an orphan | same | same |
| R-3 | Two entries of different names share one object and one is removed | `TestSharedObjectSurvivesOneRemove` | the removal lists every name's entries before deleting |
| R-4 | The removal's `guard.List` deadlocks on the blob | `TestGuardListNoDeadlock` | `WriteLock.List` reads the in-memory tree like `WriteLock.Has` |
| R-5 | `ze data rm file/<stamp>/x` offline bypasses the removal check and orphans an object | `ze data check` | consistent with R-1: an orphan is a warning |
| R-6 | A storage-2 backup restored on a device already holding the same objects | none: a write of an existing object key is a no-op by content | `ImportBlob`'s equality check compares bytes, and equal bytes pass |
| R-7 | An entry whose value is not `sha256:<64 hex>` (a hand-written `ze data write`) | `TestReadVersionRejectsBadEntry` | an error naming the entry; `ze data check` reports it as dangling |
| R-8 | An `object/<hex>` holding bytes that do not hash to its name, from `ze data write` or a repaired frame, is reused by dedup and passes today's CRC-only `check` | `TestWriteVersionRefusesWrongObject`, `TestCheckReportsWrongHashObject` | the write path verifies an existing object before it references it, `check` hashes every object, and `repair` drops a wrong-hash one |
| R-9 | A future edit calls `Storage` inside a held guard and the process HANGS with no error and no test | `TestGuardRawKeyNoDeadlock`, which would time out | the four new methods live on the guard and bypass both mutexes like `Has`; the removal path takes the guard's methods only |
| R-10 | `repair` drops a dangling entry and leaves a pointer naming it, after which `ReadActiveConfig` errors and `ze start` exits 1 | `TestRepairReportsDanglingPointer` | `repair` reports every pointer left dangling, naming the pointer and the stamp, so the operator fixes it before a restart; in `ConfigSourceFile` mode `initializeConfigSource` rebuilds the version from the file and says so in the log rather than silently |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Rollback and history: a wrong removal deletes a version's bytes. Active config is untouched (direct key), so a running daemon keeps serving |
| How is it reverted? | One commit revert for code; a store written under this spec is re-initialised (pre-release), the same rule as forward |
| Who else touches this path? | storage-1 (core), storage-2 (walks carry the keys and order them), `spec-config-at-rest-encryption` (objects are encrypted like any key) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| editor commit twice with the same config | → | `WriteVersion` → one `object/*`, two entries | `test/editor/history-dedup.et` |
| `ze config rollback` | → | `PromoteCandidate` → `ReadVersion` entry → object → active | `test/plugin/history-rollback-object.ci` |
| `ze data check` on a store with an orphan and a dangling entry | → | reachability report | `test/plugin/data-check-history.ci` |
| `ze data restore <backup> config` (storage-2) | → | `ReadVersion` on the source blob | `test/plugin/data-restore-config-object.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `WriteVersion(name, data, stamp)` | `file/<stamp>/<name>` holds `sha256:<hex>` of `data`; `object/<hex>` holds `data`; both frames carry a valid CRC; the object is written before the entry |
| AC-2 | two `WriteVersion` calls with equal `data`, different stamps or names | one object, two entries |
| AC-3 | `ReadVersion`, `ReadActiveConfig`, `ReadCandidateConfig` | return the bytes; a wrong-hash object, a missing object or a malformed entry value is an error naming the entry and the hash, never empty bytes |
| AC-4 | `Storage.ReadFile("file/<stamp>/<name>")` | returns the stored entry value `sha256:<hex>`, raw |
| AC-5 | removal of one of two entries sharing an object | the object stays; removing the second deletes it; an entry of another name sharing the object keeps it; and a version an `active`, `rollback`, `recovery` or `candidate` pointer names is RETAINED, entry and object both, because `removeVersionLocked` did not delete |
| AC-6 | `ClearCandidate` | the candidate's entry is removed and its object deleted when unreferenced |
| AC-7 | `ListVersions(name)` | unchanged shape and order; `VersionInfo` still carries `Stamp`, `Date` and `Path`, and `Path` is still the entry key `file/<stamp>/<name>`, so `ze config history` and the `data history` JSON print what they print today |
| AC-8 | `WriteGuard.ReadKey`, `WriteKey`, `RemoveKey` and `ListKeys` on the tree and on the blob, inside a held guard | each answers what the same-named `Storage` method answers outside a guard, without deadlock; `ListKeys("file/")` is recursive and returns full raw keys; the test fails by timing out, which is what calling `Storage` there would do |
| AC-9 | the storage-1 conformance table | passes on both encodings with the rows this spec adds |
| AC-10 | `ze data check` | reports an unreferenced object as a warning, and each of an entry with no object, a malformed entry value, and an `object/<hex>` whose bytes do not hash to `<hex>` as an error (exit 1), naming the key and, for the last, both hashes |
| AC-11 | `ze data repair --output` | writes a store where every entry's object exists and every object hashes to its name; a dangling or malformed entry and a wrong-hash object are dropped and reported; orphans are kept; every pointer left naming a dropped entry is reported |
| AC-12 | storage-1 `ImportBlob` and storage-2 `restore full` walks | write every `object/*` key before any `file/*` key |
| AC-13 | storage-2 `restore config` from a source whose active entry names an object the source lacks | refused before any write, naming the hash |
| AC-14 | `ze data registered` | lists `object/{hex}`, which is all registration does: it is discovery, not access control or validation |
| AC-15 | a pointer naming a stamp whose entry `repair` dropped | `ReadActiveConfig` errors naming the pointer and the stamp, `ze start` exits 1 with that error, and no empty config is served; in `ConfigSourceFile` mode the daemon rebuilds the version from the explicit file and logs that it did |
| AC-16 | `WriteVersion` when `object/<hex>` already exists but holds bytes that hash to something else | refused before the entry is written, naming the key, the expected hash and the stored one; the entry is not written, so no reference to the wrong object exists |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | commits the same config twice | two entries, one object | `history-dedup.et` |
| 2 | rolls back | pointer moves; bytes read through the object; hash verified | `history-rollback-object.ci` |
| 3 | checks a store after a disk error | orphan and dangling reported apart | `data-check-history.ci` |
| 4 | restores yesterday's config from a backup | the source's entry → object → committed on the device | `data-restore-config-object.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWriteVersionStoresObject`, `TestWriteVersionDedups`, `TestWriteVersionObjectFirst` | `internal/component/config/storage/history_test.go` | AC-1, AC-2 | |
| `TestReadVersionVerifiesHash`, `TestReadVersionMissingObject`, `TestReadVersionRejectsBadEntry`, `TestReadFileEntryRaw` | `history_test.go` | AC-3, AC-4 | |
| `TestSharedObjectSurvivesOneRemove`, `TestRemoveChecksEveryName`, `TestClearCandidateDeletesObject`, `TestPointedVersionRetainedWithObject` | `history_test.go` | AC-5, AC-6, A-3 | |
| `TestGuardRawKeyNoDeadlock`, `TestGuardListKeysRecursive` | `conformance_test.go` | AC-8, A-5, A-6 | |
| conformance rows | `conformance_test.go` | AC-9 | |
| `TestCheckReportsOrphanAndDangling`, `TestCheckReportsWrongHashObject`, `TestRepairDropsDangling`, `TestRepairReportsDanglingPointer`, `TestWriteVersionRefusesWrongObject` | `pkg/zefs/check_test.go` or the storage package, wherever the walker lives after storage-1 | AC-10, AC-11, AC-15, AC-16, R-8, R-10 | |
| `TestImportObjectsFirst`, `TestRestoreConfigRefusesMissingObject` | `import_test.go`, `restore_test.go` | AC-12, AC-13 | |
| `BenchmarkWriteVersion` | `history_test.go` | A-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| entry value | exactly `sha256:` plus 64 lowercase hex | 71 characters | 70 refused | 72 refused |
| object size | 0 to available memory | empty config hashes and stores | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `history-dedup` | `test/editor/*.et` | AC-2 through the editor | |
| `history-rollback-object`, `data-check-history`, `data-restore-config-object` | `test/plugin/*.ci` | AC-3, AC-10, AC-13 | |

## Files to Modify
- `internal/component/config/storage/store.go` - `WriteVersion`, `ListVersions` over entries, `VersionInfo` without `Path`. Design doc: `docs/architecture/storage-backends.md`
- `internal/component/config/storage/storage.go`, `internal/core/statestore/storage.go` - `ReadVersion` on `Storage`, and `ReadKey`, `WriteKey`, `RemoveKey`, `ListKeys` on `WriteGuard`; the contract is declared in the leaf tier and aliased in the component
- `internal/component/config/storage/store.go` (guard methods), `blob.go`, `tree.go` - the raw-key four on both guards, each bypassing the store mutex and the blob mutex as `Has` does
- `internal/component/config/storage/pointer.go` - `ReadVersion` resolves entry → object; `removeVersionLocked` reports whether it deleted and `ClearCandidate` runs the object sweep only when it did
- `internal/component/config/storage/import.go` (storage-1), `backup.go`, `restore.go` (storage-2) - object-first ordering; `restore config` through `ReadVersion` and the missing-object refusal
- `internal/component/config/storage/cli/cmd_integrity.go` - reachability report, `repair` dropping dangling entries
- `internal/component/config/stamp.go`, `internal/component/cli/editor.go` (`ListBackups`, `Rollback`, `readBackupContent`), `internal/component/config/cli/cmd_rollback.go`, `internal/component/config/cli/cmd_diff.go` (`resolveRollbackPath`, `loadAndResolve`) - `ReadVersion` by stamp. `cmd_history.go` and `config_data.go` publish `VersionInfo.Path` and are untouched
- `pkg/zefs/keys.go` - register `object/{hex}`
- `docs/architecture/storage-backends.md`, `docs/architecture/zefs-format.md` (key namespaces table), `docs/guide/command-reference.md` (`ze data check` output), `docs/guide/operations.md`, `ai/INDEX.md`, `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md`

## Files to Create
- `internal/component/config/storage/history.go`, `history_test.go` - hashing, object write, verified read, the removal check
- `test/editor/history-dedup.et`, `test/plugin/history-rollback-object.ci`, `data-check-history.ci`, `data-restore-config-object.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | storage core only |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | No | `ze data check` and `repair` gain report rows, no flag |
| CLI grammar (keyword before value) | N-A | no new keyword |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | Yes | the `.ci`/`.et` above |
| Pipe completeness | Yes | `ze data check` rows gain `orphan` and `dangling` fields so `\| match` selects them |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | `doctor-store-integrity` (storage-1) reports dangling entries as an error and orphans as a warning; codes in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: deduplicated, hash-verified history |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: `ze data check` report rows |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/operations.md` (history and rollback section) |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | not protocol |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | verified by grep of `docs/comparison.md` for `history` at implementation |
| 12 | Internal architecture changed? | Yes | `docs/architecture/storage-backends.md` (objects, entries, removal check), `docs/architecture/zefs-format.md` (namespace table gains `object/`) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` if it lists doctor codes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation: `./le spec citation anchors spec plan/pre-release/spec-storage-3-content-addressed-history.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ze config history` and `rollback` examples in `command-reference.md` verified; `./le site build` run and the wiki catalog committed |

Design documents declared by the `// Design:` headers of files in scope:

| Page | Affected? | Reason |
|------|-----------|--------|
| `docs/architecture/storage-backends.md` | Yes | the version model section (Phase 2) |
| `docs/architecture/zefs-format.md` | Yes | key namespace table (Phase 2) |
| `docs/architecture/config/syntax.md` | No | declared by `cmd_rollback.go` for the parser; the parser is untouched |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the history functions, `Storage.ReadVersion` and the guard's raw-key four exist, the conformance rows fail
   - Tests: conformance rows for objects and the guard's raw-key four, `history-dedup.et`
   - Files: `history.go` (stubs), `storage.go`, `internal/core/statestore/storage.go`, `store.go`, `blob.go`, `tree.go`, `keys.go`, `conformance_test.go`
   - Verify: rows fail on behavior
2. **Phase: Objects** -- write, verified read, dedup, `ReadVersion` by stamp at the three readers
   - Tests: `TestWriteVersionStoresObject`, `TestWriteVersionDedups`, `TestWriteVersionObjectFirst`, `TestReadVersionVerifiesHash`, `TestReadVersionMissingObject`, `TestReadVersionRejectsBadEntry`, `TestReadFileEntryRaw`
   - Files: `history.go`, `store.go`, `pointer.go`, `stamp.go`, `editor.go`, `cmd_rollback.go`, `cmd_diff.go`, `docs/architecture/storage-backends.md`, `docs/architecture/zefs-format.md`
   - Verify: AC-1 to AC-4, AC-7
3. **Phase: Removal** -- the check after remove and clear
   - Tests: `TestSharedObjectSurvivesOneRemove`, `TestRemoveChecksEveryName`, `TestClearCandidateDeletesObject`, `TestGuardListNoDeadlock`
   - Files: `history.go`, `pointer.go`
   - Verify: AC-5, AC-6, AC-8
4. **Phase: Walks and integrity** -- object-first ordering, restore-config refusal, check and repair
   - Tests: `TestImportObjectsFirst`, `TestRestoreConfigRefusesMissingObject`, `TestCheckReportsOrphanAndDangling`, `TestRepairDropsDangling`, the `.ci` files
   - Files: `import.go`, `backup.go`, `restore.go`, `cmd_integrity.go`, `diagnostic/codes.go`, `docs/guide/command-reference.md`, `docs/guide/operations.md`, `docs/features.md`
   - Verify: AC-10 to AC-14

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | object before entry on write; an existing object verified before it is referenced; entry before object on remove, and no object step when the entry was retained; hash verified on every `ReadVersion`; the removal lists entries of every name through the guard's recursive raw walk; `ReadFile` never dereferences |
| Naming | `object/<64 lowercase hex>`; entry value `sha256:<hex>` |
| Data flow | no caller outside the storage core reads or writes `object/*`, and inside it every object access is a RAW key call; the three byte readers of `VersionInfo.Path` call `ReadVersion`; no `Storage` method is called inside a held guard |
| Rule: `ai/rules/no-layering.md` | the copying `WriteVersion` is gone; no fallback arm reads a copy |
| Rule: `ai/rules/principles.md` | a missing object, a wrong hash and a malformed entry each stop with a named error |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Objects stored once | `TestWriteVersionDedups` |
| Hash verified on read | `TestReadVersionVerifiesHash` |
| Removal correct across names | `TestSharedObjectSurvivesOneRemove`, `TestRemoveChecksEveryName` |
| No deadlock under a guard | `TestGuardListNoDeadlock` |
| Pages updated | `./le doc wiring` green |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | an entry value is accepted only as `sha256:` plus exactly 64 lowercase hex characters; anything else is an error |
| Hash collision | SHA-256; no truncation of the name |
| Removal safety | an object is deleted only under a guard, only when the listing of `file/<stamp>/*` names no entry holding its hash |

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

- The CRC and the hash are not redundant: the CRC says "this frame is what was written", the hash says "this is the content the entry promised". A frame can be intact and still be the wrong object; a hash cannot catch a bit flip in an unhashed key.
- The first draft put the indirection in `ReadFile` and sniffed entry values to detect legacy copies. Both were hidden branches on data the store never validates; a value-based detector is unsound because `WriteVersion` accepts any bytes. Keeping `ReadFile` raw and dropping migration (pre-release) removed both.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| SHA-256 names, CRC frames kept | CRC32c as the name | a 32-bit name aliases two versions silently; the owner chose SHA-256 for the name and kept the CRC in the frame (2026-09-16) |
| Only history is content-addressed | every key | mutable keys addressed by content are an object plus a rename per edit, for no dedup |
| Targeted removal check under the caller's guard | a full mark-and-sweep on every `ClearCandidate`, or a periodic sweeper | `ClearCandidate` runs on every reload and commit; a full walk there reads every entry each time; the targeted check reads the entries once per removal and the full walk belongs to `ze data check` |
| Indirection in `ReadVersion`, `ReadFile` raw | dereference inside `ReadFile` for entry keys | a prefix branch in the generic read would make `ze data cat`, backup, import and check disagree about what a key holds; three call sites already have the stamp |
| No migration | value-sniffing or key-based legacy detection with an on-open pass | pre-release: no store on `main` holds history anyone keeps; a migration is a spec of its own the day one does |
| `sha256:<hex>` entry spelling | bare 64-hex | the repository already spells digests this way (`writeConfigActiveHash`) |
| Objects reached by raw key inside package `storage` | teach `isNamespaced` and `resolveDirKey` the `object/` prefix | the resolver's contract is carried by both encodings and by `CheckName`; extending it for a namespace no caller outside the core names buys nothing and risks the silent `file/active/` rewrite everywhere else |
| The guard gains the raw-key four | run the sweep after `Release` with its own lock | a removal that is not atomic can lose an object between the entry delete and the sweep; and a `Storage` call inside a guard hangs rather than errors, so the safe route must exist on the guard |
| `VersionInfo.Path` kept as the entry key | delete the field | it still names a real key, and deleting it changes `ze config history` text and the `data history` JSON `keyPath` for no gain |
| An existing object is verified before it is referenced | trust the key's existence | a CRC proves the bytes are what was written, never that they are what the name promises; `ze data write` and `repair` can both produce a wrong-hash object |

## Known Limitations

- Objects are per store; a backup carries them and a restore brings them back, but nothing deduplicates across stores or devices.
- Encryption of objects is `spec-config-at-rest-encryption`'s question, like every other key.
- No migration path from a copy-based store; one is written when a store outside this repository needs it.
- A crash in the legacy arm of `PromoteCandidate`, between `guard.WriteVersion` and the `rollback` pointer write, leaves a history entry no pointer names. That is an orphan of the same class as R-1 and `ze data check` reports it; it is not introduced by this spec.

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
