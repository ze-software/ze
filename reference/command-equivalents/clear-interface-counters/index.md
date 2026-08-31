# `clear interface counters`

Zero the Rx/Tx counters for every managed interface.

## Ze command

- Registry path: `clear interface counters`
- Usage: `clear interface counters`
- Mode: Daemon
- Wire method: `ze-clear:interface-counters`
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

## Arguments

No command-specific arguments listed.

## Mapping intents

### Clear interface counters

Category: Interfaces

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `clear interfaces ethernet <name> counters` (verified, vyos-cli)
  - Intent: Clear interface counters
