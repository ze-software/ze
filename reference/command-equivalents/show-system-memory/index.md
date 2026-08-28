# `show system memory`

## Ze command

- Syntax: `show system memory`
- Registry path: `show system memory`
- Mode: Read-only
- Wire method: `ze-show:system-memory-map`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show how much memory the daemon is using, from the OS's view. Returns VmRSS, VmSize, VmSwap, and thread count from /proc/self/status (Linux only). This is what the operating system reports the process is using. For the Go runtime allocator view (heap, GC), use 'show runtime memory'.

## Mapping intents

### CPU, memory, platform, and host health

Category: System

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show hardware cpu` (verified, vyos-cli)
  - Intent: CPU, memory, platform, and host health
- `show system memory` (verified, vyos-cli)
  - Intent: CPU, memory, platform, and host health
