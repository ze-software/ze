# `clear ospf process`

## Ze command

- Syntax: `clear ospf process`
- Registry path: `clear ospf process`
- Mode: Daemon
- Wire method: `ze-clear:ospf-process`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Full OSPF reset: tear down every adjacency and re-run SPF. Usage: clear ospf process. Adjacencies re-form from the next Hello; the configuration is unchanged.

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
