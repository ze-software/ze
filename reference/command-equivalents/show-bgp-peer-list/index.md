# `show bgp peer list`

List your peers, one line each.

## Ze command

- Registry path: `show bgp peer list`
- Usage: `show bgp peer list`
- Mode: Read-only
- Wire method: `ze-bgp:peer-list`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Shows name, address, ASN, state, and uptime. Quick overview without the per-peer detail.

## Arguments

No command-specific arguments listed.

## Mapping intents

### BGP peer list

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
