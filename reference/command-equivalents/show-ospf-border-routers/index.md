# `show ospf border-routers`

Show routes to OSPF area-border and AS-boundary routers.

## Ze command

- Registry path: `show ospf border-routers`
- Usage: `show ospf border-routers`
- Mode: Read-only
- Wire method: `ze-show:ospf-border-routers`
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

Lists each reachable ABR/ASBR with its router-id, cost, next-hops, and area.

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
