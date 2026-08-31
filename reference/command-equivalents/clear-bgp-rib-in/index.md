# `clear bgp rib in`

Remove all routes received from a peer.

## Ze command

- Registry path: `clear bgp rib in`
- Usage: `clear bgp rib in`
- Mode: Daemon
- Wire method: `ze-rib-api:clear-in`
- Backends: any backend
- Task support: forbidden: the MCP server never answers with a task handle
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Wipes the Adj-RIB-In for matched peers. They will need to re-advertise everything (or you can send a route-refresh). Selector: IP, name, AS pattern, glob, or *.

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
