# ZeFS File Format

ZeFS is a netcapstring-framed blob store. A single `.zefs` file holds multiple named entries (files) with hierarchical keys, zero-copy reads via mmap, and in-place update support via capacity-aware framing.

## Netcapstring

A netcapstring is a self-describing, capacity-aware binary frame. It encodes a byte sequence with extra reserved space so that small growth can be written in place without shifting subsequent entries.

### Format

With padding (cap > used):

```
<number>:<cap>:<used>:<data>,<space padding>:
```

Exact fit (cap == used):

```
<number>:<cap>:<used>:<data>:
```

All header separators are `:`. Within the data region, `,` marks end of data, followed by space padding. `:` closes the capacity region. When cap == used, `,` and `:` would share the same position -- `:` wins, so `,` is not written.

| Field | Content | Size (bytes) |
|-------|---------|-------------|
| `<number>` | Digit count of `<cap>` (decimal ASCII, no leading zeros) | variable (typically 1-2) |
| `:` | Separator | 1 |
| `<cap>` | Capacity in bytes (decimal ASCII, zero-padded to `<number>` digits) | `<number>` |
| `:` | Separator | 1 |
| `<used>` | Used bytes (decimal ASCII, zero-padded to `<number>` digits) | `<number>` |
| `:` | Separator | 1 |
| `<data>` | Actual content | `<used>` |
| `,` | Data-end marker | 1 (only when `<cap>` > `<used>`) |
| `<padding>` | Space bytes | `<cap>` - `<used>` - 1 (or 0 when `<cap>` == `<used>`) |
| `:` or `,` | Terminator (`:` wins when both share the position) | 1 |

### Properties

- **Self-describing width.** The `<number>` field tells the parser how many digits to read for `<cap>` and `<used>`. No magic constants needed.
- **Cap-first, fixed-width used.** Since `<used>` is always zero-padded to the same width as `<cap>`, and `<used>` <= `<cap>` by definition, the header size never changes when data grows within capacity. This is the critical invariant for in-place writes.
- **No artificial size limit.** The `<number>` field is itself variable-width, so entries can be arbitrarily large (limited only by available memory).

### Examples

| Data | Cap | On disk |
|------|-----|---------|
| "hello" (5 bytes), cap 16 | 16 | `2:16:05:hello,<10 spaces>:` |
| empty, cap 8 | 8 | `1:8:0:,<7 spaces>:` |
| "abcd" (4 bytes), cap 4 | 4 | `1:4:4:abcd:` |
| "x" (1 byte), cap 100 | 100 | `3:100:001:x,<98 spaces>:` |

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
5. Read `:` (verify separator)
6. Read `<used>` bytes of data
7. Skip `<cap>` - `<used>` bytes of padding (contains `,` data-end marker then spaces)
8. Read `,` or `:` (verify terminator; `:` wins when both share a position)
9. Next entry starts at the byte after the terminator

## ZeFS File

A ZeFS file is a sequence of two netcapstrings: a magic identifier followed by the container.

### Format

```
1:4:4:ZeFS:<N>:<cap>:<used>:<entries...><padding>,
```

The first netcapstring contains the magic `ZeFS`. Its terminator is `:` (not `,`) because the container follows -- `:` wins over `,` at that position. The entire file is pure netcapstrings.

### Container content

Inside the container, entries are stored as consecutive pairs of netcapstrings (key + value):

```
1:4:4:ZeFS:<N>:<cap>:<used>:
  <kN>:<kCap>:<kUsed>:<key>,<kPad>:<vN>:<vCap>:<vUsed>:<value>,<vPad>:
  <kN>:<kCap>:<kUsed>:<key>,<kPad>:<vN>:<vCap>:<vUsed>:<value>,<vPad>:
  ...
  \n
<container padding>,
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
| `1:4:4:ZeFS:` at offset 0 | Valid ZeFS file |
| Anything else | Not a ZeFS file |

## Memory mapping

On unix, the backing file is memory-mapped (`PROT_READ`, `MAP_PRIVATE`). Tree nodes hold sub-slices of the mapped region for zero-copy reads. The `ReadLock` and `WriteLock` guards scope zero-copy slice validity: callers hold the lock while processing raw bytes, and the in-process `sync.RWMutex` prevents `flush()` (which remaps the backing) from running while slices are in use.

## Concurrency model

### Single-process ownership

Only one process opens a ZeFS blob at a time. In ze, the daemon (`ze router.conf`) owns the blob. SSH editor sessions run as goroutines within the daemon process (via Wish). Terminal commands (`ze config edit`, `ze db ls`) detect the running daemon by dialing the SSH port and become SSH clients, sending commands through the daemon rather than opening the blob directly. When no daemon is running, the editor starts an ephemeral daemon, connects via SSH, and stops it when done.

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

`ze db` operates on raw blob keys. `ze init` writes `meta/` keys directly.

## Implementation

Reference implementation: `pkg/zefs/` in the ze repository.
