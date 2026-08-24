# `clear interface counters`

## Ze command

- Syntax: `clear interface counters`
- Registry path: `clear interface counters`
- Mode: Daemon
- Wire method: `ze-clear:interface-counters`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Zero the Rx/Tx counters for every managed interface. Usage: clear interface counters.

## Mapping intents

### Clear interface counters

Category: Interfaces

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `clear interfaces ethernet <name> counters` (verified, vyos-cli)
  - Intent: Clear interface counters
