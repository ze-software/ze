# `send bgp raw`

Send raw bytes into a peer's TCP stream (dangerous).

## Ze command

- Registry path: `send bgp raw`
- Usage: `send bgp <selector> raw <hex\|b64> <data> [type <open\|update\|notification\|keepalive\|route-refresh>]`
- Mode: Daemon
- Wire method: `ze-bgp:peer-raw`
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

Injects arbitrary bytes with no BGP framing and no validation. It is for conformance testing and fuzzing only, and it will break the session if it is used carelessly. With a type keyword ze writes the marker and the header, and the data carries the message body alone. Without a type keyword the data carries the whole packet, marker and header included.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |
| `encoding` | enum | yes | `hex`, `b64` |
| `data` | string | yes | any value of this type |
| `type` | enum | no | `open`, `update`, `notification`, `keepalive`, `route-refresh` |

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
