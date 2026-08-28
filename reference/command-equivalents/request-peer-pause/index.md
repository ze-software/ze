# `request peer <selector> pause`

## Ze command

- Syntax: `request peer <selector> pause`
- Registry path: `request peer pause`
- Mode: Daemon
- Wire method: `ze-bgp:peer-pause`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

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
