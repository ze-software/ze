# `send bgp withdraw id`

Withdraw one announcement by the id that show announcements reports.

## Ze command

- Registry path: `send bgp withdraw id`
- Usage: `send bgp <selector> withdraw id <id>`
- Mode: Daemon
- Wire method: `ze-bgp:withdraw-id`
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

Only a tagged announcement carries an id. The withdraw acts only when the selector typed after bgp is the one the announcement was made with, compared as text.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |
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
