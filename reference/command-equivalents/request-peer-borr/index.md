# `request peer borr <selector>`

## Ze command

- Syntax: `request peer borr <selector>`
- Registry path: `request peer borr`
- Mode: Daemon
- Wire method: `ze-bgp:peer-borr`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Start an Enhanced Route Refresh cycle (RFC 7313). Tells the peer to mark existing routes as stale. After re-sending, send EORR to purge anything not refreshed.

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
