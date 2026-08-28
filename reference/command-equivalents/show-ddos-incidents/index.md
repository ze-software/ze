# `show ddos incidents`

## Ze command

- Syntax: `show ddos incidents`
- Registry path: `show ddos incidents`
- Mode: Read-only
- Wire method: `ze-show:ddos-incidents`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show the recent DDoS incident ring (newest first): per incident the target vector (prefix/proto/port), attack family, top source addresses, peak pps/bps, start/end time, and whether it is still active.

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
