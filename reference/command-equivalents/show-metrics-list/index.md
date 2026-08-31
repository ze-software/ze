# `show metrics list`

List all registered metric names (no values).

## Ze command

- Registry path: `show metrics list`
- Usage: `show metrics list`
- Mode: Read-only
- Wire method: `ze-bgp:metrics-list`
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

Useful for discovering what metrics exist before querying them.

## Arguments

No command-specific arguments listed.

## Mapping intents

### Metrics inventory and values

Category: Operations

Ze exposes Prometheus-style metric names and values directly; vendor telemetry is usually a separate subsystem.

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
