# `show isis route`

Show IS-IS-computed routes.

## Ze command

- Registry path: `show isis route`
- Usage: `show isis route`
- Mode: Read-only
- Wire method: `ze-show:isis-route`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `ipv6`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists each prefix the SPF installed with its metric, level, up/down bit, and next-hops (address and outgoing interface).

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
