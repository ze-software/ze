# `show ospf te-database`

Show the OSPF Traffic Engineering Database (RFC 3630 / RFC 5392).

## Ze command

- Registry path: `show ospf te-database`
- Usage: `show ospf te-database`
- Mode: Read-only
- Wire method: `ze-show:ospf-te-database`
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

Router addresses plus TE links with their Link ID, local/remote address, link type, TE metric, bandwidths, admin group, and (for inter-AS links) remote AS and remote ASBR.

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
