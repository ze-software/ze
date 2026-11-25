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

### The memory encoding

`CreateMemory` returns a store whose keys live in a map and die with the
process. It is the editor test harness's tree-mode store (`./le test editor`),
and the store a functional-test DUT creates when it finds none: the `zetest`
DUT build reads `ze.test.storage=memory`, which the runner sets unless a `.ci`
carries `option=storage:value=disk`. A shipped daemon never uses it, because
it keeps nothing across a restart. Only the key space differs;
name mapping, guards, versions, metadata and observers are the store's own code,
the same that runs over the tree. It refuses a key that would be both a value
and a directory, as the tree does, returns caller-owned copies, and accepts a
config name in any directory, as a blob does, because it has no folder to
compare the name with. For the same reason a backup taken from it refuses no
target name, as from an artifact source. The conformance table every encoding
passes includes it.
<!-- source: cmd/ze/register_store_zetest.go -- createTestStore -->
<!-- source: internal/test/runner/record_dut_store.go -- dutStore -->
<!-- source: internal/component/config/storage/memory.go -- CreateMemory, memoryEncoding -->
<!-- source: internal/component/config/storage/store.go -- keyEncoding -->
<!-- source: internal/component/config/storage/backup.go -- refuseStoreName -->

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

`OpenReadOnly(dir, shown)` takes the folder twice. Every file is opened under
`dir`, and every refusal of the open (the pending intent, the blob, the missing
tree, an entry it cannot check, a folder it cannot open) names the folder as
`shown`. A daemon handler answering a remote caller opens the store beside the
path it resolved and passes the folder of the path the caller typed, so the
caller reads the refusal the local command prints and never a path it did not
type. A caller with one spelling passes it twice.

A folder that cannot be opened is named by the component the walk refused
(`zefs.ComponentError`). The store folder itself is named as `shown` on every
route. A folder on the path to it, such as a symbolic link or a parent with no
search access, keeps zefs's own refusal, which names it by its absolute path,
when `shown` and `dir` are the same spelling; otherwise the refusal says only
that a folder on the path to `shown` cannot be opened, with its cause.
A failure that holds no component (the working directory, the root, or the
close of a folder the walk holds) is passed through on one spelling; with
another, it names `shown` and the bare causes, never a path the walk built.
<!-- source: internal/component/config/storage/open.go -- Open, OpenReadOnly, storeFolder, detect, openLive, lockOwner, populateOwned -->
<!-- source: internal/component/config/cli/config_data.go -- withRuntimeStore -->

`ze init --from <source>` imports a blob through `ImportBlob`. A local path is
read in place. An `http`, `https` or `tftp` source is fetched first by the shared
helper (`internal/core/fetch`) into a `database.fetch-*` folder beside the
store, checked against `--sha256 <hex>` when given, then imported; the folder,
with the fetched copy and its retired name, is removed afterwards. It stays only
when the import stopped after recording its intent
(`PendingImportSource`), because the recovery command names that copy. Under
`--force --yes` it calls `ReplaceImportBlob`, which moves an existing tree, and
an unrelated seed, to `.replaced-<stamp>` before publication. `--from` reads no
credentials and refuses, each by name, the flags that shape a new store:
`--seed`, `--managed`, `--web-cert`, `--web-cert-name` and `--confirm`. The URL form and
`--sha256` are storage-2.
Appliance first boot imports its seed through the same API.
<!-- source: internal/plugins/init/main.go -- Run, runImport, profileFromFlags -->
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

`AcquireMaintenance` holds the same exclusive lock without opening store data.
The native installer uses it during executable replacement and removal.
The folder owner or root can take it, after descriptor-safe path validation.
This permits root to maintain a service-owned installation without granting
root a store-data opener. The caller closes the lock before ownership transfer
or daemon startup. A live owner refuses maintenance.
<!-- source: internal/component/config/storage/maintenance_linux.go -- AcquireMaintenance -->

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

Committed configuration readers use `ReadActiveConfig`, including stored editor
construction and backup show and diff. An active pointer selects its verified
version even when the raw active mirror is missing or stale. A pointer that
cannot resolve, or an object whose digest does not match, refuses the read;
the raw mirror does not replace that failure. Editor resume, replay, review,
discard and commit use `ReadActiveConfigWithGuard` while holding the existing
configuration guard. Drafts and per-account change records still use raw file
reads. Explicit loose-file editors keep their filesystem source.
<!-- source: internal/component/config/storage/pointer.go -- ReadActiveConfig, ReadActiveConfigWithGuard -->
<!-- source: internal/component/cli/editor.go -- NewEditorWithStorage, NewLooseFileEditor -->
<!-- source: internal/component/config/cli/cmd_show.go -- openBackupShowEditor -->
<!-- source: internal/component/config/cli/cmd_diff.go -- readDiffSource -->

Default editor discovery uses `ArtifactConfigNames`, which includes pointer-only
configurations, before choosing or creating a config. A direct dirty stored
`Editor.Save` with an active pointer requires `SetCommitWriter`: the owning
publisher decides live validation/reload versus offline promotion. Without that
callback, Save refuses before serialization, backup or writes instead of
reporting a mirror-only write as a committed version. Legacy mirror-only saves
and explicit loose-file saves retain their existing behavior.
<!-- source: internal/component/config/cli/cmd_edit.go -- doSelectConfig -->
<!-- source: internal/component/cli/editor_commands.go -- Save -->
<!-- source: internal/component/cli/editor_file.go -- checkSaveAuthority -->

