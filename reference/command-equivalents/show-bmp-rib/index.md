# `show bmp rib`

Show routes received via BMP monitoring sessions.

## Ze command

- Registry path: `show bmp rib`
- Usage: `show bmp rib`
- Mode: Read-only
- Wire method: `ze-show:bmp-rib`
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

Returns the BMP RIB content. Use this to verify what your collector is seeing from remote peers.

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
