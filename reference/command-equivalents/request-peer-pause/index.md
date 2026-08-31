# `request peer pause`

Pause reading from a peer's TCP socket.

## Ze command

- Registry path: `request peer pause`
- Usage: `request peer <selector> pause`
- Mode: Daemon
- Wire method: `ze-bgp:peer-pause`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |

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
