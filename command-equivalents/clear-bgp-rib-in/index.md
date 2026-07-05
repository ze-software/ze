# `clear bgp rib in`

## Ze command

- Syntax: `clear bgp rib in`
- Registry path: `clear bgp rib in`
- Mode: Daemon
- Wire method: `ze-rib-api:clear-in`
- Global pipes: yes

Remove all routes received from a peer. Wipes the Adj-RIB-In for matched peers. They will need to re-advertise everything (or you can send a route-refresh). Selector: IP, name, AS pattern, glob, or *.

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
