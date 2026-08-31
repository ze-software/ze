# `show bgp peer statistics`

Show UPDATE throughput for your peers.

## Ze command

- Registry path: `show bgp peer statistics`
- Usage: `show bgp peer <selector> statistics`
- Mode: Read-only
- Wire method: `ze-bgp:peer-statistics`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: address
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |

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
