# `show metrics name <name> [label=value`

## Ze command

- Syntax: `show metrics name <name> [label=value`
- Registry path: `show metrics name`
- Mode: Read-only
- Wire method: `ze-show:metrics-query`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show one Prometheus metric by name. Usage: show metrics name <name> [label=value ...]. Returns matching time series from the internal registry. Multiple label filters are ANDed. More targeted than the full metrics dump.

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
