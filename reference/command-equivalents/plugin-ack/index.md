# `plugin ack`

## Ze command

- Syntax: `plugin ack`
- Registry path: `plugin ack`
- Mode: Read-only
- Wire method: `ze-bgp:plugin-ack`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Choose sync or async event delivery. sync: Ze waits for your plugin to acknowledge each event before sending the next one. Safer but slower. async: events fire without waiting, giving higher throughput at the cost of backpressure control.

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
