# `show bfd session <address>`

## Ze command

- Syntax: `show bfd session <address>`
- Registry path: `show bfd session address`
- Mode: Read-only
- Wire method: `ze-bfd-api:show-session`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Show full detail for one BFD session. Pass the peer address. Returns local/remote discriminators, negotiated timers, detection time, and packet counters.

## Mapping intents

### BFD session state

Category: Routing protocols

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
