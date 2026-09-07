# `send bgp update`

Send a pre-built BGP UPDATE to a peer.

## Ze command

- Registry path: `send bgp update`
- Usage: `send bgp <selector> update <text\|hex\|b64\|cursor>`
- Mode: Daemon
- Wire method: `ze-bgp:peer-update`
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

The encoding word says how the rest of the line is read. text is the route syntax docs/architecture/api/update-syntax.md states, hex and b64 are wire octets, and cursor replays a stored position. 'show bgp encode' builds a hex payload. The tokens after the encoding word are that encoding's own grammar, and this model states the encoding alone.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |
| `encoding` | enum | yes | `text`, `hex`, `b64`, `cursor` |

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
