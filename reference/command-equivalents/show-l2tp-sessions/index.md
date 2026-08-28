# `show l2tp sessions`

## Ze command

- Syntax: `show l2tp sessions`
- Registry path: `show l2tp sessions`
- Mode: Read-only
- Wire method: `ze-l2tp-api:sessions`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

List all active L2TP sessions. One line per session: local/remote ID, parent tunnel, subscriber login, and uptime.

## Mapping intents

### L2TP tunnel and session state

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
