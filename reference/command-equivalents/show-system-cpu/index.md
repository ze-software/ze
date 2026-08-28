# `show system cpu`

## Ze command

- Syntax: `show system cpu`
- Registry path: `show system cpu`
- Mode: Read-only
- Wire method: `ze-show:system-cpu`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

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
