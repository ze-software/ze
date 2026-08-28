# `request peer <selector> resume`

## Ze command

- Syntax: `request peer <selector> resume`
- Registry path: `request peer resume`
- Mode: Daemon
- Wire method: `ze-bgp:peer-resume`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Resume reading from a previously paused peer. Usage: request peer <selector> resume.

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
