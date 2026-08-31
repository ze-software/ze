# `monitor bgp`

Live BGP peer dashboard that refreshes automatically.

## Ze command

- Registry path: `monitor bgp`
- Usage: `monitor bgp`
- Mode: Read-only
- Wire method: `ze-bgp:monitor`
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

Shows all peers with state, uptime, and prefix counts. State changes highlight as they happen. Ctrl-C to stop.

## Arguments

No command-specific arguments listed.

## Mapping intents

### Stream live BGP events or updates

Category: BGP

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