Unlocked reads return caller-owned bytes. A guard's reads may reference the blob
mapping and MUST NOT outlive `Release`. All operations inside a guarded section
MUST use that guard, including existence checks and `List`. Guarded lists see
pending writes without acquiring the store lock again. The guard carries the
raw-key pairs too: `guard.ReadKey` and `guard.ListKeys` answer what
`Storage.ReadKey` and `Storage.ListKeys` answer, literal-prefix, recursive and
sorted, in the same order on both encodings, and `guard.WriteKey` and
`guard.RemoveKey` do what `Storage.WriteKey` and `Storage.RemoveKey` do. A
caller holding a guard MUST use them: a `Storage` write, `ReadVersion` and
`Stat` wait for the lock the guard already holds, and on a blob every
`Storage` read waits for the blob mutex its `WriteLock` holds, so calling one
inside a guard hangs the process rather than returning an error. A walk over
the whole store, such as a backup, is `guard.ListKeys("")` then `guard.ReadKey` for each key; `List` cannot
serve it, because it answers only the immediate file children of a resolved
name. In-process metadata records
modification time and modifier identity on both encodings; it is not a durable
audit log. Observers receive resolved keys after successful publication and lock
release, so a callback may read the store again. A guard reports each key
it wrote and each runtime-state key it removed. Removing a configuration
key leaves the configuration write observer silent. The returned detach
function waits for callbacks admitted before removal. It can be called after a
replacement has been installed and removes only the original registration.

A runtime owner can subscribe to a prefix with `SetRuntimeObserver`. It sees
each successful write or deletion while the runtime lock still orders changes
to the key. Its callback may update memory, and must return without accessing
storage or waiting on a writer. Multiple owners can subscribe to overlapping
prefixes. A guard can publish each of its runtime changes directly to a
callback, even when the store has no subscribers. A writer passes its own
subscription to the guard to avoid observing its save twice. Every other
matching subscription still receives the change. Unsubscribing removes just
that registration. The general write observer receives the key after unlock.

An unguarded read does not wait for a guard. Three locks order the store
(`store` in `store.go`): a guard holds the configuration lock (`writer`) or
the runtime-state lock (`runtime`), or both, from `AcquireLock` to `Release`,
so writers of one key space run one at a time, and a read holds only a short
lock (`mu`) that keeps `Close` from releasing the encoding under it. A single-key read
needs no writer to finish, because the tree replaces a key's bytes with one
rename and the memory encoding locks its own map. A listing of the tree skips
a directory that a concurrent removal prunes under it, so it holds every key
present for the whole walk and never fails for that. A read can therefore see a
guarded transaction part done, a state a crash could also leave and the one a
reader in another process already sees; a reader that needs two keys to
agree takes a guard. `ReadVersion` holds the configuration lock shared, so no removal
deletes the object between its entry read and its object read. `Stat` holds it
shared too, because a guard writes a key's bytes at once but publishes its
metadata only at `Release`: without it, `Stat` could pair new bytes with the
old modification time, and the CLI editor would take its own draft for another
session's change. Only the editor's draft checks call `Stat`, and a draft is a
configuration key, which only a guard holding that lock writes. A read used to
wait for the whole guard, including every sync of keys it never asked for, so
a plugin's runtime-state read at startup passed its deadline behind the boot
record (below). A blob keeps its own rule: its readers wait for its writer,
because the writer updates the mapped file in place.

Writers are split into two key spaces, each with its own write lock. Runtime
state is every `meta/` key outside `meta/config/`: the keys statestore and the
plugin state RPCs write. Configuration is everything else: config files,
`file/` history, `object/` content and the `meta/config/` pointers. A guard
acquired with a runtime-state key holds the runtime lock alone and is refused
any configuration key. Any other guard holds the configuration lock, and takes
the runtime lock the first time it reads, writes or lists something that can
be runtime state, keeping it to `Release`; a backup's `guard.ListKeys("")`
therefore still freezes the whole store. The locks are taken configuration
first, so the two kinds cannot deadlock. The split exists because the boot
record's configuration writes sync for up to a minute on a slow disk once the
hub serves, and a plugin's state write in that window carries a deadline of
its own: OSPF persists a planned graceful restart's fact under five seconds,
and `request ospf graceful-restart` failed with "context deadline exceeded"
while the fact still landed afterwards
(`test/plugin/dut-store-slow-boot-record-ospf-graceful-restart.ci`,
`TestIncrementDoesNotWaitForAConfigurationGuard`). The two kinds share only
the `meta` directory, so the tree orders directory creation against pruning
with a short lock of its own. A blob store takes both locks for every guard,
because its one `WriteLock` serializes it anyway.
<!-- source: internal/component/config/storage/tree.go -- treeEncoding.ReadFile, treeEncoding.list, treeEncoding.parent, treeEncoding.walk, treeEncoding.Remove -->
<!-- source: internal/component/config/storage/store.go -- CheckName, ListKeys, List, guard.ReadKey, guard.ListKeys, guard.WriteKey, guard.RemoveKey, guard.Release -->
<!-- source: internal/component/config/storage/store.go -- store, acquire, RuntimeKey, guard.reach, guard.reachPrefix, guard.holdRuntime, ReadVersion, Stat, Close -->
<!-- source: internal/component/config/storage/memory.go -- memoryEncoding -->

