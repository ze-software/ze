# `show event recent`

## Ze command

- Syntax: `show event recent`
- Registry path: `show event recent`
- Mode: Read-only
- Wire method: `ze-show:event-recent`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show recent events, newest first. Each event includes timestamp, namespace, and type. Filter with namespace <name> to focus on one area, count <N> to limit output. Useful for reconstructing what happened before an incident.

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
