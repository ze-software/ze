# `show pppoe session id`

Show full detail for one PPPoE session.

## Ze command

- Registry path: `show pppoe session id`
- Usage: `show pppoe session id <id>`
- Mode: Read-only
- Wire method: `ze-pppoe-api:session`
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

Pass the session ID. Returns discovery tags, LCP/NCP state, assigned addresses, and traffic counters.

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
