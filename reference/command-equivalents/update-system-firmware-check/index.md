# `update system firmware check`

## Ze command

- Syntax: `update system firmware check`
- Registry path: `update system firmware check`
- Mode: Daemon
- Wire method: `ze-update:system-firmware-check`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Check for a new firmware version right now. Bypasses the scheduled interval timer and contacts the update server immediately. Compare the result with 'show system update'.

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