## Durable publication

Tree writes stage a 0600 file in the containing config directory, outside the
logical key tree. The writer syncs the frame, renames it into the destination and
syncs the destination directory: two barriers per write. The staging directory is
not synced, because the only change a sync would make durable there is the
temporary name's departure, and a leftover `.ze-storage-*` file is outside the
namespace and cannot appear in list, check or repair output. Newly created key
ancestors are synced before publication can proceed. Sync failures are returned
to the caller. Removing a key prunes empty key directories.

A staged frame is named `.ze-storage-<tree>.<random>`, where `<tree>` is the
tree whose owner lock covers the write: `database` for the live tree, for a
stage built under its lock, and for a loose config the live owner publishes;
the tree's own name for an offline tree opened writable. Every write that
stages a frame holds such a lock, except a memory store's loose config write
(described with `WriteConfigFile` below). The random part is base32 and holds no dot,
so a name always says which lock it belongs to, even when one tree's name
extends another's. A crash between the staging create and the rename leaves the
frame behind. The next writable open of that tree removes every regular file
named for it, after it takes the owner lock, so none of them can belong to a
live write. The live owner also removes the older form `.ze-storage-<random>`,
which no write produces any more. Frames named for another tree in the same
folder, and every other entry, are left alone. A read-only open removes
nothing. The removals are not synced: a crash that loses one brings back a name
no reader lists, and the next owner open removes it again. The sweep is
best-effort: a leftover it cannot inspect or remove is logged as a warning
naming its path and the `rm` command that removes it, and the open proceeds,
because a leftover holds no published data.
<!-- source: internal/component/config/storage/tree.go -- stageName, stagedFor, stagedLegacy, sweepStages, warnLeftover -->
<!-- source: internal/component/config/storage/open.go -- openLive, OpenTree, liveLeftover, WriteConfigFile -->


Staging creation, publication and cleanup all use retained directory descriptors.
Renaming or replacing the containing directory's pathname does not redirect
these operations into the replacement directory.

Taking a store lock does not sync its containing folder. Its stable inode
coordinates live owners through kernel `flock` state; after a host crash every
flock is gone, so a missing lock file can be recreated safely. Stored values,
tree publication, import recovery and ownership changes retain their own
durability barriers.
<!-- source: internal/component/config/storage/open.go -- lockStoreFile, openLockNode -->

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

The session init child uses `CreateSeedIn` and `ImportSeedIn` with its admitted
root and folder. These fixed-purpose entry points operate only on the canonical
seed and live tree. They compare both acquired directory identities and never resolve
the durable folder label for I/O.
<!-- source: internal/component/config/storage/seed.go -- CreateSeedIn, ImportSeedIn -->

`ImportSeedIn` uses the same import/replay
protocol and keeps the original absolute Source/Archive labels after a directory
move. An intent naming another source refuses before any source access. Callers
retain the borrowed handles through the call and close the returned store.
Ordinary init keeps its path-based entry points and credential population.
<!-- source: internal/component/config/storage/seed.go -- CreateSeedIn, ImportSeedIn -->
<!-- source: internal/plugins/init/seed.go -- runSeedChild -->

`ImportBlob` (`ze init --from`) and `RestoreBlob` (`ze data restore <file> full`)
are the two blob-to-live-tree operations, and they share one protocol. They
differ only in the source policy the intent records: `retire-source` archives
the artifact as `<source>.replaced-<stamp>`, and `keep-source` leaves it where it
is. A restore always replaces; an import replaces only under `--force`.

`DaemonRecovery` extends that same protocol across a daemon lifetime. The outer
launcher owns the bound handle; command handlers borrow its `RecoveryAdmission`.
Admission copies one source into a private `database.recovery-tmp-*` directory,
checks the complete artifact and records its digest before accepting it. Its
0700 directory and 0600 file belong to recovery rather than a browser workspace.
The caller's source can change or disappear afterward without changing the
accepted bytes. Cancellation before acceptance removes the stage; cancellation
after acceptance does not withdraw it. A second accepted source is refused.
Admission also refuses a retained recovery stage from an earlier lifetime or
failed cleanup, naming it before it writes anything. This bounds retained full
artifacts across retries rather than accumulating another copy each time.

Before acceptance, the stage also durably writes `recovery-owner.json`. This
versioned ownership record binds the target directory, stage directory,
snapshot file and empty source-lock identities, together with the exact
snapshot size and digest. Its source and target paths are labels, never
permission to replace whatever later occupies a pathname. The bounded private
record has one canonical JSON representation; missing, unknown or duplicate
fields, an unsupported version, unsafe node types or modes, changed identities
or bytes, and extra directory entries refuse recovery. It is ownership
metadata, not another import journal or store decoder.

