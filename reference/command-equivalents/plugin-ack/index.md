# `plugin ack`

Choose sync or async event delivery.

## Ze command

- Registry path: `plugin ack`
- Usage: `plugin ack`
- Mode: Read-only
- Wire method: `ze-bgp:plugin-ack`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

sync: Ze waits for your plugin to acknowledge each event before sending the next one. Safer but slower. async: events fire without waiting, giving higher throughput at the cost of backpressure control.

## Arguments

No command-specific arguments listed.

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
