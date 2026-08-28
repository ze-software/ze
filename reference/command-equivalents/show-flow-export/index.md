# `show flow export [<name>]`

## Ze command

- Syntax: `show flow export [<name>]`
- Registry path: `show flow export`
- Mode: Read-only
- Wire method: `ze-show:flow-export`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show flow export (NetFlow/IPFIX) collector status. Without arguments, lists all configured collectors. With 'name <name>', shows details for that collector including protocol stats and errors. Returns not-configured when no exporter is active.

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
