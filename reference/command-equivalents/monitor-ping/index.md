# `monitor ping`

Continuous ping with live loss and RTT statistics.

## Ze command

- Registry path: `monitor ping`
- Usage: `monitor ping`
- Mode: Read-only
- Wire method: `ze-monitor:ping`
- Backends: any backend
- Task support: required: the MCP server always answers with a task handle
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: target
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Pings <target> until you stop it. Adjust interval and timeout as needed. Shows running min/avg/max RTT and packet loss.

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
