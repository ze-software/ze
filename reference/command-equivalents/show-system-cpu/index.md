# `show system cpu`

Show CPU utilization context for the daemon.

## Ze command

- Registry path: `show system cpu`
- Usage: `show system cpu`
- Mode: Read-only
- Wire method: `ze-show:system-cpu`
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

Returns goroutine count, logical CPU count, and GOMAXPROCS setting. Useful when the box feels sluggish and you want to see if Ze is hogging threads.

## Arguments

No command-specific arguments listed.

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
