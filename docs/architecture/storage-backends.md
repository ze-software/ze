# Configuration storage

The live configuration store is a directory named `database` beside the config
file. The storage package owns key mapping, version pointers, metadata and write
notifications for both this tree and explicitly opened ZeFS artifacts. Callers
use `Storage`; they never inspect its encoding. The contract itself
(`Storage`, `WriteGuard`, `FileMeta`, `VersionInfo`) is declared once in the
leaf tier, in `internal/core/statestore`, and this package aliases those names:
a core consumer holds the handle without importing the component that
implements it.
<!-- source: internal/core/statestore/storage.go -- Storage, WriteGuard -->
<!-- source: internal/component/config/storage/storage.go -- Storage, WriteGuard -->
<!-- source: internal/component/config/storage/store.go -- store, guard -->

## Opening and ownership

`Open(configDir)` opens the live tree and holds `configDir/database.lock` until
`Close`. A second writer receives `ErrBusy`, including during startup and
replacement. The lock is outside the replaceable tree and its inode is retained.
`OpenReadOnly` permits concurrent inspection and rejects mutation with
`ErrReadOnly`. Read-only inspection sees individually published keys, without a
multi-key snapshot guarantee.

Both openers first stat `database.zefs`. Its presence, even beside a valid tree,
refuses the open and names `ze init --from <blob>`; no blob contents are read and
nothing is renamed. Absence of both forms returns `ErrNoStore` and names `ze init`.
Before either check, every opener reads `database.import-intent`: while one sits
in the folder, beside an absent tree or an existing one, the import or restore it
records is unfinished, and every opener returns `ErrImportPending` naming the
intent file, its source and its recovery command (`ze init --from <source>` or
`ze data restore <source> full`). An unreadable intent refuses the same way. The
one exception is a retiring import whose new tree is published and whose source
is already archived: that tree is the store, and the intent waits for the
recovery command to remove it. `Create` and
`CreatePopulated` refuse the same way, under the owner lock, so neither `ze
start` nor a plain `ze init` publishes an empty tree over the operator's store;
`ze init --from` resumes the import. Only `ReplacePopulated` (`ze init --force
--yes`) proceeds, because replacing whatever sits there is its explicit order.
The live opener validates the tree before it creates `database.lock`, so a
refused open leaves no lock file behind.
<!-- source: internal/component/config/storage/open.go -- Open, OpenReadOnly, detect, openLive, lockOwner, populateOwned -->

`ze init --from <source>` imports a blob through `ImportBlob`. A local path is
read in place. An `http` or `https` source is fetched first by the shared
helper (`internal/core/fetch`) into a `database.fetch-*` folder beside the
store, checked against `--sha256 <hex>` when given, then imported; the folder,
with the fetched copy and its retired name, is removed afterwards. It stays only
when the import stopped after recording its intent
(`PendingImportSource`), because the recovery command names that copy. Under
`--force --yes` it calls `ReplaceImportBlob`, which moves an existing tree, and
an unrelated seed, to `.replaced-<stamp>` before publication. `--from` reads no
credentials and refuses, each by name, the flags that shape a new store:
`--seed`, `--managed`, `--web-cert` and `--web-cert-name`. The URL form and
`--sha256` are storage-2.
Appliance first boot imports its seed through the same API.
<!-- source: internal/plugins/init/main.go -- Run, runImport -->
<!-- source: cmd/ze/ze_core_autoinit.go -- gokrazyAutoInit -->

The tree root and its intermediate directories are exactly 0700; regular frame
files are exactly 0600. All belong to the effective process user, including when
that process is root. Symlinks and non-regular leaves are refused before content
reads. The containing config directory and every ancestor keep whatever mode and
owner they have: they are not part of the store, and the root's 0700 and owner
check is what bounds it (owner decision, 2026-09-17). Ancestors are traversed
with descriptor-relative, no-follow opens, so a symlink on the way to the root
is refused rather than followed. `zefs.OpenDirectory` is the one such walk: the
store opener, tree checks and tree repair all call it, and every refusal wraps
`fs.ErrPermission`. Errors identify the first unsafe path and the permission or
ownership repair.

