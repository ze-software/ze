# `show bmp collectors`

Show BMP collector connection status.

## Ze command

- Registry path: `show bmp collectors`
- Usage: `show bmp collectors`
- Mode: Read-only
- Wire method: `ze-show:bmp-collectors`
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

Lists configured collectors with connection state, sent message counts, and error statistics. Check here if your collector is not receiving data.

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
