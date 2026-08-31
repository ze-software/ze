# `show route`

Show the kernel routing table.

## Ze command

- Registry path: `show route`
- Usage: `show route [prefix <prefix>] [limit <limit>]`
- Mode: Read-only
- Wire method: `ze-show:route`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `lookup`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists installed routes with next-hop, interface, protocol, and metric. Pass a CIDR prefix or 'default' to filter, or a route limit to cap the output.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `prefix` | string | no | any value of this type |
| `limit` | uint | no | any value of this type |

## Mapping intents

### Routing table

Category: Routing

## Vendor equivalents

### Junos MX
- `show route` (verified, junos-route)
  - Intent: Routing table

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show ip route` (verified, vyos-cli)
  - Intent: Routing table
