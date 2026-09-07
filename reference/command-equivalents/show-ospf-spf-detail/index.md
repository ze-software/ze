# `show ospf spf detail`

Explain why each route won the SPF calculation (spec-ospf-ext-14).

## Ze command

- Registry path: `show ospf spf detail`
- Usage: `show ospf spf detail`
- Mode: Read-only
- Wire method: `ze-show:ospf-spf-detail`
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

Returns the candidate paths considered for each prefix, the winning cost, and the RFC 2328 section 16.4 path-preference tie-break. The command is read-only, so the route table and the SPF run count do not change.

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
