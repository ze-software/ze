# `show health`

## Ze command

- Syntax: `show health`
- Registry path: `show health`
- Mode: Read-only
- Wire method: `ze-show:health`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Is this box healthy? One command to find out. Returns per-component health (bgp, fib, iface, plugins, l2tp, etc.) plus an overall status. Each component reports healthy, degraded, or unhealthy with a reason. Start here when troubleshooting.

## Mapping intents

### CPU, memory, platform, and host health

Category: System

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show hardware cpu` (verified, vyos-cli)
  - Intent: CPU, memory, platform, and host health
- `show system memory` (verified, vyos-cli)
  - Intent: CPU, memory, platform, and host health
