# `show ospf segment-routing`

Show OSPFv2 (IPv4) Segment Routing state (RFC 8665).

## Ze command

- Registry path: `show ospf segment-routing`
- Usage: `show ospf segment-routing`
- Mode: Read-only
- Wire method: `ze-show:ospf-segment-routing`
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

The configured SRGB/SRLB label ranges, the advertised SR-Algorithm, this node's node Prefix-SIDs, and the Adjacency-SIDs allocated per adjacency.

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
