# `show ospf ipv6 neighbor`

Show the OSPFv3 (IPv6) neighbors.

## Ze command

- Registry path: `show ospf ipv6 neighbor`
- Usage: `show ospf ipv6 neighbor`
- Mode: Read-only
- Wire method: `ze-show:ospfv3-neighbor`
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

Each neighbor carries the link-local address as its identity, the adjacency state, the DR and BDR by Router ID, and the dead time.

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
