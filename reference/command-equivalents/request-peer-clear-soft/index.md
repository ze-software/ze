# `request peer clear soft`

Soft-clear a peer without dropping the session.

## Ze command

- Registry path: `request peer clear soft`
- Usage: `request peer <selector> clear soft`
- Mode: Daemon
- Wire method: `ze-bgp:peer-clear-soft`
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

Sends ROUTE-REFRESH for every negotiated AFI/SAFI, causing the peer to re-send all routes. No session bounce, no traffic impact.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |

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