`TransferOwnership` is an explicit privileged installer operation. It takes the
same stable lock, prepares and validates its iterative traversal before changing
owners, then transfers the folder and its descendants without reading secrets.
A failed transfer attempts to restore the original ownership and reports repair
instructions. Ordinary opens never transfer ownership.
<!-- source: internal/component/config/storage/tree.go -- openFolder, openFolderMode, openNode, secureNode -->
<!-- source: pkg/zefs/check_tree_unix.go -- OpenDirectory -->
<!-- source: internal/component/config/storage/ownership.go -- TransferOwnership -->

## Keys and reads

A logical key maps unchanged to `database/<key>`. Each file holds exactly one
netcapstring, including its CRC32c. Reads reject bad checksums, truncation and any
bytes after the frame. Tree traversal is iterative and follows filesystem path
limits. Values are limited by addressable memory; the existing ZeFS blob import
limit remains 256 MiB.

Config operations map a bare name to `file/active/<name>`. An explicit path must
belong to that store's config directory. Belonging is decided by directory
IDENTITY, not by the spelling of the path: the check stats both directories and
accepts them when they are the same one. A store folder and a caller's path
disagree in text whenever a symlink stands between them, and the framed-tree
opener canonicalizes its own folder, so a text comparison refuses a config that
is inside the store. A directory that cannot be stat'ed is refused. Already namespaced `file/` and `meta/`
keys pass through unchanged. `List` returns immediate file children as keys that
`ReadFile` accepts unchanged. `ReadKey`, `WriteKey`, `RemoveKey` and `ListKeys`
bypass config-name mapping. `ListKeys` recursively returns sorted keys matching
its literal prefix, including partial leaf prefixes; an empty prefix selects
all keys.

Unlocked reads return caller-owned bytes. A guard's reads may reference the blob
mapping and MUST NOT outlive `Release`. All operations inside a guarded section
MUST use that guard, including existence checks and `List`. Guarded lists see
pending writes without acquiring the store lock again. The guard carries the
raw-key pairs too: `guard.ReadKey` and `guard.ListKeys` answer what
`Storage.ReadKey` and `Storage.ListKeys` answer, literal-prefix, recursive and
sorted, in the same order on both encodings, and `guard.WriteKey` and
`guard.RemoveKey` do what `Storage.WriteKey` and `Storage.RemoveKey` do. A
caller holding a guard MUST use them: the `Storage` methods take the store
mutex the guard already holds, so calling one inside a guard hangs the process
rather than returning an error. A walk over the whole store, such as a
backup, is `guard.ListKeys("")` then `guard.ReadKey` for each key; `List` cannot
serve it, because it answers only the immediate file children of a resolved
name. In-process metadata records
modification time and modifier identity on both encodings; it is not a durable
audit log. Observers receive resolved keys after successful publication and lock
release, so a callback may read the store again.
<!-- source: internal/component/config/storage/tree.go -- treeEncoding.ReadFile, treeEncoding.list -->
<!-- source: internal/component/config/storage/store.go -- CheckName, ListKeys, List, guard.ReadKey, guard.ListKeys, guard.WriteKey, guard.RemoveKey, guard.Release -->

## Durable publication

Tree writes stage a 0600 file in the containing config directory, outside the
logical key tree. The writer syncs the frame, renames it into the destination and
syncs both affected directories. Newly created key ancestors are synced before
publication can proceed. Sync failures are returned to the caller. Leftover
`.ze-storage-*` files are outside the namespace and cannot appear in list, check
or repair output. Removing a key prunes empty key directories.
Staging creation, publication and cleanup all use retained directory descriptors.
Renaming or replacing the containing directory's pathname does not redirect
these operations into the replacement directory.

