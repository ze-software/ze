# `plugin ack`

## Ze command

- Syntax: `plugin ack`
- Registry path: `plugin ack`
- Mode: Read-only
- Wire method: `ze-bgp:plugin-ack`
- Global pipes: yes

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
