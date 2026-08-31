# `show l2tp sessions`

List all active L2TP sessions.

## Ze command

- Registry path: `show l2tp sessions`
- Usage: `show l2tp sessions`
- Mode: Read-only
- Wire method: `ze-l2tp-api:sessions`
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

One line per session: local/remote ID, parent tunnel, subscriber login, and uptime.

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
