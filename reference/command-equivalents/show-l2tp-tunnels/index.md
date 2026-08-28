# `show l2tp tunnels`

## Ze command

- Syntax: `show l2tp tunnels`
- Registry path: `show l2tp tunnels`
- Mode: Read-only
- Wire method: `ze-l2tp-api:tunnels`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

List all active L2TP tunnels. One line per tunnel: local/remote ID, peer address, session count, and uptime.

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
