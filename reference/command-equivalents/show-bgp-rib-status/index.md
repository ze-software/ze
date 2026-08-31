# `show bgp rib status`

Get a quick RIB overview without dumping routes.

## Ze command

- Registry path: `show bgp rib status`
- Usage: `show bgp rib status`
- Mode: Read-only
- Wire method: `ze-rib-api:status`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Shows total peers, received/accepted/advertised route counts, and per-family breakdowns. Use this to confirm convergence after a peer comes up.

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
