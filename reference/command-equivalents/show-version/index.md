# `show version`

Show the running Ze version and build date.

## Ze command

- Registry path: `show version`
- Usage: `show version`
- Mode: Read-only
- Wire method: `ze-show:version`
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

You can verify which release is deployed on this box.

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
