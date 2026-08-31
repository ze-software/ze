# `show ospf neighbor`

Show OSPF neighbors.

## Ze command

- Registry path: `show ospf neighbor`
- Usage: `show ospf neighbor`
- Mode: Read-only
- Wire method: `ze-show:ospf-neighbor`
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

Returns each neighbor's router-id, interface, adjacency state, DR/BDR, priority, dead time, and address.

## Arguments

No command-specific arguments listed.

## Mapping intents

### OSPF neighbor state

Category: Routing protocols

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
