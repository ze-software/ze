# `request peer refresh`

Ask a peer to re-send all routes (RFC 2918).

## Ze command

- Registry path: `request peer refresh`
- Usage: `request peer <selector> refresh`
- Mode: Daemon
- Wire method: `ze-bgp:peer-refresh`
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

Sends a ROUTE-REFRESH message for the specified AFI/SAFI. The peer will re-advertise its entire Adj-RIB-Out.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |

## Mapping intents

### Refresh BGP routes without a hard reset

Category: BGP

## Vendor equivalents

### Junos MX
- `clear bgp neighbor <peer> soft-inbound` (verified, junos-clear-bgp)
  - Intent: Refresh BGP routes without a hard reset

### IOS XR
- `clear bgp ipv4 unicast <peer> soft in` (verified, iosxr-bgp-commands)
  - Intent: Refresh BGP routes without a hard reset

### SR OS

No equivalent listed.

### VyOS
- `reset bgp <peer> soft in` (verified, vyos-cli)
  - Intent: Refresh BGP routes without a hard reset
