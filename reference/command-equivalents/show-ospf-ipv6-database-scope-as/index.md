# `show ospf ipv6 database scope as`

Show only AS-scope (S2/S1 = 10) LSAs.

## Ze command

- Registry path: `show ospf ipv6 database scope as`
- Usage: `show ospf ipv6 database scope as`
- Mode: Read-only
- Wire method: `ze-show:ospfv3-database-scope-as`
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

RFC 5340 Section 2.3 states that an AS-scope LSA is flooded throughout the routing domain, and that a router which originates one is an AS Boundary Router. Ze reads the AS-wide store for it, and no area appears in the answer.

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
