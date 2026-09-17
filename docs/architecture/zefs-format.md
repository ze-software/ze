# ZeFS File Format

ZeFS is the blob artifact format used for seeds, backups and explicit imports. A single `.zefs` file holds named entries with hierarchical keys, zero-copy reads via mmap, and capacity-aware framing. The live store is a directory of framed values, described in [Storage backends](storage-backends.md); opening that store never converts or falls back to a blob.
<!-- source: pkg/zefs/store.go -- Store implementation -->

## Netcapstring

A netcapstring is a self-describing, capacity-aware binary frame. It encodes a byte sequence with extra reserved space so that small growth can be written in place without shifting subsequent entries.

### Format

With padding (cap > used):

```
<number>:<cap>:<used>:<crc>\n<data><space padding>\n
```

Exact fit (cap == used):

```
<number>:<cap>:<used>:<crc>\n<data>\n
```

The header separators are `:` (between number, cap, used, and crc) and `\n` (after crc). The header occupies its own line, making it easy to inspect with text tools. Unused capacity is space-filled. `\n` terminates both the header and the data region.

| Field | Content | Size (bytes) |
|-------|---------|-------------|
| `<number>` | Digit count of `<cap>` (decimal ASCII, no leading zeros) | variable (typically 1-2) |
| `:` | Separator | 1 |
| `<cap>` | Capacity in bytes (decimal ASCII, zero-padded to `<number>` digits) | `<number>` |
| `:` | Separator | 1 |
| `<used>` | Used bytes (decimal ASCII, zero-padded to `<number>` digits) | `<number>` |
| `:` | Separator | 1 |
| `<crc>` | CRC32c of the `<used>` data bytes (8-char zero-padded lowercase hex) | 8 |
| `\n` | Header terminator (0x0A) | 1 |
| `<data>` | Actual content | `<used>` |
| `<padding>` | Space bytes (0x20) | `<cap>` - `<used>` |
| `\n` | Terminator (0x0A) | 1 |
<!-- source: pkg/zefs/netcapstring.go -- netcapstring encoding/decoding -->

### Properties

- **Self-describing width.** The `<number>` field tells the parser how many digits to read for `<cap>` and `<used>`. No magic constants needed.
- **Cap-first, fixed-width used.** Since `<used>` is always zero-padded to the same width as `<cap>`, and `<used>` <= `<cap>` by definition, the header size never changes when data grows within capacity. This is the critical invariant for in-place writes.
- **Per-record CRC32c.** Each netcapstring carries a CRC32c (Castagnoli, hardware-accelerated on arm64/amd64) of its `<used>` data bytes. Corruption is detected on decode. The container's CRC covers all encoded entries, giving whole-file structural verification.
- **Bounded parsing.** Decimal fields must fit a Go `int`, and capacity must fit the available bytes including the terminator. Integrity tools also impose file-size and entry-count limits.

### Examples

| Data | Cap | On disk |
|------|-----|---------|
| "hello" (5 bytes), cap 16 | 16 | `2:16:05:9a71bb4c\nhello<11 spaces>\n` |
| empty, cap 8 | 8 | `1:8:0:00000000\n<8 spaces>\n` |
| "abcd" (4 bytes), cap 4 | 4 | `1:4:4:92c80a31\nabcd\n` |
| "x" (1 byte), cap 100 | 100 | `3:100:001:a93c5f93\nx<99 spaces>\n` |

### Header length

The total header length for a given capacity is: `3 + digitCount(digitCount(cap)) + 2 * digitCount(cap) + 1 + 8` (the `+ 1 + 8` is the colon separator and 8-char CRC hex).

| Capacity range | Header bytes |
|---------------|-------------|
| 0-9 | 15 |
| 10-99 | 17 |
| 100-999 | 19 |
| 1000-9999 | 21 |

### Capacity growth

New keys are exact fit except under `file/active/`, where `writeFileNoFlush` reserves 20 extra bytes. Data capacity is data length + 10%, both on first write and on growth. `encode` uses exact-fit container capacity for the encoded entries plus their terminating newline.
<!-- source: pkg/zefs/store.go -- writeFileNoFlush, encode -->

### Parsing

1. Scan forward until next `:` to get the `<number>` field (parse as integer N)
2. Read N bytes for `<cap>` (parse as integer)
3. Read `:` (verify separator)
4. Read N bytes for `<used>` (parse as integer)
5. Read `:` (verify separator)
6. Read 8 bytes for `<crc>` (parse as hex uint32)
7. Read `\n` (verify header terminator)
8. Read `<used>` bytes of data
9. Verify CRC32c of data matches `<crc>` from header
10. Skip `<cap>` - `<used>` bytes of space padding
11. Read `\n` (verify terminator)
12. Next entry starts at the byte after the terminator

