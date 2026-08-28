# `request bgp rib withdraw`

## Ze command

- Syntax: `request bgp rib withdraw`
- Registry path: `request bgp rib withdraw`
- Mode: Daemon
- Wire method: `ze-rib-api:withdraw`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Withdraw a route from the Adj-RIB-In. Removes a previously injected or received route from a peer's Adj-RIB-In, triggering best-path recomputation.

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
