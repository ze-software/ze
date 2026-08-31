# `request bgp rib withdraw`

Withdraw a route from the Adj-RIB-In.

## Ze command

- Registry path: `request bgp rib withdraw`
- Usage: `request bgp rib withdraw`
- Mode: Daemon
- Wire method: `ze-rib-api:withdraw`
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

Removes a previously injected or received route from a peer's Adj-RIB-In, triggering best-path recomputation.

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
