# `show log recent [<component>] [<count>] [<level>]`

## Ze command

- Syntax: `show log recent [<component>] [<count>] [<level>]`
- Registry path: `show log recent`
- Mode: Read-only
- Wire method: `ze-bgp:log-recent`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Show recent log entries from the in-memory ring. Filters (all optional): level <lvl>, component <name>, count <N>. Newest entries first. Useful when you cannot access the log file directly.

## Mapping intents

### Logs, warnings, and errors

Category: Operations

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show log` (verified, vyos-cli)
  - Intent: Logs, warnings, and errors
