# `show ospf database router`

Show only Router-LSAs (Type 1).

## Ze command

- Registry path: `show ospf database router`
- Usage: `show ospf database router`
- Mode: Read-only
- Wire method: `ze-show:ospf-database-router`
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

RFC 2328 Section 12.4.1 states that every router originates a Router-LSA. The LSA describes the collected states of the router's interfaces to one area. It is flooded throughout a single area only, and Ze reads the per-area store for it.

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
