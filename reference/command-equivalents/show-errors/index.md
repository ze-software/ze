# `show errors`

## Ze command

- Syntax: `show errors`
- Registry path: `show errors`
- Mode: Read-only
- Wire method: `ze-show:errors`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Show recent errors across all subsystems, newest first. This is the first place to look when something goes wrong. Filter with source <name> to narrow to one subsystem, count <N> to limit output.

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
