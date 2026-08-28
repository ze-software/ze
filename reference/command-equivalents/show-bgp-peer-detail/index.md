# `show bgp peer <selector> detail`

## Ze command

- Syntax: `show bgp peer <selector> detail`
- Registry path: `show bgp peer detail`
- Mode: Read-only
- Wire method: `ze-bgp:peer-detail`
- Answer shape: tab
- Address fields: local-ip, next-hop-address
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show full detail for one or more peers. Usage: show bgp peer <selector> detail. The selector can be an IP, peer name, AS pattern (as65001), glob, or *.

## Mapping intents

### Detailed BGP peer state

Category: BGP

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
