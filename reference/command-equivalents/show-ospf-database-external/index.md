# `show ospf database external`

Show only AS-external-LSAs (Type 5).

## Ze command

- Registry path: `show ospf database external`
- Usage: `show ospf database external`
- Mode: Read-only
- Wire method: `ze-show:ospf-database-external`
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

RFC 2328 Section 12.4.4 states that an AS boundary router originates an AS-external-LSA. The LSA describes a route to a destination in another Autonomous System, and it is flooded throughout the AS. Ze reads the AS-wide store for it, so no area appears in the answer.

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
