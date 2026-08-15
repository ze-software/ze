# `request peer <selector> pause`

## Ze command

- Syntax: `request peer <selector> pause`
- Registry path: `request peer pause`
- Mode: Daemon
- Wire method: `ze-bgp:peer-pause`
- Global pipes: yes

Pause reading from a peer's TCP socket. Usage: request peer <selector> pause.

## Mapping intents

### Pause or resume a BGP peer without deleting configuration

Category: BGP

Ze exposes explicit runtime flow control; most vendors use config disable or administrative tools.

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
