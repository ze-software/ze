# `show host thermal`

## Ze command

- Syntax: `show host thermal`
- Registry path: `show host thermal`
- Mode: Read-only
- Wire method: `ze-show:host-thermal`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show temperature sensors and thermal throttle events. Returns hwmon sensor readings and per-CPU throttle counters. Non-zero throttle counts mean the box has been running hot enough to slow down.

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
