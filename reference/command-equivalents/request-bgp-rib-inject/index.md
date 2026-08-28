# `request bgp rib inject`

## Ze command

- Syntax: `request bgp rib inject`
- Registry path: `request bgp rib inject`
- Mode: Daemon
- Wire method: `ze-rib-api:inject`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Inject a synthetic route into the Adj-RIB-In. Behaves as if the route was received from a peer. Use this for testing policy filters or simulating route announcements.

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
