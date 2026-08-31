# `show l2tp tunnel history`

Show state transitions for a tunnel over time.

## Ze command

- Registry path: `show l2tp tunnel history`
- Usage: `show l2tp tunnel history`
- Mode: Read-only
- Wire method: `ze-l2tp-api:tunnel-history`
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

Timestamped FSM entries showing how the tunnel reached its current state. Use this to diagnose a tunnel establishment failure.

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
