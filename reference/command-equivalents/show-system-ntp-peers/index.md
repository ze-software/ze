# `show system ntp peers`

## Ze command

- Syntax: `show system ntp peers`
- Registry path: `show system ntp peers`
- Mode: Read-only
- Wire method: `ze-show:system-ntp-peers`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Show NTP peers with offset, RTT, stratum, and reachability. Tells you whether your clock is synced and how far off each NTP server thinks you are.

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
