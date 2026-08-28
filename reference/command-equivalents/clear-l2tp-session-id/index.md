# `clear l2tp session id`

## Ze command

- Syntax: `clear l2tp session id`
- Registry path: `clear l2tp session id`
- Mode: Daemon
- Wire method: `ze-l2tp-api:session-teardown`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Disconnect one subscriber session. Sends a CDN to gracefully close the session. Pass the local session ID: clear l2tp session id <id> [reason <text>] [cause <code>].

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
