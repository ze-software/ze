# `show anomaly shape`

Show the shadow-first anomaly responder status.

## Ze command

- Registry path: `show anomaly shape`
- Usage: `show anomaly shape`
- Mode: Read-only
- Wire method: `ze-show:anomaly-shape`
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

Returns the mode (shadow or armed), the action, and the kill-switch state. It also returns the armed source entities with their live firewall actions.

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
