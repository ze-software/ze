# `show capture`

Show captured control-plane messages.

## Ze command

- Registry path: `show capture`
- Usage: `show capture [protocol <l2tp\|bgp>] [tunnel-id <tunnel-id>] [count <count>] [peer <peer>]`
- Mode: Read-only
- Wire method: `ze-show:capture`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `interface`, `raw`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Returns protocol messages you previously enabled capture for. Without a protocol keyword, shows all protocols. Use this to debug session establishment issues.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `protocol` | enum | no | `l2tp`, `bgp` |
| `tunnel-id` | string | no | any value of this type |
| `count` | uint | no | any value of this type |
| `peer` | string | no | any value of this type |

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
