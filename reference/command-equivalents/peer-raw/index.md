# `peer raw <selector>`

## Ze command

- Syntax: `peer raw <selector>`
- Registry path: `peer raw`
- Mode: Daemon
- Wire method: `ze-bgp:peer-raw`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Send raw bytes into a peer's TCP stream (dangerous). Injects arbitrary bytes with no BGP framing or validation. Intended for conformance testing and fuzzing only. Will likely break the session if used carelessly.

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
