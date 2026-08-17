# `clear l2tp tunnel id`

## Ze command

- Syntax: `clear l2tp tunnel id`
- Registry path: `clear l2tp tunnel id`
- Mode: Daemon
- Wire method: `ze-l2tp-api:tunnel-teardown`
- Global pipes: yes

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
