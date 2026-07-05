# `request peer clear soft <selector>`

## Ze command

- Syntax: `request peer clear soft <selector>`
- Registry path: `request peer clear soft`
- Mode: Daemon
- Wire method: `ze-bgp:peer-clear-soft`
- Global pipes: yes

Soft-clear a peer without dropping the session. Sends ROUTE-REFRESH for every negotiated AFI/SAFI, causing the peer to re-send all routes. No session bounce, no traffic impact.

## Mapping intents

No vendor equivalent has been curated yet for this Ze command.
## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
