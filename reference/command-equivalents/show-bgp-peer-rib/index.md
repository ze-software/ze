# `show bgp peer <selector> rib [scope|filters|terminal]`

## Ze command

- Syntax: `show bgp peer <selector> rib [scope\|filters\|terminal]`
- Registry path: `show bgp peer rib`
- Mode: Read-only
- Wire method: `ze-bgp:peer-rib`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on rows: none

Show RIB data scoped to one peer. Usage: show bgp peer <selector> rib [scope|filters|terminal].

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
