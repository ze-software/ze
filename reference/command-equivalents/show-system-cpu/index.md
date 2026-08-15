# `show system cpu`

## Ze command

- Syntax: `show system cpu`
- Registry path: `show system cpu`
- Mode: Read-only
- Wire method: `ze-show:system-cpu`
- Global pipes: yes

Show CPU utilization context for the daemon. Returns goroutine count, logical CPU count, and GOMAXPROCS setting. Useful when the box feels sluggish and you want to see if Ze is hogging threads.

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