`DecodeNetcapstringRef` validates the frame and CRC without copying its payload.
It returns a capped slice into the caller's buffer and the next offset. Blob
decoding chains frames through that offset. A tree value must contain exactly
one frame, so its reader also requires the next offset to equal the file length;
appended bytes and concatenated frames are errors.

## ZeFS File

A ZeFS file is a sequence of two netcapstrings: a magic identifier followed by the container.

### Format

```
1:4:4:<crc>\nZeFS\n<N>:<cap>:<used>:<crc>\n<entries...><padding>\n
```

The first netcapstring contains the magic `ZeFS`. Its header ends with `\n` and its terminator is also `\n` because cap == used. The entire file is pure netcapstrings, all terminated by `\n`.

### Container content

Inside the container, entries are stored as consecutive pairs of netcapstrings (key + value):

```
1:4:4:<crc>\nZeFS\n<N>:<cap>:<used>:<crc>\n
  <kN>:<kCap>:<kUsed>:<kCRC>\n<key><kPad>\n<vN>:<vCap>:<vUsed>:<vCRC>\n<value><vPad>\n
  <kN>:<kCap>:<kUsed>:<kCRC>\n<key><kPad>\n<vN>:<vCap>:<vUsed>:<vCRC>\n<value><vPad>\n
  ...
  \n
<container padding>\n
```

Each entry consists of:
1. A netcapstring containing the key (hierarchical path, e.g., `etc/ze/router.conf`)
2. A netcapstring containing the value (file content)

The entry list ends with a `\n` byte. The container may have additional space padding after the newline (reserved capacity for future entries).

### Keys

Keys are hierarchical paths using `/` as separator. They must be valid `fs.ValidPath` names (no leading `/`, no `.` or `..` components, no empty segments).

### Parsing a ZeFS file

1. Decode the first netcapstring (magic)
2. Verify its data is `ZeFS`
3. Decode the second netcapstring (container)
4. Within the container data, decode entry pairs until `\n`, null, or space byte

### Magic detection

| Bytes | Meaning |
|-------|---------|
| `1:4:4:<crc>\nZeFS\n` at offset 0 | Valid ZeFS file |
| Anything else | Not a ZeFS file |

## Memory mapping

On unix, the backing file is memory-mapped (`PROT_READ`, `MAP_PRIVATE`). Tree nodes hold sub-slices of the mapped region for zero-copy reads. The `ReadLock` and `WriteLock` guards scope zero-copy slice validity: callers hold the lock while processing raw bytes, and the in-process `sync.RWMutex` prevents `flush()` (which remaps the backing) from running while slices are in use.
<!-- source: pkg/zefs/mmap_unix.go -- mmap implementation -->
<!-- source: pkg/zefs/lock.go -- ReadLock, WriteLock guards -->

## Concurrency model

### Single-process ownership

Blob artifacts are opened offline and require one owning process. The blob's mutex protects goroutines within that process; it does not exclude another process. Live operations use the daemon's tree-backed storage handle and its lifetime ownership lock, as described in [Storage backends](storage-backends.md).

### In-process locking

All blob concurrency is in-process, handled by `sync.RWMutex`:

| Guard | Mutex | Blob access |
|-------|-------|-------------|
| `ReadLock` | `RLock` (shared) | Zero-copy reads; multiple readers concurrent |
| `WriteLock` | `Lock` (exclusive) | Batched writes; single writer, blocks readers |

`WriteLock` batches changes in memory and flushes on `Release()`. A flush uses
either in-place writes or a full temp-and-rename rewrite, as detailed below.
The guard's slices remain valid only while the guard is held.

## Key Namespaces

Keys follow a `<namespace>/<qualifier>/<path>` convention to prevent collisions between metadata and config files.

| Namespace | Purpose | Example |
|-----------|---------|---------|
| `meta/` | Instance metadata (credentials, identity, flags) | `meta/auth/local/username`, `meta/instance/managed` |
| `file/active/` | Current committed config files | `file/active/router.conf` |
| `file/draft/` | Live edits in progress | `file/draft/router.conf` |
| `file/<date>/` | Historical config versions | `file/20260318-100000.000/router.conf` |
<!-- source: pkg/zefs/keys.go -- KeyLocalAdminUsername -->

The configuration storage layer maps config paths into this key space and also
exposes raw-key operations for data commands. Its mapping, per-config pointers
and recursive enumeration contract are described in
[Storage backends](storage-backends.md). `ze data` accepts either an explicit
blob artifact or the live tree root; mutations require offline ownership.

