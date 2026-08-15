# `show system memory`

## Ze command

- Syntax: `show system memory`
- Registry path: `show system memory`
- Mode: Read-only
- Wire method: `ze-show:system-memory-map`
- Global pipes: yes

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