`Create` auto-creates an empty tree. `CreatePopulated` runs its callback against a
private `database.init-tmp-*` tree and publishes only after successful population.
Publication uses the operating system's no-replace rename, so an existing empty
directory is never overwritten. Init refuses an existing store. An auto-create
loser can reopen the winner after its writer closes; while it remains owned,
`ErrBusy` preserves the one-writer rule.

`ReplacePopulated` prepares the replacement before moving the previous store to
`database.replaced-<stamp>` and publishing the new tree under the same ownership
lock. Backups remain available if a later publication step fails.
<!-- source: internal/component/config/storage/tree.go -- installBytes, treeEncoding.parent, treeEncoding.Remove -->
<!-- source: internal/component/config/storage/open.go -- Create, CreatePopulated, ReplacePopulated, populateOwned -->
<!-- source: pkg/zefs/check_tree_linux.go -- RenameNoReplace -->
<!-- source: pkg/zefs/check_tree_darwin.go -- RenameNoReplace -->

`zefs.RenameNoReplace` is the one no-replace rename; the storage package has
none of its own. Linux uses `renameat2` with `RENAME_NOREPLACE`; macOS uses
`renameatx_np` with `RENAME_EXCL`. FreeBSD 14 has no `renameat2`: a regular
file is hard-linked to the target with `linkat`, which refuses an existing
name, then the source name is unlinked; a directory is claimed with `mkdirat`,
which refuses an existing name, then `renameat` replaces only that empty
placeholder. The FreeBSD path is cross-compiled and vetted, not run. Publication
fails if the kernel or filesystem lacks the no-replace operation. It never falls
back to an overwriting rename. The FreeBSD directory path has one window Linux
and macOS do not: a crash between `mkdirat` and `renameat` leaves an empty
`database` placeholder beside the complete stage (`database.init-tmp-*` or
`database.import-tmp-*`), and the next open accepts that placeholder as an
empty store, because an empty directory is also what `ze init` with nothing to
seed publishes. An operator who finds an empty store beside a stage on FreeBSD
removes the placeholder and re-runs the `ze init` that was interrupted.
<!-- source: pkg/zefs/check_tree_freebsd.go -- RenameNoReplace -->

Secure directory traversal and tree publication are supported only on Linux,
Darwin and FreeBSD. On other platforms, including DragonFly,
`zefs.OpenDirectory` and `zefs.RenameNoReplace` retain their public signatures
but return the same unsupported-platform error as framed-tree integrity and
repair. These two operations do not inspect or alter the filesystem, and an
unsupported directory open never returns a descriptor. Compiling a consumer
there is not a secure-storage port or evidence of runtime support.
<!-- source: pkg/zefs/check_tree_other.go -- OpenDirectory, RenameNoReplace, walkFrameTree, repairFrameTree -->

## Import and artifacts

`ImportBlob` (`ze init --from`) and `RestoreBlob` (`ze data restore <file> full`)
are the two blob-to-live-tree operations, and they share one protocol. They
differ only in the source policy the intent records: `retire-source` archives
the artifact as `<source>.replaced-<stamp>`, and `keep-source` leaves it where it
is. A restore always replaces; an import replaces only under `--force`.

Each checks the source, copies every raw key into a private
`database.import-tmp-*` stage, compares both key sets and every value, syncs the
stage, and then writes one immutable `database.import-intent` before any
destination moves. The intent records the policy, the source and its digest, the
archive name (`retire-source` only), the stage name and its device/inode, and one
descriptor for each previous destination: the old `database` tree and an
unrelated `database.zefs`, each either `absent` or `present` with its device,
inode and the exact `.replaced-<stamp>` name it will take. An init of the
canonical seed itself records the seed as `source`. Every retirement name is
chosen once and checked free before the intent is written. A missing or unknown
field refuses: absence is never implied.

