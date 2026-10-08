# `show capture`

Show captured control\-plane messages\.

## Ze command

- Registry path: `show capture`
- Usage: `show capture [protocol <l2tp\|bgp>] [tunnel-id <tunnel-id>] [count <count>] [peer <peer>]`
- Mode: Read-only
- Wire method: `ze-diag:show-capture`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `interface`, `raw`
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Returns protocol messages you previously enabled capture for\. Without a protocol keyword\, shows all protocols\. Use this to debug session establishment issues\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `protocol` | enum | no | `l2tp`, `bgp` | Protocol filter\. | Restricts the answer to one protocol key\. Absent\, the answer carries an l2tp key and a bgp key\, each holding its messages or \'capture not enabled\'\. |
| `tunnel-id` | string | no | any value of this type | L2TP tunnel ID filter\. | Keeps only the L2TP messages of the tunnel with this local tunnel ID\. The BGP messages are not affected\. |
| `count` | uint | no | any value of this type | Maximum number of messages\. | At most this many messages are returned for each protocol\. Absent or 0\, every buffered message is returned\. |
| `peer` | string | no | any value of this type | Peer address filter\. | Keeps only the messages exchanged with this peer address\, for L2TP and BGP alike\. |

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
