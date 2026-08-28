# `request peer eorr <selector>`

## Ze command

- Syntax: `request peer eorr <selector>`
- Registry path: `request peer eorr`
- Mode: Daemon
- Wire method: `ze-bgp:peer-eorr`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Finish an Enhanced Route Refresh cycle (RFC 7313). The peer purges any routes not re-advertised since the matching BORR. Only send this after the peer has finished re-advertising.

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
