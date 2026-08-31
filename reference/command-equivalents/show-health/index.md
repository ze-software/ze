# `show health`

Show the health of every component and the overall status of this box.

## Ze command

- Registry path: `show health`
- Usage: `show health`
- Mode: Read-only
- Wire method: `ze-show:health`
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

Returns per-component health (bgp, fib, iface, plugins, l2tp, etc.) plus an overall status. Each component reports healthy, degraded, or unhealthy with a reason. Start here when troubleshooting. This runs a probe NOW and reports what the daemon is doing at this moment. 'show plugins' answers the other question, by replaying the setup outcome each plugin recorded once, before main(), when it set itself up.

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
