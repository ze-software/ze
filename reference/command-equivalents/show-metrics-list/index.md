# `show metrics list`

## Ze command

- Syntax: `show metrics list`
- Registry path: `show metrics list`
- Mode: Read-only
- Wire method: `ze-bgp:metrics-list`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

List all registered metric names (no values). Useful for discovering what metrics exist before querying them.

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
