# `show warnings`

Show active warnings across all subsystems.

## Ze command

- Registry path: `show warnings`
- Usage: `show warnings`
- Mode: Read-only
- Wire method: `ze-show:warnings`
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

Displays any conditions that need your attention (degraded peers, resource limits approaching, etc.). Use 'source <name>' to filter to a single subsystem.

## Arguments

No command-specific arguments listed.

## Mapping intents

### Logs, warnings, and errors

Category: Operations

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show log` (verified, vyos-cli)
  - Intent: Logs, warnings, and errors
