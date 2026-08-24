# `show debug`

## Ze command

- Syntax: `show debug`
- Registry path: `show debug`
- Mode: Read-only
- Wire method: `ze-debug:debug-state`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Show live debug state from the running daemon. Lists every registered subsystem with its current log level and any active flag or scope filters. Unlike 'debug show' (which reads the stored profile), this reflects actual runtime state.

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
