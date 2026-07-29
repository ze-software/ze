# ZeFS File Format

ZeFS is a netcapstring-framed blob store. A single `.zefs` file holds multiple named entries (files) with hierarchical keys, zero-copy reads via mmap, and in-place update support via capacity-aware framing.
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
- **No artificial size limit.** The `<number>` field is itself variable-width, so entries can be arbitrarily large (limited only by available memory).

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

Keys are exact fit (keys never change). Data capacity is data length + 10%, both on first write and on growth.

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

Only one process opens a ZeFS blob at a time. In ze, the daemon (`ze start router.conf`) owns the blob. SSH editor sessions run as goroutines within the daemon process (via Wish). Terminal commands (`ze config edit`, `ze data ls`) detect the running daemon by dialing the SSH port and become SSH clients, sending commands through the daemon rather than opening the blob directly. When no daemon is running, the editor starts an ephemeral daemon, connects via SSH, and stops it when done.

### In-process locking

All blob concurrency is in-process, handled by `sync.RWMutex`:

| Guard | Mutex | Blob access |
|-------|-------|-------------|
| `ReadLock` | `RLock` (shared) | Zero-copy reads; multiple readers concurrent |
| `WriteLock` | `Lock` (exclusive) | Batched writes; single writer, blocks readers |

`WriteLock` batches all writes in memory and flushes atomically on `Release()`. No cross-process `flock` is needed because only one process has the blob open.

### Daemon mutual exclusion

The SSH server binds to its configured listen address on startup. If the port is already in use, the daemon fails with a clear error (port conflict), preventing two daemon instances.

### Terminal commands as SSH clients

When the daemon is running, terminal processes connect via SSH and send commands. The daemon's config component executes operations with mutex protection and returns results via the SSH session.

| Scenario | Terminal behavior |
|----------|-------------------|
| Daemon running | SSH client to daemon |
| No daemon | Ephemeral daemon started, then SSH client |

## Key Namespaces

Keys follow a `<namespace>/<qualifier>/<path>` convention to prevent collisions between metadata and config files.

| Namespace | Purpose | Example |
|-----------|---------|---------|
| `meta/` | Instance metadata (credentials, identity, flags) | `meta/auth/local/username`, `meta/instance/managed` |
| `file/active/` | Current committed config files | `file/active/router.conf` |
| `file/draft/` | Live edits in progress | `file/draft/router.conf` |
| `file/<date>/` | Historical config versions | `file/20260318-100000.000/router.conf` |
<!-- source: pkg/zefs/keys.go -- KeyLocalAdminUsername -->

The Storage interface (`internal/component/config/storage/`) translates filesystem paths to namespaced keys via `resolveKey()`. The function is idempotent: already-namespaced keys pass through unchanged, so `List()` results can be fed back to `ReadFile()` without double-prefixing.
<!-- source: internal/component/config/storage/ -- Storage interface, resolveKey -->

`ze data` operates on raw blob keys. `ze init` writes `meta/` keys directly.

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

## In-place writes

When a value changes but fits within its existing slot capacity, `flush()` uses `pwrite` to update only the changed entry and the container header, avoiding a full file rewrite.

| Condition | Write strategy |
|-----------|---------------|
| Entry value fits slot capacity | pwrite: entry header+data + container header CRC |
| Entry value exceeds slot capacity | Full rewrite via temp+rename |
| Entry added within container capacity | pwrite: append entry + update container header |
| Entry added exceeding container capacity | Full rewrite via temp+rename |
| Entry removed | Full rewrite (entries shift) |
| Non-unix platform | Full rewrite (no pwrite) |

After pwrite, the backing mmap is released and re-acquired so the read path sees the changes. On non-unix platforms, the in-place path falls back to full rewrite.
<!-- source: pkg/zefs/store.go -- flushInPlace, flushFull -->
<!-- source: pkg/zefs/pwrite_unix.go -- pwriteRegions -->

## Integrity checking

`zefs.Check(path)` reads a store file, validates the magic, container CRC, and every entry's CRC32c. Returns a structured `CheckReport` with per-entry status.

`zefs.Repair(src, dst)` scans a potentially corrupt store entry-by-entry, skips entries with CRC mismatches or parse errors, and writes valid entries to a new store. The source file is never modified.

`zefs.MoveAside(path)` renames a store file to `<path>.replaced-<date>` (local time) and returns the backup path, preserving the original for post-mortem. It is the shared backup step used when a store must be replaced rather than repaired in place.

The config storage layer self-heals on top of these. When `storage.NewBlob` opens a store that exists but is unreadable (corrupt, or a 0-byte file left by an interrupted or concurrent write), it moves the bad file aside with `MoveAside` and recreates a fresh store, so a corrupt store recovers automatically instead of wedging on every open. `ze init --force` uses the same `MoveAside` backup before writing a new database.

CLI: `ze data check`, `ze data repair --output <path>`, `ze data encode`.
<!-- source: pkg/zefs/check.go -- Check, Repair, MoveAside -->
<!-- source: internal/component/config/storage/blob.go -- NewBlob corrupt-store self-heal -->
<!-- source: internal/plugins/init/main.go -- moveAsideDB uses zefs.MoveAside -->

## Implementation

Reference implementation: `pkg/zefs/` in the ze repository.
<!-- source: pkg/zefs/store.go -- Store, ReadLock, WriteLock -->
<!-- source: pkg/zefs/tree.go -- in-memory tree representation -->
<!-- source: pkg/zefs/check.go -- Check, Repair, CheckReport, RepairReport -->
<!-- source: pkg/zefs/pwrite_unix.go -- pwrite for in-place writes -->
<!-- source: pkg/zefs/pwrite_other.go -- pwrite fallback for non-unix -->
<!-- source: pkg/zefs/file.go -- file-level operations -->
