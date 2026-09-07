# `show ospf database network`

Show only Network-LSAs (Type 2).

## Ze command

- Registry path: `show ospf database network`
- Usage: `show ospf database network`
- Mode: Read-only
- Wire method: `ze-show:ospf-database-network`
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

RFC 2328 Section 12.4.2 states that the Designated Router originates a Network-LSA for a broadcast or NBMA network. The LSA lists the routers connected to that network. It is flooded throughout a single area only. A point-to-point link has no Designated Router, so it produces none.

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
