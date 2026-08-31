# `show runtime memory`

Show the Go runtime allocator memory stats.

## Ze command

- Registry path: `show runtime memory`
- Usage: `show runtime memory`
- Mode: Read-only
- Wire method: `ze-show:system-memory`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Returns allocated bytes, heap in-use, total allocations, GC cycles, and last GC pause duration. Compare over time to spot leaks. For the OS-level process memory (RSS/VSZ) use 'show system memory'.

## Arguments

No command-specific arguments listed.

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
