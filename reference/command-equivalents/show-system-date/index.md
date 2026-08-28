# `show system date`

## Ze command

- Syntax: `show system date`
- Registry path: `show system date`
- Mode: Read-only
- Wire method: `ze-show:system-date`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show the daemon's current wall-clock time and timezone. Useful for correlating log timestamps when the box is in a different timezone than you are.

## Mapping intents

### System time and NTP state

Category: System

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show date` (verified, vyos-cli)
  - Intent: System time and NTP state
- `show ntp` (verified, vyos-cli)
  - Intent: System time and NTP state
