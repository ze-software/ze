# `show ospf virtual-links`

Show OSPF virtual links (RFC 2328 section 15).

## Ze command

- Registry path: `show ospf virtual-links`
- Usage: `show ospf virtual-links`
- Mode: Read-only
- Wire method: `ze-show:ospf-virtual-links`
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

Lists each configured virtual link with its transit area, remote router-id, adjacency state, computed cost, and transit next hop.

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
