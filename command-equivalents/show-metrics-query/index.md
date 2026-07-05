# `show metrics-query <name> [label=value`

## Ze command

- Syntax: `show metrics-query <name> [label=value`
- Registry path: `show metrics-query`
- Mode: Read-only
- Wire method: `ze-show:metrics-query`
- Global pipes: yes

Query a specific Prometheus metric by name. Usage: show metrics-query <name> [label=value ...]. Returns matching time series from the internal registry. Multiple label filters are ANDed. More targeted than the full metrics dump.

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
