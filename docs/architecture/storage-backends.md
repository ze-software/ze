# Configuration storage

The live configuration store is a directory named `database` beside the config
file. The storage package owns key mapping, version pointers, metadata and write
notifications for both this tree and explicitly opened ZeFS artifacts. Callers
use `Storage`; they never inspect its encoding.
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
refuses the open and names `ze init from`; no blob contents are read and nothing
is renamed. Absence of both forms returns `ErrNoStore` and names `ze init`.
<!-- source: internal/component/config/storage/open.go -- Open, OpenReadOnly, detect, lockOwner -->

The standalone `ze init from` command is planned for storage-2. The implemented
`ImportBlob` API already serves explicit appliance first-boot import.
<!-- source: internal/plugins/init/main.go -- Run -->
<!-- source: cmd/ze/ze_core_autoinit.go -- gokrazyAutoInit -->

The tree root and its intermediate directories are exactly 0700; regular frame
files are exactly 0600. All belong to the effective process user, including when
that process is root. Symlinks and non-regular leaves are refused before content
reads. The containing config directory can retain an ordinary safe mode such as
0755. Ancestors are traversed with descriptor-relative, no-follow opens and must
belong to root or the effective process user. A group/other-writable directory
without the sticky bit is refused unless an earlier caller-owned ancestor
denies all group/other access. Leaving that private ancestor through `..` ends
its protection. Errors identify the first unsafe path and the permission or
ownership repair.

`TransferOwnership` is an explicit privileged installer operation. It takes the
same stable lock, prepares and validates its iterative traversal before changing
owners, then transfers the folder and its descendants without reading secrets.
A failed transfer attempts to restore the original ownership and reports repair
instructions. Ordinary opens never transfer ownership.
<!-- source: internal/component/config/storage/tree.go -- openFolder, openNode, secureNode -->
<!-- source: internal/component/config/storage/ownership.go -- TransferOwnership -->

## Keys and reads

A logical key maps unchanged to `database/<key>`. Each file holds exactly one
netcapstring, including its CRC32c. Reads reject bad checksums, truncation and any
bytes after the frame. Tree traversal is iterative and follows filesystem path
limits. Values are limited by addressable memory; the existing ZeFS blob import
limit remains 256 MiB.

Config operations map a bare name to `file/active/<name>`. An explicit path must
belong to that store's config directory. Already namespaced `file/` and `meta/`
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
<!-- source: internal/component/config/storage/store.go -- checkName, ListKeys, List, guard.Release -->

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
<!-- source: internal/component/config/storage/rename_linux.go -- renameNoReplace -->
<!-- source: internal/component/config/storage/rename_darwin.go -- renameNoReplace -->

Linux and FreeBSD use `renameat2`; macOS uses `renameatx_np`. Publication fails
if the kernel or filesystem lacks the no-replace operation. It never falls
back to an overwriting rename.
<!-- source: internal/component/config/storage/rename_freebsd.go -- renameNoReplace -->

## Import and artifacts

`ImportBlob` is the only blob-to-live-tree operation. It checks the source,
copies every raw key, and compares both key sets and every value before
publication. Its durable `database.import-intent` records the source digest,
private stage, destination device/inode and archive name. An interrupted import
can resume its own published destination only when that identity and the complete
key/value comparison still match. An unrelated or changed tree is refused.

The importer alone retires the source to `.replaced-<stamp>`, syncs its directory,
and removes the intent. If a crash leaves both tree and seed, ordinary `Open`
still refuses; repeating the explicit importer completes retirement. Incomplete
private staging names never count as a live store.
<!-- source: internal/component/config/storage/import.go -- ImportBlob, importOwned, verifyImportIdentity, finishImport -->

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
