# `show isis interface`

Show IS-IS-enabled circuits.

## Ze command

- Registry path: `show isis interface`
- Usage: `show isis interface`
- Mode: Read-only
- Wire method: `ze-show:isis-interface`
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

Returns level, circuit type, metric, hello interval, hold multiplier, passive flag, DIS state, and the count of Up adjacencies per circuit.

## Arguments

No command-specific arguments listed.

## Mapping intents

### IS-IS neighbors, database, routes, and interfaces

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
