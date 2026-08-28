# `request peer <selector> teardown [cease-subcode]`

## Ze command

- Syntax: `request peer <selector> teardown [cease-subcode]`
- Registry path: `request peer teardown`
- Mode: Daemon
- Wire method: `ze-bgp:peer-teardown`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Tear down a peer session. Usage: request peer <selector> teardown [cease-subcode].

## Mapping intents

### Hard reset one BGP peer

Category: BGP

## Vendor equivalents

### Junos MX
- `clear bgp neighbor <peer>` (verified, junos-clear-bgp)
  - Intent: Hard reset one BGP peer

### IOS XR
- `clear bgp ipv4 unicast <peer>` (verified, iosxr-bgp-commands)
  - Intent: Hard reset one BGP peer

### SR OS

No equivalent listed.

### VyOS
- `reset bgp <peer>` (verified, vyos-cli)
  - Intent: Hard reset one BGP peer