Only a successful `DaemonRecovery.Close`, after every daemon borrower returns,
produces `ClosedRecovery`. Its `Apply` verifies the accepted digest again and
uses the existing keep-source import protocol under a newly acquired owner lock.
The destination and snapshot directories remain open by identity, so moving a
directory and replacing its pathname cannot redirect the restore. Another writer
that acquires the store first refuses recovery. On an unclean daemon result the
outer launcher uses `Discard` instead. A failed close or import retains its
snapshot and names the recovery command; no source an import intent can reference
is deleted. A completed restore removes the private source. Blob and memory
handles keep their normal lifetime but admit no full live-tree replacement.
<!-- source: internal/component/config/storage/daemon_recovery.go -- NewDaemonRecovery, RecoveryFor, RecoveryAdmission.StageWriter, DaemonRecovery.Close, ClosedRecovery.Apply, ClosedRecovery.Discard -->
<!-- source: cmd/ze/ze_core_start_recovery.go -- runOwnedDaemonStore, daemonStoreFailure -->
<!-- source: cmd/ze/ze_core_start.go -- cmdStart -->
<!-- source: internal/component/config/storage/daemon_recovery_snapshot.go -- buildRecoverySnapshot, applyRecovery, applyRecoveryStore -->

A retained owned source is resolved explicitly with
`ze data --path <actual-config-directory>/database restore <actual-private-stage>/snapshot.zefs full`
after all store owners stop. The diagnostic quotes both exact paths. The native
full-restore adapter recognizes the reserved private namespace and calls
`ResumeRecovery`; invalid or absent ownership refuses rather than falling
through to generic restore. Resume verifies the same ownership under the
existing importer's source lock, runs that same keep-source import/replay,
closes the returned writer, and only then removes its proven private entries.
A failed import or imported-writer close preserves the source. `RestoreBlob`
itself always keeps its caller's artifact, including a caller that deliberately
uses that generic API on a private snapshot. No forwarding storage wrapper is
needed: explicit resume owns the complete import-and-close operation.

After a directory move, resume opens the actual current source and target
directories and verifies their identities. Its retained source directory/root
uses the immutable original source label recorded by ownership, allowing an
existing immutable import intent to replay without rewriting it. A stale target
pathname pointing to a replacement directory refuses. Doctor opened against
the actual moved store produces a fresh command for its current paths.

Cleanup never sweeps recovery-looking directory names. It checks the exact
owned stage and its three known entries before unlinking them. An unknown,
tampered or partially cleaned stage remains unresolved and blocks further
admission. To resolve that inspection debt, stop all store owners, verify which
live store is intended, and preserve the entire unresolved directory by moving
it outside the configuration directory without overwriting any destination.
Do not delete individual unknown files. Doctor explains this owner action;
once the directory is preserved elsewhere, new recovery admission is available.
<!-- source: internal/component/config/storage/daemon_recovery_ownership.go -- recoveryOwnership, writeRecoveryOwnership, readRecoveryOwnership, verifyRecoverySource, verifyRecoveryNames -->
<!-- source: internal/component/config/storage/daemon_recovery_manual.go -- RecoverySource, ResumeRecovery, openRecoverySnapshot, PendingRecoveryCommand -->
<!-- source: internal/component/config/storage/daemon_recovery_snapshot.go -- completeRecoveryImport, removeRecoverySnapshot, removeRecoveryNodes -->
<!-- source: internal/component/config/storage/cli/cmd_restore.go -- cmdRestoreFullThrough -->

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

Blob creation and opening pair the no-follow folder with an `os.Root`, checking
both opened directory identities before use. Staging, blob writes and reopening
after publication retain that root. A replaced directory pathname cannot redirect
those operations.
<!-- source: internal/component/config/storage/blob.go -- blobRoot, populateBlob, OpenBlob -->

Import opens its source directory once and retains both its
no-follow folder and its ZeFS root through validation, hashing, replay and source
retirement. Intent Source and Archive remain ordinary durable pathnames. Operations
within that import use their leaf names under the retained directory. A later
recovery invocation opens the directory anew and validates the recorded state.
<!-- source: internal/component/config/storage/import_source.go -- importSource, openImportSource -->

Artifact open and import validate owner and mode on the descriptor passed to
`zefs.OpenFileIn`. Import's bounded `CheckFile` consumes that same descriptor.
`OpenBlob` checks the entry before it creates a sibling lock, so an unsafe
entry leaves no lock file behind. It closes that observation and takes the lock.
It then opens and validates the descriptor it consumes.
<!-- source: internal/component/config/storage/blob.go -- OpenBlob -->
<!-- source: internal/component/config/storage/import_source.go -- openImportSource -->

A cooperative writer can publish a replacement between the early check and lock acquisition.
The opener must load that latest publication rather than the earlier observation.
Loading consumes the descriptor validated under the lock, without another
pathname open between that validation and consumption.
This binds admission to the consumed inode. It does not freeze its bytes against
a same-user writer. Source locks, digest checks and replay rules still apply.
<!-- source: internal/component/config/storage/blob.go -- blobRoot, populateBlob, OpenBlob -->
<!-- source: internal/component/config/storage/import_source.go -- importSource, openImportSource -->

### Registered inspection inputs

