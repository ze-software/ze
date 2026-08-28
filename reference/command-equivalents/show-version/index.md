# `show version`

## Ze command

- Syntax: `show version`
- Registry path: `show version`
- Mode: Read-only
- Wire method: `ze-show:version`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show the running Ze version and build date. You can verify which release is deployed on this box.

## Mapping intents

### Software version and uptime

Category: System

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show system uptime` (verified, vyos-cli)
  - Intent: Software version and uptime
