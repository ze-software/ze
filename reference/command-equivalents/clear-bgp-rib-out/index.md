# `clear bgp rib out`

Re-advertise all routes to a peer.

## Ze command

- Registry path: `clear bgp rib out`
- Usage: `clear bgp rib out`
- Mode: Daemon
- Wire method: `ze-rib-api:clear-out`
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

Triggers a full Adj-RIB-Out replay to the selected peers. Useful after a policy change to push updated attributes without tearing down the session. Selector: IP, name, AS pattern, glob, or *.

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
