# `set debug timeout`

Set how long debug output stays enabled.

## Ze command

- Registry path: `set debug timeout`
- Mode: Offline
- Wire method: `not listed`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: none
- Pipes, when the answer has rows: none
- Pipes, while streaming: none
- Pipes, local process only: none
- Command pipes: none
- Pipe aliases: none

The duration is written as 30m, 1h or 90s, seconds are rounded up to minutes, and the longest accepted value is 24h. Zero disables the timer.

## Arguments

No command-specific arguments listed.

## Mapping intents

No vendor equivalent has been curated yet for this Ze command.

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