## Key Registry

All known key patterns are registered in `pkg/zefs/keys.go` via `MustRegister()`, following the same pattern as `env.MustRegister()` for environment variables. Each `KeyEntry` has a Pattern, Description, and Private flag.
<!-- source: pkg/zefs/registry.go -- MustRegister, KeyEntry -->
<!-- source: pkg/zefs/keys.go -- registered key definitions -->

Template keys use `{param}` placeholders for variable segments. The `Key()` method substitutes params and validates them (rejects empty and `..`). `Prefix()` and `Dir()` extract the fixed prefix for directory listing.

| Method | Purpose | Example |
|--------|---------|---------|
| `.Pattern` | Raw pattern string | `"meta/history/{username}/{mode}"` |
| `.Key(params...)` | Instantiate with concrete values | `KeyHistory.Key("alice", "edit")` returns `"meta/history/alice/edit"` |
| `.Prefix()` | Fixed prefix with trailing `/` | `KeyFileActive.Prefix()` returns `"file/active/"` |
| `.Dir()` | Fixed prefix without trailing `/` | `KeyFileActive.Dir()` returns `"file/active"` |

Discovery: `ze data registered` lists all public key patterns. `ze data registered <pattern>` shows details for one.

The Private flag hides a PATTERN from that listing. It is not a file mode, and
ZeFS has no per-key mode: `BlobStore.WriteFile` accepts its `perm` argument and
ignores it, and `storeFileInfo.Mode()` returns `0o444` for every entry that is
not a directory. The mode that
protects stored key material is the blob file's own. `atomicWrite` creates it
with `os.CreateTemp`, which gives 0600, and renames it into place, so the whole
store is 0600 from the moment it is created.
<!-- source: pkg/zefs/store.go -- BlobStore.WriteFile, atomicWrite -->
<!-- source: pkg/zefs/file.go -- storeFileInfo.Mode -->

Two registered keys hold the daemon's own certificate authority. `meta/ca/cert`
is the root certificate, which is public material an operator copies into a
peer's trust anchor, so it stays listable. `meta/ca/key` is the root private key
and is Private.
<!-- source: pkg/zefs/keys.go -- KeyCACert, KeyCAKey -->
<!-- source: internal/component/pki/ca.go -- LoadOrGenerateRoot -->

Both are written once, on the first daemon start that finds no root, and read on
every start after it. A restart therefore presents the same root, so a copy an
operator already distributed keeps working.

## In-place writes

When a value changes but fits within its existing slot capacity, `flush()` uses `pwrite` to update only the changed entry and the container header, avoiding a full file rewrite.

| Condition | Write strategy |
|-----------|---------------|
| Existing entry value fits slot capacity and layout is unchanged | pwrite: entry header+data + container header CRC |
| Entry value exceeds slot capacity | Full rewrite via temp+rename |
| Entry added, regardless of container capacity | Full rewrite via temp+rename |
| Entry removed | Full rewrite (entries shift) |
| Non-unix platform | Full rewrite (no pwrite) |

After pwrite, the backing mmap is released and re-acquired so the read path sees the changes. On non-unix platforms, the in-place path falls back to full rewrite.
<!-- source: pkg/zefs/store.go -- flushInPlace, flushFull -->
<!-- source: pkg/zefs/pwrite_unix.go -- pwriteRegions -->

## Integrity checking

`zefs.Check(path)` reads a blob, validates the magic, container CRC, and each
entry's CRC32c, and returns a `CheckReport`. A failed container checksum remains
a failure even when its framing permits a bounded scan of the inner entries.
The scan reports the damaged key where recoverable, otherwise the blob path
and byte offset.

`zefs.Repair(src, dst)` reads a damaged blob and writes recoverable entries to
a new artifact. It follows intact frame boundaries and stops when a boundary
cannot be recovered; bytes resembling headers inside damaged payloads do not
become keys.

`zefs.CheckPath(path)` and `zefs.RepairPath(src, dst)` dispatch on a blob file or
a framed directory tree. On Linux and Darwin, traversal opens every source and
destination-parent component from `/` using descriptor-relative, no-follow opens.
It refuses symlinks anywhere in those paths and special files in the tree.
Ancestors are not checked for mode or owner: the tree root's own check bounds
the tree (owner decision, 2026-09-17). The tree root and its descendants
require caller ownership, directories exactly 0700 and files exactly 0600,
including when the caller is root.

