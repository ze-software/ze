# `show metrics name`

Show one Prometheus metric by name.

## Ze command

- Registry path: `show metrics name`
- Usage: `show metrics name <name> [label <key> <value> ...]`
- Mode: Read-only
- Wire method: `ze-show:metrics-query`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `label`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Returns matching time series from the internal registry. Multiple label filters are ANDed. More targeted than the full metrics dump.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `name` | string | yes | any value of this type |

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