`show data list prefix <prefix>` narrows the stored-key listing, and
`show data registered pattern <pattern>` selects one exact declared key
pattern. Both selectors are YANG leaves, so generated command forms describe
the same inputs as the text command. An unknown registered pattern is a
failure on both the standalone and daemon routes. The standalone
`ze data registered` adapter and the daemon handler read one typed pattern
producer and render its answer through the command layer.
<!-- source: internal/plugins/config-storage/yang/ze-storage-cli-cmd.yang -- show data list, show data registered -->
<!-- source: internal/component/config/storage/cli/registered.go -- registeredAnswer -->
<!-- source: internal/component/config/storage/cli/storage_data.go -- dataList, dataRegistered -->
<!-- source: internal/component/config/storage/cli/main.go -- cmdRegistered -->

`show data encode content <base64> part full|header|crc` produces the same
bytes as `ze data encode` and its `--header` or `--crc` forms. Optional
`capacity` reserves the requested frame size; it must hold the input and fit
within one remote blob answer. The default adds ten percent of the input
length. The command registers supplied-input metadata so file/paste controls
discover it without a web-owned command list. It reads no store and returns
a blob, preserving binary frame bytes through JSON base64 encoding.
<!-- source: internal/component/config/storage/cli/encode.go -- dataEncode, encodeNetcap -->
<!-- source: internal/component/config/storage/cli/register.go -- init -->

### Restoring a config from an artifact

The detached `request config artifact import` command and standalone
`ze data --path <artifact> import <file>` share `ImportConfig`. The caller's
complete bytes remain unvalidated configuration input: importing a new basename
writes its active file key, while importing an existing active pointer or mirror
publishes native version history. A pending candidate refuses replacement of
an existing configuration. Retained confirmed commits and unfinished file
commits refuse import. The artifact adapter accepts a
private source token and a configuration basename, rather than a daemon path;
the owner stages the complete destination artifact and applies its normal quotas.
<!-- source: internal/component/config/storage/cli/main.go -- ImportConfig -->
<!-- source: internal/component/config/storage/import_config.go -- ImportConfig -->
<!-- source: internal/component/config/storage/publication_intent.go -- refuseConfigIntentLocked -->
<!-- source: internal/component/config/artifact/raw.go -- ChangeStoreFromSource -->

