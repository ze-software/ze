# `show warnings`

## Ze command

- Syntax: `show warnings`
- Registry path: `show warnings`
- Mode: Read-only
- Wire method: `ze-show:warnings`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show active warnings across all subsystems. Displays any conditions that need your attention (degraded peers, resource limits approaching, etc.). Use 'source <name>' to filter to a single subsystem.

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
