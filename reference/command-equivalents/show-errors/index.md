# `show errors`

Show recent errors across all subsystems, newest first.

## Ze command

- Registry path: `show errors`
- Usage: `show errors`
- Mode: Read-only
- Wire method: `ze-show:errors`
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

This is the first place to look when something goes wrong. Filter with source <name> to narrow to one subsystem, count <N> to limit output.

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