Progress is never stored. Replay classifies where each recorded identity sits
now: the old tree at `database` or at its retirement name, the seed likewise,
the new tree at its stage or at `database`, and the source at its name or its
archive. Only a state the protocol can produce is accepted, and it is accepted
before anything moves. Replay then verifies the stage or the published tree
against the source, moves the old tree and then the seed with a no-replace
rename, publishes the stage, and completes by policy, syncing the folder after
each effect. An identity at the wrong name, both or neither location of a
recorded node, a missing stage without a published tree, a changed source, a
policy or source that differs from the command, or an unrelated replacement
refuses without moving or deleting anything; each refusal names
`database.import-intent`, the conflict and the recovery command. A new import
removes every `database.import-tmp-*` stage before it builds its own; a replay
never does, and never chooses another stage or stamp.

The importer alone retires the source to `.replaced-<stamp>`, moves the source's
`.lock` file beside the archive (every blob artifact keeps its lock next to it, and
the held lock refuses a concurrent creator of the retired name until it moves),
syncs its directory, and then removes the intent, in that order, so a crash between
the steps leaves a state the next import recognizes as finished. If a crash leaves both tree and
seed, ordinary `Open` still refuses; repeating the explicit importer completes
retirement. Incomplete private staging names never count as a live store.
<!-- source: internal/component/config/storage/import.go -- ImportBlob, ReplaceImportBlob, RestoreBlob, importOwned, startImport, removeStaleStages, retireSource -->
<!-- source: internal/component/config/storage/import_replay.go -- importIntent, classifyImport, replayImport, finishImport, pendingImport -->

`OpenBlob` and `CreateBlob` address explicit artifacts. Their format remains the
ZeFS format documented in [zefs-format.md](zefs-format.md).
Blob readers take shared artifact locks and writers take exclusive locks, because
an artifact writer can update its mmap-backed file in place. Concurrent live-tree
inspection remains safe without blocking its writer.
`CreateBlobPopulated` seeds an unpublished artifact and optionally retains a
replacement backup. Creating `database.zefs` beside a live tree is refused.
Artifact files require regular 0600 owner-controlled inodes, but their output
directory may be an ordinary directory. `OpenTree` addresses an exact offline
tree path, including repair output; the canonical `database` name uses the live
opener and the same owner lock.
<!-- source: internal/component/config/storage/blob.go -- OpenBlob, CreateBlob, CreateBlobPopulated -->
<!-- source: internal/component/config/storage/open.go -- OpenTree -->

### Restoring a config from an artifact

