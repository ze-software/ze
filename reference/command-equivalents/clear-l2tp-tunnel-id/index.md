# `clear l2tp tunnel id`

## Ze command

- Syntax: `clear l2tp tunnel id`
- Registry path: `clear l2tp tunnel id`
- Mode: Daemon
- Wire method: `ze-l2tp-api:tunnel-teardown`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Gracefully tear down one L2TP tunnel. Sends a StopCCN to the peer. All sessions on this tunnel will be disconnected. Pass the local tunnel ID: clear l2tp tunnel id <id>.

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
