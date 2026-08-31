# `show system date`

Show the daemon's current wall-clock time and timezone.

## Ze command

- Registry path: `show system date`
- Usage: `show system date`
- Mode: Read-only
- Wire method: `ze-show:system-date`
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

Useful for correlating log timestamps when the box is in a different timezone than you are.

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
