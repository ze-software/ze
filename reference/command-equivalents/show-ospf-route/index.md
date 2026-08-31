# `show ospf route`

Show OSPF-computed routes.

## Ze command

- Registry path: `show ospf route`
- Usage: `show ospf route`
- Mode: Read-only
- Wire method: `ze-show:ospf-route`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `fast-reroute`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists each prefix with its path type (intra/inter/external-1/2), cost, next-hops, and area.

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
