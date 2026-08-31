# `show bfd sessions`

List all active BFD sessions.

## Ze command

- Registry path: `show bfd sessions`
- Usage: `show bfd sessions`
- Mode: Read-only
- Wire method: `ze-bfd-api:show-sessions`
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

One line per session: peer address, state, negotiated tx/rx intervals, and detect multiplier.

## Arguments

No command-specific arguments listed.

## Mapping intents

### BFD session state

Category: Routing protocols

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
