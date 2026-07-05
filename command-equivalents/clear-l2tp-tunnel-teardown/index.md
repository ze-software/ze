# `clear l2tp tunnel teardown`

## Ze command

- Syntax: `clear l2tp tunnel teardown`
- Registry path: `clear l2tp tunnel teardown`
- Mode: Daemon
- Wire method: `ze-l2tp-api:tunnel-teardown`
- Global pipes: yes

Gracefully tear down one L2TP tunnel. Sends a StopCCN to the peer. All sessions on this tunnel will be disconnected. Pass the local tunnel ID.

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
