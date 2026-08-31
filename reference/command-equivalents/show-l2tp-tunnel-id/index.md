# `show l2tp tunnel id`

Show full detail for one L2TP tunnel.

## Ze command

- Registry path: `show l2tp tunnel id`
- Usage: `show l2tp tunnel id <id>`
- Mode: Read-only
- Wire method: `ze-l2tp-api:tunnel`
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

Pass the local tunnel ID. Returns control channel state, peer endpoint, hello interval, and all assigned sessions.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `id` | string | yes | any value of this type |

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
