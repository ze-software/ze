# `show flow recent [<dst>]`

## Ze command

- Syntax: `show flow recent [<dst>]`
- Registry path: `show flow recent`
- Mode: Read-only
- Wire method: `ze-show:flow-recent`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show recent conntrack flow records from the bounded recent-flow ring. Without arguments, returns every ring record (oldest to newest, up to the configured recent-flow-ring capacity). With 'dst <prefix>', filters to flows whose destination is inside that prefix. The ring is fed only while conntrack export is enabled; the filter is by destination prefix (conntrack is host-global and carries no ingress interface).

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
