# `show doctor`

## Ze command

- Syntax: `show doctor`
- Registry path: `show doctor`
- Mode: Read-only
- Wire method: `ze-show:doctor`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Check if this box is ready to run Ze. Verifies runtime dependencies: required files, sockets, ports, and kernel modules. Each check reports pass or fail with a reason. Run this before first start or after changing the platform setup.

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
