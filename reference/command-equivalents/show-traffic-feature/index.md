# `show traffic feature [<name>]`

## Ze command

- Syntax: `show traffic feature [<name>]`
- Registry path: `show traffic feature`
- Mode: Read-only
- Wire method: `ze-show:traffic-feature`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show neutral per-source traffic feature signals: fan-out (distinct destinations), out/in byte ratio (exfiltration), destination-port entropy, new-peer, rare-port/proto, and coarse beaconing. Without arguments, shows the top source entities. With 'name <address>', filters to one source.

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