`ReadRestoreSource` runs the artifact check (`CheckArtifact`, the same
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

Browser-created system backups use `BackupBounded` over that same installed
store. It keeps the source guard across a complete size preflight and native
copy, uses exact-fit encoding and checks cancellation between keys. The
preflight includes a conservative framing allowance before creating output;
keys and values alone do not establish the encoded artifact's size. The private
artifact owner supplies the smaller of its remaining shared allowance and its
single-artifact limit, and removes output on any creation failure. This changes
neither the ordinary `Backup` API nor its caller-selected spare policy.
<!-- source: internal/component/config/storage/backup_bounded.go -- BackupBounded, checkBackupAllowance -->
<!-- source: internal/component/config/artifact/system_snapshot.go -- SnapshotSystem -->

Private artifact selected-config restore uses the same `ReadRestoreSource` and
`RestoreConfigFromSource` publication as the daemon's path adapter. Source
selection returns complete owned bytes before the artifact owner lock releases;
the live candidate and reload run afterward. Neither adapter changes the
installed runtime target. A missing reload owner refuses before candidate
staging. The managed-client branch requires a configured served client and
publishes that client's configuration without reloading the hub.
<!-- source: internal/component/config/storage/cli/data_rpc.go -- handleDataRestore, RestoreConfigFromSource, restoreDaemonConfig -->
<!-- source: internal/component/config/cli/artifact_restore.go -- artifactRestoreConfig -->

`request data repair` and the daemon's `show data check` walk the frame files
of the live tree (`zefs.RepairPath`, `zefs.CheckPath`) rather than its keys.
That walk reads one directory entry, then opens it, so a key a writer removes
between the two is gone at the open: the walk then names it in the report's
`Vanished` list rather than reading it (`zefs-format.md`, "Integrity checking"). On the
daemon, `HoldWrites` runs the walk under one guard that takes the
configuration lock and then the runtime-state lock and holds both until the
walk returns, so every writer, a plugin's runtime-state write included, waits
for it, the walk reads one moment of the store, as a backup does, and no entry
vanishes. The local `ze data check` and `ze data repair` hold no lock of the
daemon's, so their reports can list vanished entries. Readers are not held. `Salvage` judges
the output by the backup's store-owned names (`refuseStoreName`) and writes
the new tree with `zefs.RepairPath`, which refuses an output that exists or
lies inside the source. A long salvage holds every writer for its length, the
price a backup already pays.
The local and daemon repair routes grade one salvage verdict. Its history
rows retain their severity and whether a key was dropped, and the daemon
document lists each recovered key alongside its count. A dropped corrupt
history entry is reported as a warning about a partial salvage. A pointer
that still cannot resolve is a failed repair with the salvage document
retained and the command that checks the written output; it is never a
successful repair merely because the frame copy completed.

A complete daemon repair response is limited to 8 MiB. If detailed rows exceed
that limit, the command returns an explicit report failure with the saved path
and counts. `salvage-written` confirms the completed write, and `details-omitted`
states that the rows are absent. The response gives the local command that
checks the saved salvage. It never reports a truncated list as complete.
<!-- source: internal/component/config/storage/cli/data_rpc_repair.go -- repairAnswer, repairDetailsTooLarge, repairReportRefusalWithFix -->
<!-- source: internal/component/config/storage/restore.go -- ReadRestoreSource, RestoreConfig -->
<!-- source: internal/component/config/storage/backup.go -- Backup, ownedStore, CheckArtifactPath, refuseStoreName -->
<!-- source: internal/component/config/storage/salvage.go -- HoldWrites, Salvage -->
<!-- source: internal/component/config/storage/cli/repair_verdict.go -- newRepairVerdict, printHistory, exitCode -->
<!-- source: internal/component/config/storage/cli/data_rpc_repair.go -- repairAnswer -->

### Editing an uploaded backup

The private artifact owner's root is selected from
`environment/backup/directory` or its `ze.backup.directory` OS override.
Its default appends `backups` to the existing platform data directory; unresolved
unconfigured development layouts retain the OS temporary folder. An invalid
explicit location refuses instead of falling back. Uploads and download
snapshots share this policy. The configured parent remains user-owned: its mode
and unrelated files survive, and the owner removes only its random private
children at close. A configuration commit prepares a missing parent and checks
write access before acceptance; only a successful commit changes the selected
folder. Existing workspace and download paths stay valid in their original
roots. Empty retired roots are removed, failed cleanup remains explicit debt,
and Close removes all owned roots. Preparation retains no speculative private
root, so repeated changes cannot accumulate empty roots. The leaf is not
written into the OS environment. Removing it restores the default unless the
OS override is set.
The shared server binds a directory guard from its actual installed live store.
It resolves existing link prefixes and reuses `StoreOwnedPath` to refuse the
live tree and reserved entries before creating the configured parent. Ordinary
sibling backup folders remain permitted; the guard does not infer a store from
a default configuration path.
<!-- source: internal/component/config/artifact/directory.go -- configuredBackupParent, backupDirectory -->
<!-- source: internal/component/config/artifact/owner.go -- ensureRoot, Close -->
<!-- source: internal/component/config/storage/backup_directory.go -- CheckBackupDirectory, resolveBackupDirectory -->

`CheckArtifact` applies the same complete ZeFS integrity verdict to an uploaded
artifact that a restore uses. Its retained source descriptor takes the import
decoder's 256 MiB bound and the artifact's owner and permission checks.
`ArtifactConfigNames` reads the already opened artifact and returns sorted
configuration names for the browser picker. An explicit active pointer always
identifies a configuration. Among mirror-only names, a name owned by another
configuration's editor (`<name>.draft` or `<name>.change.<user>`) is saved editor
state and is omitted. The editor's path producers and this census share their
suffix declarations. A suffix-bearing name with no owning configuration remains
selectable, as do ordinary alternate extensions and names with no extension.
The existing restore reader keeps its complete pointer-and-mirror census,
including an explicitly selected legacy mirror with an editor-like name.
The shared private artifact owner reads its selected committed bytes
through `ReadActiveConfig`; saved drafts and pending candidates are separate
records and are not its starting configuration.
<!-- source: internal/component/config/storage/import.go -- CheckArtifact -->
<!-- source: internal/component/config/storage/restore.go -- ArtifactConfigNames, artifactConfigNames -->
<!-- source: internal/component/config/storage/editor_record.go -- EditorDraftSuffix, EditorChangeSeparator -->
<!-- source: internal/component/cli/editor_session.go -- DraftPath, ChangePath, changePrefix -->
<!-- source: internal/component/config/storage/pointer.go -- ReadActiveConfig -->

`PublishArtifactConfig` accepts only a writable blob artifact. Its caller must
validate the edited content and serialize review, save and export for its
workspace. Publication uses the ordinary version writer and active/rollback
promotion under one storage guard, but never replaces or consumes an existing
candidate pointer. That candidate must resolve before publication starts.
New versions skip occupied timestamps, and adopting a mirror-only source finds
a free timestamp before the new commit. Both searches use the guarded key
census as their bound, so adoption preserves earlier history and pending bytes.
The selected configuration's saved draft, per-user changes, other configurations
and unrelated binary keys remain unchanged. A live tree is refused so this path
cannot bypass its daemon's reload transaction. Errors retain the normal
active-pointer commit boundary: an accepted version is not deleted because a
later storage operation failed. Guard release is part of the reported result.
<!-- source: internal/component/config/storage/artifact_workspace.go -- PublishArtifactConfig, checkArtifactCandidate, withdrawArtifactVersion -->
<!-- source: internal/component/config/storage/pointer.go -- writeNewVersionLocked, freeVersionTimeLocked, promoteVersionLocked -->

## Renaming a stored configuration

`RenameConfig` moves a configuration's complete named state under one
configuration guard. It includes every `file/<namespace>/<name>` entry,
`meta/config/<name>/` descendant, and the saved `.draft` and `.change.<account>`
siblings the editor owns. An editor-like sibling with its own dated version or
per-config identity metadata is a separate configuration and remains separate,
including a staged identity that has no active pointer yet. Shared content objects
are unchanged, and global audit and command-history records retain their
historical identities. Process-local modification metadata follows each moved
key. The low-level `Storage.Rename` remains a single-entry operation.

The source may have an active mirror, an active pointer, or both. A destination
with any owned mirror, pointer, version or editor record is refused before any
write. An unfinished explicit-file commit or confirmed-commit record also
refuses the rename, because its embedded identity cannot be moved safely.
Every selected source byte is read before publication. The store writes the
complete destination before removing any source key; source pointers are
removed before source versions, and the active mirror is removed last.

Publication uses the existing per-key durability contract, rather than claiming
a multi-key crash-atomic operation. An interruption can retain both names.
An ordinary publication failure compensates the operation while the guard is
held. Destination keys are removed only while a complete source still exists
or after every source key has been successfully restored. If source restoration
fails, the complete destination and its saved work remain, and the error names
that retained configuration and both failure causes.
<!-- source: internal/component/config/storage/rename_config.go -- RenameConfig, prepareConfigRename, configRenamePlan.publish, restoreConfigRenameSource -->

The daemon serves `request data config rename old <name> new <name>` through
the same transaction. Its running configuration name and configured managed
client fetch names are refused: the rename cannot safely change those launch
or serving bindings. Other stored configurations are renamed without a reload.
The response names `old` and `new`. Successful mutations record
`data-config-rename` through the dispatcher with the original actor and surface;
a refused operation records no successful mutation.
<!-- source: internal/component/config/storage/cli/data_rpc_rename.go -- handleDataConfigRename -->
<!-- source: internal/component/config/storage/cli/register_data_rpc.go -- init -->
<!-- source: internal/core/audit/action_config_rename.go -- ActionDataConfigRename -->

## Rolling back a stored configuration

`RollbackConfig` selects a one-based revision and publishes its verified bytes
as a new active version under one configuration guard. Its history order is the
existing `ListVersions` order, which `Editor.ListBackups` already uses: newest
dated entry first. Both readers use `versionsForName`, so an adapter keeps no
second ordering rule. The selected entry resolves through its content object
with the existing hash verification. A missing index or corrupt object refuses
the operation without substituting another revision.

The selected old version remains in history. Publication uses the same guarded
body and active-pointer commit point as `RestoreConfig`, retaining the previous
active as rollback, refreshing the active mirror and clearing only the candidate
it accepted. Another staged candidate refuses publication. An unfinished file
commit or recorded confirmed-commit window also refuses the operation before a
write, because its recovery could replace the rollback's publication.
<!-- source: internal/component/config/storage/rollback_config.go -- RollbackConfig, admitConfigRollbackLocked -->
<!-- source: internal/component/config/storage/history_revision.go -- readRollbackRevisionLocked, versionsForName -->
<!-- source: internal/component/config/storage/restore.go -- restoreConfigLocked -->

The daemon serves `request data config rollback name <name> revision <N>`.
For its own running source, the live confirmed-window worker admits selection
and the existing candidate/reload transaction publishes the result. Its stored
window record is checked inside that worker too. Other stored configurations
use `RollbackConfig`; a managed client name must be one this hub serves. Both
paths preserve existing staged work and report the selected revision and the
new active version. Successful mutations record `data-config-rollback` with
the original caller and surface.

The result carries `name`, `revision`, `source-version`, `source-time` and
`version`. `source-time` is the selected version's timestamp in JSON's RFC3339
time representation, so a remote standalone adapter retains the existing human
timestamp without guessing a timezone from a version stamp.
<!-- source: internal/component/config/storage/cli/data_rpc_rollback.go -- handleDataConfigRollback, rollbackDaemonConfig, configRolledBack -->
<!-- source: internal/core/audit/action_config_rollback.go -- ActionDataConfigRollback -->

`ReadRollbackRevision` is the writable owner's guarded pre-publication selector
for the active daemon path. It applies the same identity-intent admission and
returns caller-owned verified bytes. General read-only history inspection,
including artifact comparisons, retains `ListVersions` and `ReadVersion` and
does not acquire a writable guard. Local loose-file rollback retains its real
file writer. Stored rename and rollback share `StoredConfigName` admission;
raw storage keys and names outside the selected store are refused before a
request can be normalized to a basename.
<!-- source: internal/component/config/storage/history_revision.go -- ReadRollbackRevision -->
<!-- source: internal/component/config/storage/config_name.go -- StoredConfigName -->

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

Candidate staging has two entries. `WriteCandidateVersion` and
`WriteCandidateVersionWithGuard` stage an operator's change, and first ask the
freeze of a managed device: its own stored configuration is refused with
`ErrConfigFrozen` before anything is written. `StageMachineCandidate` stages
for a writer no operator decides (a hub push, a revert, a boot record, schema
recovery) and `StageSourceCandidate` stages the source's own bytes for the
signal reload; neither asks the freeze. `RestoreConfig`, `ImportConfig` over
an existing configuration, `RollbackConfig` and `CommitConfigWithGuard` stage
through the operator entry. `EnsureActiveVersion` and `PublishArtifactConfig`
write a version without staging a candidate and are not frozen. The scope and
the reason are in `fleet-config.md`, "The freeze".
<!-- source: internal/component/config/storage/pointer.go -- WriteCandidateVersionWithGuard, StageMachineCandidate, StageSourceCandidate, stageCandidateLocked -->
<!-- source: internal/component/config/storage/freeze.go -- refuseFrozenConfigLocked, ErrConfigFrozen -->

### The boot record

A daemon no longer records the configuration it booted from (for an explicit
file: a candidate promoted to active) before its first listener, because each
of the record's syncs waits behind any other writer on the disk, and the first
listener used to wait for all of them. `DeferBootRecord` arms the record on the
bound handle where the record used to run. `Start` runs it once in its own
goroutine, against a view of that handle the gate does not hold, once the hub
serves: after plugin startup, the service listeners and the ready file. The
record holds the configuration lock across its syncs, so any configuration
write waits for it; a read does not, and neither does a runtime-state write
(above). Started before the plugin wait, it made bgp's Stage 2 read of its
Graceful Restart marker wait on a slow disk until the startup barrier, which
allows a few seconds with no plugin progress, failed the start
(`test/encode/dut-store-slow-boot-record-starts.ci` holds it there). So neither
a listener nor the plugin handshake waits for it. `Wait` runs before the handle closes. The owner lock, `recoverFileCommit`, stale-candidate
cleanup and the SSH host key all stay before serving.

A stop waits for the record before it prints `Ze stopped.`
(`awaitBootRecordAtStop` in `cmd/ze/hub/config_source.go`), so the store holds
the boot configuration when the process exits and an offline reader finds it.
The record's syncs therefore cost the same disk time as before the move: they
are paid after serving, and a stop that arrives soon after serving pays what is
left. A record still running when the stop reaches it is announced on stdout
(`Stopping: the store is still recording the boot configuration, because the
disk is slow; ...`). The wait has no bound, because abandoning the record would
leave the previous configuration active for every offline reader; a second
Ctrl+C forces the exit, and the next start records the same file.

While the record is pending, `ReadActiveConfig` of the bound name and the
handle's own `ReadFile` of it (the read an editor opens the configuration with,
so a runtime `update bgp config` or an API session started in that window)
answer the boot bytes from memory, the handle's `Exists` of it answers true,
and `ReadCandidateConfig` answers no candidate: those bytes are the accepted
configuration before they are durable. `Exists` agrees with `ReadFile` so a
caller that creates a missing file, as the web server does at startup with a
placeholder, never writes over the name the daemon boots from. Every config
write takes the config lock of the bound name, and `configStore.AcquireLock`
holds that lock back until the record lands, at most 30 seconds. Past the bound
the write is refused with an error naming the slow disk and the retry. A failed
record is logged at error level with its fix (`ze data check`, then restart),
and every later write of the bound name returns that failure, while the daemon
keeps running the configuration it loaded. Locks of other keys (runtime state,
the SSH host key) never wait. The gate matches the store key the bound path
resolves to, not its base name. A daemon that stops before it starts the record
refuses a waiting write with "ze is stopping" and the retry, not the
`ze data check` fix, because nothing failed.

