# `show pppoe statistics`

## Ze command

- Syntax: `show pppoe statistics`
- Registry path: `show pppoe statistics`
- Mode: Read-only
- Wire method: `ze-pppoe-api:statistics`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show PPPoE protocol message counters. Returns PADI, PADO, PADR, PADS, PADT counts, active sessions, and errors. A rising PADI count with flat PADS means sessions are not completing.

## Mapping intents

### PPPoE subscriber sessions

Category: VPN and access

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
