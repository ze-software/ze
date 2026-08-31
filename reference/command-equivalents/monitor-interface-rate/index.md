# `monitor interface rate`

Stream per-second traffic rates for your interfaces.

## Ze command

- Registry path: `monitor interface rate`
- Usage: `monitor interface rate`
- Mode: Read-only
- Wire method: `ze-monitor:interface-rate`
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

Shows rx/tx bytes and packets per second, updating every second. Optionally pass an interface name to watch just one link.

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
