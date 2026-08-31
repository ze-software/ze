# `show bgp peer rib`

Show RIB data scoped to one peer.

## Ze command

- Registry path: `show bgp peer rib`
- Usage: `show bgp peer <selector> rib [sent\|advertised\|received\|sent-received]`
- Mode: Read-only
- Wire method: `ze-bgp:peer-rib`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `scope`
- Answer shape: tab
- Address fields: peer, next-hop
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

### Routes received from one BGP peer

Category: BGP

### Routes advertised to one BGP peer

Category: BGP

Ze uses pipe filters on the peer RIB command for received and advertised views.

## Vendor equivalents

### Junos MX
- `show route advertising-protocol bgp <peer>` (verified, junos-route)
  - Intent: Routes advertised to one BGP peer
- `show route receive-protocol bgp <peer>` (verified, junos-route)
  - Intent: Routes received from one BGP peer

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
