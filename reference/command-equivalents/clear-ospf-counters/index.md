# `clear ospf counters`

## Ze command

- Syntax: `clear ospf counters`
- Registry path: `clear ospf counters`
- Mode: Daemon
- Wire method: `ze-clear:ospf-counters`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Reset the OSPF SPF-run history. Usage: clear ospf counters. Monotonic Prometheus series are not reset; the SPF-run log is cleared.

## Mapping intents

### Clear OSPF process, counters, or neighbor state

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
