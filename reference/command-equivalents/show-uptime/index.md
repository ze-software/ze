# `show uptime`

Show how long the daemon has been running.

## Ze command

- Registry path: `show uptime`
- Usage: `show uptime`
- Mode: Read-only
- Wire method: `ze-show:uptime`
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

Returns the start time and elapsed uptime. Handy after a maintenance window to confirm the process restarted.

## Arguments

No command-specific arguments listed.

## Mapping intents

### Software version and uptime

Category: System

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show system uptime` (verified, vyos-cli)
  - Intent: Software version and uptime
