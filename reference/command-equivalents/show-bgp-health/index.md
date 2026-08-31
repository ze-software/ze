# `show bgp health`

Quick health check for all your BGP peers.

## Ze command

- Registry path: `show bgp health`
- Usage: `show bgp health`
- Mode: Read-only
- Wire method: `ze-show:bgp-health`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: peer
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists every peer with address, state, ASN, and uptime. Reports how many are not Established. Much faster than 'show bgp peer *' when you just need a status overview.

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
