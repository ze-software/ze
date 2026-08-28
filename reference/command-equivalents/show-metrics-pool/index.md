# `show metrics pool`

## Ze command

- Syntax: `show metrics pool`
- Registry path: `show metrics pool`
- Mode: Read-only
- Wire method: `ze-bgp:pool-stats`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show attribute pool memory usage and dedup efficiency. Returns allocated entries, reference counts, and deduplication hit rates per attribute type. Watch the dedup rate to gauge how much memory pooling is saving you.

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
