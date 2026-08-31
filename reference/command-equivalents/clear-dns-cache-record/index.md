# `clear dns cache record`

Evict DNS cache entries for one record name, or one name and type when a type is provided.

## Ze command

- Registry path: `clear dns cache record`
- Usage: `clear dns cache record <name> [type <A\|AAAA\|MX\|NS\|TXT\|CNAME\|PTR>]`
- Mode: Daemon
- Wire method: `ze-clear:dns-cache-record`
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
| `name` | string | yes | any value of this type |
| `type` | enum | no | `A`, `AAAA`, `MX`, `NS`, `TXT`, `CNAME`, `PTR` |

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
