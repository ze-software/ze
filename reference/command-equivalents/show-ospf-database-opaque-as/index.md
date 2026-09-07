# `show ospf database opaque-as`

Show only AS-scope opaque-LSAs (Type 11, RFC 5250).

## Ze command

- Registry path: `show ospf database opaque-as`
- Usage: `show ospf database opaque-as`
- Mode: Read-only
- Wire method: `ze-show:ospf-database-opaque-as`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `detail`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

RFC 5250 Section 3 floods link-state type-11 throughout the autonomous system. It takes the scope of an AS-external (type-5) LSA. A type-11 opaque LSA reaches every transit area. It is not flooded into a stub area or an NSSA. A router does not originate one into a connected stub area or NSSA. One received from a neighbor inside such an area is rejected.

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
