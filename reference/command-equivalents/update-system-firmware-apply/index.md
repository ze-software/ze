# `update system firmware apply`

## Ze command

- Syntax: `update system firmware apply`
- Registry path: `update system firmware apply`
- Mode: Daemon
- Wire method: `ze-update:system-firmware-apply`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Full upgrade: download, verify, stage, and restart. Runs the complete update cycle in one command. Only available on platforms where Ze owns the update lifecycle (e.g. gokrazy). The box will reboot into the new version.

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
