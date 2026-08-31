# `monitor traceroute`

Live mtr-style traceroute that updates continuously.

## Ze command

- Registry path: `monitor traceroute`
- Usage: `monitor traceroute`
- Mode: Read-only
- Wire method: `ze-monitor:traceroute`
- Backends: any backend
- Task support: required: the MCP server always answers with a task handle
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: addr
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Shows each hop with running RTT statistics. Keeps probing so you can watch path changes and latency shifts over time.

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
