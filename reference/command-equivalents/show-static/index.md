# `show static`

Show static routes defined in the configuration.

## Ze command

- Registry path: `show static`
- Usage: `show static`
- Mode: Read-only
- Wire method: `ze-show:static`
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

Lists each static route with its prefix, next-hop, and interface.

## Arguments

No command-specific arguments listed.

## Mapping intents

### Static route configuration or state

Category: Routing

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
