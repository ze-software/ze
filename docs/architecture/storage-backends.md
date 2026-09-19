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
nothing is renamed. Absence of both forms returns `ErrNoStore` and names `ze init`,
unless `database.import-intent` sits beside the absent tree: then an import
crashed after moving the old tree to `database.replaced-<stamp>` and before
publishing its stage, and every opener returns `ErrImportPending` naming the
intent file, its source and `ze init --from <source>`. `Create` and
`CreatePopulated` refuse the same way, under the owner lock, so neither `ze
start` nor a plain `ze init` publishes an empty tree over the operator's store;
`ze init --from` resumes the import. Only `ReplacePopulated` (`ze init --force
--yes`) proceeds, because replacing whatever sits there is its explicit order.
The live opener validates the tree before it creates `database.lock`, so a
refused open leaves no lock file behind.
<!-- source: internal/component/config/storage/open.go -- Open, OpenReadOnly, detect, missingTree, openLive, lockOwner, populateOwned -->

`ze init --from <path>` imports a local blob through `ImportBlob`. Under
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
pending writes without acquiring the store lock again. In-process metadata records
modification time and modifier identity on both encodings; it is not a durable
audit log. Observers receive resolved keys after successful publication and lock
release, so a callback may read the store again.
<!-- source: internal/component/config/storage/tree.go -- treeEncoding.ReadFile, treeEncoding.list -->
<!-- source: internal/component/config/storage/store.go -- CheckName, ListKeys, List, guard.Release -->

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

## Import and artifacts

`ImportBlob` is the only blob-to-live-tree operation. It checks the source,
copies every raw key, and compares both key sets and every value before
publication. Its durable `database.import-intent` records the source digest,
private stage, destination device/inode and archive name. An interrupted import
resumes its own published destination only when the tree's device and inode
match the intent. While the source is not yet retired, every key and value is
compared to it as well. Once the source is retired the import is finished apart
from the intent, so a tree a daemon has changed since is accepted and only the
intent is removed. An unrelated tree, a changed source, and an intent naming
another source are refused; each refusal names `database.import-intent` and the
repair: `ze init --from <source>`, or removing the intent once `database` holds
the wanted tree. The unrelated-tree refusal also names the stage the intent
holds, and an import removes every `database.import-tmp-*` stage before it
builds a new one, so a stage left by a removed intent does not outlive the next
import.

The importer alone retires the source to `.replaced-<stamp>`, moves the source's
`.lock` file beside the archive (every blob artifact keeps its lock next to it, and
the held lock refuses a concurrent creator of the retired name until it moves),
syncs its directory, and then removes the intent, in that order, so a crash between
the steps leaves a state the next import recognizes as finished. If a crash leaves both tree and
seed, ordinary `Open` still refuses; repeating the explicit importer completes
retirement. Incomplete private staging names never count as a live store.
<!-- source: internal/component/config/storage/import.go -- ImportBlob, ReplaceImportBlob, importOwned, removeStaleStages, resumeImport, verifyImportIdentity, finishImport, retireSource -->

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

## Version pointers and recovery

Pointers are per configuration name: `meta/config/<name>/active`, `candidate`,
`rollback` and `recovery`. Historical content is `file/<stamp>/<name>`. A durable
version precedes its pointer. Promotion records rollback before publishing
active, then refreshes the active mirror and clears candidate. Repeating a
promotion after active was published preserves the existing rollback reference.
Candidate cleanup retains any version still referenced by active, rollback or
recovery, including after a crash between pointer publications. Nameless legacy
pointers are never read.

Explicit-file source selection is retained by the runtime. `WriteConfigFile`
provides durable loose-file publication through a no-follow parent traversal;
the caller owns conflict detection and the persistent file-commit intent.
Bare stored-config startup reads
the active pointer, falling back to the active mirror only when no pointer exists.
<!-- source: internal/component/config/storage/pointer.go -- PromoteCandidate, ClearCandidate, ReadActiveConfig, pointerPath -->
<!-- source: internal/component/config/storage/open.go -- WriteConfigFile -->
<!-- source: pkg/zefs/keys.go -- KeyConfigActive, KeyConfigFileCommit -->