The gate lives in the daemon's handle, so a reader in another process does not
see it. Raw-key readers (`ze data cat`, backup) see the store as it is, and the
read-only CLI readers that open the store themselves (`withRuntimeStore` in
`config_data.go`, `cmd_history.go`, `cmd_diff.go`) read the previous active
configuration, or none on a first start, until the record lands.

A crash between serving and the record loses nothing: the source is unchanged,
and the next start reads it and runs the record again. A crash inside the record
leaves at most a candidate, which the next start's stale-candidate cleanup
clears before it records.
<!-- source: internal/component/config/storage/boot_record.go -- BootRecord, DeferBootRecord, await, covers, pendingBootConfig -->
<!-- source: internal/component/config/cli/config_data.go -- withRuntimeStore -->
<!-- source: internal/component/config/storage/source.go -- configStore.AcquireLock, configStore.ReadFile, configStore.Exists -->
<!-- source: cmd/ze/hub/main.go -- runYANGConfig -->

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
the caller owns conflict detection and the persistent file-commit intent. It
refuses a store that is closed, a read-only tree, or a blob store, and, for a
tree, a path whose folder is not the store's own folder, so a tree's staged
frame is always one that owner sweeps.
A memory store (the functional-test DUT without `option=storage:value=disk`)
has no folder, no lock and no sweep, so it publishes the file under the
reserved frame name `.ze-storage-.<random>`, which no sweep matches; a crash
leftover of it stays until an operator removes it.
Bare stored-config startup reads
the active pointer, falling back to the active mirror only when no pointer exists.
<!-- source: internal/component/config/storage/pointer.go -- PromoteCandidate, ClearCandidate, ReadActiveConfig, pointerPath, removeVersionLocked -->
<!-- source: internal/component/config/storage/history.go -- writeVersionObject, readVersionEntry, sweepObject, objectsFirst, ReadVersionEntry -->
<!-- source: internal/component/config/storage/history_check.go -- CheckHistory, RepairHistory, inspectHistory -->
<!-- source: internal/component/config/storage/open.go -- WriteConfigFile -->
<!-- source: pkg/zefs/keys.go -- KeyConfigActive, KeyConfigFileCommit -->
