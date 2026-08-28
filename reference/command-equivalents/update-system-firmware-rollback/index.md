# `update system firmware rollback`

## Ze command

- Syntax: `update system firmware rollback`
- Registry path: `update system firmware rollback`
- Mode: Daemon
- Wire method: `ze-update:system-firmware-rollback`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Roll back to the previous firmware and restart. Reverts to the prior image. Only available on platforms with A/B partitioning (e.g. gokrazy). Use this if the new version has issues.

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
