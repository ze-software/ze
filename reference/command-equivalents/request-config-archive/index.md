# `request config archive`

## Ze command

- Syntax: `request config archive`
- Registry path: `request config archive`
- Mode: Daemon
- Wire method: `ze-config-archive:trigger`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Save a snapshot of the current running configuration. Captures the config into the store for later rollback or comparison. Optional name labels the snapshot; defaults to a timestamp.

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
