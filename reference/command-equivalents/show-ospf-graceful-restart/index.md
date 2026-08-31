# `show ospf graceful-restart`

Show OSPFv2 (IPv4) Graceful Restart state (RFC 3623).

## Ze command

- Registry path: `show ospf graceful-restart`
- Usage: `show ospf graceful-restart`
- Mode: Read-only
- Wire method: `ze-show:ospf-graceful-restart`
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

The restarter state (in-restart or not, grace end, reason) and the per-neighbor helper sessions (which neighbors are being helped and their remaining grace).

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
