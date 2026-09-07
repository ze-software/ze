# `show ospf database opaque-link detail`

Decode each link-local opaque LSA body (RFC 5250).

## Ze command

- Registry path: `show ospf database opaque-link detail`
- Usage: `show ospf database opaque-link detail`
- Mode: Read-only
- Wire method: `ze-show:ospf-database-opaque-link-detail`
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

A body with a typed decoder returns its TLVs, which are TE, Router-Information, Extended and Segment-Routing. Every other body returns a generic type, length and hex view (spec-ospf-ext-14).

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