Darwin's fixed `/tmp` and `/var` system aliases select `/private/tmp` and
`/private/var` directly before that walk. Their symlink entries are never
followed, and the canonical base and every supplied component remain checked.
Other platforms retain blob support and explicitly refuse tree integrity operations.

Tree checks verify one complete frame per key. Tree repair skips corrupt frames,
preserves good frames byte-for-byte in a new tree, and never changes the source.
Its private staging directory is beside the destination, outside the key tree;
publication refuses an existing destination with Linux `RENAME_NOREPLACE`,
Darwin `RENAME_EXCL`, or on FreeBSD a `linkat` or `mkdirat` claim of the target
name. Staging creation, key writes, publication and cleanup all
use retained directory descriptors, so a replaced ancestor cannot redirect them.
Files and directories are synced before publication and created with modes
suitable for reopening the live store.
Traversal is iterative and enumerates directory entries in bounded batches.
Tree depth, key count and value size have no smaller integrity-only limits than
the live writer: native path, descriptor and available-memory limits apply.
Each frame read is bounded by the opened inode's size and refuses a size change.

`zefs.MoveAside(path)` preserves a file or tree under
`<path>.replaced-<date>T<time>`. Live open is non-mutating: a blob or corrupt store
is refused rather than moved aside and replaced with empty credentials.

CLI: `ze data check`, `ze data repair --output <path>`, `ze data encode`.
Checks retain exit codes 0 for clean, 1 for corruption, and 2 for unreadable or
unsafe paths.
<!-- source: pkg/zefs/check.go -- Check, Repair, CheckPath, RepairPath, MoveAside -->
<!-- source: pkg/zefs/check_tree_unix.go -- OpenDirectory, walkFrameTree, repairFrameTree -->
<!-- source: pkg/zefs/check_tree_linux.go -- RenameNoReplace -->
<!-- source: pkg/zefs/check_tree_darwin.go -- RenameNoReplace, trustedFramePath -->
<!-- source: pkg/zefs/check_tree_freebsd.go -- RenameNoReplace -->

### Integrity design decisions

- **Per-record CRC in the header, not a whole-file checksum under a key.** The
  container is itself a netcapstring, so one mechanism gives both entry-level
  pinpointing and whole-file coverage. A checksum stored as an entry would give
  neither.
- **CRC32c, not SHA-256.** The threat model is accidental corruption and bit
  rot, not an adversary rewriting the store. CRC32c is hardware-accelerated on
  arm64 and amd64.
- **In-place pwrite is not atomic, and that is the accepted trade.** A crash
  mid-write leaves corruption the CRC detects, which is a weaker durability
  guarantee than temp and rename. It is accepted because detection plus
  `Repair` is a recovery path, and because it finally uses the capacity design
  the format was built for.
- **The container CRC is written twice.** It covers the entry bytes, so it
  cannot be known before they exist. The encoder writes a placeholder, writes
  the entries, then patches the CRC.
- **Repair never writes over the source.** The caller keeps the damaged file
  for a post-mortem.
<!-- source: pkg/zefs/store.go -- flushInPlace, flushFull, container CRC patching -->
<!-- source: internal/component/config/storage/cli/cmd_integrity.go -- check, repair and encode commands -->
<!-- source: internal/component/config/storage/open.go -- Open, Create -->

## Runtime state through statestore

Runtime state uses the daemon's shared store handle through
`internal/core/statestore`, which declares the `Storage` contract the config
storage component implements, with registered keys from `pkg/zefs/keys.go`.
Plugins persist through the daemon rather than opening another store.
`Put` retains its best-effort no-store result for unwired callers; operations
that require acknowledged persistence use the state RPC contract.

The appliance imports its `database.zefs` seed explicitly into `database/` on
first boot and retires the seed only after verification. Persistent runtime
state then lives in framed keys under that tree. See
[Storage backends](storage-backends.md) for ownership, import recovery and the
live-store contract.
<!-- source: internal/core/statestore/statestore.go -- Put, Get, SetStore -->

## Implementation

Reference implementation: `pkg/zefs/` in the ze repository.
<!-- source: pkg/zefs/store.go -- BlobStore, Create, Open -->
<!-- source: pkg/zefs/lock.go -- ReadLock, WriteLock -->
<!-- source: pkg/zefs/tree.go -- in-memory tree representation -->
<!-- source: pkg/zefs/check.go -- Check, Repair, CheckReport, RepairReport -->
<!-- source: pkg/zefs/pwrite_unix.go -- pwrite for in-place writes -->
<!-- source: pkg/zefs/pwrite_other.go -- pwrite fallback for non-unix -->
<!-- source: pkg/zefs/file.go -- file-level operations -->
