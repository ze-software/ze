# `show system ntp`

NTP clock synchronization status.

## Ze command

- Registry path: `show system ntp`
- Usage: `show system ntp`
- Mode: Read-only
- Wire method: `ze-show:system-ntp`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `peers`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

## Arguments

No command-specific arguments listed.

## Mapping intents

### System time and NTP state

Category: System

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show date` (verified, vyos-cli)
  - Intent: System time and NTP state
- `show ntp` (verified, vyos-cli)
  - Intent: System time and NTP state
