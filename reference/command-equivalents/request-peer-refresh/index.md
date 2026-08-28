# `request peer refresh <selector>`

## Ze command

- Syntax: `request peer refresh <selector>`
- Registry path: `request peer refresh`
- Mode: Daemon
- Wire method: `ze-bgp:peer-refresh`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Ask a peer to re-send all routes (RFC 2918). Sends a ROUTE-REFRESH message for the specified AFI/SAFI. The peer will re-advertise its entire Adj-RIB-Out.

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
