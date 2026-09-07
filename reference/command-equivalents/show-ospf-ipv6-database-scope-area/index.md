# `show ospf ipv6 database scope area`

Show only area-scope (S2/S1 = 01) LSAs.

## Ze command

- Registry path: `show ospf ipv6 database scope area`
- Usage: `show ospf ipv6 database scope area`
- Mode: Read-only
- Wire method: `ze-show:ospfv3-database-scope-area`
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

RFC 5340 Section 2.3 gives area scope to the router-LSA, the network-LSA, the inter-area-prefix-LSA, the inter-area-router-LSA and the intra-area-prefix-LSA. Ze reads the per-area store for them.

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
