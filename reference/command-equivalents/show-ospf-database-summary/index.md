# `show ospf database summary`

Show only Summary-LSAs (Type 3, inter-area network).

## Ze command

- Registry path: `show ospf database summary`
- Usage: `show ospf database summary`
- Mode: Read-only
- Wire method: `ze-show:ospf-database-summary`
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

RFC 2328 Section 12.4.3 states that an area border router originates a Summary-LSA. A Type 3 Summary-LSA describes a route to a network outside the area and inside the AS. Ze reads the per-area store for it.

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
