# ZeFS File Format

ZeFS is a netcapstring-framed blob store. A single `.zefs` file holds multiple named entries (files) with hierarchical keys, zero-copy reads via mmap, and in-place update support via capacity-aware framing.

## Netcapstring

A netcapstring is a self-describing, capacity-aware binary frame. It encodes a byte sequence with extra reserved space so that small growth can be written in place without shifting subsequent entries.

### Format

With padding (cap > used):

```
<number>:<cap>:<used>\n<data><space padding>\n
```

Exact fit (cap == used):

```
<number>:<cap>:<used>\n<data>\n
```

The header separators are `:` (between number, cap, and used) and `\n` (after used). The header occupies its own line, making it easy to inspect with text tools. Unused capacity is space-filled. `\n` terminates both the header and the data region.

| Field | Content | Size (bytes) |
|-------|---------|-------------|
| `<number>` | Digit count of `<cap>` (decimal ASCII, no leading zeros) | variable (typically 1-2) |
| `:` | Separator | 1 |
| `<cap>` | Capacity in bytes (decimal ASCII, zero-padded to `<number>` digits) | `<number>` |
| `:` | Separator | 1 |
| `<used>` | Used bytes (decimal ASCII, zero-padded to `<number>` digits) | `<number>` |
| `\n` | Header terminator (0x0A) | 1 |
| `<data>` | Actual content | `<used>` |
| `<padding>` | Space bytes (0x20) | `<cap>` - `<used>` |
| `\n` | Terminator (0x0A) | 1 |

### Properties

- **Self-describing width.** The `<number>` field tells the parser how many digits to read for `<cap>` and `<used>`. No magic constants needed.
- **Cap-first, fixed-width used.** Since `<used>` is always zero-padded to the same width as `<cap>`, and `<used>` <= `<cap>` by definition, the header size never changes when data grows within capacity. This is the critical invariant for in-place writes.
- **No artificial size limit.** The `<number>` field is itself variable-width, so entries can be arbitrarily large (limited only by available memory).

### Examples

| Data | Cap | On disk |
|------|-----|---------|
| "hello" (5 bytes), cap 16 | 16 | `2:16:05\nhello<11 spaces>\n` |
| empty, cap 8 | 8 | `1:8:0\n<8 spaces>\n` |
| "abcd" (4 bytes), cap 4 | 4 | `1:4:4\nabcd\n` |
| "x" (1 byte), cap 100 | 100 | `3:100:001\nx<99 spaces>\n` |

### Header length

The total header length for a given capacity is: `3 + digitCount(digitCount(cap)) + 2 * digitCount(cap)`.

| Capacity range | Header bytes |
|---------------|-------------|
| 0-9 | 6 |
| 10-99 | 8 |
| 100-999 | 10 |
| 1000-9999 | 12 |

### Capacity growth

Keys are exact fit (keys never change). Data capacity is data length + 10%, both on first write and on growth.

### Parsing

1. Scan forward until next `:` to get the `<number>` field (parse as integer N)
2. Read N bytes for `<cap>` (parse as integer)
3. Read `:` (verify separator)
4. Read N bytes for `<used>` (parse as integer)
5. Read `\n` (verify header terminator)
6. Read `<used>` bytes of data
7. Skip `<cap>` - `<used>` bytes of space padding
8. Read `\n` (verify terminator)
9. Next entry starts at the byte after the terminator

## ZeFS File

A ZeFS file is a sequence of two netcapstrings: a magic identifier followed by the container.

### Format

```
1:4:4\nZeFS\n<N>:<cap>:<used>\n<entries...><padding>\n
```

The first netcapstring contains the magic `ZeFS`. Its header ends with `\n` and its terminator is also `\n` because cap == used. The entire file is pure netcapstrings, all terminated by `\n`.

### Container content

Inside the container, entries are stored as consecutive pairs of netcapstrings (key + value):

```
1:4:4\nZeFS\n<N>:<cap>:<used>\n
  <kN>:<kCap>:<kUsed>\n<key><kPad>\n<vN>:<vCap>:<vUsed>\n<value><vPad>\n
  <kN>:<kCap>:<kUsed>\n<key><kPad>\n<vN>:<vCap>:<vUsed>\n<value><vPad>\n
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
| `1:4:4\nZeFS\n` at offset 0 | Valid ZeFS file |
| Anything else | Not a ZeFS file |

## Memory mapping

On unix, the backing file is memory-mapped (`PROT_READ`, `MAP_PRIVATE`). Tree nodes hold sub-slices of the mapped region for zero-copy reads. The `ReadLock` and `WriteLock` guards scope zero-copy slice validity: callers hold the lock while processing raw bytes, and the in-process `sync.RWMutex` prevents `flush()` (which remaps the backing) from running while slices are in use.

## Concurrency model

### Single-process ownership

Only one process opens a ZeFS blob at a time. In ze, the daemon (`ze router.conf`) owns the blob. SSH editor sessions run as goroutines within the daemon process (via Wish). Terminal commands (`ze config edit`, `ze data ls`) detect the running daemon by dialing the SSH port and become SSH clients, sending commands through the daemon rather than opening the blob directly. When no daemon is running, the editor starts an ephemeral daemon, connects via SSH, and stops it when done.

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
| `meta/` | Instance metadata (credentials, identity, flags) | `meta/ssh/username`, `meta/instance/managed` |
| `file/active/` | Current committed config files | `file/active/etc/ze/router.conf` |
| `file/draft/` | Live edits in progress (future) | `file/draft/etc/ze/router.conf` |
| `file/<date>/` | Historical config versions (future) | `file/20260318-100000/etc/ze/router.conf` |

The Storage interface (`internal/component/config/storage/`) translates filesystem paths to namespaced keys via `resolveKey()`. The function is idempotent: already-namespaced keys pass through unchanged, so `List()` results can be fed back to `ReadFile()` without double-prefixing.

`ze data` operates on raw blob keys. `ze init` writes `meta/` keys directly.

## Implementation

Reference implementation: `pkg/zefs/` in the ze repository.
