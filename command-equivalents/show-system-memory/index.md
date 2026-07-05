# `show system memory`

## Ze command

- Syntax: `show system memory`
- Registry path: `show system memory`
- Mode: Read-only
- Wire method: `ze-show:system-memory`
- Global pipes: yes

Show how much memory the daemon is using. Returns allocated bytes, heap in-use, total allocations, GC cycles, and last GC pause duration. Compare over time to spot leaks.

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
