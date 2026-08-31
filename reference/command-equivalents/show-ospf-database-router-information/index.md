# `show ospf database router-information`

Show the Router Information LSAs (RFC 7770) for both address families.

## Ze command

- Registry path: `show ospf database router-information`
- Usage: `show ospf database router-information`
- Mode: Read-only
- Wire method: `ze-show:ospf-database-router-information`
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

OSPFv2 opaque type 4 and OSPFv3 function code 12, decoded into the advertised informational capability bits and the TLV list.

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
