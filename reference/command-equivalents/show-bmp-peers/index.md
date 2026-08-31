# `show bmp peers`

Show BGP peers as seen through BMP monitoring.

## Ze command

- Registry path: `show bmp peers`
- Usage: `show bmp peers`
- Mode: Read-only
- Wire method: `ze-show:bmp-peers`
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

Lists peers reported via BMP with their state and route statistics.

## Arguments

No command-specific arguments listed.

## Mapping intents

### BMP collector, peer, and session visibility

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