`ReadRestoreSource` runs the artifact check (`checkArtifact`, the same
`zefs.Check` verdict import uses), opens the artifact read-only through
`OpenBlob`, and selects one config: the name the operator gave, else the only
config, else the one named like the device, else a refusal listing every name.
A config is a name with a `meta/config/<name>/active` pointer or a
`file/active/<name>` mirror; the bytes are the active version, read entry to
object with the hash verified, or the mirror when there is no pointer. A source
whose active entry names an object the artifact lacks is refused before any
write, naming the hash. `Backup` and import write every `object/*` key before
any other key, so an interrupted walk leaves an unreferenced object, never an
entry naming a missing one. Offline, `RestoreConfig` writes the candidate and
promotes it under ONE guard, so a crash leaves either nothing or a normal
candidate; a failure before the active pointer moves withdraws the candidate
and its version. The daemon's `request data restore` stages the same bytes as
the candidate and runs its SIGHUP reload, which promotes last; it is refused
while a confirmed-commit window is open (`confirm.WriteOutside`), because the
window's revert would wipe it. `request data
backup` runs `Backup` over the daemon's own bound handle (`ownedStore` looks
through `BindConfigSource`), under the lock every commit takes.
<!-- source: internal/component/config/storage/restore.go -- ReadRestoreSource, RestoreConfig -->
<!-- source: internal/component/config/storage/backup.go -- Backup, ownedStore, CheckArtifactPath -->

## Version pointers and recovery

Pointers are per configuration name: `meta/config/<name>/active`, `candidate`,
`rollback` and `recovery`. A durable version precedes its pointer. Promotion
records rollback before publishing active, then refreshes the active mirror and
clears candidate. Repeating a promotion after active was published preserves
the existing rollback reference. When the active pointer names a version whose
entry or object is absent (a repaired store), promotion leaves rollback where it
is instead of recording the unresolvable stamp; any other resolution failure
aborts the promotion. Candidate cleanup retains any version still referenced by
active, rollback or recovery, including after a crash between pointer
publications, and clears a candidate pointer whose entry is already absent.
Nameless legacy pointers are never read.

### Content-addressed history

History is content-addressed. A version's bytes are stored once, under
`object/<hex>`, where `<hex>` is the lowercase SHA-256 of the bytes. The dated
entry `file/<stamp>/<name>` holds `sha256:<hex>` (71 characters), never a copy.
Two commits of an unchanged config, or two names holding equal bytes, share one
object. Only dated history is content-addressed: `file/active`, `file/draft`,
`file/template` and `meta/` stay direct keys. Every key, objects included,
keeps its frame CRC. The CRC proves a frame holds the bytes that were written;
the hash proves they are the content the entry promised.

| Operation | Behavior |
|-----------|----------|
| `WriteVersion` | Hash the bytes, write the object when absent, then the entry. An existing object is reused only when its stored bytes hash to its name, else `ErrHistoryObject` names the key and both hashes and no entry is written. Object before entry, so a crash leaves an orphan object, never a dangling entry |
| `ReadVersion(name, stamp)` | Entry, then object, then the hash verified over the bytes. A missing entry or object keeps `fs.ErrNotExist`; a malformed entry value or a wrong-hash object is `ErrHistoryObject`. `ReadActiveConfig` names `meta/config/<name>/active` and the stamp, and never falls back to the mirror while the pointer exists |
| Removal | Under the caller's guard: a stamp a pointer names is retained, entry and object. Otherwise the entry is deleted, then every dated entry of every name is read through `guard.ListKeys("file/")` and `guard.ReadKey`, and the object is deleted only when none names it. Mutable `file/active` values never retain an object |
| `ReadFile`, `ReadKey` | Stay raw: an entry reads as `sha256:<hex>`, so `ze data cat`, backup, import and check see what is stored |
| `CheckHistory`, `RepairHistory` | One walk over every raw key: each object hashed, each entry resolved, each of the four pointers of a name resolved. An unreferenced object is a warning; a dangling entry, a malformed entry, a wrong-hash object and a pointer naming a missing entry are errors. `RepairHistory` drops the first three kinds of error key, keeps orphans, and never retargets a pointer. A listed key that does not read stops the walk with an error naming it: a read failure is never reported, or repaired, as absence. `ze data check` therefore walks history only when the frame check found no corrupt frame |

Objects are reached by raw key only, from inside package `storage`. The
name-based API would move them: `resolveKey` maps a name outside `meta/` and
`file/` to `file/active/<base>`. `VersionInfo.Path` stays the entry key, so
`ze config history` prints it; callers holding it read the bytes through
`ReadVersionEntry`. There is no migration and no reader for copy-style entries:
Ze is pre-release, and a store written before this format is re-initialised.

Explicit-file source selection is retained by the runtime. `WriteConfigFile`
provides durable loose-file publication through a no-follow parent traversal;
the caller owns conflict detection and the persistent file-commit intent.
Bare stored-config startup reads
the active pointer, falling back to the active mirror only when no pointer exists.
<!-- source: internal/component/config/storage/pointer.go -- PromoteCandidate, ClearCandidate, ReadActiveConfig, pointerPath, removeVersionLocked -->
<!-- source: internal/component/config/storage/history.go -- writeVersionObject, readVersionEntry, sweepObject, objectsFirst, ReadVersionEntry -->
<!-- source: internal/component/config/storage/history_check.go -- CheckHistory, RepairHistory, inspectHistory -->
<!-- source: internal/component/config/storage/open.go -- WriteConfigFile -->
<!-- source: pkg/zefs/keys.go -- KeyConfigActive, KeyConfigFileCommit -->
