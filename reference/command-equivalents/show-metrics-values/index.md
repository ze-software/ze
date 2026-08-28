# `show metrics values`

## Ze command

- Syntax: `show metrics values`
- Registry path: `show metrics values`
- Mode: Read-only
- Wire method: `ze-bgp:metrics-values`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Dump all metrics in Prometheus text format. Outputs every registered metric with labels and values. Suitable for feeding into Prometheus, Grafana, or curl-based monitoring.

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
