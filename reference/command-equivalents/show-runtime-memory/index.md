# `show runtime memory`

## Ze command

- Syntax: `show runtime memory`
- Registry path: `show runtime memory`
- Mode: Read-only
- Wire method: `ze-show:system-memory`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show the Go runtime allocator memory stats. Returns allocated bytes, heap in-use, total allocations, GC cycles, and last GC pause duration. Compare over time to spot leaks. For the OS-level process memory (RSS/VSZ) use 'show system memory'.

## Mapping intents

No vendor equivalent has been curated yet for this Ze command.
## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
