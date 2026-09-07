# `show ospf database nssa-external`

Show only NSSA-external-LSAs (Type 7, RFC 3101).

## Ze command

- Registry path: `show ospf database nssa-external`
- Usage: `show ospf database nssa-external`
- Mode: Read-only
- Wire method: `ze-show:ospf-database-nssa-external`
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

RFC 3101 Section 2.3 states that a Type 7 LSA is advertised only within a single NSSA. A border router does not flood it into the backbone. It translates selected Type 7 LSAs into Type 5 instead, so a translated route appears under the AS-external view and not here.

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
