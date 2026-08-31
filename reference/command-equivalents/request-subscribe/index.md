# `request subscribe`

Start receiving events of one or more types.

## Ze command

- Registry path: `request subscribe`
- Usage: `request subscribe`
- Mode: Daemon
- Wire method: `ze-bgp:subscribe`
- Backends: any backend
- Task support: required: the MCP server always answers with a task handle
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Events are delivered asynchronously to your plugin session until you unsubscribe. Use 'show event list' to see available event types.

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
