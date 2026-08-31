# `resolve irr prefix`

Get all prefixes announced by an AS-SET's members.

## Ze command

- Registry path: `resolve irr prefix`
- Usage: `resolve irr prefix`
- Mode: Read-only
- Wire method: `ze-resolve:irr-prefix`
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

Expands the AS-SET, then returns every route/route6 object for each member ASN. Use this to build or verify prefix filters.

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
