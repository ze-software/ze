# `clear bgp rib out`

## Ze command

- Syntax: `clear bgp rib out`
- Registry path: `clear bgp rib out`
- Mode: Daemon
- Wire method: `ze-rib-api:clear-out`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Re-advertise all routes to a peer. Triggers a full Adj-RIB-Out replay to the selected peers. Useful after a policy change to push updated attributes without tearing down the session. Selector: IP, name, AS pattern, glob, or *.

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
