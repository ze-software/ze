# `clear l2tp tunnel id`

Gracefully tear down one L2TP tunnel.

## Ze command

- Registry path: `clear l2tp tunnel id`
- Usage: `clear l2tp tunnel id`
- Mode: Daemon
- Wire method: `ze-l2tp-api:tunnel-teardown`
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

Sends a StopCCN to the peer. Every session on this tunnel is disconnected. Pass the local tunnel ID: clear l2tp tunnel id <id>.

## Arguments

No command-specific arguments listed.

## Mapping intents

### L2TP tunnel and session state

Category: VPN and access

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
