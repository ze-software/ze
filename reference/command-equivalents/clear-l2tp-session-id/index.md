# `clear l2tp session id`

## Ze command

- Syntax: `clear l2tp session id`
- Registry path: `clear l2tp session id`
- Mode: Daemon
- Wire method: `ze-l2tp-api:session-teardown`
- Global pipes: yes

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
