# `show bgp peer capabilities`

Show what capabilities were negotiated with a peer.

## Ze command

- Registry path: `show bgp peer capabilities`
- Usage: `show bgp peer <selector> capabilities`
- Mode: Read-only
- Wire method: `ze-bgp:peer-capabilities`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: peer
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |

## Mapping intents

### Negotiated BGP capabilities for a peer

Category: BGP

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
